package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/libdns/libdns"
)

// fetchRecords fetches all records from the DNS provider and returns a map of records that are created by this program.
func (app *App) fetchRecords() (map[string][]RecordMeta, error) {
	recordsOut := make(map[string][]RecordMeta)

	for _, domain := range app.opts.domains {
		records, err := app.provider.GetRecords(context.Background(), domain+".")
		if err != nil {
			return nil, fmt.Errorf("error fetching records: %w", err)
		}

		ownedRecordNames := app.filterOwnedRecords(records)
		recordsOut = app.collectOwnedRecords(records, ownedRecordNames, domain, recordsOut)
	}

	return recordsOut, nil
}

// filterOwnedRecords filters the records created by this program.
func (app *App) filterOwnedRecords(records []libdns.Record) []string {
	var ownedRecordNames []string
	for _, record := range records {
		if record.Type == "TXT" && strings.Contains(record.Value, fmt.Sprintf("owner=%s", app.opts.owner)) {
			ownedRecordNames = append(ownedRecordNames, record.Name)
		}
	}
	return ownedRecordNames
}

// collectOwnedRecords collects all possible records for the records created by this program.
func (app *App) collectOwnedRecords(records []libdns.Record, ownedRecordNames []string, domain string, recordsOut map[string][]RecordMeta) map[string][]RecordMeta {
	for _, record := range records {
		if Contains(ownedRecordNames, record.Name) {
			relativeName := libdns.RelativeName(strings.TrimRight(record.Name, "."), domain)
			recordsOut[relativeName] = append(recordsOut[relativeName], RecordMeta{Zone: domain, Records: []libdns.Record{record}})
		}
	}
	return recordsOut
}

// cleanupRecords iterates on the records map and checks if the service still exists in the cluster. If it doesn't, it deletes the DNS records.
func (app *App) cleanupRecords(recordsMap map[string][]RecordMeta) {
	app.Lock()
	defer app.Unlock()

	deleteRecords := app.getOutdatedRecords(recordsMap)
	app.deleteOutdatedRecords(deleteRecords, recordsMap)
}

// getOutdatedRecords identifies records that no longer have a corresponding service in the cluster.
func (app *App) getOutdatedRecords(recordsMap map[string][]RecordMeta) []string {
	var deleteRecords []string

	for name, records := range recordsMap {
		for _, rec := range records {
			for _, r := range rec.Records {
				if r.Type != "TXT" {
					continue
				}

				val := strings.Trim(r.Value, "\"") // Removed the start and end quotes
				app.lo.Debug("parsing TXT record to get identifier", "value", val)

				data := strings.Split(val, " ")
				var namespace, svc string
				if strings.HasPrefix(data[0], "service=") {
					svc = strings.Split(data[0], "=")[1]
				}
				if strings.HasPrefix(data[1], "namespace=") {
					namespace = strings.Split(data[1], "=")[1]
				}

				id := namespace + "_" + svc
				if _, exists := app.services[id]; !exists {
					app.lo.Debug("deleting outdated record", "id", id)
					deleteRecords = append(deleteRecords, name)
				}
			}
		}
	}

	return deleteRecords
}

// deleteOutdatedRecords deletes the outdated DNS records from the DNS provider.
func (app *App) deleteOutdatedRecords(deleteRecords []string, recordsMap map[string][]RecordMeta) {
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
				app.lo.Error("error deleting records", err)
				continue
			}
		}
	}
}
