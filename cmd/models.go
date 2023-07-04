package main

import (
	"time"

	"github.com/libdns/libdns"
)

const (
	// HostnameAnnotationKey is the annotated tag for defining hostname.
	HostnameAnnotationKey = "external-dns/hostname"
	// TTLAnnotationKey is the annotated tag for defining TTL.
	TTLAnnotationKey = "external-dns/ttl"
	// DefaultTTL is the TTL to set for records if unspecified or unparseable.
	DefaultTTL = time.Second * 30
)

// ServiceMeta contains minimal items from a api.ServiceRegistration event.
type ServiceMeta struct {
	Name      string   // Human Name of the service.
	Namespace string   // Namespace to which the service belongs to.
	Job       string   // Job to which the service belongs to.
	Addresses []string // Address of all backend services which is fetched by calling Nomad HTTP API.
	Tags      []string // Tags in the given service.
	DNSName   string   // DNS name of the service.
}

// DNSProvider wraps the required libdns interfaces.
type DNSProvider interface {
	libdns.RecordAppender
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
