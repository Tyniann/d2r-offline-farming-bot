package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type fakeQueueRunUnit struct {
	runID         string
	events        *[]string
	continuations *[]bool
	result        SupervisorRunResult
}

func (u *fakeQueueRunUnit) StartOrVerifyGame(_ context.Context, active bool) error {
	*u.events = append(*u.events, fmt.Sprintf("start:%s:%t", u.runID, active))
	return nil
}

func (u *fakeQueueRunUnit) VerifySameGame(context.Context) error {
	*u.events = append(*u.events, "verify:"+u.runID)
	return nil
}

func (u *fakeQueueRunUnit) RunToTown(_ context.Context, _ SupervisorRunRequest, sameGameContinuation bool) SupervisorRunResult {
	*u.events = append(*u.events, "run:"+u.runID)
	if u.continuations != nil {
		*u.continuations = append(*u.continuations, sameGameContinuation)
	}
	return u.result
}

func (u *fakeQueueRunUnit) ExitGame(context.Context) error {
	*u.events = append(*u.events, "exit:"+u.runID)
	return nil
}

func (u *fakeQueueRunUnit) Close() {
	*u.events = append(*u.events, "close:"+u.runID)
}

func newFakeRuntimeQueueRunner(events *[]string) *RuntimeQueueRunner {
	return &RuntimeQueueRunner{newUnit: func(runID string) (queueRunUnit, error) {
		return &fakeQueueRunUnit{runID: runID, events: events, result: SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}}, nil
	}}
}

