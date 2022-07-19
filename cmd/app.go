package main

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/zerodha/logf"
)

type Opts struct {
	updateInterval time.Duration
	pruneInterval  time.Duration
	domains        []string
}

// App is the global container that holds
// objects of various routines that run on boot.
type App struct {
	sync.RWMutex

	log  logf.Logger
	opts Opts

	provider    DNSProvider
	nomadClient *api.Client

	services map[string]ServiceMeta
}

// Start initialises the subscription stream in background and waits
// for context to be cancelled to exit.
func (app *App) Start(ctx context.Context) {
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		app.UpdateServices(ctx, app.opts.updateInterval)
	}()

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
				app.log.Error("error fetching services", "error", err)
				continue
			}
			// Handles DNS updates for these services.
			// This function read locks the services to determine whether to update records or not.
			app.updateRecords(services, app.opts.domains)

			// Add the updated services map to the app once the records are synced..
			app.Lock()
			app.services = services
			app.Unlock()

		case <-ctx.Done():
			app.log.Warn("context cancellation received, quitting worker")
			return
		}
	}
}

// PruneServices fetches Nomad services from all the namespaces
// at periodic interval and updates the records in upstream DNS providers.
// This is a blocking function so the caller must invoke as a goroutine.
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
				app.log.Error("error fetching records", "error", err)
				continue
			}
			// Handles DNS deletes for unused records.
			// This function write locks the services to cleanup unused records.
			app.cleanupRecords(records)

		case <-ctx.Done():
			app.log.Warn("context cancellation received, quitting worker")
			return
		}
	}
}
