package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/libdns/libdns"
)

// fetchRecords fetches all records from the DNS provider.
// It then checks all the records that are created by this program (by checking the TXT record for the hostname)
// and returns a map of records that are created by this program.
func (app *App) fetchRecords() (map[string][]RecordMeta, error) {
	recordsOut := make(map[string][]RecordMeta, 0)
	recordNames := make([]string, 0)

	// For each zone, get the updated DNS records from the provider.
	for _, d := range app.opts.domains {
		records, err := app.provider.GetRecords(context.Background(), d+".")
		if err != nil {
			return recordsOut, fmt.Errorf("error fetching records: %w", err)
		}

		// Iterate on these records of type TXT and find out whether it's created by this program
		for _, r := range records {
			if r.Type == "TXT" {
				if strings.Contains(r.Value, fmt.Sprintf("owner=%s", app.opts.owner)) {
					recordNames = append(recordNames, r.Name)
				}
			}
		}

		// For all the records in this domain that are created by this program, we need to add all possible records.
		for _, r := range records {
			if Contains(recordNames, r.Name) {
				// Set a proper relative name before we delete.
				// The r.Name from get records consists of FQDN but when we delete we need the relative name only.
				r.Name = libdns.RelativeName(strings.TrimRight(r.Name, "."), d)
				recordsOut[r.Name] = append(recordsOut[r.Name], RecordMeta{Zone: d, Records: []libdns.Record{r}})
			}
		}
	}

	return recordsOut, nil
}

// cleanupRecords iterates on the records map and does a map lookup to check
// if the service still exists in the cluster. In case the job is dead/service is no more,
// it adds these records and enqueues them for deleting. It then deletes both the TXT and A recod
// created for the hostname.
func (app *App) cleanupRecords(recordsMap map[string][]RecordMeta) {
	app.Lock()
	defer app.Unlock()

	deleteRecords := make([]string, 0)

	for name, records := range recordsMap {
		for _, rec := range records {
			for _, r := range rec.Records {
				if r.Type != "TXT" {
					continue
				}
				// Eg: "service=redis-cache namespace=dev created-by=nomad-external-dns"
				// The TXT record is wrapped around `"`, so we need to remove them.
				val := r.Value[1 : len(r.Value)-1]
				app.lo.Debug("parsing TXT record to get identifier", "value", val)

				// Split the string to parse each component.
				data := strings.Split(val, " ")
				var namespace, svc string
				if strings.HasPrefix(data[0], "service=") {
					svc = strings.Split(data[0], "=")[1]
				}
				if strings.HasPrefix(data[1], "namespace=") {
					namespace = strings.Split(data[1], "=")[1]
				}
				// Check if this service exists in the map or not.
				id := namespace + "_" + svc
				if _, exists := app.services[id]; !exists {
					app.lo.Debug("deleting outdated record", "id", id)
					deleteRecords = append(deleteRecords, name)
				}
			}
		}
	}

	app.lo.Info("pruning old DNS records", "count", len(deleteRecords))
	for _, rec := range deleteRecords {
		recMeta, exists := recordsMap[rec]
		// Unlikely to happen.
		if !exists {
			continue
		}
		for _, m := range recMeta {
			app.lo.Info("deleting record", "zone", m.Zone, "records", m.Records)
			_, err := app.provider.DeleteRecords(context.Background(), m.Zone+".", m.Records)
			if err != nil {
				app.lo.Error("error deleting records", "error", err)
				continue
			}
		}
	}
}
