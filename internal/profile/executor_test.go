package profile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type actionMock struct {
	skills    []uint16
	belts     []int
	castDelay time.Duration
}

type telemetryMock struct {
	events []Event
	err    error
}

func (m *telemetryMock) EmitProfile(event Event) error {
	m.events = append(m.events, event)
	return m.err
}

func (m *actionMock) CastSkillAtWorld(_ time.Time, id uint16, _, _ world.Position) error {
	if m.castDelay > 0 {
		time.Sleep(m.castDelay)
	}
	m.skills = append(m.skills, id)
	return nil
}

func TestHookSettleStartsAfterBlockingCastCompletes(t *testing.T) {
	actions := &actionMock{castDelay: 75 * time.Millisecond}
	e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
	state, now := profileState(), time.Now()
	if got := e.TickHook(context.Background(), HookTownReady, state, EncounterTarget{}, now); got.Status != StatusAction {
		t.Fatalf("action=%+v", got)
	}
	if got := e.TickHook(context.Background(), HookTownReady, state, EncounterTarget{}, now.Add(150*time.Millisecond)); got.Status != StatusPending {
		t.Fatalf("settle counted from tick start: %+v", got)
	}
	if got := e.TickHook(context.Background(), HookTownReady, state, EncounterTarget{}, now.Add(200*time.Millisecond)); got.Status != StatusComplete {
		t.Fatalf("complete=%+v", got)
	}
}

func TestHookDelayWaitsForStableLifecycleWindowBeforeInput(t *testing.T) {
	definition := testDefinition()
	definition.Hooks[HookTownReady][0].Delay = 1500 * time.Millisecond
	actions := &actionMock{}
	e, _ := NewExecutor(config.NewLogger("error"), definition, actions)
	state, now := profileState(), time.Now()
	if got := e.TickHook(context.Background(), HookTownReady, state, EncounterTarget{}, now); got.Status != StatusPending {
		t.Fatalf("start=%+v", got)
	}
	if got := e.TickHook(context.Background(), HookTownReady, state, EncounterTarget{}, now.Add(time.Second)); got.Status != StatusPending || len(actions.skills) != 0 {
		t.Fatalf("early=%+v skills=%v", got, actions.skills)
	}
	if got := e.TickHook(context.Background(), HookTownReady, state, EncounterTarget{}, now.Add(1500*time.Millisecond)); got.Status != StatusAction || len(actions.skills) != 1 {
		t.Fatalf("ready=%+v skills=%v", got, actions.skills)
	}
}
func (m *actionMock) CastBelt(slot int) error { m.belts = append(m.belts, slot); return nil }

func testDefinition() Definition {
	return Definition{ID: "necro", CharacterClass: world.CharacterClassNecromancer,
		Hooks: map[Hook][]Action{HookTownReady: {{SkillID: 68, Target: TargetSelf, OncePerGame: true, Settle: 100 * time.Millisecond}}},
		Resources: ResourcePolicy{
			Healing: ResourceRule{UseBelowPercent: 65, BeltSlots: []int{1}, Cooldown: 4 * time.Second}, Mana: ResourceRule{UseBelowPercent: 35, BeltSlots: []int{2, 3}, Cooldown: 4 * time.Second}, Rejuvenation: ResourceRule{UseBelowPercent: 35, BeltSlots: []int{4}, Cooldown: time.Second},
			Throttle: time.Second, VerifyTimeout: time.Second,
		}}
}

func profileState() world.State {
	return world.State{Valid: true, Phase: world.GamePhaseInGame, At: time.Now(), Area: world.LookupArea(world.RogueEncampment),
		Player:   world.Player{HP: 100, MaxHP: 100, Mana: 100, MaxMana: 100, Position: world.Position{X: 10, Y: 10}},
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer}}
}

