package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/libdns/libdns"
)

const (
	// HostnameAnnotationKey is the annotated tag for defininig hostname.
	HostnameAnnotationKey = "external-dns/hostname"
	// TTLAnnotationKey is the annotated tag for defininig TTL.
	TTLAnnotationKey = "external-dns/ttl"
	// DefaultTTL is the TTL to set for records if unspecified or unparseable.
	DefaultTTL = time.Second * 30
)

// ServiceMeta contains minimal items from a api.ServiceRegistration event.
// We only care about these fields.
type ServiceMeta struct {
	Name      string   // Human Name of the service.
	Namespace string   // Namespace to which the service belongs to.
	Job       string   // Job to which the service belongs to.
	Addresses []string // Address of all backend services which is fetched by calling Nomad HTTP API.
	Tags      []string // Tags in the given service.
}

// DNSProvider wraps the required libdns interfaces.
// The providers must satisfy this interface.
type DNSProvider interface {
	libdns.RecordGetter
	libdns.RecordSetter
	libdns.RecordDeleter
}

// RecordMeta wraps around `libdns.Record`
// and adds additional fields.
type RecordMeta struct {
	Records []libdns.Record
	Zone    string
}

//fetchServices fetches the list of services from the Nomad API
// and creates a map for the active services.
func (app *App) fetchServices() (map[string]ServiceMeta, error) {
	services := make(map[string]ServiceMeta, 0)

	servicesList, _, err := app.nomadClient.Services().List(&api.QueryOptions{Namespace: "*"})
	if err != nil {
		return nil, fmt.Errorf("error listing services: %w", err)
	}

	app.lo.Debug("fetched services", "count", len(servicesList))
	for _, l := range servicesList {
		for _, s := range l.Services {
			app.lo.Debug("fetching service details", "svc", s.ServiceName, "namespace", l.Namespace)
			svcObjects, _, err := app.nomadClient.Services().Get(s.ServiceName, (&api.QueryOptions{Namespace: l.Namespace}))
			if err != nil {
				return nil, fmt.Errorf("error fetching service detail: %w", err)
			}
			// If no service objects found, ignore
			if len(svcObjects) == 0 {
				continue
			}
			// We use hostname/TTL from annotated tags. If they are missing, ignore this service.
			if len(svcObjects[0].Tags) == 0 {
				continue
			}
			// Add all the addresses here.
			addr := make([]string, 0, len(svcObjects))
			for _, s := range svcObjects {
				// Only append the service if it's not added before. Otherwise AWS complains of a duplicate entry.
				if !Contains(addr, s.Address) {
					addr = append(addr, s.Address)
				}
			}
			// We use `[0]` element because these details are same for all elements.
			// Only address is different and for that we've already iterated and prepared a list.
			svcMeta := ServiceMeta{
				Name:      svcObjects[0].ServiceName,
				Namespace: svcObjects[0].Namespace,
				Job:       svcObjects[0].JobID,
				Tags:      svcObjects[0].Tags,
				Addresses: addr,
			}
			// Add the service to the map.
			services[GetPrefix(svcObjects[0])] = svcMeta
		}
	}
	return services, nil
}

// updateRecords takes a copy of the map of services fetched from Nomad
// and updates the DNS records for each service.
func (app *App) updateRecords(services map[string]ServiceMeta, domains []string) {
	app.RLock()
	defer app.RUnlock()

	var (
		update = false
	)

	for k, v := range services {
		// Before creating the record, first check if it exists in the map or not.
		// If it exists and the record values are also same, then don't do any DNS update.
		_, exists := app.services[k]
		if exists {
			for _, a := range app.services[k].Addresses {
				if !Contains(v.Addresses, a) {
					update = true
				}
			}
		} else {
			update = true
		}

		if !update {
			continue
		}

		record, err := v.ToRecord(domains)
		if err != nil {
			app.lo.Error("error converting service to record", "error", err)
			continue
		}
		app.lo.Info("setting dns records", "records", record.Records, "zone", record.Zone)
		_, err = app.provider.SetRecords(context.Background(), record.Zone, record.Records)
		if err != nil {
			app.lo.Error("error adding records to zone", "error", err)
			continue
		}
	}
}

// ToRecord converts a service meta object to a libdns record.
// This is used to send to upstream DNS providers.
func (s *ServiceMeta) ToRecord(domains []string) (RecordMeta, error) {
	var (
		host   string
		zone   string
		err    error
		ttl    time.Duration
		record RecordMeta
	)

	// We extract the hostname/TTL from the annotated tags, so return if they're not present in the svc object.
	if len(s.Tags) == 0 {
		return record, fmt.Errorf("tags cannot be empty")
	}

	// Parse the hostname and TTL from tags.
	for _, tag := range s.Tags {
		if strings.HasPrefix(tag, HostnameAnnotationKey) {
			host, zone, err = GetRecordName(strings.Split(tag, HostnameAnnotationKey+"=")[1], domains)
			if err != nil {
				return record, fmt.Errorf("error fetching hostname from tags: %w", err)
			}
		}
		if strings.HasPrefix(tag, TTLAnnotationKey) {
			ttl, err = ParseTTL(strings.Split(tag, TTLAnnotationKey+"=")[1])
			if err != nil {
				ttl = DefaultTTL
			}
		}
	}

	// At this point both the host and zone should be non empty.
	if host == "" && zone == "" {
		return record, fmt.Errorf("error parsing host: %s or zone: %s", host, zone)
	}

	// Format the zone as a proper FQDN.
	if zone[len(zone)-1:] != "." {
		zone = zone + "."
	}

	// Prepare a record object.
	// We add a TXT record as well as an identifier for the service.
	// This will be used for pruning unused records on the provider.
	record = RecordMeta{
		Records: []libdns.Record{
			{
				Type:  "A",
				Name:  strings.TrimSpace(host),
				Value: strings.Join(s.Addresses, ","),
				TTL:   ttl,
			},
			{
				Type:  "TXT",
				Name:  strings.TrimSpace(host),
				Value: fmt.Sprintf("service=%s namespace=%s created-by=nomad-external-dns", s.Name, s.Namespace),
				TTL:   ttl,
			},
		},
		Zone: zone,
	}
	return record, nil

}
