package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/version"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// Runtime bundles initialized application components.
type Runtime struct {
	Config  *config.Config
	Options Options
	Log     *slog.Logger
	logFile *os.File

	Process   processController
	Memory    *memory.Reader
	Probe     snapshotReader
	World     *world.Model
	Input     inputController
	Bindings  configBindingSource
	Tasks     *tasks.Runner
	Pathing   *pathing.Navigator
	TownWalk  *pathing.TownWalker
	Loot      *loot.Filter
	Telemetry *telemetry.Recorder
}

// New builds a Runtime from config and CLI/runtime options.
func New(cfg *config.Config, opts Options) (rt *Runtime, err error) {
	var runTelemetry *telemetry.Recorder
	logLevel := cfg.App.LogLevel
	if opts.Verbose {
		logLevel = "debug"
	}
	log, logFile, logFilePath, err := config.NewFileLogger(logLevel, "logs", cfg.App.Name, time.Now())
	if err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}
	defer func() {
		if err != nil && logFile != nil {
			_ = logFile.Close()
		}
		if err != nil && runTelemetry != nil {
			_ = runTelemetry.Close()
		}
	}()
	log = log.With("app", cfg.App.Name)
	log.Info("file logging enabled", "path", logFilePath)

	offsetsPath := cfg.ResolvePath(cfg.Memory.OffsetsFile)
	offsetSet, err := memory.ResolveOffsetSet(offsetsPath)
	if err != nil {
		return nil, fmt.Errorf("load offsets: %w", err)
	}

	expectedVersion := cfg.Memory.GameVersion
	if expectedVersion == "" {
		expectedVersion = offsetSet.D2RVersion
	}
	if cfg.Memory.GameVersion != "" && offsetSet.D2RVersion != "" && cfg.Memory.GameVersion != offsetSet.D2RVersion {
		log.Warn("memory.game_version does not match offset set",
			"config_game_version", cfg.Memory.GameVersion,
			"offset_d2r_version", offsetSet.D2RVersion,
		)
	}

	offsetsSource := "(default)"
	if offsetsPath != "" {
		offsetsSource = offsetsPath
	}
	log.Info("offset configuration",
		"game_version", expectedVersion,
		"offset_set", offsetSet.Name,
		"offsets_file", offsetsSource,
		"attach_timeout_ms", cfg.Process.AttachTimeoutMs,
	)

	mem := memory.NewReader(log)
	proc := process.New(log, cfg.Process.ProcessName)
	inputCtrl, err := input.NewController(log, mapInputConfig(cfg.Input), mapSafetyConfig(cfg.Input))
	if err != nil {
		return nil, fmt.Errorf("input controller: %w", err)
	}
	bindings, err := newConfigBindingSource(cfg.Input.Bindings)
	if err != nil {
		return nil, fmt.Errorf("input bindings: %w", err)
	}

	pathingCfg := mapPathingConfig(cfg.Pathing)
	if err := pathingCfg.Validate(); err != nil {
		return nil, fmt.Errorf("pathing config: %w", err)
	}
	nav := pathing.NewNavigator(log, pathing.Deps{
		Input:    inputCtrl,
		Bindings: bindings,
		Config:   pathingCfg,
	})
	waypoints := pathing.NewWaypointActions(log, inputCtrl, pathingCfg)
	townPortals := pathing.NewTownPortalActions(log, inputCtrl, pathingCfg)
	townWalker := pathing.NewTownWalker(log, inputCtrl, pathingCfg)
	personalStash := pathing.NewPersonalStashActions(log, inputCtrl, pathingCfg)
	runCfg := mapRunConfig(cfg.Runs)
	combat := newCombatAdapter(log, inputCtrl, bindings, pathingCfg, runCfg.CountessCombat.AttackInterval)
	runActions := newRunActionsAdapter(log, inputCtrl, bindings)
	inventoryLock, err := loot.NewInventoryLock(cfg.Loot.InventoryLock)
	if err != nil {
		return nil, fmt.Errorf("loot inventory lock: %w", err)
	}
	pickit, err := loadPickit(cfg)
	if err != nil {
		return nil, err
	}
	runSelection := resolveRunSelection(opts, cfg)
	if err := validateRunMode(runSelection, cfg, opts, log); err != nil {
		return nil, err
	}
	if runSelection.Run != "" {
		runTelemetry, err = telemetry.New(cfg.Telemetry.Directory, runSelection.Run, runSelection.Phase)
		if err != nil {
			return nil, fmt.Errorf("create run telemetry: %w", err)
		}
		log.Info("run telemetry enabled", "run_id", runTelemetry.RunID(), "path", runTelemetry.Path())
	}
	lootFilter := loot.NewFilter(log, inventoryLock, pickit)
	stashExecutor, err := loot.NewStashExecutor(log, lootFilter, inputCtrl, mapLootStashConfig(cfg.Loot.Stash))
	if err != nil {
		return nil, fmt.Errorf("loot stash config: %w", err)
	}
	lootActions := newLootActionsAdapter(log, lootFilter, cfg.Loot.Pickup, inputCtrl, pathingCfg, stashExecutor, runTelemetry)
	routePlayback := newRoutePlaybackAdapter(log, cfg.ResolvePath(cfg.Routes.Directory), expectedVersion, nav, runTelemetry)

	probe := memory.NewProbeReader(mem, offsetSet)
	probe.SetScannedCachePath(cfg.ResolvePath(memory.DefaultScannedCacheFile))

	rt = &Runtime{
		Config:   cfg,
		Options:  opts,
		Log:      log,
		logFile:  logFile,
		Process:  proc,
		Memory:   mem,
		Probe:    probe,
		World:    world.NewModel(log),
		Input:    inputCtrl,
		Bindings: bindings,
		Tasks: tasks.NewRunner(log, runSelection, runCfg, tasks.Deps{
			Input:    inputCtrl,
			Pathing:  nav,
			Waypoint: waypoints,
			Portal:   townPortals,
			TownWalk: townWalker,
			Stash:    personalStash,
			Combat:   combat,
			Actions:  runActions,
			Loot:     lootActions,
			Route:    routePlayback,
		}),
		Pathing:   nav,
		TownWalk:  townWalker,
		Loot:      lootFilter,
		Telemetry: runTelemetry,
	}

	if err := rt.verifyEnvironment(); err != nil {
		return nil, err
	}

	rt.Memory.Bind(proc)

	rt.verifyComponents()
	return rt, nil
}

