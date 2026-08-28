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
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type fakeQueueRunUnit struct {
	runID          string
	events         *[]string
	continuations  *[]bool
	result         SupervisorRunResult
	skills         SupervisorRunResult
	startLifecycle []telemetry.Event
	exitLifecycle  []telemetry.Event
}

func (u *fakeQueueRunUnit) StartOrVerifyGame(_ context.Context, active bool, emit queueLifecycleTelemetry) error {
	*u.events = append(*u.events, fmt.Sprintf("start:%s:%t", u.runID, active))
	for _, event := range u.startLifecycle {
		if err := emit(event); err != nil {
			return err
		}
	}
	return nil
}

func (u *fakeQueueRunUnit) VerifySameGame(context.Context) error {
	*u.events = append(*u.events, "verify:"+u.runID)
	return nil
}

func (u *fakeQueueRunUnit) VerifyProfileSkills(context.Context) SupervisorRunResult {
	*u.events = append(*u.events, "skills:"+u.runID)
	if u.skills.Disposition != "" || u.skills.Reason != "" {
		return u.skills
	}
	return SupervisorRunResult{}
}

func (u *fakeQueueRunUnit) RunToTown(_ context.Context, _ SupervisorRunRequest, sameGameContinuation bool) SupervisorRunResult {
	*u.events = append(*u.events, "run:"+u.runID)
	if u.continuations != nil {
		*u.continuations = append(*u.continuations, sameGameContinuation)
	}
	return u.result
}

func (u *fakeQueueRunUnit) ExitGame(_ context.Context, _ SupervisorRunResult, emit queueLifecycleTelemetry) error {
	*u.events = append(*u.events, "exit:"+u.runID)
	for _, event := range u.exitLifecycle {
		if err := emit(event); err != nil {
			return err
		}
	}
	return nil
}

func (u *fakeQueueRunUnit) Close() {
	*u.events = append(*u.events, "close:"+u.runID)
}

