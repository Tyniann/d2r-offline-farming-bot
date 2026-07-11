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
	phaseFlag := flag.String("phase", "", "optional run phase (e.g. travel-marsh or travel-cellar5 with --run countess)")
	pathingTest := flag.String("pathing-test", "", "manual pathing test spec (teleport:TX,TY | hover:watch | inspect:entrances|layout | move-area:<id|name> | click-entity:waypoint|entrance | pickup:item)")
	pathingTestTimeoutMs := flag.Int("pathing-test-timeout-ms", 120000, "timeout in ms for the pathing test mode")
	offlineDifficulty := flag.String("offline-difficulty-test", "", "select normal, nightmare, or hell from the prepared offline character screen")
	routeCommand := flag.String("route", "", "route command (list | inspect:<id> | validate:<id> | record:<id> | play-segment:<id>/<segment-id> | play:<id>)")
	routeName := flag.String("route-name", "", "display name for a route recording; only valid with record")
	routeDifficulty := flag.String("route-difficulty", "", "recording label: normal, nightmare, or hell; required with record")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("d2rbot %s (%s)\n", version.Version, version.Commit)
		return
	}

	opts := app.Options{
		Probe:                *probe,
		Verbose:              *verbose,
		InputTest:            *inputTest,
		InputTestObserveMs:   *inputTestObserveMs,
		Run:                  *runFlag,
		RunPhase:             *phaseFlag,
		PathingTest:          *pathingTest,
		PathingTestTimeoutMs: *pathingTestTimeoutMs,
		OfflineDifficulty:    *offlineDifficulty,
		Route:                *routeCommand,
		RouteName:            *routeName,
		RouteDifficulty:      *routeDifficulty,
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
	defer func() {
		if err := rt.CloseLog(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: close log file: %v\n", err)
		}
	}()

	if opts.InputTest != "" {
		return rt.RunInputTest(opts.InputTest)
	}
	if opts.Route != "" {
		return rt.RunRouteCommand(opts.Route)
	}
	if opts.OfflineDifficulty != "" {
		return rt.RunOfflineDifficultyTest(opts.OfflineDifficulty)
	}
	if opts.PathingTest != "" {
		return rt.RunPathingTest(opts.PathingTest)
	}
	return rt.Run()
}
