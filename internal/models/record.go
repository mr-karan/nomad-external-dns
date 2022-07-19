package models

import "github.com/libdns/libdns"

// RecordMeta wraps around `libdns.Record`
// and adds additional fields.
type RecordMeta struct {
	Record libdns.Record
	Zone   string
}
