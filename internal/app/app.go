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
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// Runtime bundles initialized application components.
type Runtime struct {
	Config  *config.Config
	Options Options
	Log     *slog.Logger

	Process processController
	Memory  *memory.Reader
	Probe   snapshotReader
	World   *world.Model
	Input   *input.Controller
	Tasks   *tasks.Runner
	Pathing *pathing.Navigator
	Loot    *loot.Filter
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
	rt := &Runtime{
		Config:  cfg,
		Options: opts,
		Log:     log,
		Process: proc,
		Memory:  mem,
		Probe:   memory.NewProbeReader(mem, offsetSet),
		World:   world.NewModel(log),
		Input:   input.NewController(log),
		Tasks:   tasks.NewRunner(log),
		Pathing: pathing.NewNavigator(log),
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
		"poll_interval_ms", rt.Config.Runtime.PollIntervalMs,
		"target_process", rt.Config.Process.ProcessName,
		"probe_enabled", rt.Options.Probe,
		"verbose", rt.Options.Verbose,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	go func() {
		select {
		case <-sigCh:
			rt.Log.Info("shutdown signal received")
			cancel()
		case <-ctx.Done():
		}
	}()

	defer func() {
		if err := rt.Process.Detach(); err != nil {
			rt.Log.Error("process detach failed", "error", err)
		}
	}()

	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	state := &runState{}

	for {
		select {
		case <-ctx.Done():
			rt.Log.Info("shutting down")
			return nil
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

func (rt *Runtime) logProbeSnapshot(prev, snap memory.Snapshot, heartbeat, verbose bool) {
	if snap.Valid {
		level := slog.LevelInfo
		if verbose && isPositionOnlyProbeChange(prev, snap) {
			level = slog.LevelDebug
		}
		rt.Log.Log(context.Background(), level, "probe state",
			"stats_source", snap.StatsSource,
			"hp", snap.HP,
			"max_hp", snap.MaxHP,
			"mana", snap.Mana,
			"max_mana", snap.MaxMana,
			"area_id", snap.AreaID,
			"pos_x", snap.PosX,
			"pos_y", snap.PosY,
		)
		return
	}

	if heartbeat {
		rt.Log.Debug("probe unavailable", "reason", snap.Reason)
		return
	}
	rt.Log.Info("probe unavailable", "reason", snap.Reason)
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
