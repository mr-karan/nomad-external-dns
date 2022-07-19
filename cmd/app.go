package main

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/zerodha/logf"
)

type Opts struct {
	syncInterval time.Duration
	domains      []string
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

	// Before we start listening to the event stream, fetch existing services.
	// if err := app.fetchExistingAllocs(); err != nil {
	// 	app.log.Fatalw("error initialising index store", "error", err)
	// }

	wg.Add(1)
	go func() {
		defer wg.Done()
		app.SyncServices(ctx, app.opts.syncInterval)
	}()

	// Wait for all routines to finish.
	wg.Wait()
}

// SyncServices fetches Nomad services from all the namespaces
// at periodic interval and updates the records in upstream DNS providers.
// This is a blocking function so the caller must invoke as a goroutine.
func (app *App) SyncServices(ctx context.Context, refreshInterval time.Duration) {
	var (
		refreshTicker = time.NewTicker(refreshInterval).C
	)

	for {
		select {
		case <-refreshTicker:
			services, err := app.fetchServices()
			if err != nil {
				app.log.Error("error fetching services", "error", err)
			}
			// For each service, do a DNS update.
			app.updateRecords(services, app.opts.domains)

			// Add the updated services map to the app.
			app.Lock()
			app.services = services
			app.Unlock()

		case <-ctx.Done():
			app.log.Warn("context cancellation received, quitting worker")
			return
		}
	}
}
