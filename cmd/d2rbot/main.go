package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/version"
)

func main() {
	defaultConfig := filepath.Join("configs", "config.yaml")
	configPath := flag.String("config", defaultConfig, "path to YAML config file")
	probe := flag.Bool("probe", false, "enable world-state logging (memory snapshots are always read when attached)")
	verbose := flag.Bool("verbose", false, "enable debug logging (shows position changes with --probe)")
	inputTest := flag.String("input-test", "", "manual input test spec (e.g. belt:1, portal, skill:1, center-click, click:640,360)")
	inputTestObserveMs := flag.Int("input-test-observe-ms", 3000, "observation window in ms after input-test actions")
	runFlag := flag.String("run", "", "active farming run (e.g. countess); overrides runs.active in config")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("d2rbot %s (%s)\n", version.Version, version.Commit)
		return
	}

	opts := app.Options{
		Probe:              *probe,
		Verbose:            *verbose,
		InputTest:          *inputTest,
		InputTestObserveMs: *inputTestObserveMs,
		Run:                *runFlag,
	}
	if err := run(*configPath, opts); err != nil {
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

	if opts.InputTest != "" {
		return rt.RunInputTest(opts.InputTest)
	}
	return rt.Run()
}
