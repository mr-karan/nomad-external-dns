package main

import (
	"context"
)

// updateRecords goes through each service in the given map
// and propagates DNS record changes for new or updated services.
// The check to see if a service has to be updated reduces the number of
// API calls to the DNS provider.
// func (app *App) updateRecords(services map[string]ServiceMeta, domains []string) {
// 	app.RLock()
// 	defer app.RUnlock()

// 	records := make(map[string][]libdns.Record)

// 	for key, service := range services {
// 		if isNewOrUpdatedService(app.services[key], service) {
// 			app.lo.Debug("Service is new or updated", "service", service.DNSName)

// 			// Convert service to records and cache
// 			record, err := service.ToRecord(domains, app.opts.owner)
// 			if err != nil {
// 				app.lo.Error("error converting service to record", "error", err)
// 				continue
// 			}
// 			records[record.Zone] = append(records[record.Zone], record.Records...)

// 			// Cache service metadata.
// 			app.services[key] = service
// 		}
// 	}

// 	// Batch update records
// 	for zone, recs := range records {
// 		_, err := app.provider.SetRecords(context.Background(), zone, recs)
// 		if err != nil {
// 			app.lo.Error("error setting records to zone", "error", err)
// 			continue
// 		}
// 		app.lo.Info("Updated DNS records", "zone", zone, "records", recs)
// 	}
// }

// isNewOrUpdatedService checks if the service is new or has been updated.
func isNewOrUpdatedService(existingService, newService ServiceMeta) bool {
	// If the service does not exist or its addresses or tags have changed,
	// it's considered a new or updated service.
	return existingService.Name == "" ||
		!sameStringSlice(existingService.Addresses, newService.Addresses) ||
		!sameStringSlice(existingService.Tags, newService.Tags)
}

// TODO: This is the non-batched version.
// Test the batched version and remove this later.
// updateRecords goes through each service in the given map
// and propagates DNS record changes for new or updated services.
// The check to see if a service has to be updated reduces the number of
// API calls to the DNS provider.
func (app *App) updateRecords(services map[string]ServiceMeta, domains []string) {
	app.RLock()
	defer app.RUnlock()

	for key, service := range services {
		if isNewOrUpdatedService(app.services[key], service) {
			app.lo.Debug("Service is new or updated", "service", service.DNSName)
			if err := app.propogateChange(key, service, domains); err != nil {
				app.lo.Error("Error updating DNS records for service", "service", service.DNSName, "error", err)
				// Continue processing other services even if this one fails.
				continue
			}
		}
	}
}

// propogateChange updates DNS records for the given service and returns any error encountered.
func (app *App) propogateChange(key string, svc ServiceMeta, domains []string) error {
	record, err := svc.ToRecord(domains, app.opts.owner)
	if err != nil {
		app.lo.Error("error converting service to record", "error", err)
		return err
	}

	_, err = app.provider.SetRecords(context.Background(), record.Zone, record.Records)
	if err != nil {
		app.lo.Error("error setting records to zone", "error", err)
		return err
	}

	app.lo.Info("Updated DNS records", "zone", record.Zone, "records", record.Records)
	app.services[key] = svc
	return nil
}
