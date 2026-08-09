package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/api"
	apiui "github.com/Tyniann/d2r-offline-farming-bot/internal/api/ui"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/version"
)

func main() {
	defaultConfig := filepath.Join("configs", "config.yaml")
	configPath := flag.String("config", defaultConfig, "path to YAML config file")
	dataRoot := flag.String("data-root", "", "absolute installed data root; loads configs/config.yaml below it")
	provisionDataRoot := flag.Bool("provision-data-root", false, "provision an installed data root and exit without starting the runtime")
	defaultsRoot := flag.String("defaults-root", "", "absolute read-only default bundle used with --provision-data-root")
	importRoot := flag.String("import-root", "", "absolute existing data root imported with --provision-data-root")
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
	mercenaryProbe := flag.String("mercenary-probe", "", "read-only hireling evidence label (e.g. not-hired, alive-healthy, alive-injured, dead, area-transition)")
	mercenaryProbeTimeoutMs := flag.Int("mercenary-probe-timeout-ms", 45000, "timeout in ms for a read-only mercenary probe capture")
	cowProbe := flag.String("cow-probe", "", "read-only Phase-20.0 evidence label (e.g. stony-tristram, wirts-body, cow-life-death-ce, cube-open)")
	cowProbeTimeoutMs := flag.Int("cow-probe-timeout-ms", 20000, "timeout in ms for a read-only Cow evidence capture")
	screenAnchorCapture := flag.String("screen-anchor-capture", "", "capture a named 1280x720 frontend screenshot for Phase 7.3 calibration")
	sessionInspect := flag.Bool("session-inspect", false, "validate and print the resolved autonomous-session plan without attaching or sending input")
	runsInspect := flag.Bool("runs-inspect", false, "print read-only run metadata and availability as stable JSON")
	waypointTargetsInspect := flag.Bool("waypoint-targets-inspect", false, "print registered read-only waypoint target calibration as stable JSON")
	sessionMaxRuns := flag.Int("session-max-runs", 0, "override the finite autonomous-session run count (0 uses config)")
	routeCommand := flag.String("route", "", "route command (list | inspect/validate/record/play:<id> | play-segment:<id>/<segment-id> | inspect/record/validate/play-egress:<act2|act3|act4|act5>)")
	routeName := flag.String("route-name", "", "display name for a route recording; only valid with record")
	routeDifficulty := flag.String("route-difficulty", "", "recording label: normal, nightmare, or hell; required with record")
	townInspect := flag.Bool("town-inspect", false, "write one read-only Phase-9.1 Town data-availability report")
	townTest := flag.String("town-test", "", "isolated Town interaction test (akara-shop | item-services:mephisto)")
	showVersion := flag.Bool("version", false, "print version and exit")
	desktopHandshakePipe := flag.String("desktop-handshake-pipe", "", "private one-shot Electron handshake pipe")
	flag.Parse()

	if *showVersion {
		fmt.Printf("d2rbot %s (%s)\n", version.Version, version.Commit)
		return
	}
	if *provisionDataRoot {
		result, err := provisionInstalledDataRoot(context.Background(), *dataRoot, *defaultsRoot, *importRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "error: encode provisioning result: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *defaultsRoot != "" || *importRoot != "" {
		fmt.Fprintln(os.Stderr, "error: --defaults-root and --import-root require --provision-data-root")
		os.Exit(1)
	}

	opts := app.Options{
		Desktop:                 *desktopHandshakePipe != "",
		DesktopHandshakePipe:    *desktopHandshakePipe,
		Probe:                   *probe,
		Verbose:                 *verbose,
		InputTest:               *inputTest,
		InputTestObserveMs:      *inputTestObserveMs,
		Run:                     *runFlag,
		RunPhase:                *phaseFlag,
		PathingTest:             *pathingTest,
		PathingTestTimeoutMs:    *pathingTestTimeoutMs,
		OfflineDifficulty:       *offlineDifficulty,
		OfflineCharacter:        *offlineCharacter,
		OfflineExitTest:         *offlineExitTest,
		UIStateProbe:            *uiStateProbe,
		UIStateProbeTimeoutMs:   *uiStateProbeTimeoutMs,
		MercenaryProbe:          *mercenaryProbe,
		MercenaryProbeTimeoutMs: *mercenaryProbeTimeoutMs,
		CowProbe:                *cowProbe,
		CowProbeTimeoutMs:       *cowProbeTimeoutMs,
		ScreenAnchorCapture:     *screenAnchorCapture,
		SessionInspect:          *sessionInspect,
		RunsInspect:             *runsInspect,
		WaypointTargetsInspect:  *waypointTargetsInspect,
		SessionMaxRuns:          *sessionMaxRuns,
		Route:                   *routeCommand,
		RouteName:               *routeName,
		RouteDifficulty:         *routeDifficulty,
		TownInspect:             *townInspect,
		TownTest:                *townTest,
	}
	if err := runWithDataRoot(*configPath, *dataRoot, opts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

type provisioningResult struct {
	SchemaVersion   int                `json:"schema_version"`
	Status          app.DataRootStatus `json:"status"`
	DiagnosticCount int                `json:"diagnostic_count"`
}

func provisionInstalledDataRoot(ctx context.Context, target, defaultsRoot, importRoot string) (provisioningResult, error) {
	if defaultsRoot == "" == (importRoot == "") {
		return provisioningResult{}, fmt.Errorf("exactly one of --defaults-root or --import-root is required")
	}
	manager, err := app.NewDataRootManager(target)
	if err != nil {
		return provisioningResult{}, err
	}
	var result app.DataRootResult
	if defaultsRoot != "" {
		result, err = manager.InitializeDefaults(ctx, defaultsRoot)
	} else {
		result, err = manager.Import(ctx, importRoot)
	}
	if err != nil {
		return provisioningResult{}, err
	}
	return provisioningResult{SchemaVersion: 1, Status: result.Status, DiagnosticCount: len(result.Diagnostics)}, nil
}

func run(configPath string, opts app.Options) error {
	return runWithDataRoot(configPath, "", opts)
}

func runWithDataRoot(configPath, dataRoot string, opts app.Options) error {
	if opts.DesktopHandshakePipe != "" && (dataRoot == "" || !opts.Desktop) {
		return fmt.Errorf("--desktop-handshake-pipe requires desktop mode and --data-root")
	}
	cfg, err := loadConfig(configPath, dataRoot)
	if err != nil {
		return err
	}
	var operatorSettings *app.OperatorSettingsStore
	var dataRootLock *app.DataRootLock
	if dataRoot != "" {
		if opts.DesktopHandshakePipe != "" {
			dataRootLock, err = app.AcquireDataRootLock(dataRoot)
			if err != nil {
				return err
			}
			defer dataRootLock.Close()
		}
		store, settings, settingsErr := app.OpenOperatorSettings(cfg)
		if settingsErr != nil {
			return fmt.Errorf("load operator settings: %w", settingsErr)
		}
		app.ApplyOperatorSettingsToConfig(cfg, settings)
		operatorSettings = store
	}
	if opts.SessionMaxRuns < 0 {
		return fmt.Errorf("--session-max-runs must be >= 0")
	}
	if opts.SessionMaxRuns > 0 {
		cfg.Session.MaxRuns = opts.SessionMaxRuns
	}
	if validationErr := validateDesktopMode(opts); validationErr != nil {
		return validationErr
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

	if shouldRunSession(cfg, opts) {
		return app.RunConfiguredQueue(cfg)
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
	if opts.Desktop {
		return runDesktopAPI(cfg, rt, operatorSettings, opts.DesktopHandshakePipe)
	}

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
	if opts.MercenaryProbe != "" {
		return rt.RunMercenaryProbe(opts.MercenaryProbe)
	}
	if opts.CowProbe != "" {
		return rt.RunCowProbe(opts.CowProbe)
	}
	if opts.ScreenAnchorCapture != "" {
		return rt.RunScreenAnchorCapture(opts.ScreenAnchorCapture)
	}
	if opts.PathingTest != "" {
		return rt.RunPathingTest(opts.PathingTest)
	}
	return rt.Run()
}

func loadConfig(configPath, dataRoot string) (*config.Config, error) {
	if dataRoot == "" {
		return config.Load(configPath)
	}
	if configPath != filepath.Join("configs", "config.yaml") {
		return nil, fmt.Errorf("--data-root and a custom --config path are mutually exclusive")
	}
	return config.LoadFromDataRoot(dataRoot)
}

func validateDesktopMode(opts app.Options) error {
	if !opts.Desktop {
		return nil
	}
	if opts.Probe || opts.InputTest != "" || opts.Run != "" || opts.RunPhase != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.MercenaryProbe != "" || opts.CowProbe != "" || opts.SessionInspect || opts.RunsInspect || opts.WaypointTargetsInspect || opts.SessionMaxRuns != 0 || opts.Route != "" || opts.RouteName != "" || opts.RouteDifficulty != "" || opts.TownInspect || opts.TownTest != "" {
		return fmt.Errorf("desktop mode is mutually exclusive with session, run, inspect, probe, route, town, and test modes")
	}
	return nil
}

func runDesktopAPI(cfg *config.Config, rt *app.Runtime, operatorSettings *app.OperatorSettingsStore, desktopHandshakePipe string) error {
	if desktopHandshakePipe == "" {
		return fmt.Errorf("desktop mode requires a private handshake pipe")
	}
	assets, err := apiui.FS()
	if err != nil {
		return err
	}
	publisher := telemetry.NewLivePublisher(256, 64)
	defer publisher.Close()
	backend, err := api.NewLiveBackend(cfg, publisher)
	if err != nil {
		return err
	}
	if settingsBindErr := backend.SetOperatorSettingsStore(operatorSettings); settingsBindErr != nil {
		return settingsBindErr
	}
	backend.Update(rt.CurrentUIStatus(""), app.SupervisorSnapshot{State: app.SupervisorStateIdle})
	server, err := api.New(api.Config{Backend: backend, Assets: assets, Logger: rt.Log, Events: publisher})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	backend.StartHistoryMaintenance(ctx)
	var monitorMu sync.Mutex
	var monitorCancel context.CancelFunc
	var monitorDone chan error
	monitorRunning := false
	startMonitor := func() {
		monitorMu.Lock()
		defer monitorMu.Unlock()
		if monitorRunning || ctx.Err() != nil {
			return
		}
		monitorCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		monitorCancel, monitorDone = cancel, done
		monitorRunning = true
		go func() { done <- rt.RunUIMonitor(monitorCtx, backend.UpdateRuntime) }()
	}
	stopMonitor := func() error {
		monitorMu.Lock()
		if !monitorRunning {
			monitorMu.Unlock()
			return nil
		}
		cancel, done := monitorCancel, monitorDone
		monitorRunning = false
		monitorMu.Unlock()
		cancel()
		return <-done
	}
	queueRunner, err := app.NewRuntimeQueueRunner(cfg, backend.UpdateRuntime)
	if err != nil {
		return err
	}
	supervisor, err := app.NewSessionSupervisor(queueRunner)
	if err != nil {
		return err
	}
	var pauseHotkeySequence atomic.Uint64
	var stopAfterRunHotkeySequence atomic.Uint64
	queueRunner.SetPauseAfterRunHandler(func() error {
		snapshot := supervisor.Snapshot()
		updated, pauseErr := supervisor.PauseAfterRun(app.SupervisorCommandMeta{
			CommandID:          fmt.Sprintf("pause-hotkey-%d", pauseHotkeySequence.Add(1)),
			ExpectedGeneration: snapshot.Generation,
		})
		if pauseErr != nil {
			return pauseErr
		}
		backend.UpdateSupervisor(updated)
		return nil
	})
	queueRunner.SetStopAfterRunHandler(func() error {
		snapshot := supervisor.Snapshot()
		updated, stopErr := supervisor.StopAfterRun(app.SupervisorCommandMeta{
			CommandID:          fmt.Sprintf("stop-after-run-hotkey-%d", stopAfterRunHotkeySequence.Add(1)),
			ExpectedGeneration: snapshot.Generation,
		})
		if stopErr != nil {
			return stopErr
		}
		backend.UpdateSupervisor(updated)
		return nil
	})
	if err := backend.SetSessionSupervisor(supervisor, stopMonitor, queueRunner.BeginQueue); err != nil {
		return err
	}
	startMonitor()
	backend.SetSelectionHandler(func(request app.CharacterSelectionRequest) error {
		if err := stopMonitor(); err != nil {
			return fmt.Errorf("stop passive monitor for selection: %w", err)
		}
		err := rt.ApplyCharacterSelection(ctx, request)
		if ctx.Err() == nil {
			startMonitor()
		}
		return err
	})
	backend.SetCharacterCaptureHandler(func(captureCtx context.Context, targetPath string) error {
		if err := stopMonitor(); err != nil {
			return fmt.Errorf("stop passive monitor for character capture: %w", err)
		}
		defer func() {
			if ctx.Err() == nil {
				startMonitor()
			}
		}()
		return rt.CaptureCharacterSelectionAnchor(captureCtx, targetPath)
	})
	backend.SetRouteWorkflowHandler(func(request api.RouteWorkflowRequest, finishRequests <-chan struct{}, reporter app.RouteWorkflowReporter) error {
		if err := stopMonitor(); err != nil {
			return fmt.Errorf("stop passive monitor for route workflow: %w", err)
		}
		defer func() {
			if ctx.Err() == nil {
				startMonitor()
			}
		}()
		switch request.Operation {
		case "record":
			status := backend.Status()
			if status.Selection.Difficulty == "" {
				return fmt.Errorf("vor der Aufnahme muss Charakter und Schwierigkeit bestätigt sein")
			}
			return rt.RunRouteRecordWithRoleFinish(request.RunID, pathing.RouteRole(request.RouteRole), status.Selection.Difficulty, status.Selection.Character, finishRequests, reporter)
		case "system_record":
			return rt.RunSystemEgressRecordWithFinish(town.OriginAct(request.Act), finishRequests, reporter)
		case "system_test":
			return rt.RunTownEgressPlay(town.OriginAct(request.Act))
		case "test":
			return rt.RunCandidateTestWithProgress(request.CandidateID, reporter)
		default:
			return fmt.Errorf("unbekannter Routen-Workflow %q", request.Operation)
		}
	})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		lastGeneration := supervisor.Snapshot().Generation
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				snapshot := supervisor.Snapshot()
				if snapshot.Generation == lastGeneration {
					continue
				}
				lastGeneration = snapshot.Generation
				backend.UpdateSupervisor(snapshot)
				switch snapshot.State {
				case app.SupervisorStateIdle, app.SupervisorStateIdleInGame, app.SupervisorStatePausedBetweenRuns, app.SupervisorStateStoppedError:
					startMonitor()
				}
			}
		}
	}()
	if err := server.Start(); err != nil {
		stop()
		_ = stopMonitor()
		return err
	}
	handshakeCtx, handshakeCancel := context.WithTimeout(ctx, 5*time.Second)
	defer handshakeCancel()
	if err := app.WriteDesktopHandshake(handshakeCtx, desktopHandshakePipe, app.DesktopHandshake{
		SchemaVersion: 1, CorePID: os.Getpid(), Generation: 1,
		BaseURL: server.URL(), BootstrapURL: server.BootstrapURL(),
	}); err != nil {
		stop()
		_ = server.Shutdown(context.Background())
		_ = stopMonitor()
		return fmt.Errorf("desktop handshake: %w", err)
	}
	<-ctx.Done()
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := supervisor.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown session supervisor: %w", err)
	}
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if monitorErr := stopMonitor(); monitorErr != nil {
		return fmt.Errorf("stop UI monitor: %w", monitorErr)
	}
	return nil
}

func shouldRunSession(cfg *config.Config, opts app.Options) bool {
	// Specialized CLI modes must never be swallowed by session.enabled.
	// Keep this aligned with app.SessionExecutionRequested.
	return cfg.Session.Enabled && app.SessionExecutionRequested(opts)
}
