package app

import (
	"errors"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type telemetryEmitterMock struct {
	events []telemetry.Event
	err    error
}

func (m *telemetryEmitterMock) Emit(event telemetry.Event) error {
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, event)
	return nil
}

func TestCountInventoryFullCandidatesCountsOnlyExplicitNoFit(t *testing.T) {
	report := loot.DecisionReport{Decisions: []loot.ItemDecision{
		{Stage: loot.DecisionStageFail, Kind: loot.DecisionKindFail, Reason: loot.DecisionReasonInventoryFull},
		{Stage: loot.DecisionStageFail, Kind: loot.DecisionKindFail, Reason: loot.DecisionReasonCapacityUnsafe},
		{Stage: loot.DecisionStagePickCandidate, Kind: loot.DecisionKindPickCandidate, Reason: loot.DecisionReasonPickitMatch},
	}}
	if got := countInventoryFullCandidates(report); got != 1 {
		t.Fatalf("countInventoryFullCandidates() = %d, want 1", got)
	}
}

func TestLootScanEmitsDropAndPickitTelemetry(t *testing.T) {
	pickit, err := loot.CompilePickitRules("test", []loot.PickitRuleSpec{{ProfileID: "keys", RuleID: "rune", Action: loot.ActionKeep, Expression: `[type] == "rune"`, ProfileRevision: 3, AssignmentRevision: 8}})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := loot.NewInventoryLock([][]int{{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}})
	if err != nil {
		t.Fatal(err)
	}
	emitter := &telemetryEmitterMock{}
	adapter := &lootActionsAdapter{log: config.NewLogger("error"), filter: loot.NewFilter(config.NewLogger("error"), lock, pickit), skipped: map[uint32]bool{}, telemetry: emitter}
	state := world.State{At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.TowerCellarLevel5), Items: []world.Item{{UnitID: 7, TxtFileNo: 627, Code: "r03", Name: "Tir Rune", Type: "rune", Quality: world.ItemQualityNormal, Location: world.ItemLocationGround, Width: 1, Height: 1}}}
	result := adapter.Scan(state)
	if result.TelemetryFailed || len(emitter.events) != 2 || emitter.events[0].Event != telemetry.DropSeen || emitter.events[1].Event != telemetry.PickitMatch {
		t.Fatalf("result=%+v events=%+v", result, emitter.events)
	}
	match := emitter.events[1]
	if emitter.events[0].Stage != telemetry.HistoryStageLoot || match.Stage != telemetry.HistoryStageLoot || emitter.events[0].ItemKey != "base:r03:normal" || match.ItemKey != emitter.events[0].ItemKey {
		t.Fatalf("item correlation drop=%+v match=%+v", emitter.events[0], match)
	}
	if emitter.events[0].PickitProfileID != match.PickitProfileID || emitter.events[0].PickitRuleID != match.PickitRuleID || emitter.events[0].PickitProfileRevision != match.PickitProfileRevision || emitter.events[0].PickitAssignmentRevision != match.PickitAssignmentRevision {
		t.Fatalf("pickit correlation drop=%+v match=%+v", emitter.events[0], match)
	}
	if match.PickitProfileID != "keys" || match.PickitRuleID != "rune" || match.PickitAction != "keep" || match.PickitProfileRevision != 3 || match.PickitAssignmentRevision != 8 {
		t.Fatalf("pickit telemetry = %+v", match)
	}
}

