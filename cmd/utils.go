package main

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// getHostname returns the hostname and the zone for a given hostname annotated tag.
func getRecordName(s string, zones []string) (string, string, error) {
	// For a given list of zone filters, determine if the hostname belongs to that zone.
	for _, z := range zones {
		if strings.Contains(s, z) {
			// If a valid zone is found, return the host and zone separately.
			return strings.TrimRight(strings.Split(s, z)[0], "."), z, nil
		}
	}

	return "", "", fmt.Errorf("hostname doesn't contain a valid zone TLD")
}

// parseTTL parses TTL from string, returning duration in seconds.
func parseTTL(s string) (time.Duration, error) {
	ttl, err := time.ParseDuration(s)
	if err != nil {
		return -1, err
	}
	return ttl, nil
}

// Determines whether the IP is IPv4 or IPv6
// and returns the relevant record for it (A/AAAA).
func getRecordType(s string) string {
	ip := net.ParseIP(s)
	switch {
	case ip.To4() != nil:
		return "A"
	case ip.To16() != nil:
		return "AAAA"
	default:
		return defaultRecordType
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
