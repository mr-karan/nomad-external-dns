package main

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
	"golang.org/x/exp/slog"
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

	lo          *slog.Logger
	opts        Opts
	provider    DNSProvider
	nomadClient *api.Client
	services    map[string]ServiceMeta
}

// Start initialises background workers and waits for them to exit on cancellation.
func (app *App) Start(ctx context.Context) {
	var wg sync.WaitGroup

	app.runWorker(ctx, &wg, app.opts.updateInterval, app.UpdateServices, "updater")
	app.runWorker(ctx, &wg, app.opts.pruneInterval, app.PruneRecords, "pruner")

	// Wait for all routines to finish.
	wg.Wait()
}

// runWorker is a helper function to encapsulate the goroutine spawning and error handling logic.
func (app *App) runWorker(ctx context.Context, wg *sync.WaitGroup, interval time.Duration, workerFunc func(context.Context), workerName string) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				workerFunc(ctx)
			case <-ctx.Done():
				app.lo.Warn("Context cancellation received, terminating worker", "worker", workerName)
				return
			}
		}
	}()
}

// UpdateServices fetches Nomad services from all the namespaces
// and updates the records in upstream DNS providers.
func (app *App) UpdateServices(ctx context.Context) {
	// Fetch the list of services from the cluster.
	services, err := app.FetchNomadServices()
	if err != nil {
		app.lo.Error("Failed to fetch services", err)
		return
	}

	// Update DNS records for the services fetched.
	// This function holds a read lock to determine whether to update records or not.
	app.UpdateRecords(services, app.opts.domains)

	// Add the updated services map to the app once the records are synced.
	app.Lock()
	app.services = services
	app.Unlock()
}

// PruneRecords fetches the records for all zones from the DNS provider.
// It then checks whether the service exists in Nomad cluster or not.
// If it doesn't exist then it prunes the record in Provider.
func (app *App) PruneRecords(ctx context.Context) {
	// Fetch the list of records from the DNS provider.
	records, err := app.fetchRecords()
	if err != nil {
		app.lo.Error("Failed to fetch records", err)
		return
	}

	app.lo.Debug("Fetched records", "count", len(records))

	// Handles DNS deletes for unused records.
	// This function write locks the services to cleanup unused records.
	app.cleanupRecords(records)
}
