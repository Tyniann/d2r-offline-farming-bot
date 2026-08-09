package app

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

// RunConfiguredQueue executes the YAML-authoritative CLI queue through the
// same supervisor, game lifecycle, run executor, and exit owner as the UI.
// CLI startup begins at the prepared offline character screen and therefore
// never adopts an already active game implicitly.
func RunConfiguredQueue(cfg *config.Config, resolver *CharacterLoadoutResolver) error {
	plan, err := configuredFarmQueuePlan(cfg)
	if err != nil {
		return err
	}
	runner, err := NewRuntimeQueueRunner(cfg, nil)
	if err != nil {
		return err
	}
	runner.SetLoadoutResolver(resolver)
	if err := runner.BeginQueue(false); err != nil {
		return err
	}
	defer runner.CloseQueue()
	return runConfiguredQueue(context.Background(), plan, runner)
}

func configuredFarmQueuePlan(cfg *config.Config) (FarmQueuePlan, error) {
	if cfg == nil {
		return FarmQueuePlan{}, fmt.Errorf("configured farm queue requires config")
	}
	selection := FarmQueueValidationContext{Character: cfg.Session.Character, Difficulty: cfg.Session.Difficulty}
	return ValidateFarmQueue(cfg, FarmQueueValidationRequest{
		RunIDs: cfg.Session.Queue, Character: selection.Character, Difficulty: selection.Difficulty,
	}, selection)
}

func runConfiguredQueue(ctx context.Context, plan FarmQueuePlan, runner FarmQueueLifecycleRunner) error {
	supervisor, err := NewSessionSupervisor(runner)
	if err != nil {
		return err
	}
	var stopSequence atomic.Uint64
	if runtimeRunner, ok := runner.(*RuntimeQueueRunner); ok {
		runtimeRunner.SetStopAfterRunHandler(func() error {
			snapshot := supervisor.Snapshot()
			_, stopErr := supervisor.StopAfterRun(SupervisorCommandMeta{
				CommandID: fmt.Sprintf("cli-stop-after-run-%d", stopSequence.Add(1)), ExpectedGeneration: snapshot.Generation,
			})
			return stopErr
		})
	}
	if _, err := supervisor.StartQueue(SupervisorCommandMeta{CommandID: "cli-queue-start", ExpectedGeneration: 0}, plan); err != nil {
		return err
	}
	if err := supervisor.Wait(ctx); err != nil {
		return err
	}
	snapshot := supervisor.Snapshot()
	if snapshot.State == SupervisorStateStoppedError {
		return fmt.Errorf("configured farm queue stopped: %s", snapshot.LastResult.Reason)
	}
	return nil
}
