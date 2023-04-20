package main

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/zerodha/logf"
)

// Opts represents certain configurable items.
type Opts struct {
	updateInterval time.Duration
	pruneInterval  time.Duration
	owner          string
	domains        []string
	dryRun         bool
}

// App is the global container that holds
// objects of various routines that run on boot.
type App struct {
	sync.RWMutex

	lo          logf.Logger
	opts        Opts
	provider    DNSProvider
	nomadClient *api.Client
	services    map[string]ServiceMeta
}

// Start initialises background workers and waits for them to exit on cancellation.
func (app *App) Start(ctx context.Context) {
	wg := &sync.WaitGroup{}

	// Validate that prune_interval must always be greater than update_interval.
	// If it's less, there's a chance that the provider records will be deleted
	// before the services are updated inside the map.
	if app.opts.pruneInterval < app.opts.updateInterval {
		app.lo.Fatal("prune_interval needs to be higher than update_interval")
	}

	// Start a background worker for fetching and updating DNS records.
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.UpdateServices(ctx, app.opts.updateInterval)
	}()

	// Start a background worker for pruning outdated records that may exist in the DNS provider.
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.PruneRecords(ctx, app.opts.pruneInterval)
	}()

	// Wait for all routines to finish.
	wg.Wait()
}

// UpdateServices fetches Nomad services from all the namespaces
// at periodic interval and updates the records in upstream DNS providers.
// This is a blocking function so the caller must invoke as a goroutine.
func (app *App) UpdateServices(ctx context.Context, updateInterval time.Duration) {
	var (
		ticker = time.NewTicker(updateInterval).C
	)

	for {
		select {
		case <-ticker:
			// Fetch the list of services from the cluster.
			services, err := app.fetchServices()
			if err != nil {
				app.lo.Error("error fetching services", "error", err)
				continue
			}
			// Update DNS records for the services fetched.
			// This function holds a read lock to determine whether to update records or not.
			app.updateRecords(services, app.opts.domains)

			// Add the updated services map to the app once the records are synced.
			app.Lock()
			app.services = services
			app.Unlock()

		case <-ctx.Done():
			app.lo.Warn("context cancellation received, quitting update services worker")
			return
		}
	}
}

// PruneRecords fetches the records for all zones from the DNS provider.
// It then checks whether the service exists in Nomad cluster or not.
// If it doesn't exist then it prunes the record in Provider.
func (app *App) PruneRecords(ctx context.Context, pruneInterval time.Duration) {
	var (
		ticker = time.NewTicker(pruneInterval).C
	)

	for {
		select {
		case <-ticker:
			// Fetch the list of records from the DNS provider.
			records, err := app.fetchRecords()
			if err != nil {
				app.lo.Error("error fetching records", "error", err)
				continue
			}
			app.lo.Debug("fetched records", "count", len(records))
			// Handles DNS deletes for unused records.
			// This function write locks the services to cleanup unused records.
			app.cleanupRecords(records)

		case <-ctx.Done():
			app.lo.Warn("context cancellation received, quitting prune records worker")
			return
		}
	}
}
