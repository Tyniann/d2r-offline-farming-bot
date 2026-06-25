package app

import (
	"fmt"
	"log/slog"
	"runtime"

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

	Process  *process.Service
	Memory   *memory.Reader
	World    *world.Model
	Input    *input.Controller
	Tasks    *tasks.Runner
	Pathing  *pathing.Navigator
	Loot     *loot.Filter
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
	rt.Log.Info("d2rbot scaffold started",
		"poll_interval_ms", rt.Config.Runtime.PollIntervalMs,
		"target_process", rt.Config.Process.ProcessName,
	)
	rt.Log.Info("no game automation yet — scaffold only")
	return nil
}
