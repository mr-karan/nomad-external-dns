package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/libdns/libdns"
)

// fetchRecords retrieves all records from the DNS provider and filters ones that are owned by this program.
func (app *App) fetchRecords() (map[string][]RecordMeta, error) {
	ownedRecords := make(map[string][]RecordMeta)

	for _, domain := range app.opts.domains {
		records, err := app.provider.GetRecords(context.Background(), domain+".")
		if err != nil {
			return nil, fmt.Errorf("error fetching records: %w", err)
		}

		for _, record := range records {
			if record.Type == "TXT" && strings.Contains(record.Value, fmt.Sprintf("owner=%s", app.opts.owner)) {
				record.Name = strings.TrimSuffix(record.Name, domain+".")
				relativeName := libdns.RelativeName(record.Name, domain)
				ownedRecords[relativeName] = append(ownedRecords[relativeName], RecordMeta{Zone: domain, Records: []libdns.Record{record}})
			}
		}
	}

	return ownedRecords, nil
}

// cleanupRecords identifies outdated records and deletes them from the DNS provider.
func (app *App) cleanupRecords(recordsMap map[string][]RecordMeta) {
	app.Lock()
	defer app.Unlock()

	outdatedRecords := app.identifyOutdatedRecords(recordsMap)
	app.deleteOutdatedRecords(outdatedRecords, recordsMap)
}

// identifyOutdatedRecords finds records that no longer have a corresponding service in the cluster.
func (app *App) identifyOutdatedRecords(recordsMap map[string][]RecordMeta) []string {
	var outdatedRecords []string

	for name, records := range recordsMap {
		for _, rec := range records {
			for _, r := range rec.Records {
				if r.Type != "TXT" {
					continue
				}

				val := strings.Trim(r.Value, "\"") // Removes the start and end quotes
				data := strings.Split(val, " ")

				var namespace, svc string
				for _, d := range data {
					if strings.HasPrefix(d, "service=") {
						svc = strings.Split(d, "=")[1]
					} else if strings.HasPrefix(d, "namespace=") {
						namespace = strings.Split(d, "=")[1]
					}
				}

				id := namespace + "_" + svc
				if _, exists := app.services[id]; !exists {
					outdatedRecords = append(outdatedRecords, name)
				}
			}
		}
	}

	return outdatedRecords
}

// deleteOutdatedRecords removes outdated DNS records from the DNS provider.
func (app *App) deleteOutdatedRecords(outdatedRecords []string, recordsMap map[string][]RecordMeta) {
	app.lo.Info("Pruning old DNS records", "count", len(outdatedRecords))

	for _, record := range outdatedRecords {
		recordMeta, exists := recordsMap[record]
		if !exists {
			// This is unlikely to happen.
			continue
		}

		for _, meta := range recordMeta {
			_, err := app.provider.DeleteRecords(context.Background(), meta.Zone+".", meta.Records)
			if err != nil {
				app.lo.Error("Error deleting records", "error", err)
				continue
			}

			app.lo.Info("Deleted record", "zone", meta.Zone, "records", meta.Records)
		}
	}
}
