package app

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

var (
	errInputRequiredForRun  = errors.New("input.enabled must be true when a run is configured")
	errUnknownRun           = errors.New("unknown run name")
	errRunInputTestConflict = errors.New("--run and --input-test are mutually exclusive")
)

// resolveActiveRun returns the configured run name; CLI overrides YAML.
func resolveActiveRun(opts Options, cfg *config.Config) string {
	if opts.Run != "" {
		return opts.Run
	}
	return cfg.Runs.Active
}

func mapRunConfig(runs config.RunsConfig) tasks.RunConfig {
	return tasks.RunConfig{
		StepTimeout: time.Duration(runs.StepTimeoutMs) * time.Millisecond,
	}
}

// validateRunMode checks run prerequisites after resolving CLI vs config.
func validateRunMode(runName string, cfg *config.Config, opts Options, log *slog.Logger) error {
	if runName == "" {
		return nil
	}
	if opts.InputTest != "" {
		return errRunInputTestConflict
	}
	if !cfg.Input.Enabled {
		return errInputRequiredForRun
	}
	if !tasks.IsKnownRun(runName) {
		return fmt.Errorf("%w: %q", errUnknownRun, runName)
	}

	source := "config"
	if opts.Run != "" {
		source = "cli"
	}
	log.Info("task run configured", "run", runName, "source", source)
	return nil
}

func (rt *Runtime) shouldTickTasks(cur world.State) bool {
	if rt.Tasks.ConfiguredRun() == "" {
		return false
	}
	if rt.Tasks.Terminal() || rt.Tasks.WasReset() {
		return false
	}
	if !rt.Config.Input.Enabled {
		return false
	}
	if !cur.Valid || !rt.Input.Bound() {
		return false
	}
	if cur.Phase != world.GamePhaseInGame {
		return false
	}
	st := rt.Input.Status()
	return st.Enabled && !st.Paused && !st.Stopped
}
