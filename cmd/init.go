package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	route53 "github.com/mr-karan/libdns-route53"
	flag "github.com/spf13/pflag"
	"github.com/zerodha/logf"
)

// initLogger initializes logger instance.
func initLogger(ko *koanf.Koanf) logf.Logger {
	opts := logf.Opts{EnableCaller: true}
	if ko.String("app.log_level") == "debug" {
		opts.Level = logf.DebugLevel
	}
	if ko.String("app.env") == "dev" {
		opts.EnableColor = true
	}
	return logf.New(opts)
}

// initConfig loads config to `ko` object.
func initConfig(cfgDefault string, envPrefix string) *koanf.Koanf {
	var (
		ko = koanf.New(".")
		f  = flag.NewFlagSet("front", flag.ContinueOnError)
	)

	// Configure Flags.
	f.Usage = func() {
		fmt.Println(f.FlagUsages())
		os.Exit(0)
	}

	// Register `--config` flag.
	cfgPath := f.String("config", cfgDefault, "Path to a config file to load.")

	// Parse and Load Flags.
	err := f.Parse(os.Args[1:])
	if err != nil {
		fmt.Printf("error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Load the config files from the path provided.
	fmt.Printf("attempting to load config from file: %s\n", *cfgPath)

	err = ko.Load(file.Provider(*cfgPath), toml.Parser())
	if err != nil {
		// If the default config is not present, print a warning and continue reading the values from env.
		if *cfgPath == cfgDefault {
			fmt.Printf("unable to open sample config file: %v\n", err)
		} else {
			fmt.Printf("error loading config: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("attempting to read config from env vars\n")
	// Load environment variables if the key is given
	// and merge into the loaded config.
	if envPrefix != "" {
		err = ko.Load(env.Provider(envPrefix, ".", func(s string) string {
			return strings.Replace(strings.ToLower(
				strings.TrimPrefix(s, envPrefix)), "__", ".", -1)
		}), nil)
		if err != nil {
			fmt.Printf("error loading env config: %v\n", err)
			os.Exit(1)
		}
	}

	return ko
}

// initNomadClient initialises a Nomad API client.
func initNomadClient() (*api.Client, error) {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}
	return client, nil
}

func initOpts(ko *koanf.Koanf) Opts {
	return Opts{
		updateInterval: ko.MustDuration("app.update_interval"),
		pruneInterval:  ko.MustDuration("app.prune_interval"),
		domains:        ko.MustStrings("dns.domain_filters"),
		dryRun:         ko.Bool("app.dry_run"),
		owner:          ko.MustString("dns.owner_uuid"),
	}
}

// initProvider initialises a DNS controller object to interact with
// the upstream DNS provider.
func initProvider(ko *koanf.Koanf) (DNSProvider, error) {
	var (
		provider DNSProvider
		err      error
	)

	switch ko.MustString("dns.provider") {
	case "route53":
		provider, err = route53.NewProvider(context.Background(), route53.Opt{
			Region: ko.MustString("provider.route53.region"), // libdns defaults to us-east-1 so this **must** be provided.
		})
		if err != nil {
			return nil, err
		}

		// TODO: Test this out.
	// case "cloudflare":
	// 	provider = &cloudflare.Provider{APIToken: ko.MustString("provider.cloudflare.api_token")}
	default:
		return nil, fmt.Errorf("unknown provider type")
	}

	// Initialise the controller object.
	return provider, nil
}
