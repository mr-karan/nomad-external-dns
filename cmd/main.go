package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var (
	// Version of the build. This is injected at build-time.
	buildString = "unknown"
	cfgPath     = "config.sample.toml"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ko := initConfig(cfgPath, "NOMAD_EXTERNAL_DNS_")

	app, err := initApp(ko)
	if err != nil {
		log.Fatalf("Unable to initialize the app: %v", err)
	}

	app.lo.Info("Starting nomad-external-dns", "version", buildString)
	app.Start(ctx)
}
