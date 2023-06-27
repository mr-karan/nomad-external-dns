package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

var (
	// Version of the build. This is injected at build-time.
	buildString = "unknown"
	exit        = func() { os.Exit(1) }
)

func main() {
	// Create a new context which gets cancelled upon receiving `SIGINT`/`SIGTERM`.
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Initialise and load the config.
	ko := initConfig("config.sample.toml", "NOMAD_EXTERNAL_DNS_")

	// Initialise a new instance of app.
	app := App{
		lo:       initLogger(ko),
		opts:     initOpts(ko),
		services: make(map[string]ServiceMeta, 0),
	}

	// Initialise DNS provider.
	prov, err := initProvider(ko)
	if err != nil {
		app.lo.Error("error initialising dns provider", err)
		exit()
	}
	app.provider = prov

	// Initialise nomad api client.
	client, err := initNomadClient()
	if err != nil {
		app.lo.Error("error initialising nomad api client", err)
		exit()
	}
	app.nomadClient = client

	app.lo.Info("initialised nomad client", "addr", app.nomadClient.Address())

	// Start an instance of app.
	app.lo.Info("starting nomad-external-dns",
		"version", buildString,
	)

	// Validate that prune_interval must always be greater than update_interval.
	// If it's less, there's a chance that the provider records will be deleted
	// before the services are updated inside the map.
	if app.opts.pruneInterval < app.opts.updateInterval {
		app.lo.Warn("prune_interval needs to be higher than update_interval")
		exit()
	}

	app.Start(ctx)
}