func TestTownHookCastsOnceAndSettles(t *testing.T) {
	actions := &actionMock{}
	e, err := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
	if err != nil {
		t.Fatal(err)
	}
	state, now := profileState(), time.Now()
	if got := e.TickHook(context.Background(), HookTownReady, state, EncounterTarget{}, now); got.Status != StatusAction {
		t.Fatalf("first=%+v", got)
	}
	if got := e.TickHook(context.Background(), HookTownReady, state, EncounterTarget{}, now.Add(50*time.Millisecond)); got.Status != StatusPending {
		t.Fatalf("settle=%+v", got)
	}
	if got := e.TickHook(context.Background(), HookTownReady, state, EncounterTarget{}, now.Add(150*time.Millisecond)); got.Status != StatusComplete {
		t.Fatalf("complete=%+v", got)
	}
	if len(actions.skills) != 1 {
		t.Fatalf("skills=%v", actions.skills)
	}
}

func TestBossHookPinsTargetAndCastsOncePerIndexedEncounterAction(t *testing.T) {
	definition := testDefinition()
	definition.Hooks[HookBossEngage] = []Action{{SkillID: 88, Target: TargetBoss, OncePerEncounter: true}}
	actions := &actionMock{}
	e, _ := NewExecutor(config.NewLogger("error"), definition, actions)
	state, now := profileState(), time.Now()
	target := EncounterTarget{UnitID: 99, Position: world.Position{X: 20, Y: 20}}
	if got := e.TickHook(context.Background(), HookBossEngage, state, target, now); got.Status != StatusAction || got.SkillID != 88 {
		t.Fatalf("first=%+v", got)
	}
	if got := e.TickHook(context.Background(), HookBossEngage, state, target, now.Add(time.Millisecond)); got.Status != StatusComplete {
		t.Fatalf("repeat=%+v", got)
	}
	if len(actions.skills) != 1 {
		t.Fatalf("skills=%v", actions.skills)
	}
	target.ActionIndex = 1
	if got := e.TickHook(context.Background(), HookBossEngage, state, target, now.Add(2*time.Millisecond)); got.Status != StatusAction || got.SkillID != 88 {
		t.Fatalf("second indexed action=%+v", got)
	}
	if got := e.TickHook(context.Background(), HookBossEngage, state, target, now.Add(3*time.Millisecond)); got.Status != StatusComplete {
		t.Fatalf("second indexed completion=%+v", got)
	}
	if len(actions.skills) != 2 {
		t.Fatalf("skills=%v, want two indexed casts", actions.skills)
	}
}