func loadPickit(cfg *config.Config) (*loot.Pickit, error) {
	path := cfg.ResolvePath(cfg.Loot.PickitFile)
	pickit, err := loot.LoadPickit(path)
	if err != nil {
		return nil, fmt.Errorf("pickit config invalid: %w", err)
	}
	return pickit, nil
}

// CloseLog closes the runtime log file when file logging is active.
func (rt *Runtime) CloseLog() error {
	if rt == nil {
		return nil
	}
	var telemetryErr error
	if rt.Telemetry != nil {
		telemetryErr = rt.Telemetry.Close()
		rt.Telemetry = nil
	}
	var logErr error
	if rt.logFile != nil {
		logErr = rt.logFile.Close()
		rt.logFile = nil
	}
	return errors.Join(telemetryErr, logErr)
}

func (rt *Runtime) verifyEnvironment() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("unsupported OS %q: bot targets Windows/D2R", runtime.GOOS)
	}
	rt.Log.Info("runtime environment ok",
		"goos", runtime.GOOS,
		"goarch", runtime.GOARCH,
		"num_cpu", runtime.NumCPU(),
	)
	return nil
}

func (rt *Runtime) verifyComponents() {
	components := map[string]bool{
		"process": rt.Process.Ready(),
		"memory":  rt.Memory.Ready(),
		"world":   rt.World.Ready(),
		"input":   rt.Input.Ready(),
		"tasks":   rt.Tasks.Ready(),
		"pathing": rt.Pathing.Ready(),
		"loot":    rt.Loot.Ready(),
	}

	for name, ready := range components {
		rt.Log.Info("component ready", "name", name, "ready", ready)
	}
}

func (rt *Runtime) Run() error {
	rt.Log.Info("d2rbot started",
		"version", version.Version,
		"commit", version.Commit,
		"poll_interval_ms", rt.Config.Runtime.PollIntervalMs,
		"target_process", rt.Config.Process.ProcessName,
		"probe_enabled", rt.Options.Probe,
		"verbose", rt.Options.Verbose,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt.startShutdownSignals(ctx, cancel)

	defer func() {
		if err := rt.Process.Detach(); err != nil {
			rt.Log.Error("process detach failed", "error", err)
		}
	}()
	defer rt.Input.Unbind()

	hotkeyEvents, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	state := &runState{}

	for {
		select {
		case <-ctx.Done():
			rt.Log.Info("shutting down")
			return nil
		case ev := <-hotkeyEvents:
			rt.handleHotkeyEvent(ev, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				rt.Log.Error("run loop stopped", "error", err)
				return err
			}
		}
	}
}

func (rt *Runtime) startShutdownSignals(ctx context.Context, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			rt.Log.Info("shutdown signal received")
			rt.Input.Stop("signal")
			cancel()
		case <-ctx.Done():
			signal.Stop(sigCh)
		}
	}()
}

func (rt *Runtime) startHotkeys(ctx context.Context) (<-chan input.HotkeyEvent, error) {
	hotkeyEvents := make(chan input.HotkeyEvent, 4)
	hotkeyReady := make(chan error, 1)
	rt.Input.ListenHotkeys(ctx, hotkeyEvents, hotkeyReady)

	if err := <-hotkeyReady; err != nil {
		return nil, fmt.Errorf("hotkey listener: %w", err)
	}
	return hotkeyEvents, nil
}

func (rt *Runtime) logProcessStateChange(prev, next process.State) {
	if prev == next {
		return
	}

	st := rt.Process.Status()
	switch next {
	case process.StateAttached:
		rt.Log.Info("process attached",
			"pid", st.PID,
			"process", st.Process,
			"module_base", fmt.Sprintf("0x%X", st.ModuleBase),
		)
	case process.StateLost:
		rt.Log.Info("process lost",
			"pid", st.PID,
			"process", st.Process,
		)
	case process.StateDetached:
		rt.Log.Info("process detached", "process", st.Process)
	}
}
