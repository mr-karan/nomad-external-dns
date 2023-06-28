package main

import (
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
)

// Contains checks if a string is present in a slice of strings.
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// getServiceID generates a unique ID for a service.
func getServiceID(svcMeta *ServiceMeta) string {
	return svcMeta.Namespace + "_" + svcMeta.Name + "_" + svcMeta.DNSName
}

// sameStringSlice checks if two string slices have the same elements, regardless of order.
func sameStringSlice(s1, s2 []string) bool {
	// Make copies of the slices, because sort.Strings() sorts in place
	s1Copy := append([]string{}, s1...)
	s2Copy := append([]string{}, s2...)

	sort.Strings(s1Copy)
	sort.Strings(s2Copy)

	if len(s1Copy) != len(s2Copy) {
		return false
	}
	for i := range s1Copy {
		if s1Copy[i] != s2Copy[i] {
			return false
		}
	}
	return true
}

// hasHostnameAnnotation checks if the provided tags contain a hostname annotation.
// The function iterates over the tags and returns true if a tag with the HostnameAnnotationKey prefix is found.
// If no such tag is found, the function returns false.
func hasHostnameAnnotation(tags []string) bool {
	for _, t := range tags {
		if strings.HasPrefix(t, HostnameAnnotationKey) {
			return true
		}
	}
	return false
}

// uniqueAddresses generates a slice of unique addresses from a provided slice of ServiceRegistration pointers.
// It uses a map as a set to remove duplicates, adding each address to the map.
// Then, it creates a slice of addresses from the keys of the map.
// The function returns this slice of unique addresses.
func uniqueAddresses(svcRegistrations []*api.ServiceRegistration) []string {
	uniqueAddrs := make(map[string]struct{})
	for _, s := range svcRegistrations {
		uniqueAddrs[s.Address] = struct{}{}
	}
	addr := make([]string, 0, len(uniqueAddrs))
	for key := range uniqueAddrs {
		addr = append(addr, key)
	}
	return addr
}

// getDNSNameFromTags extracts the DNS name from the service's tags.
func getDNSNameFromTags(tags []string) string {
	for _, tag := range tags {
		if strings.HasPrefix(tag, HostnameAnnotationKey+"=") {
			return strings.TrimPrefix(tag, HostnameAnnotationKey+"=")
		}
	}
	return ""
}

// EnsureFQDN makes sure the domain name is fully qualified (i.e., ends with a dot).
func EnsureFQDN(name string) string {
	if !strings.HasSuffix(name, ".") {
		return name + "."
	}
	return name
}
