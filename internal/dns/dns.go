package dns

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/libdns/libdns"
	"github.com/zerodha/logf"
)

const (
	// Annotated tag for defininig hostname.
	hostnameAnnotationKey = "external-dns/hostname"
	// Annotated tag for defininig TTL.
	ttlAnnotationKey = "external-dns/ttl"
	// Default TTL to set for records.
	defaultTTL = time.Second * 30
	// Default DNS record type.
	defaultRecordType = "A"
)

// Provider wraps the required libdns interfaces.
// The providers must satisfy this interface.
type Provider interface {
	libdns.RecordSetter
	libdns.RecordGetter
	libdns.RecordDeleter
}

type Controller struct {
	log      logf.Logger
	domains  []string
	provider Provider
}

// NewController initialises a controller object to interact with the DNS providers.
func NewController(provider Provider, log logf.Logger, domains []string) *Controller {
	return &Controller{
		provider: provider,
		log:      log,
		domains:  domains,
	}
}

// AddRecord adds a DNS record for the given Nomad service object.
func (c *Controller) AddRecord(svc *api.ServiceRegistration) error {
	// We extract the hostname/TTL from the annotated tags, so return if they're not present in the svc object.
	if len(svc.Tags) == 0 {
		return fmt.Errorf("tags cannot be empty")
	}

	var (
		host string
		zone string
		err  error
		ttl  time.Duration
	)

	// Parse the required values from tags.
	for _, tag := range svc.Tags {
		if strings.HasPrefix(tag, hostnameAnnotationKey) {
			host, zone, err = getRecordName(strings.Split(tag, hostnameAnnotationKey+"=")[1], c.domains)
			if err != nil {
				return err
			}
		}
		if strings.HasPrefix(tag, ttlAnnotationKey) {
			ttl, err = parseTTL(strings.Split(tag, ttlAnnotationKey+"=")[1])
			if err != nil {
				c.log.Error("error parsing ttl, setting default", err)
				ttl = defaultTTL
			}
		}
	}

	// At this point both the host and zone should be non empty.
	if host == "" && zone == "" {
		return fmt.Errorf("error parsing host: %s or zone: %s", host, zone)
	}

	// Format the zone as a proper FQDN.
	if zone[len(zone)-1:] != "." {
		zone = zone + "."
	}

	// Prepare a record object.
	rec := libdns.Record{
		Type:  getRecordType(svc.Address),
		Name:  strings.TrimSpace(host),
		Value: svc.Address,
		TTL:   ttl,
	}
	c.log.Debug("setting records", "type", rec.Type, "name", rec.Name, "zone", zone, "ttl", ttl)

	// If record doesn't exist, create.
	records, err := c.provider.SetRecords(context.Background(), zone, []libdns.Record{rec})
	if err != nil {
		return fmt.Errorf("error adding records to zone: %w", err)
	}
	fmt.Println(records)
	return nil
}

// // Delete deletes the DNS record for the given Nomad service.
// func (app *App) Delete(svc *api.ServiceRegistration) {
// 	// rec := libdns.Record{}
// 	// app.provider.AppendRecords(context.Background())
// }
