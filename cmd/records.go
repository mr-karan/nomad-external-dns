package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/libdns/libdns"
)

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
				if strings.Contains(r.Value, "created-by=nomad-external-dns") {
					recordNames = append(recordNames, r.Name)
				}
			}
		}
		// For all the records in this domain that are created by this program, we need to add all possible records.
		for _, r := range records {
			if Contains(recordNames, r.Name) {
				// Set a proper relative name before we delete.
				r.Name = libdns.RelativeName(strings.TrimRight(r.Name, "."), d)
				recordsOut[r.Name] = append(recordsOut[r.Name], RecordMeta{Zone: d, Records: []libdns.Record{r}})
			}
		}
	}

	return recordsOut, nil
}

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
				data := strings.Split(r.Value, " ")
				var namespace, svc string
				if strings.HasPrefix(data[0], "service=") {
					svc = strings.Split(data[0], "=")[1]
				}
				if strings.HasPrefix(data[1], "namespace=") {
					namespace = strings.Split(data[0], "=")[1]
				}
				// Check if this service exists in the map or not.
				id := namespace + "_" + svc
				app.log.Debug("checking if service exists in map", "id", id)
				if _, exists := app.services[id]; !exists {
					deleteRecords = append(deleteRecords, name)
				}
			}

		}
	}

	app.log.Debug("pruning outdated records", "count", len(deleteRecords))
	for _, rec := range deleteRecords {
		recMeta, exists := recordsMap[rec]
		if !exists {
			continue
		}
		for _, m := range recMeta {
			app.log.Debug("deleting record", "zone", m.Zone, "records", m.Records)
			_, err := app.provider.DeleteRecords(context.Background(), m.Zone+".", m.Records)
			if err != nil {
				app.log.Error("error deleting records", "error", err)
				continue
			}
		}
	}
}
