package main

import (
	"encoding/json"
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
	phaseFlag := flag.String("phase", "", "optional run phase (e.g. travel-entry or play-route with --run countess)")
	pathingTest := flag.String("pathing-test", "", "manual pathing test spec (including record-town-edge:<id> or play-town-graph:<start,...,end>)")
	pathingTestTimeoutMs := flag.Int("pathing-test-timeout-ms", 120000, "timeout in ms for the pathing test mode")
	offlineDifficulty := flag.String("offline-difficulty-test", "", "start an offline game on normal, nightmare, or hell from the verified character screen")
	offlineCharacter := flag.String("offline-character", "", "expected selected character for --offline-difficulty-test (e.g. MrBones)")
	offlineExitTest := flag.Bool("offline-exit-test", false, "run one isolated Memory-gated Save & Exit test from Rogue Encampment")
	uiStateProbe := flag.String("ui-state-probe", "", "read-only UI-buffer capture label (e.g. gameplay, quit-menu, character-screen, difficulty-dialog)")
	uiStateProbeTimeoutMs := flag.Int("ui-state-probe-timeout-ms", 30000, "timeout in ms for a read-only UI-state capture")
	screenAnchorCapture := flag.String("screen-anchor-capture", "", "capture a named 1280x720 frontend screenshot for Phase 7.3 calibration")
	sessionInspect := flag.Bool("session-inspect", false, "validate and print the resolved autonomous-session plan without attaching or sending input")
	runsInspect := flag.Bool("runs-inspect", false, "print read-only run metadata and availability as stable JSON")
	waypointTargetsInspect := flag.Bool("waypoint-targets-inspect", false, "print registered read-only waypoint target calibration as stable JSON")
	sessionMaxRuns := flag.Int("session-max-runs", 0, "override the finite autonomous-session run count (0 uses config)")
	routeCommand := flag.String("route", "", "route command (list | inspect/validate/record/play:<id> | play-segment:<id>/<segment-id> | inspect/record/validate/play-egress:act3)")
	routeName := flag.String("route-name", "", "display name for a route recording; only valid with record")
	routeDifficulty := flag.String("route-difficulty", "", "recording label: normal, nightmare, or hell; required with record")
	townInspect := flag.Bool("town-inspect", false, "write one read-only Phase-9.1 Town data-availability report")
	townTest := flag.String("town-test", "", "isolated Town interaction test (akara-shop | item-services:mephisto)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("d2rbot %s (%s)\n", version.Version, version.Commit)
		return
	}

	opts := app.Options{
		Probe:                  *probe,
		Verbose:                *verbose,
		InputTest:              *inputTest,
		InputTestObserveMs:     *inputTestObserveMs,
		Run:                    *runFlag,
		RunPhase:               *phaseFlag,
		PathingTest:            *pathingTest,
		PathingTestTimeoutMs:   *pathingTestTimeoutMs,
		OfflineDifficulty:      *offlineDifficulty,
		OfflineCharacter:       *offlineCharacter,
		OfflineExitTest:        *offlineExitTest,
		UIStateProbe:           *uiStateProbe,
		UIStateProbeTimeoutMs:  *uiStateProbeTimeoutMs,
		ScreenAnchorCapture:    *screenAnchorCapture,
		SessionInspect:         *sessionInspect,
		RunsInspect:            *runsInspect,
		WaypointTargetsInspect: *waypointTargetsInspect,
		SessionMaxRuns:         *sessionMaxRuns,
		Route:                  *routeCommand,
		RouteName:              *routeName,
		RouteDifficulty:        *routeDifficulty,
		TownInspect:            *townInspect,
		TownTest:               *townTest,
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
	if opts.SessionMaxRuns < 0 {
		return fmt.Errorf("--session-max-runs must be >= 0")
	}
	if opts.SessionMaxRuns > 0 {
		cfg.Session.MaxRuns = opts.SessionMaxRuns
	}
	if opts.SessionInspect {
		plan, planErr := app.ResolveSessionPlan(cfg, opts)
		if planErr != nil {
			return planErr
		}
		encoded, encodeErr := json.MarshalIndent(plan, "", "  ")
		if encodeErr != nil {
			return fmt.Errorf("encode session plan: %w", encodeErr)
		}
		fmt.Println(string(encoded))
		return nil
	}
	if opts.RunsInspect {
		report, reportErr := app.ResolveRunsInspectReport(cfg, opts)
		if reportErr != nil {
			return reportErr
		}
		encoded, encodeErr := json.MarshalIndent(report, "", "  ")
		if encodeErr != nil {
			return fmt.Errorf("encode run availability: %w", encodeErr)
		}
		fmt.Println(string(encoded))
		return nil
	}
	if opts.WaypointTargetsInspect {
		report, reportErr := app.ResolveWaypointTargetsInspectReport(opts)
		if reportErr != nil {
			return reportErr
		}
		encoded, encodeErr := json.MarshalIndent(report, "", "  ")
		if encodeErr != nil {
			return fmt.Errorf("encode waypoint target calibration: %w", encodeErr)
		}
		fmt.Println(string(encoded))
		return nil
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
	if opts.TownInspect {
		return rt.RunTownInspect()
	}
	if opts.TownTest != "" {
		return rt.RunTownTest(opts.TownTest)
	}
	if opts.OfflineDifficulty != "" {
		return rt.RunOfflineDifficultyTest(opts.OfflineDifficulty)
	}
	if opts.OfflineExitTest {
		return rt.RunOfflineExitTest()
	}
	if opts.UIStateProbe != "" {
		return rt.RunUIStateProbe(opts.UIStateProbe)
	}
	if opts.ScreenAnchorCapture != "" {
		return rt.RunScreenAnchorCapture(opts.ScreenAnchorCapture)
	}
	if opts.PathingTest != "" {
		return rt.RunPathingTest(opts.PathingTest)
	}
	if shouldRunSession(cfg, opts) {
		return rt.RunSession()
	}
	return rt.Run()
}

func shouldRunSession(cfg *config.Config, opts app.Options) bool {
	return cfg.Session.Enabled && opts.Run == "" && opts.RunPhase == "" && !opts.Probe
}