func TestRouteKeepScanUsesActivePickitAndIgnoresSellAndDistantKeep(t *testing.T) {
	pickit, err := loot.CompilePickitRules("test", []loot.PickitRuleSpec{
		{ProfileID: "player-choice", RuleID: "sell-ring", Action: loot.ActionSell, Expression: `[type] == "ring"`, ProfileRevision: 6, AssignmentRevision: 13},
		{ProfileID: "player-choice", RuleID: "keep-rune", Action: loot.ActionKeep, Expression: `[type] == "rune"`, ProfileRevision: 6, AssignmentRevision: 13},
	})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := loot.NewInventoryLock([][]int{{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}})
	if err != nil {
		t.Fatal(err)
	}
	adapter := &lootActionsAdapter{
		log: config.NewLogger("error"), filter: loot.NewFilter(config.NewLogger("error"), lock, pickit),
		profile: config.ProfileResourcesConfig{Mana: config.ResourceRuleConfig{BeltSlots: []int{2}}},
		skipped: map[uint32]bool{},
	}
	state := world.State{
		At: time.Now(), Valid: true, Phase: world.GamePhaseInGame,
		Area: world.LookupArea(world.ArcaneSanctuary), Player: world.Player{Position: world.Position{X: 100, Y: 100}},
		Items: []world.Item{
			{UnitID: 1, Code: "rin", Type: "ring", Location: world.ItemLocationGround, Width: 1, Height: 1, Position: world.Position{X: 101, Y: 100}},
			{UnitID: 2, Code: "r01", Type: "rune", Location: world.ItemLocationGround, Width: 1, Height: 1, Position: world.Position{X: 106, Y: 100}},
			{UnitID: 3, Code: "r02", Type: "rune", Location: world.ItemLocationGround, Width: 1, Height: 1, Position: world.Position{X: 140, Y: 100}},
			{UnitID: 4, Code: "mp5", Type: "mpot", Location: world.ItemLocationGround, Width: 1, Height: 1, Position: world.Position{X: 102, Y: 100}},
		},
	}
	result := adapter.ScanRouteKeep(state, 30)
	if !result.HasTarget || result.CandidateCount != 1 || result.NextTarget.UnitID != 2 ||
		result.NextTarget.PickitProfileID != "player-choice" || result.NextTarget.PickitRuleID != "keep-rune" ||
		result.NextTarget.PickitAction != "keep" || result.NextTarget.PickitProfileRevision != 6 ||
		result.NextTarget.PickitAssignmentRevision != 13 {
		t.Fatalf("route scan = %+v", result)
	}
}

