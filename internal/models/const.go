package models

import "time"

const (
	HostnameAnnotationKey = "external-dns/hostname" // Annotated tag for defininig hostname.
	TTLAnnotationKey      = "external-dns/ttl"      // Annotated tag for defininig TTL.
	DefaultTTL            = time.Second * 30        // Default TTL to set for records.
	DefaultRecordType     = "A"                     // Default DNS record type.
)
