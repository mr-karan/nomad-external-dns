package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/libdns/cloudflare"
	route53 "github.com/mr-karan/libdns-route53"
	"github.com/mr-karan/nomad-events-sink/pkg/stream"
	"github.com/mr-karan/nomad-external-dns/internal/dns"
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

func initStream(ctx context.Context, ko *koanf.Koanf, cb stream.CallbackFunc) (*stream.Stream, error) {
	s, err := stream.New(
		ko.MustString("stream.data_dir"),
		ko.MustDuration("stream.commit_index_interval"),
		cb,
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("error initialising stream")
	}
	return s, nil
}

func initOpts(ko *koanf.Koanf) Opts {
	return Opts{
		maxReconnectAttempts: ko.MustInt("stream.max_reconnect_attempts"),
		nomadDataDir:         ko.MustString("stream.nomad_data_dir"),
	}
}

// initController initialises a DNS controller object to interact with
// the upstream DNS provider.
func initController(ko *koanf.Koanf, log logf.Logger) (*dns.Controller, error) {
	var (
		provider dns.Provider
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

	case "cloudflare":
		provider = &cloudflare.Provider{APIToken: ko.MustString("provider.cloudflare.api_token")}
	default:
		return nil, fmt.Errorf("unknown provider type")
	}

	// Initialise the controller object.
	return dns.NewController(provider, log, ko.MustStrings("dns.domain_filters")), nil
}