func newFakeRuntimeQueueRunner(events *[]string) *RuntimeQueueRunner {
	return &RuntimeQueueRunner{newUnit: func(runID string) (queueRunUnit, error) {
		return &fakeQueueRunUnit{runID: runID, events: events, result: SupervisorRunResult{Disposition: QueueRunAdvance, ExitAuthorization: ExitAuthorizationVerifiedRogueTown}}, nil
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
		{name: "explicit idle in game with town", state: SupervisorStateIdleInGame, runtime: inGame, want: true},
		{name: "idle in game on character screen", state: SupervisorStateIdleInGame, runtime: UIStatusSnapshot{ProcessState: "attached", WindowBound: true, WorldValid: true, WorldPhase: world.GamePhaseMenu.String()}, want: false},
		{name: "idle in game without world", state: SupervisorStateIdleInGame, want: false},
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

func TestQueueGameStartDetailIsGermanAndHidesRawCause(t *testing.T) {
	got := queueGameStartDetail(fmt.Errorf("session game expected in_game, got menu"))
	if got == "" || strings.Contains(got, "expected in_game") || strings.Contains(got, "got menu") {
		t.Fatalf("detail = %q", got)
	}
	if !isMissingActiveQueueGame(fmt.Errorf("session game expected in_game, got menu")) {
		t.Fatal("menu mismatch must fall back to character-screen start")
	}
	if isMissingActiveQueueGame(fmt.Errorf("session character mismatch: active=%q expected=%q", "Bones", "Hammer")) {
		t.Fatal("character mismatch must not fall back to selector")
	}
}

func TestQueueGameStartDetailReportsUnsupportedResolution(t *testing.T) {
	got := queueGameStartDetail(fmt.Errorf("start queue game: offline exit requires 1280x720, got 1920x1080"))
	if !strings.Contains(got, "1920 × 1080") || !strings.Contains(got, "1280 × 720") {
		t.Fatalf("detail = %q", got)
	}
	if strings.Contains(got, "offline exit") || strings.Contains(got, "requires 1280x720") || strings.Contains(got, "sicher gestartet") {
		t.Fatalf("raw or generic start copy leaked: %q", got)
	}
}

func TestConfiguredQueueCharacterSelectionReusesCatalogBoundSelector(t *testing.T) {
	cfg := &config.Config{}
	cfg.Session.Character = "mrbones"
	cfg.Session.Difficulty = "hell"
	catalog := CharacterCatalog{Revision: 7, Characters: []CharacterCatalogEntry{
		{Name: "MrBeek", AnchorPath: "beek.png", ExpectedClass: "warlock"},
		{Name: "MrBones", AnchorPath: "bones.png", ExpectedClass: "necromancer"},
		{Name: "MrHammer", AnchorPath: "hammer.png", ExpectedClass: "paladin"},
	}}

	selection, err := configuredQueueCharacterSelection(cfg, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if selection.Character != "MrBones" || selection.Difficulty != "hell" || selection.CatalogRevision != 7 ||
		selection.CharacterCount != 3 || selection.AnchorPath != "bones.png" || selection.ExpectedClass != "necromancer" {
		t.Fatalf("selection = %+v", selection)
	}
}

func TestConfiguredQueueCharacterSelectionRejectsMissingSave(t *testing.T) {
	cfg := &config.Config{}
	cfg.Session.Character = "MrBones"
	_, err := configuredQueueCharacterSelection(cfg, CharacterCatalog{Revision: 1, Characters: []CharacterCatalogEntry{{Name: "MrHammer"}}})
	if err == nil || !strings.Contains(err.Error(), "missing from the offline save catalog") {
		t.Fatalf("missing save error = %v", err)
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
	if err := runner.BeginQueue(true); err != nil {
		t.Fatal(err)
	}
	request := SupervisorRunRequest{DefinitionID: "countess"}
	if err := runner.StartGame(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if result := runner.Run(context.Background(), request); result.Disposition != QueueRunAdvance || result.ExitAuthorization != ExitAuthorizationVerifiedRogueTown {
		t.Fatalf("run result = %+v", result)
	}
	if reflect.DeepEqual(events, []string{"start:countess:true", "verify:countess", "run:countess", "exit:countess"}) {
		t.Fatal("RunToTown performed an exit")
	}
	if err := runner.ExitGame(context.Background(), request, SupervisorRunResult{Reason: "queue_wrap", ExitAuthorization: ExitAuthorizationVerifiedRogueTown}); err != nil {
		t.Fatal(err)
	}
	if err := runner.ExitGame(context.Background(), request, SupervisorRunResult{Reason: "duplicate", ExitAuthorization: ExitAuthorizationVerifiedRogueTown}); err != nil {
		t.Fatal(err)
	}
	want := []string{"start:countess:true", "verify:countess", "skills:countess", "run:countess", "exit:countess"}
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

func TestQueueRunTelemetryEventPreservesRecoveryContext(t *testing.T) {
	event := queueRunTelemetryEvent(SupervisorRunResult{
		Disposition: QueueRunRetryCurrent, Reason: queueReasonRetryReturnFailed,
		OriginalReason: "route_threat_out_of_range", RecoveryReason: "town_portal_enter_failed",
		ExitAuthorization: ExitAuthorizationMemoryGatedCurrentArea,
	}, SupervisorRunRequest{ExecutionID: "run-1", DefinitionID: "countess", QueueIndex: 2, Cycle: 1, Retry: 3, GameID: "game-4"})
	if event.Event != telemetry.RunAborted || event.OriginalReason != "route_threat_out_of_range" ||
		event.RecoveryReason != "town_portal_enter_failed" || event.ExitAuthorization != string(ExitAuthorizationMemoryGatedCurrentArea) || event.Retry != 3 {
		t.Fatalf("event = %+v", event)
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
	if success.Disposition != QueueRunRetryCurrent || success.Reason != "route_threat_out_of_range" || success.ExitAuthorization != ExitAuthorizationVerifiedRogueTown {
		t.Fatalf("successful controlled retry = %+v", success)
	}

	failed, err := controlledRetryResult(context.Background(), "route_threat_out_of_range", func(context.Context) error {
		return errors.New("portal unavailable")
	})
	if err == nil || failed.Disposition != QueueRunRetryCurrent || failed.Reason != queueReasonRetryReturnFailed || failed.ExitAuthorization != ExitAuthorizationMemoryGatedCurrentArea ||
		failed.OriginalReason != "route_threat_out_of_range" || failed.RecoveryReason != "retry_return_execution_failed" {
		t.Fatalf("failed controlled retry = %+v, err=%v", failed, err)
	}

	missing, err := controlledRetryResult(context.Background(), "route_threat_out_of_range", nil)
	if err == nil || missing.Disposition != QueueRunRetryCurrent || missing.Reason != queueReasonRetryReturnFailed || missing.ExitAuthorization != ExitAuthorizationMemoryGatedCurrentArea ||
		missing.OriginalReason != "route_threat_out_of_range" || missing.RecoveryReason != "retry_return_not_wired" {
		t.Fatalf("missing controlled retry = %+v, err=%v", missing, err)
	}

	typed, err := controlledRetryResult(context.Background(), "mercenary_died_during_run", func(context.Context) error {
		return &retryReturnFailure{Reason: "town_portal_not_found", Err: errors.New("portal did not appear")}
	})
	if err == nil || typed.OriginalReason != "mercenary_died_during_run" || typed.RecoveryReason != "town_portal_not_found" {
		t.Fatalf("typed controlled retry = %+v, err=%v", typed, err)
	}
}

func TestRestartableFailureSkipsPortalReturnWhenAlreadyInAct1Town(t *testing.T) {
	t.Parallel()

	called := false
	town := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment)}
	got, err := restartableFailureResult(context.Background(), "stash_approach_failed", town, func(context.Context) error {
		called = true
		return errors.New("must not cast a town portal from Act 1")
	})
	if err != nil || called {
		t.Fatalf("Act-1 retry recovered through portal return: result=%+v err=%v called=%t", got, err, called)
	}
	if got.Disposition != QueueRunRetryCurrent || got.Reason != "stash_approach_failed" || got.ExitAuthorization != ExitAuthorizationVerifiedRogueTown {
		t.Fatalf("Act-1 retry = %+v", got)
	}

	field := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.BlackMarsh)}
	fieldResult, err := restartableFailureResult(context.Background(), "hard_stuck", field, func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if fieldResult.Disposition != QueueRunRetryCurrent || fieldResult.ExitAuthorization != ExitAuthorizationVerifiedRogueTown {
		t.Fatalf("field retry = %+v", fieldResult)
	}
}

func TestRuntimeQueueRunnerStopsBeforeRunWhenSkillsMissing(t *testing.T) {
	var events []string
	runner := &RuntimeQueueRunner{newUnit: func(runID string) (queueRunUnit, error) {
		return &fakeQueueRunUnit{
			runID:  runID,
			events: &events,
			result: SupervisorRunResult{Disposition: QueueRunAdvance, ExitAuthorization: ExitAuthorizationVerifiedRogueTown},
			skills: SupervisorRunResult{
				Disposition:       QueueRunStop,
				Reason:            reasonProfileRequiredSkillsMissing,
				Detail:            "MrBones fehlen für Knochen-Speer: Teleport. Die Queue wurde beendet.",
				ExitAuthorization: ExitAuthorizationMemoryGatedCurrentArea,
			},
		}, nil
	}}
	if err := runner.BeginQueue(false); err != nil {
		t.Fatal(err)
	}
	req := SupervisorRunRequest{DefinitionID: "countess", QueueIndex: 0}
	if err := runner.StartGame(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	got := runner.Run(context.Background(), req)
	if got.Disposition != QueueRunStop || got.Reason != reasonProfileRequiredSkillsMissing || got.ExitAuthorization != ExitAuthorizationMemoryGatedCurrentArea {
		t.Fatalf("gate result = %+v", got)
	}
	if !strings.Contains(got.Detail, "Teleport") {
		t.Fatalf("detail = %q", got.Detail)
	}
	want := []string{"start:countess:false", "verify:countess", "skills:countess"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v (RunToTown must not run)", events, want)
	}
	if runner.skillsVerified {
		t.Fatal("failed gate must not mark skillsVerified")
	}
}

func TestBeginQueueFreezesLoadoutAcrossStoreMutation(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	replacement := cloneOperatorSettings(assigned.Settings)
	value := replacement.Characters["mrbones"]
	value.ProfileBindings = map[string]OperatorProfileBindings{"necro_bone_spear": necroBoneSpearBindingsFixture()}
	value.InventoryLock = &OperatorInventoryLock{Grid: sampleInventoryGrid(false)}
	replacement.Characters["mrbones"] = value
	updated, err := store.Update(assigned.Settings.Revision, replacement)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.Session.Character = "MrBones"
	resolver := NewCharacterLoadoutResolver(store, store.profiles, updated.Settings.Input)
	runner := &RuntimeQueueRunner{config: cfg}
	runner.SetLoadoutResolver(resolver)
	if err = runner.BeginQueue(false); err != nil {
		t.Fatal(err)
	}
	if runner.frozenLoadout == nil || runner.frozenLoadout.Revision != updated.Settings.Revision || runner.frozenLoadout.ProfileID != "necro_bone_spear" {
		t.Fatalf("frozen=%+v", runner.frozenLoadout)
	}
	frozenRevision := runner.frozenLoadout.Revision
	boneCast, err := runner.frozenLoadout.Bindings.Resolve(memory.SkillBoneSpear)
	if err != nil || boneCast.SelectKey != "f8" {
		t.Fatalf("frozen bone spear=%+v err=%v", boneCast, err)
	}

	mutated := cloneOperatorSettings(updated.Settings)
	mutatedValue := mutated.Characters["mrbones"]
	mutatedBindings := mutatedValue.ProfileBindings["necro_bone_spear"]
	mutatedBindings.Skills["bone_spear"] = "f4"
	mutatedValue.ProfileBindings["necro_bone_spear"] = mutatedBindings
	mutatedValue.InventoryLock.Grid[0][0] = 0
	mutated.Characters["mrbones"] = mutatedValue
	after, err := store.Update(updated.Settings.Revision, mutated)
	if err != nil {
		t.Fatal(err)
	}
	if after.Settings.Revision == frozenRevision {
		t.Fatal("store revision did not advance")
	}
	stillFrozen, err := runner.frozenLoadout.Bindings.Resolve(memory.SkillBoneSpear)
	if err != nil || stillFrozen.SelectKey != "f8" {
		t.Fatalf("frozen loadout mutated in place=%+v err=%v", stillFrozen, err)
	}
	if runner.frozenLoadout.InventoryGrid[0][0] != 1 {
		t.Fatalf("frozen inventory mutated in place=%v", runner.frozenLoadout.InventoryGrid[0][0])
	}

	runner.frozenLoadout.InventoryGrid[0][0] = 99
	fresh, err := resolver.Resolve("MrBones")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.InventoryGrid[0][0] == 99 {
		t.Fatal("frozen inventory aliased into resolver snapshot")
	}
	if fresh.Revision != after.Settings.Revision {
		t.Fatalf("fresh revision=%d want %d", fresh.Revision, after.Settings.Revision)
	}

	if err = runner.BeginQueue(false); err != nil {
		t.Fatal(err)
	}
	if runner.frozenLoadout.Revision != after.Settings.Revision {
		t.Fatalf("next BeginQueue revision=%d want %d", runner.frozenLoadout.Revision, after.Settings.Revision)
	}
	nextCast, err := runner.frozenLoadout.Bindings.Resolve(memory.SkillBoneSpear)
	if err != nil || nextCast.SelectKey != "f4" {
		t.Fatalf("next freeze cast=%+v err=%v", nextCast, err)
	}
}

func TestBeginQueueRejectsMissingLoadoutResolver(t *testing.T) {
	runner := &RuntimeQueueRunner{config: &config.Config{Session: config.SessionConfig{Character: "MrBones"}}}
	err := runner.BeginQueue(false)
	if err == nil || !strings.Contains(err.Error(), "loadout resolver is unavailable") {
		t.Fatalf("BeginQueue error=%v", err)
	}
	if runner.frozenLoadout != nil {
		t.Fatal("missing resolver must not leave a frozen loadout")
	}
}

func TestRuntimeQueueRunnerClearsSkillsVerifiedOnBeginQueue(t *testing.T) {
	var events []string
	runner := newFakeRuntimeQueueRunner(&events)
	if err := runner.BeginQueue(false); err != nil {
		t.Fatal(err)
	}
	req := SupervisorRunRequest{DefinitionID: "countess"}
	if err := runner.StartGame(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	runner.Run(context.Background(), req)
	if !runner.skillsVerified {
		t.Fatal("expected skillsVerified after successful gate")
	}
	if err := runner.BeginQueue(false); err != nil {
		t.Fatal(err)
	}
	if runner.skillsVerified {
		t.Fatal("BeginQueue must clear skillsVerified for a new queue session")
	}
}

func TestRuntimeQueueRunnerKeepsGameAcrossFreshRunUnits(t *testing.T) {
	var events []string
	var continuations []bool
	runner := newFakeRuntimeQueueRunner(&events)
	runner.newUnit = func(runID string) (queueRunUnit, error) {
		return &fakeQueueRunUnit{runID: runID, events: &events, continuations: &continuations, result: SupervisorRunResult{Disposition: QueueRunAdvance, ExitAuthorization: ExitAuthorizationVerifiedRogueTown}}, nil
	}
	if err := runner.BeginQueue(false); err != nil {
		t.Fatal(err)
	}
	countess := SupervisorRunRequest{DefinitionID: "countess", QueueIndex: 0}
	mephisto := SupervisorRunRequest{DefinitionID: "mephisto", QueueIndex: 1}
	if err := runner.StartGame(context.Background(), countess); err != nil {
		t.Fatal(err)
	}
	runner.Run(context.Background(), countess)
	runner.Run(context.Background(), mephisto)
	want := []string{"start:countess:false", "verify:countess", "skills:countess", "run:countess", "close:countess", "verify:mephisto", "run:mephisto"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	if !reflect.DeepEqual(continuations, []bool{false, true}) {
		t.Fatalf("same-game continuations = %v, want [false true]", continuations)
	}
}

func TestRuntimeQueueUnitRearmsReadinessOnlyForNewGame(t *testing.T) {
	runtime := &Runtime{}
	unit := &runtimeQueueUnit{runtime: runtime}

	unit.rearmRunReadinessForNewGame(true)
	if runtime.runReadinessPending {
		t.Fatal("adopted game unexpectedly rearmed run readiness")
	}
	unit.rearmRunReadinessForNewGame(false)
	if !runtime.runReadinessPending {
		t.Fatal("verified new game did not rearm run readiness")
	}
}

func TestRuntimeQueueRunnerRevalidatesPausedGame(t *testing.T) {
	var events []string
	runner := newFakeRuntimeQueueRunner(&events)
	if err := runner.BeginQueue(true); err != nil {
		t.Fatal(err)
	}
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
	store, _ := newOperatorSettingsTestStore(t)
	initial, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Profiles = store.profiles
	runner := newFakeRuntimeQueueRunner(&events)
	runner.config = cfg
	runner.SetLoadoutResolver(NewCharacterLoadoutResolver(store, cfg.Profiles, assigned.Settings.Input))
	if err = runner.BeginQueue(true); err != nil {
		t.Fatal(err)
	}
	runner.persistEvents = true
	countess := SupervisorRunRequest{DefinitionID: "countess", ExecutionID: "run-001", GameID: "game-001"}
	mephisto := SupervisorRunRequest{DefinitionID: "mephisto", ExecutionID: "run-002", GameID: "game-001"}
	if err = runner.StartGame(context.Background(), countess); err != nil {
		t.Fatal(err)
	}
	runner.Run(context.Background(), countess)
	runner.Run(context.Background(), mephisto)
	if err = runner.ExitGame(context.Background(), mephisto, SupervisorRunResult{Reason: "queue_wrap", ExitAuthorization: ExitAuthorizationVerifiedRogueTown}); err != nil {
		t.Fatal(err)
	}
	if err = runner.FinishQueue(SupervisorRunResult{Disposition: QueueRunStop, Reason: string(QueueReasonRunBudgetExhausted), ExitAuthorization: ExitAuthorizationNone}, SupervisorStateIdle); err != nil {
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

func TestRuntimeQueueRunnerCorrelatesRecoveryLifecycleTelemetry(t *testing.T) {
	directory := t.TempDir()
	cfg := &config.Config{
		Telemetry: config.TelemetryConfig{Directory: directory},
		Session:   config.SessionConfig{Character: "MrHammer", Difficulty: "nightmare", MaxRuns: 2, MaxDurationMs: 60000},
		Memory:    config.MemoryConfig{GameVersion: "3.2.92777"},
	}
	var calls []string
	unit := &fakeQueueRunUnit{
		runID: "mephisto", events: &calls,
		startLifecycle: []telemetry.Event{
			{Event: telemetry.StartTownNormalizationStarted, Act: "act3", AreaID: uint32(world.KurastDocks), RouteFile: "spawn-waypoint.yaml"},
			{Event: telemetry.StartTownNormalizationCompleted, Act: "act3", AreaID: uint32(world.KurastDocks), TargetAreaID: uint32(world.RogueEncampment), Confirmed: true},
		},
		exitLifecycle: []telemetry.Event{
			{Event: telemetry.DirectExitStarted, AreaID: uint32(world.KurastDocks), OriginalReason: "route_transition_failed", RecoveryReason: "town_portal_not_found"},
			{Event: telemetry.DirectExitCompleted, AreaID: uint32(world.KurastDocks), Confirmed: true},
		},
	}
	runner := &RuntimeQueueRunner{
		config: cfg, persistEvents: true,
		newUnit: func(string) (queueRunUnit, error) { return unit, nil },
	}
	request := SupervisorRunRequest{DefinitionID: "mephisto", ExecutionID: "run-recovery", GameID: "game-recovery", QueueIndex: 1, Cycle: 2, Retry: 3}
	if err := runner.StartGame(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	result := SupervisorRunResult{
		Disposition: QueueRunRetryCurrent, Reason: "retry_current", OriginalReason: "route_transition_failed",
		RecoveryReason: "town_portal_not_found", ExitAuthorization: ExitAuthorizationMemoryGatedCurrentArea,
	}
	if err := runner.ExitGame(context.Background(), request, result); err != nil {
		t.Fatal(err)
	}
	runner.CloseQueue()

	files, err := filepath.Glob(filepath.Join(directory, "session-*.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatalf("session files=%v err=%v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []telemetry.EventName{
		telemetry.StartTownNormalizationStarted, telemetry.StartTownNormalizationCompleted,
		telemetry.DirectExitStarted, telemetry.DirectExitCompleted,
	} {
		if !strings.Contains(string(data), `"event":"`+string(name)+`"`) {
			t.Fatalf("event %q missing from %s", name, data)
		}
	}
	if !strings.Contains(string(data), `"run_id":"run-recovery"`) || !strings.Contains(string(data), `"retry":3`) ||
		!strings.Contains(string(data), `"queue_index":1`) || !strings.Contains(string(data), `"queue_cycle":2`) {
		t.Fatalf("recovery correlation missing from %s", data)
	}
}
