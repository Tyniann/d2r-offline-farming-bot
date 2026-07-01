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
	"github.com/Tyniann/d2r-offline-farming-bot/internal/version"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// Runtime bundles initialized application components.
type Runtime struct {
	Config  *config.Config
	Options Options
	Log     *slog.Logger

	Process  processController
	Memory   *memory.Reader
	Probe    snapshotReader
	World    *world.Model
	Input    inputController
	Bindings configBindingSource
	Tasks    *tasks.Runner
	Pathing  *pathing.Navigator
	Loot     *loot.Filter
}

// New builds a Runtime from config and CLI/runtime options.
func New(cfg *config.Config, opts Options) (*Runtime, error) {
	logLevel := cfg.App.LogLevel
	if opts.Verbose {
		logLevel = "debug"
	}
	log := config.NewLogger(logLevel)
	log = log.With("app", cfg.App.Name)

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
	nav := pathing.NewNavigator(log)
	inputCtrl, err := input.NewController(log, mapInputConfig(cfg.Input), mapSafetyConfig(cfg.Input))
	if err != nil {
		return nil, fmt.Errorf("input controller: %w", err)
	}
	bindings, err := newConfigBindingSource(cfg.Input.Bindings)
	if err != nil {
		return nil, fmt.Errorf("input bindings: %w", err)
	}

	probe := memory.NewProbeReader(mem, offsetSet)
	probe.SetScannedCachePath(cfg.ResolvePath(memory.DefaultScannedCacheFile))

	runName := resolveActiveRun(opts, cfg)
	if err := validateRunMode(runName, cfg, opts, log); err != nil {
		return nil, err
	}

	rt := &Runtime{
		Config:   cfg,
		Options:  opts,
		Log:      log,
		Process:  proc,
		Memory:   mem,
		Probe:    probe,
		World:    world.NewModel(log),
		Input:    inputCtrl,
		Bindings: bindings,
		Tasks: tasks.NewRunner(log, runName, mapRunConfig(cfg.Runs), tasks.Deps{
			Input:   inputCtrl,
			Pathing: nav,
		}),
		Pathing: nav,
		Loot:    loot.NewFilter(log),
	}

	if err := rt.verifyEnvironment(); err != nil {
		return nil, err
	}

	rt.Memory.Bind(proc)

	rt.verifyComponents()
	return rt, nil
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
