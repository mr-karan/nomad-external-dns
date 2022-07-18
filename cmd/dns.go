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
	libdns.RecordAppender
	libdns.RecordGetter
	libdns.RecordDeleter
}

// AddRecord adds a DNS record for the given Nomad service object.
func (app *App) updateRecord(rec libdns.Record, zone string) error {
	// TODO: Check if a record already exists. If yes, then append a new entry for it.
	// TODO: Else set a new one.
	fmt.Println("setting", rec)
	records, err := app.provider.SetRecords(context.Background(), zone, []libdns.Record{rec})
	if err != nil {
		return fmt.Errorf("error adding records to zone: %w", err)
	}
	fmt.Println(records)
	return nil
}

// prepareRecord takes a Nomad service object and parses various tags and applies
// formatting to convert to a libdns.Record object.
func (app *App) prepareRecord(svc *api.ServiceRegistration) (libdns.Record, string, error) {
	var (
		host   string
		zone   string
		err    error
		ttl    time.Duration
		record libdns.Record
	)

	// We extract the hostname/TTL from the annotated tags, so return if they're not present in the svc object.
	if len(svc.Tags) == 0 {
		return record, zone, fmt.Errorf("tags cannot be empty")
	}

	// Parse the hostname and TTL from tags.
	for _, tag := range svc.Tags {
		if strings.HasPrefix(tag, hostnameAnnotationKey) {
			host, zone, err = getRecordName(strings.Split(tag, hostnameAnnotationKey+"=")[1], app.opts.domains)
			if err != nil {
				return record, zone, fmt.Errorf("error fetching hostname from tags: %w", err)
			}
		}
		if strings.HasPrefix(tag, ttlAnnotationKey) {
			ttl, err = parseTTL(strings.Split(tag, ttlAnnotationKey+"=")[1])
			if err != nil {
				app.log.Warn("error parsing ttl, setting default", err)
				ttl = defaultTTL
			}
		}
	}

	// At this point both the host and zone should be non empty.
	if host == "" && zone == "" {
		return record, zone, fmt.Errorf("error parsing host: %s or zone: %s", host, zone)
	}

	// Format the zone as a proper FQDN.
	// TODO: Check libdns for formatting.
	if zone[len(zone)-1:] != "." {
		zone = zone + "."
	}

	// Fetch all the Address of this service object.
	svcObj, _, err := app.stream.Client.Services().Get(svc.ServiceName, &api.QueryOptions{Namespace: svc.Namespace})
	if err != nil {
		app.log.Error("error fetching service object", "svc", svc.ServiceName, "namespace", svc.Namespace)
	}
	// Add all the addresses here.
	addr := make([]string, 0, len(svcObj))
	for _, s := range svcObj {
		// Only append the service if it's not added before. Otherwise AWS complains of a duplicate entry.
		if !contains(addr, s.Address) {
			addr = append(addr, s.Address)
		}
	}

	// Prepare a record object.
	record = libdns.Record{
		Type:  getRecordType(svc.Address),
		Name:  strings.TrimSpace(host),
		Value: strings.Join(addr, ","),
		TTL:   ttl,
	}
	return record, zone, nil

}
