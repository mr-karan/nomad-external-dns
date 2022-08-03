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
		app.lo.Fatal("error initialising dns provider", "error", err)
	}
	app.provider = prov

	// Initialise nomad api client.
	client, err := initNomadClient()
	if err != nil {
		app.lo.Fatal("error initialising nomad api client", "error", err)
	}
	app.nomadClient = client

	app.lo.Info("initialised nomad client", "addr", app.nomadClient.Address())

	// Start an instance of app.
	app.lo.Info("starting nomad-external-dns",
		"version", buildString,
	)
	app.Start(ctx)
}
