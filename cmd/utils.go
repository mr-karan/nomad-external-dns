package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
)

// GetRecordName returns the hostname and the zone for a given hostname annotated tag.
func GetRecordName(s string, zones []string) (string, string, error) {
	// For a given list of zone filters, determine if the hostname belongs to that zone.
	for _, z := range zones {
		if strings.Contains(s, z) {
			// If a valid zone is found, return the host and zone separately.
			return strings.TrimRight(strings.Split(s, z)[0], "."), z, nil
		}
	}

	return "", "", fmt.Errorf("hostname doesn't contain a valid zone TLD")
}

// ParseTTL parses TTL from string, returning duration in seconds.
func ParseTTL(s string) (time.Duration, error) {
	ttl, err := time.ParseDuration(s)
	if err != nil {
		return -1, err
	}
	return ttl, nil
}

// Contains checks if an element exists in the slice.
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// GetPrefix returns a unique identifier for a service in a cluster.
// Each namespace can only have a unique service name.
// For eg, for a service name `hello` and namespace as `default`
// this returns `default_hello`.
func GetPrefix(svc *api.ServiceRegistration) string {
	return fmt.Sprintf("%s_%s", svc.Namespace, svc.ServiceName)
}