func TestCanAdoptQueueGameUsesConfirmedPassiveRuntime(t *testing.T) {
	inGame := UIStatusSnapshot{
		ProcessState: "attached",
		WindowBound:  true,
		WorldValid:   true,
		WorldPhase:   world.GamePhaseInGame.String(),
		AreaID:       uint32(world.RogueEncampment),
	}
	tests := []struct {
		name    string
		state   SupervisorState
		runtime UIStatusSnapshot
		want    bool
	}{
		{name: "explicit idle in game", state: SupervisorStateIdleInGame, want: true},
		{name: "passive monitor confirmed open game", state: SupervisorStateIdle, runtime: inGame, want: true},
		{name: "character screen", state: SupervisorStateIdle, runtime: UIStatusSnapshot{ProcessState: "attached", WindowBound: true, WorldValid: true, WorldPhase: world.GamePhaseMenu.String()}, want: false},
		{name: "wrong start area", state: SupervisorStateIdle, runtime: UIStatusSnapshot{ProcessState: "attached", WindowBound: true, WorldValid: true, WorldPhase: world.GamePhaseInGame.String(), AreaID: uint32(world.BlackMarsh)}, want: false},
		{name: "detached", state: SupervisorStateIdle, runtime: UIStatusSnapshot{}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanAdoptQueueGame(test.state, test.runtime); got != test.want {
				t.Fatalf("CanAdoptQueueGame() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestFocusVerifiedQueueGameReusesGuardedInputFocus(t *testing.T) {
	controller := &mockInput{}
	if err := focusVerifiedQueueGame(controller); err != nil {
		t.Fatal(err)
	}
	if controller.focusCalls != 1 {
		t.Fatalf("focus calls = %d, want 1", controller.focusCalls)
	}
	controller.focusErr = errors.New("foreground denied")
	if err := focusVerifiedQueueGame(controller); err == nil || !strings.Contains(err.Error(), "focus verified queue game") {
		t.Fatalf("focus failure = %v", err)
	}
}

func TestRuntimeQueueRunnerSeparatesRunFromExit(t *testing.T) {
	var events []string
	runner := newFakeRuntimeQueueRunner(&events)
	runner.BeginQueue(true)
	request := SupervisorRunRequest{DefinitionID: "countess"}
	if err := runner.StartGame(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if result := runner.Run(context.Background(), request); result.Disposition != QueueRunAdvance || !result.SafeToExit {
		t.Fatalf("run result = %+v", result)
	}
	if reflect.DeepEqual(events, []string{"start:countess:true", "verify:countess", "run:countess", "exit:countess"}) {
		t.Fatal("RunToTown performed an exit")
	}
	if err := runner.ExitGame(context.Background(), request, "queue_wrap"); err != nil {
		t.Fatal(err)
	}
	if err := runner.ExitGame(context.Background(), request, "duplicate"); err != nil {
		t.Fatal(err)
	}
	want := []string{"start:countess:true", "verify:countess", "run:countess", "exit:countess"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestQueueRunTerminalEventMatchesDisposition(t *testing.T) {
	tests := []struct {
		name        string
		disposition QueueRunDisposition
		reason      string
		want        telemetry.EventName
	}{
		{name: "advance", disposition: QueueRunAdvance, want: telemetry.RunCompleted},
		{name: "retry", disposition: QueueRunRetryCurrent, want: telemetry.RunAborted},
		{name: "stop failed", disposition: QueueRunStop, reason: "town_portal_enter_failed", want: telemetry.RunFailed},
		{name: "emergency stop", disposition: QueueRunStop, reason: string(SupervisorReasonEmergencyStopRequested), want: telemetry.RunAborted},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := queueRunTerminalEvent(SupervisorRunResult{Disposition: test.disposition, Reason: test.reason}); got != test.want {
				t.Fatalf("terminal = %q, want %q", got, test.want)
			}
		})
	}
}

func TestControlledRetryResultRequiresVerifiedTownReturn(t *testing.T) {
	t.Parallel()

	success, err := controlledRetryResult(context.Background(), "route_threat_out_of_range", func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if success.Disposition != QueueRunRetryCurrent || success.Reason != "route_threat_out_of_range" || !success.SafeToExit {
		t.Fatalf("successful controlled retry = %+v", success)
	}

	failed, err := controlledRetryResult(context.Background(), "route_threat_out_of_range", func(context.Context) error {
		return errors.New("portal unavailable")
	})
	if err == nil || failed.Disposition != QueueRunStop || failed.Reason != queueReasonRetryReturnFailed || failed.SafeToExit {
		t.Fatalf("failed controlled retry = %+v, err=%v", failed, err)
	}

	missing, err := controlledRetryResult(context.Background(), "route_threat_out_of_range", nil)
	if err == nil || missing.Disposition != QueueRunStop || missing.Reason != queueReasonRetryReturnFailed || missing.SafeToExit {
		t.Fatalf("missing controlled retry = %+v, err=%v", missing, err)
	}
}

func TestRuntimeQueueRunnerKeepsGameAcrossFreshRunUnits(t *testing.T) {
	var events []string
	var continuations []bool
	runner := newFakeRuntimeQueueRunner(&events)
	runner.newUnit = func(runID string) (queueRunUnit, error) {
		return &fakeQueueRunUnit{runID: runID, events: &events, continuations: &continuations, result: SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}}, nil
	}
	runner.BeginQueue(false)
	countess := SupervisorRunRequest{DefinitionID: "countess", QueueIndex: 0}
	mephisto := SupervisorRunRequest{DefinitionID: "mephisto", QueueIndex: 1}
	if err := runner.StartGame(context.Background(), countess); err != nil {
		t.Fatal(err)
	}
	runner.Run(context.Background(), countess)
	runner.Run(context.Background(), mephisto)
	want := []string{"start:countess:false", "verify:countess", "run:countess", "close:countess", "verify:mephisto", "run:mephisto"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	if !reflect.DeepEqual(continuations, []bool{false, true}) {
		t.Fatalf("same-game continuations = %v, want [false true]", continuations)
	}
}

func TestRuntimeQueueRunnerRevalidatesPausedGame(t *testing.T) {
	var events []string
	runner := newFakeRuntimeQueueRunner(&events)
	runner.BeginQueue(true)
	countess := SupervisorRunRequest{DefinitionID: "countess"}
	mephisto := SupervisorRunRequest{DefinitionID: "mephisto"}
	if err := runner.StartGame(context.Background(), countess); err != nil {
		t.Fatal(err)
	}
	if err := runner.RevalidateGame(context.Background(), mephisto); err != nil {
		t.Fatal(err)
	}
	want := []string{"start:countess:true", "close:countess", "verify:mephisto"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestRuntimeQueueRunnerRoutesPauseAfterRun(t *testing.T) {
	runner := &RuntimeQueueRunner{}
	calls := 0
	runner.SetPauseAfterRunHandler(func() error {
		calls++
		return nil
	})
	if err := runner.requestPauseAfterRun(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("pause-after-run calls = %d, want 1", calls)
	}
}

func TestRuntimeQueueRunnerRoutesStopAfterRun(t *testing.T) {
	runner := &RuntimeQueueRunner{}
	calls := 0
	runner.SetStopAfterRunHandler(func() error {
		calls++
		return nil
	})
	if err := runner.requestStopAfterRun(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("stop-after-run calls = %d, want 1", calls)
	}
}

func TestRuntimeQueueRunnerPersistsGameAndRunBoundaries(t *testing.T) {
	var events []string
	cfg := &config.Config{Telemetry: config.TelemetryConfig{Directory: t.TempDir()}, Session: config.SessionConfig{Character: "MrBones", Difficulty: "nightmare", MaxRuns: 4, MaxDurationMs: 60000}, Memory: config.MemoryConfig{GameVersion: "3.2.92777"}}
	runner := newFakeRuntimeQueueRunner(&events)
	runner.config = cfg
	runner.persistEvents = true
	runner.BeginQueue(true)
	countess := SupervisorRunRequest{DefinitionID: "countess", ExecutionID: "run-001", GameID: "game-001"}
	mephisto := SupervisorRunRequest{DefinitionID: "mephisto", ExecutionID: "run-002", GameID: "game-001"}
	if err := runner.StartGame(context.Background(), countess); err != nil {
		t.Fatal(err)
	}
	runner.Run(context.Background(), countess)
	runner.Run(context.Background(), mephisto)
	if err := runner.ExitGame(context.Background(), mephisto, "queue_wrap"); err != nil {
		t.Fatal(err)
	}
	if err := runner.FinishQueue(SupervisorRunResult{Disposition: QueueRunStop, Reason: string(QueueReasonRunBudgetExhausted)}, SupervisorStateIdle); err != nil {
		t.Fatal(err)
	}
	runner.CloseQueue()
	files, err := filepath.Glob(filepath.Join(cfg.Telemetry.Directory, "session-*.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatalf("session files=%v err=%v", files, err)
	}
	file, err := os.Open(files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var persisted []telemetry.Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event telemetry.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		persisted = append(persisted, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	got := make([]telemetry.EventName, len(persisted))
	for i, event := range persisted {
		got[i] = event.Event
	}
	want := []telemetry.EventName{telemetry.SessionStarted, telemetry.GameStarted, telemetry.RunStarted, telemetry.RunCompleted, telemetry.RunStarted, telemetry.RunCompleted, telemetry.GameExited, telemetry.SessionCompleted}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("persisted events = %v, want %v", got, want)
	}
	if persisted[2].GameID != persisted[4].GameID || persisted[2].RunID == persisted[4].RunID {
		t.Fatalf("run correlation = %+v / %+v", persisted[2], persisted[4])
	}
	for _, event := range persisted {
		if event.SchemaVersion != telemetry.HistorySchemaVersion || event.Stream != telemetry.HistoryStreamSession || event.Mode != telemetry.HistoryModeProductiveFarming || event.SessionID == "" || event.Character != "MrBones" || event.Difficulty != "nightmare" || event.GameVersion != "3.2.92777" {
			t.Fatalf("incomplete schema-3 session event: %+v", event)
		}
	}
	reader, readerErr := telemetry.NewHistoryReader(cfg.Telemetry.Directory)
	if readerErr != nil {
		t.Fatal(readerErr)
	}
	if _, err := reader.Read(filepath.Base(files[0])); err != nil {
		t.Fatalf("persisted queue session violates history reader contract: %v", err)
	}
	if persisted[1].RunID != "" || persisted[1].Run != "" || persisted[6].RunID != "" || persisted[6].Run != "" {
		t.Fatalf("game lifecycle leaked run context: started=%+v exited=%+v", persisted[1], persisted[6])
	}
}
