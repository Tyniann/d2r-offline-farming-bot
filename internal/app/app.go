package app

import (
	"context"
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
	Config *config.Config
	Log    *slog.Logger

	Process *process.Service
	Memory  *memory.Reader
	World   *world.Model
	Input   *input.Controller
	Tasks   *tasks.Runner
	Pathing *pathing.Navigator
	Loot    *loot.Filter
}

func New(cfg *config.Config) (*Runtime, error) {
	log := config.NewLogger(cfg.App.LogLevel)
	log = log.With("app", cfg.App.Name)

	rt := &Runtime{
		Config:  cfg,
		Log:     log,
		Process: process.New(log, cfg.Process.ProcessName),
		Memory:  memory.NewReader(log),
		World:   world.NewModel(log),
		Input:   input.NewController(log),
		Tasks:   tasks.NewRunner(log),
		Pathing: pathing.NewNavigator(log),
		Loot:    loot.NewFilter(log),
	}

	if err := rt.verifyEnvironment(); err != nil {
		return nil, err
	}

	rt.Memory.Bind(rt.Process)

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

	attached := false
	waitingLogged := false
	var lastFatalErr string
	var lastLoggedState process.State

	for {
		select {
		case <-ctx.Done():
			rt.Log.Info("shutting down")
			return nil
		case <-ticker.C:
			if !attached {
				if err := rt.Process.Attach(ctx); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					if process.IsFatal(err) {
						errMsg := err.Error()
						if errMsg != lastFatalErr {
							rt.Log.Error("process attach failed", "error", err)
							lastFatalErr = errMsg
						}
					} else if !waitingLogged {
						rt.Log.Info("waiting for target process",
							"process", rt.Config.Process.ProcessName,
						)
						waitingLogged = true
					}
					continue
				}
				attached = true
				waitingLogged = false
				lastFatalErr = ""
				rt.logProcessStateChange(lastLoggedState, process.StateAttached)
				lastLoggedState = process.StateAttached
			}

			st := rt.Process.Poll()
			if st.State == process.StateLost {
				attached = false
				rt.logProcessStateChange(lastLoggedState, process.StateLost)
				lastLoggedState = process.StateLost
			}
		}
	}
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
