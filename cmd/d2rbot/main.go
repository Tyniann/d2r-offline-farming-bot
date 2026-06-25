package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

func main() {
	defaultConfig := filepath.Join("configs", "config.yaml")
	configPath := flag.String("config", defaultConfig, "path to YAML config file")
	probe := flag.Bool("probe", false, "enable Phase-1 state probing (memory snapshots)")
	verbose := flag.Bool("verbose", false, "enable debug logging (shows position changes with --probe)")
	flag.Parse()

	if err := run(*configPath, app.Options{Probe: *probe, Verbose: *verbose}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath string, opts app.Options) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	rt, err := app.New(cfg, opts)
	if err != nil {
		return err
	}

	return rt.Run()
}
