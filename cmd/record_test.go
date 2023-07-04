package main

import (
	"testing"
	"time"

	"github.com/libdns/libdns"
	"github.com/stretchr/testify/assert"
)

func TestToRecord(t *testing.T) {
	tests := []struct {
		name      string
		service   *ServiceMeta
		domains   []string
		owner     string
		want      RecordMeta
		wantError bool
	}{
		{
			name: "valid service",
			service: &ServiceMeta{
				Name:      "redis",
				Namespace: "default",
				Job:       "redis-job",
				Addresses: []string{"192.168.1.1"},
				Tags:      []string{"external-dns/hostname=redis.test.internal", "external-dns/ttl=30s"},
			},
			domains: []string{"test.internal"},
			owner:   "test-owner",
			want: RecordMeta{
				Zone: "test.internal.",
				Records: []libdns.Record{
					{
						Type:  "A",
						Name:  "redis",
						Value: "192.168.1.1",
						TTL:   30 * time.Second,
					},
					{
						Type:  "TXT",
						Name:  "redis",
						Value: "service=redis namespace=default owner=test-owner created-by=nomad-external-dns",
						TTL:   30 * time.Second,
					},
				},
			},
		},
		{
			name: "empty tags",
			service: &ServiceMeta{
				Name:      "redis",
				Namespace: "default",
				Job:       "redis-job",
				Addresses: []string{"192.168.1.1"},
				Tags:      []string{},
			},
			domains:   []string{"test.internal"},
			owner:     "test-owner",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.service.ToRecord(tt.domains, tt.owner)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
