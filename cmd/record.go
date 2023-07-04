package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/libdns/libdns"
)

// ToRecord converts a service meta object to a libdns record.
// This is used to send to upstream DNS providers.
func (s *ServiceMeta) ToRecord(domains []string, owner string) (RecordMeta, error) {
	var (
		host string
		zone string
		ttl  = DefaultTTL // Set default TTL
		err  error
	)

	if len(s.Tags) == 0 {
		return RecordMeta{}, fmt.Errorf("tags cannot be empty")
	}

	// Parse the hostname and TTL from tags.
	host, zone, ttl, err = s.parseTags(domains)
	if err != nil {
		return RecordMeta{}, err
	}

	zone = EnsureFQDN(zone)

	record := prepareRecord(s, host, zone, ttl, owner)
	return record, nil
}

// parseTags parses service tags to extract hostname, zone and ttl.
func (s *ServiceMeta) parseTags(domains []string) (host, zone string, ttl time.Duration, err error) {
	for _, tag := range s.Tags {
		if strings.HasPrefix(tag, HostnameAnnotationKey) {
			host, zone, err = parseHost(tag, domains)
			if err != nil {
				return
			}
		} else if strings.HasPrefix(tag, TTLAnnotationKey) {
			ttl, err = parseTTL(tag)
			if err != nil {
				ttl = DefaultTTL
			}
		}
	}
	return
}

// parseHost extracts host and zone from a given tag.
func parseHost(tag string, domains []string) (host, zone string, err error) {
	split := strings.Split(tag, HostnameAnnotationKey+"=")
	if len(split) != 2 {
		return "", "", fmt.Errorf("error splitting tag %s: expected 2 elements, got %d", tag, len(split))
	}

	// For a given list of domain filters, determine if the hostname belongs to that domain.
	for _, domain := range domains {
		if strings.Contains(split[1], domain) {
			// If a valid domain is found, return the host and domain separately.
			return strings.TrimRight(strings.Split(split[1], domain)[0], "."), domain, nil
		}
	}

	return "", "", fmt.Errorf("hostname doesn't contain a valid domain TLD")
}

// parseTTL extracts ttl from a given tag.
func parseTTL(tag string) (ttl time.Duration, err error) {
	split := strings.Split(tag, TTLAnnotationKey+"=")
	if len(split) != 2 {
		return -1, fmt.Errorf("error splitting tag %s: expected 2 elements, got %d", tag, len(split))
	}
	ttl, err = time.ParseDuration(split[1])
	if err != nil {
		return -1, err
	}
	return ttl, nil
}

func prepareRecord(s *ServiceMeta, host, zone string, ttl time.Duration, owner string) RecordMeta {
	// Generate comma-separated list of addresses
	addresses := strings.Join(s.Addresses, ",")

	// Create an A record with all addresses
	aRecord := libdns.Record{
		Type:  "A",
		Name:  host,
		Value: addresses,
		TTL:   ttl,
	}

	// Create a TXT record with metadata
	txtRecord := libdns.Record{
		Type: "TXT",
		Name: host,
		Value: fmt.Sprintf(
			"service=%s namespace=%s owner=%s created-by=nomad-external-dns",
			s.Name, s.Namespace, owner,
		),
		TTL: ttl,
	}

	// Combine the A and TXT records
	records := []libdns.Record{aRecord, txtRecord}

	return RecordMeta{
		Zone:    zone,
		Records: records,
	}
}
