package main

import (
	"context"
	"sync"

	"github.com/hashicorp/nomad/api"
	"github.com/mr-karan/nomad-events-sink/pkg/stream"
	"github.com/mr-karan/nomad-external-dns/internal/service"
	"github.com/mr-karan/nomad-external-dns/internal/utils"
	"github.com/zerodha/logf"
)

type Opts struct {
	maxReconnectAttempts int
	nomadDataDir         string
	domains              []string
}

// App is the global container that holds
// objects of various routines that run on boot.
type App struct {
	sync.RWMutex

	log  logf.Logger
	opts Opts

	stream   *stream.Stream
	provider service.DNSProvider

	services map[string]*service.ServiceMeta
}

// Start initialises the subscription stream in background and waits
// for context to be cancelled to exit.
func (app *App) Start(ctx context.Context) {
	wg := &sync.WaitGroup{}

	// Before we start listening to the event stream, fetch existing services.
	// if err := app.fetchExistingAllocs(); err != nil {
	// 	app.log.Fatalw("error initialising index store", "error", err)
	// }

	// Initialise index store from disk to continue reading
	// from last event which is processed.
	if err := app.stream.InitIndex(ctx); err != nil {
		app.log.Fatal("error initialising index store", "error", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		// Subscribe to "Allocation" topic.
		if err := app.stream.Subscribe(ctx, string(api.TopicService), app.opts.maxReconnectAttempts); err != nil {
			app.log.Error("error subscribing to events", "topic", string(api.TopicService), "error", err)
		}
	}()

	// Wait for all routines to finish.
	wg.Wait()
}

// handleEvent is the callback function that is registered with stream. This function
// is called whenever a new event comes in the stream.
func (app *App) handleEvent(e api.Event, meta stream.Meta) {
	if e.Topic != api.TopicService {
		return
	}

	// Get the service object.
	svc, err := e.Service()
	if err != nil {
		app.log.Error("error fetching service", "error", err)
		return
	}
	app.log.Debug("received service event",
		"type", e.Type,
		"id", svc.ID,
		"name", svc.ServiceName,
		"namespace", svc.Namespace,
		"alloc", svc.AllocID,
		"job", svc.JobID,
		"addr", svc.Address,
		"port", svc.Port,
	)

	// Event Types: https://www.nomadproject.io/api-docs/events#event-types
	switch e.Type {
	case "ServiceRegistration":
		app.log.Info("handling svc registration event", "svc", svc.ServiceName)
		// Check if service object exists in the map.
		if _, ok := app.services[utils.GetPrefix(svc)]; !ok {
			app.log.Debug("creating a new service controller", "svc", svc.ServiceName, "namespace", svc.Namespace)
			//If it doesn't exist, create and set in map.
			svcMeta := service.InitController(service.Opts{
				Logger:    app.log,
				Name:      svc.ServiceName,
				Job:       svc.JobID,
				Namespace: svc.Namespace,
				ID:        svc.ID,
				Domains:   app.opts.domains,
				Client:    *app.stream.Client,
				Provider:  app.provider,
			})
			svcMeta.Start()
			app.services[utils.GetPrefix(svc)] = svcMeta
		}
		// Add the event to the channel inside svcMeta
		svcMeta := app.services[utils.GetPrefix(svc)]
		svcMeta.Receive(svc)
	default:
		return
	}

}