func TestResourcePriorityAndVerification(t *testing.T) {
	actions := &actionMock{}
	e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
	state, now := profileState(), time.Now()
	state.Player.HP = 20
	state.Player.Mana = 10
	state.Items = []world.Item{{UnitID: 44, Type: "rpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 3}}
	if got := e.TickResources(state, now); got.Status != StatusAction || got.Resource != ResourceRejuvenation || got.BeltSlot != 4 {
		t.Fatalf("action=%+v", got)
	}
	if got := e.TickResources(state, now.Add(100*time.Millisecond)); got.Status != StatusPending {
		t.Fatalf("pending=%+v", got)
	}
	state.Items = nil
	if got := e.TickResources(state, now.Add(200*time.Millisecond)); got.Status != StatusComplete {
		t.Fatalf("verified=%+v", got)
	}
	if len(actions.belts) != 1 || actions.belts[0] != 4 {
		t.Fatalf("belts=%v", actions.belts)
	}
}

func TestResourceDoesNotPressEmptyOrWrongSlot(t *testing.T) {
	actions := &actionMock{}
	e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
	state := profileState()
	state.Player.Mana = 20
	state.Items = []world.Item{{UnitID: 1, Type: "hpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 1}}
	got := e.TickResources(state, time.Now())
	if got.Status != StatusComplete || got.Reason != "mana_potion_unavailable" || len(actions.belts) != 0 {
		t.Fatalf("got=%+v belts=%v", got, actions.belts)
	}
}

func TestResourceCooldownAccountsForGradualPotionEffect(t *testing.T) {
	actions := &actionMock{}
	e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
	state, now := profileState(), time.Now()
	state.Player.Mana = 20
	state.Items = []world.Item{{UnitID: 10, Type: "mpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 1}}
	if got := e.TickResources(state, now); got.Status != StatusAction {
		t.Fatalf("first=%+v", got)
	}
	state.Items = []world.Item{{UnitID: 11, Type: "mpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 1}}
	if got := e.TickResources(state, now.Add(100*time.Millisecond)); got.Status != StatusComplete {
		t.Fatalf("verified=%+v", got)
	}
	if got := e.TickResources(state, now.Add(2*time.Second)); got.Reason != "mana_potion_cooldown" {
		t.Fatalf("cooldown=%+v", got)
	}
	if got := e.TickResources(state, now.Add(4*time.Second)); got.Status != StatusAction {
		t.Fatalf("after cooldown=%+v", got)
	}
	if len(actions.belts) != 2 {
		t.Fatalf("belts=%v", actions.belts)
	}
}

func TestProfileTelemetryContainsHookAndConfirmedPotionContext(t *testing.T) {
	actions, trace := &actionMock{}, &telemetryMock{}
	definition := testDefinition()
	definition.Hooks[HookBossEngage] = []Action{{SkillID: 88, Target: TargetBoss, OncePerEncounter: true}}
	e, _ := NewExecutor(config.NewLogger("error"), definition, actions)
	e.SetTelemetry(trace)
	state, now := profileState(), time.Now()
	target := EncounterTarget{UnitID: 273, Position: world.Position{X: 20, Y: 20}}
	if got := e.TickHook(context.Background(), HookBossEngage, state, target, now); got.Status != StatusAction {
		t.Fatalf("hook=%+v", got)
	}
	state.Player.Mana = 20
	state.Items = []world.Item{{UnitID: 44, Type: "mpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 1}}
	if got := e.TickResources(state, now.Add(2*time.Second)); got.Status != StatusAction {
		t.Fatalf("potion=%+v", got)
	}
	state.Items = nil
	if got := e.TickResources(state, now.Add(2100*time.Millisecond)); got.Status != StatusComplete {
		t.Fatalf("confirmation=%+v", got)
	}
	if len(trace.events) != 3 {
		t.Fatalf("events=%+v", trace.events)
	}
	if got := trace.events[0]; got.Name != EventHookAction || got.Profile != "necro" || got.SkillID != 88 || got.TargetUnitID != 273 {
		t.Fatalf("hook event=%+v", got)
	}
	if got := trace.events[1]; got.Name != EventPotionRequested || got.Resource != ResourceMana || got.ThresholdPercent != 35 || got.BeltSlot != 2 || got.PotionUnitID != 44 {
		t.Fatalf("request event=%+v", got)
	}
	if got := trace.events[2]; got.Name != EventPotionConfirmed || !got.Confirmed || got.PotionUnitID != 44 {
		t.Fatalf("confirmed event=%+v", got)
	}
}

func TestProfileTelemetryFailureStopsProgressAndResetClearsPendingInput(t *testing.T) {
	actions, trace := &actionMock{}, &telemetryMock{err: errors.New("disk full")}
	e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
	e.SetTelemetry(trace)
	state, now := profileState(), time.Now()
	state.Player.Mana = 20
	state.Items = []world.Item{{UnitID: 44, Type: "mpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 1}}
	if got := e.TickResources(state, now); got.Status != StatusFailed || got.Reason != "profile_telemetry_failed" {
		t.Fatalf("failure=%+v", got)
	}
	e.Reset()
	e.SetTelemetry(nil)
	state.Player.Mana = 100
	if got := e.TickResources(state, now.Add(time.Second)); got.Status != StatusComplete {
		t.Fatalf("after reset=%+v", got)
	}
	if len(actions.belts) != 1 {
		t.Fatalf("unexpected trailing input: %v", actions.belts)
	}
}
