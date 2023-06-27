package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/libdns/libdns"
)

// cleanupRecords identifies outdated DNS records and deletes them from the DNS provider.
// This method is locked to prevent concurrent modification of shared resources.
func (app *App) cleanupRecords() error {
	app.Lock()         // Lock to prevent concurrent modifications
	defer app.Unlock() // Unlock when function execution is finished

	app.lo.Debug("Starting cleanup of DNS records")

	// Fetch all DNS records owned by this program
	recordsMap, err := app.fetchRecords()
	if err != nil {
		return fmt.Errorf("error fetching records: %w", err)
	}

	// Identify records that are outdated i.e., not present in the current service list
	outdatedRecords := identifyOutdatedRecords(app.services, recordsMap)
	app.lo.Debug("Identified outdated records", "count", len(outdatedRecords), "records", outdatedRecords)

	// Delete the outdated records from the DNS provider
	if err := app.deleteOutdatedRecords(outdatedRecords, recordsMap); err != nil {
		return fmt.Errorf("error deleting outdated records: %w", err)
	}

	return nil
}

// identifyOutdatedRecords compares the current service list with the DNS records and identifies which records are outdated.
func identifyOutdatedRecords(services map[string]ServiceMeta, recordsMap map[string][]RecordMeta) []string {
	outdatedRecords := make([]string, 0)
	for recordName := range recordsMap {
		if _, exists := services[recordName]; !exists {
			outdatedRecords = append(outdatedRecords, recordName)
		}
	}
	return outdatedRecords
}

// fetchRecords retrieves all records from the DNS provider and filters ones that are owned by this program.
// It groups the owned records by domain name.
func (app *App) fetchRecords() (map[string][]RecordMeta, error) {
	ownedRecords := make(map[string][]RecordMeta)

	// Iterate over all configured domains
	for _, domain := range app.opts.domains {
		zone := EnsureFQDN(domain)

		// Get all DNS records for this zone
		records, err := app.provider.GetRecords(context.Background(), zone)
		if err != nil {
			return nil, fmt.Errorf("error fetching records for zone %s: %w", zone, err)
		}

		// Filter out records that are not owned by this program
		recordNames := filterOwnedRecords(records, app.opts.owner)

		// Build a map of owned records grouped by the record name
		// For A records, if multiple records exist for the same name, their values are concatenated
		groupOwnedRecords(&ownedRecords, records, recordNames, zone)
	}

	return ownedRecords, nil
}

// filterOwnedRecords iterates over all records and returns a slice of names of records that are owned by this program.
func filterOwnedRecords(records []libdns.Record, owner string) []string {
	recordNames := make([]string, 0)
	for _, r := range records {
		if r.Type == "TXT" && strings.Contains(r.Value, fmt.Sprintf("owner=%s", owner)) {
			recordNames = append(recordNames, r.Name)
		}
	}
	return recordNames
}

// groupOwnedRecords groups the owned records by their name. For A records with the same name, their values are concatenated.
func groupOwnedRecords(ownedRecords *map[string][]RecordMeta, records []libdns.Record, recordNames []string, zone string) {
	for _, rec := range records {
		if Contains(recordNames, rec.Name) {
			dns := rec.Name
			rec.Name = strings.TrimSuffix(rec.Name, zone) // Overwrite to set a proper relative name before we delete.

			// If an A record with the same name already exists, append the IP address to the existing record.
			if rec.Type == "A" && len((*ownedRecords)[dns]) > 0 {
				(*ownedRecords)[dns][0].Records[0].Value += "," + rec.Value
			} else {
				(*ownedRecords)[dns] = append((*ownedRecords)[dns], RecordMeta{Zone: EnsureFQDN(zone), Records: []libdns.Record{rec}})
			}
		}
	}
}

// deleteOutdatedRecords removes the outdated DNS records from the DNS provider.
func (app *App) deleteOutdatedRecords(outdatedRecords []string, recordsMap map[string][]RecordMeta) error {
	app.lo.Debug("Starting deletion of outdated DNS records", "count", len(outdatedRecords))

	// Iterate over all outdated records
	for _, record := range outdatedRecords {
		recordMeta, exists := recordsMap[record]
		if !exists {
			// This is unlikely to happen but we skip to the next iteration just in case
			continue
		}

		// Delete each outdated record
		for _, meta := range recordMeta {
			_, err := app.provider.DeleteRecords(context.Background(), EnsureFQDN(meta.Zone), meta.Records)
			if err != nil {
				app.lo.Error("Error deleting records", "error", err)
				continue
			}

			app.lo.Info("Deleted record successfully", "zone", meta.Zone, "records", meta.Records)
		}
	}

	return nil
}
