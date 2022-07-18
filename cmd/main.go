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
		log:  initLogger(ko),
		opts: initOpts(ko),
	}

	// Initialise DNS controller.
	prov, err := initProvider(ko, app.log)
	if err != nil {
		app.log.Fatal("error initialising provider", "error", err)
	}
	app.provider = prov

	// Initialise nomad events stream.
	strm, err := initStream(ctx, ko, app.handleEvent)
	if err != nil {
		app.log.Fatal("error initialising stream", "error", err)
	}
	app.stream = strm

	// Start an instance of app.
	app.log.Info("booting nomad alloc logger",
		"version", buildString,
	)
	app.Start(ctx)
}