func TestRouteSupplyScanFillsOnlyMissingAssignedColumnsWithExactPotions(t *testing.T) {
	pickit, err := loot.CompilePickitRules("test", nil)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := loot.NewInventoryLock([][]int{{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}})
	if err != nil {
		t.Fatal(err)
	}
	adapter := &lootActionsAdapter{
		log: config.NewLogger("error"), filter: loot.NewFilter(config.NewLogger("error"), lock, pickit),
		profile: config.ProfileResourcesConfig{
			Healing:      config.ResourceRuleConfig{BeltSlots: []int{1}},
			Mana:         config.ResourceRuleConfig{BeltSlots: []int{2, 3}},
			Rejuvenation: config.ResourceRuleConfig{BeltSlots: []int{4}},
		},
		skipped: map[uint32]bool{},
	}
	state := world.State{
		At: time.Now(), Valid: true, Phase: world.GamePhaseInGame,
		Area: world.LookupArea(world.MooMooFarm), Player: world.Player{Position: world.Position{X: 100, Y: 100}},
	}
	for _, gridX := range []int{0, 4, 8, 12} {
		state.Items = append(state.Items, world.Item{UnitID: uint32(100 + gridX), Code: "hp4", Type: "hpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: gridX})
	}
	for _, gridX := range []int{1, 5, 9, 13, 2, 6, 10} {
		state.Items = append(state.Items, world.Item{UnitID: uint32(200 + gridX), Code: "mp4", Type: "mpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: gridX})
	}
	for _, gridX := range []int{3, 7, 11, 15} {
		state.Items = append(state.Items, world.Item{UnitID: uint32(300 + gridX), Code: "rvl", Type: "rpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: gridX})
	}
	state.Items = append(state.Items,
		world.Item{UnitID: 1, TxtFileNo: 606, Code: "hp5", Type: "hpot", Name: "Super Healing Potion", Location: world.ItemLocationGround, Position: world.Position{X: 101, Y: 100}},
		world.Item{UnitID: 2, Code: "mp4", Type: "mpot", Name: "Greater Mana Potion", Location: world.ItemLocationGround, Position: world.Position{X: 102, Y: 100}},
		world.Item{UnitID: 3, TxtFileNo: 611, Code: "mp5", Type: "mpot", Name: "Super Mana Potion", Location: world.ItemLocationGround, Position: world.Position{X: 106, Y: 100}},
		world.Item{UnitID: 4, TxtFileNo: 531, Code: "rvl", Type: "rpot", Name: "Full Rejuvenation Potion", Location: world.ItemLocationGround, Position: world.Position{X: 103, Y: 100}},
	)

	result := adapter.ScanRouteKeep(state, 30)
	if !result.HasTarget || result.CandidateCount != 1 || result.NextTarget.UnitID != 3 || result.NextTarget.Code != "mp5" || result.NextTarget.PickitAction != "" {
		t.Fatalf("route supply scan = %+v, want only missing-column mp5", result)
	}
}

func TestPhase14LootTargetPreservesExactIdentityAndPickitContext(t *testing.T) {
	target := loot.PickupTarget{
		UnitID: 91, Code: "uap", Name: "Harlequin Crest", Quality: world.ItemQualityUnique,
		IdentityKind: world.ItemIdentityUnique, IdentityKey: "Harlequin Crest", IdentityValid: true,
		Pickit: loot.PickitResult{ProfileID: "mephisto", RuleID: "keep-shako", Action: loot.ActionKeep, ProfileRevision: 4, AssignmentRevision: 12},
		AreaID: world.DuranceOfHateLevel3,
	}
	roundTrip := mapLootPickupTarget(mapTaskLootTarget(target))
	if roundTrip.IdentityKind != target.IdentityKind || roundTrip.IdentityKey != target.IdentityKey || !roundTrip.IdentityValid ||
		roundTrip.Pickit.ProfileID != target.Pickit.ProfileID || roundTrip.Pickit.RuleID != target.Pickit.RuleID ||
		roundTrip.Pickit.Action != target.Pickit.Action || roundTrip.Pickit.ProfileRevision != target.Pickit.ProfileRevision ||
		roundTrip.Pickit.AssignmentRevision != target.Pickit.AssignmentRevision {
		t.Fatalf("round trip=%+v want=%+v", roundTrip, target)
	}
	event := telemetry.Event{}
	applyItemTelemetry(&event, roundTrip.Code, roundTrip.Name, roundTrip.Quality, roundTrip.IdentityKind, roundTrip.IdentityKey, roundTrip.IdentityValid)
	applyPickitTelemetry(&event, roundTrip.Pickit)
	if event.ItemKey != "unique:Harlequin Crest" || event.PickitRuleID != "keep-shako" || event.PickitAssignmentRevision != 12 {
		t.Fatalf("event=%+v", event)
	}
}

func TestTelemetryFailureIsStickyAndBlocksFollowingLootInput(t *testing.T) {
	emitter := &telemetryEmitterMock{err: errors.New("disk full")}
	adapter := &lootActionsAdapter{log: config.NewLogger("error"), filter: &loot.Filter{}, skipped: map[uint32]bool{}, telemetry: emitter}
	result := adapter.Scan(world.State{Items: []world.Item{{UnitID: 7, Location: world.ItemLocationGround}}})
	if !result.TelemetryFailed || adapter.telemetryErr == nil {
		t.Fatalf("result=%+v telemetryErr=%v", result, adapter.telemetryErr)
	}
	if err := adapter.StartPickup(tasks.LootTarget{UnitID: 7, AreaID: world.TowerCellarLevel5}); err == nil {
		t.Fatal("StartPickup() after telemetry failure error=nil")
	}
	stash := adapter.TickStash(world.State{}, time.Now())
	if stash.Status != tasks.LootStashTelemetryFailed || !stash.Done {
		t.Fatalf("TickStash()=%+v, want telemetry_failed", stash)
	}
}
