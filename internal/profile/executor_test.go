package profile

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type actionMock struct {
	skills    []uint16
	belts     []int
	mercBelts []int
	castDelay time.Duration
}

type routeCombatActionMock struct {
	targets []world.Monster
	skills  []uint16
	sent    bool
	err     error
	stops   int
}

func (m *routeCombatActionMock) CastAttackAtMonster(_ time.Time, skillID uint16, _ world.Player, target world.Monster) (bool, error) {
	m.targets = append(m.targets, target)
	m.skills = append(m.skills, skillID)
	return m.sent, m.err
}

func (m *routeCombatActionMock) StopAttack() error { m.stops++; return nil }

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

func (m *actionMock) CastBeltForMercenary(slot int) error {
	m.mercBelts = append(m.mercBelts, slot)
	return nil
}

func testDefinition() Definition {
	return Definition{ID: "necro", CharacterClass: world.CharacterClassNecromancer,
		Hooks: map[Hook][]Action{HookTownReady: {{SkillID: 68, Target: TargetSelf, OncePerGame: true, Settle: 100 * time.Millisecond}}},
		Resources: ResourcePolicy{
			Healing: ResourceRule{UseBelowPercent: 65, BeltSlots: []int{1}, Cooldown: 4 * time.Second}, Mana: ResourceRule{UseBelowPercent: 35, BeltSlots: []int{2, 3}, Cooldown: 4 * time.Second}, Rejuvenation: ResourceRule{UseBelowPercent: 35, BeltSlots: []int{4}, Cooldown: time.Second},
			Mercenary: MercenaryResourcePolicy{Enabled: true, ResourceRule: ResourceRule{UseBelowPercent: 50, BeltSlots: []int{1}, Cooldown: 4 * time.Second}},
			Throttle:  time.Second, VerifyTimeout: time.Second,
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

func TestSkipInitialDelayRetainsHookAction(t *testing.T) {
	definition := testDefinition()
	definition.Hooks[HookTownReady][0].Delay = 5 * time.Second
	actions := &actionMock{}
	executor, _ := NewExecutor(config.NewLogger("error"), definition, actions)
	state, now := profileState(), time.Now()
	executor.SkipInitialDelay(HookTownReady)
	if got := executor.TickHook(context.Background(), HookTownReady, state, EncounterTarget{}, now); got.Status != StatusAction {
		t.Fatalf("hook=%+v, want immediate action", got)
	}
	if len(actions.skills) != 1 {
		t.Fatalf("skills=%v, want Bone Armor action retained", actions.skills)
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
	if got := e.TickHook(context.Background(), HookBossEngage, state, target, now.Add(time.Second)); got.Status != StatusComplete {
		t.Fatalf("repeat=%+v", got)
	}
	if len(actions.skills) != 1 {
		t.Fatalf("skills=%v", actions.skills)
	}
	target.ActionIndex = 1
	if got := e.TickHook(context.Background(), HookBossEngage, state, target, now.Add(2*time.Second)); got.Status != StatusAction || got.SkillID != 88 {
		t.Fatalf("second indexed action=%+v", got)
	}
	if got := e.TickHook(context.Background(), HookBossEngage, state, target, now.Add(3*time.Second)); got.Status != StatusComplete {
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
	if got := e.TickResources(state, ResourceContext{}, now); got.Status != StatusAction || got.Resource != ResourceRejuvenation || got.BeltSlot != 4 {
		t.Fatalf("action=%+v", got)
	}
	if got := e.TickResources(state, ResourceContext{}, now.Add(100*time.Millisecond)); got.Status != StatusPending {
		t.Fatalf("pending=%+v", got)
	}
	state.Items = nil
	if got := e.TickResources(state, ResourceContext{}, now.Add(200*time.Millisecond)); got.Status != StatusComplete {
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
	got := e.TickResources(state, ResourceContext{}, time.Now())
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
	if got := e.TickResources(state, ResourceContext{}, now); got.Status != StatusAction {
		t.Fatalf("first=%+v", got)
	}
	state.Items = []world.Item{{UnitID: 11, Type: "mpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 1}}
	if got := e.TickResources(state, ResourceContext{}, now.Add(100*time.Millisecond)); got.Status != StatusComplete {
		t.Fatalf("verified=%+v", got)
	}
	if got := e.TickResources(state, ResourceContext{}, now.Add(2*time.Second)); got.Reason != "mana_potion_cooldown" {
		t.Fatalf("cooldown=%+v", got)
	}
	if got := e.TickResources(state, ResourceContext{}, now.Add(4*time.Second)); got.Status != StatusAction {
		t.Fatalf("after cooldown=%+v", got)
	}
	if len(actions.belts) != 2 {
		t.Fatalf("belts=%v", actions.belts)
	}
}

func TestRouteResourcePriorityUsesCriticalHPThenEmergencyManaThenRejuvenation(t *testing.T) {
	t.Run("critical hp wins", func(t *testing.T) {
		actions := &actionMock{}
		e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
		state, now := profileState(), time.Now()
		state.Player.HP = 20
		state.Player.Mana = 5
		state.Items = []world.Item{
			{UnitID: 40, Type: "rpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 3},
			{UnitID: 20, Type: "mpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 1},
		}
		got := e.TickResources(state, ResourceContext{MobilityCritical: true, Threatened: true, EmergencyMana: true}, now)
		if got.Status != StatusAction || got.Resource != ResourceRejuvenation || got.BeltSlot != 4 {
			t.Fatalf("critical HP result = %+v", got)
		}
	})

	t.Run("emergency mana", func(t *testing.T) {
		actions := &actionMock{}
		e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
		state, now := profileState(), time.Now()
		state.Player.HP = 100
		state.Player.Mana = 5
		state.Items = []world.Item{
			{UnitID: 40, Type: "rpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 3},
			{UnitID: 20, Type: "mpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 1},
		}
		got := e.TickResources(state, ResourceContext{MobilityCritical: true, Threatened: true, EmergencyMana: true}, now)
		if got.Status != StatusAction || got.Resource != ResourceMana || got.BeltSlot != 2 {
			t.Fatalf("emergency result = %+v", got)
		}
	})

	t.Run("rejuvenation fallback", func(t *testing.T) {
		actions := &actionMock{}
		e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
		state, now := profileState(), time.Now()
		state.Player.HP = 100
		state.Player.Mana = 5
		state.Items = []world.Item{{UnitID: 40, Type: "rpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 3}}
		got := e.TickResources(state, ResourceContext{MobilityCritical: true, Threatened: true, EmergencyMana: true}, now)
		if got.Status != StatusAction || got.Resource != ResourceRejuvenation || got.BeltSlot != 4 {
			t.Fatalf("fallback result = %+v", got)
		}
	})

	t.Run("mobility mana before normal healing", func(t *testing.T) {
		actions := &actionMock{}
		e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
		state, now := profileState(), time.Now()
		state.Player.HP = 50
		state.Player.Mana = 19
		state.Items = []world.Item{
			{UnitID: 10, Type: "hpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 0},
			{UnitID: 20, Type: "mpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 1},
		}
		got := e.TickResources(state, ResourceContext{MobilityCritical: true}, now)
		if got.Status != StatusAction || got.Resource != ResourceMana || got.BeltSlot != 2 {
			t.Fatalf("mobility result = %+v", got)
		}
	})

	t.Run("mobility mana yields to merc heal", func(t *testing.T) {
		actions := &actionMock{}
		e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
		state, now := profileState(), time.Now()
		state.Player.HP = 100
		state.Player.Mana = 19
		state.Mercenary = world.Mercenary{
			HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true,
			UnitID: 7, HP: 44, MaxHP: 90,
		}
		state.Items = []world.Item{
			{UnitID: 10, Type: "hpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 0},
			{UnitID: 20, Type: "mpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 1},
		}
		got := e.TickResources(state, ResourceContext{MobilityCritical: true, AllowMercenary: true}, now)
		if got.Status != StatusAction || got.Resource != ResourceHealing || got.BeltSlot != 1 {
			t.Fatalf("merc during mobility = %+v", got)
		}
		if len(actions.mercBelts) != 1 || len(actions.belts) != 0 {
			t.Fatalf("belts=%v mercBelts=%v", actions.belts, actions.mercBelts)
		}
	})

	t.Run("missing emergency potion", func(t *testing.T) {
		actions := &actionMock{}
		e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
		state, now := profileState(), time.Now()
		state.Player.HP = 100
		state.Player.Mana = 5
		got := e.TickResources(state, ResourceContext{MobilityCritical: true, Threatened: true, EmergencyMana: true}, now)
		if got.Status != StatusComplete || got.Reason != "rejuvenation_potion_unavailable" || len(actions.belts) != 0 {
			t.Fatalf("missing result = %+v belts=%v", got, actions.belts)
		}
	})
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
	if got := e.TickResources(state, ResourceContext{}, now.Add(2*time.Second)); got.Status != StatusAction {
		t.Fatalf("potion=%+v", got)
	}
	state.Items = nil
	if got := e.TickResources(state, ResourceContext{}, now.Add(2100*time.Millisecond)); got.Status != StatusComplete {
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
	if got := e.TickResources(state, ResourceContext{}, now); got.Status != StatusFailed || got.Reason != "profile_telemetry_failed" {
		t.Fatalf("failure=%+v", got)
	}
	e.Reset()
	e.SetTelemetry(nil)
	state.Player.Mana = 100
	if got := e.TickResources(state, ResourceContext{}, now.Add(time.Second)); got.Status != StatusComplete {
		t.Fatalf("after reset=%+v", got)
	}
	if len(actions.belts) != 1 {
		t.Fatalf("unexpected trailing input: %v", actions.belts)
	}
}

func TestRouteClearSingleTargetUsesOnlyConfirmedMonsterCombatSurface(t *testing.T) {
	definition := testDefinition()
	definition.ID = "necro_bone_spear"
	executor, _ := NewExecutor(config.NewLogger("error"), definition, &actionMock{})
	actions := &routeCombatActionMock{}
	if err := executor.ConfigureRouteClear(RouteClearSingleTarget, 66, 84, actions); err != nil {
		t.Fatal(err)
	}
	request := RouteClearRequest{
		RunID: "summoner", DefinitionID: "necro_bone_spear",
		Player: world.Player{Position: world.Position{X: 100, Y: 100}},
		Target: world.Monster{NPCID: world.ArcaneSpecter, UnitID: 77, Position: world.Position{X: 110, Y: 100}},
		Mode:   RouteClearThreat, AssessmentAt: time.Now(),
	}
	if got := executor.TickRouteClear(context.Background(), request, request.AssessmentAt); got.Status != StatusPending ||
		got.SkillID != 66 || got.ActionKind != RouteClearActionCurse {
		t.Fatalf("opener aim result = %+v", got)
	}
	actions.sent = true
	if got := executor.TickRouteClear(context.Background(), request, request.AssessmentAt.Add(time.Second)); got.Status != StatusAction ||
		got.SkillID != 66 || got.ActionKind != RouteClearActionCurse {
		t.Fatalf("opener cast result = %+v", got)
	}
	if got := executor.TickRouteClear(context.Background(), request, request.AssessmentAt.Add(2*time.Second)); got.Status != StatusAction ||
		got.SkillID != 84 || got.ActionKind != RouteClearActionAttack {
		t.Fatalf("attack cast result = %+v", got)
	}
	if len(actions.targets) != 3 || actions.targets[0].UnitID != 77 ||
		!reflect.DeepEqual(actions.skills, []uint16{66, 66, 84}) {
		t.Fatalf("targets=%+v skills=%v", actions.targets, actions.skills)
	}
	executor.ResetRouteClear()
	if actions.stops != 1 {
		t.Fatalf("stops = %d", actions.stops)
	}
	if got := executor.TickRouteClear(context.Background(), request, request.AssessmentAt.Add(3*time.Second)); got.SkillID != 66 ||
		got.ActionKind != RouteClearActionCurse {
		t.Fatalf("opener after reset = %+v", got)
	}
}

func TestRouteClearRejectsUnknownStrategyAndMissingTarget(t *testing.T) {
	definition := testDefinition()
	definition.ID = "necro_bone_spear"
	executor, _ := NewExecutor(config.NewLogger("error"), definition, &actionMock{})
	if err := executor.ConfigureRouteClear("rotation", 66, 84, &routeCombatActionMock{}); err == nil {
		t.Fatal("unknown strategy was accepted")
	}
	if got := executor.TickRouteClear(context.Background(), RouteClearRequest{}, time.Now()); got.Status != StatusFailed {
		t.Fatalf("unconfigured result = %+v", got)
	}
}

func TestRouteClearDensityReliefSkipsThreatCurseOpener(t *testing.T) {
	definition := testDefinition()
	definition.ID = "necro_bone_spear"
	executor, _ := NewExecutor(config.NewLogger("error"), definition, &actionMock{})
	actions := &routeCombatActionMock{sent: true}
	if err := executor.ConfigureRouteClear(RouteClearSingleTarget, 66, 84, actions); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	result := executor.TickRouteClear(context.Background(), RouteClearRequest{
		RunID: "summoner", DefinitionID: "necro_bone_spear",
		Player:       world.Player{Position: world.Position{X: 100, Y: 100}},
		Target:       world.Monster{NPCID: world.ArcaneSpecter, UnitID: 91, Position: world.Position{X: 110, Y: 100}},
		Mode:         RouteClearDensityRelief,
		AssessmentAt: now,
	}, now)
	if result.Status != StatusAction || result.SkillID != 84 || result.ActionKind != RouteClearActionAttack {
		t.Fatalf("density relief result = %+v", result)
	}
}

func TestRouteClearPreservesUnprojectableTargetAsPending(t *testing.T) {
	definition := testDefinition()
	definition.ID = "necro_bone_spear"
	executor, _ := NewExecutor(config.NewLogger("error"), definition, &actionMock{})
	actions := &routeCombatActionMock{err: ErrRouteClearTargetUnprojectable}
	if err := executor.ConfigureRouteClear(RouteClearSingleTarget, 66, 84, actions); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	result := executor.TickRouteClear(context.Background(), RouteClearRequest{
		RunID: "summoner", DefinitionID: "necro_bone_spear",
		Player:       world.Player{Position: world.Position{X: 100, Y: 100}},
		Target:       world.Monster{NPCID: world.ArcaneGhoulLord, UnitID: 83, Position: world.Position{X: 130, Y: 100}},
		Mode:         RouteClearThreat,
		AssessmentAt: now,
	}, now)
	if result.Status != StatusPending || result.Reason != RouteClearReasonTargetUnprojectable {
		t.Fatalf("unprojectable result = %+v", result)
	}
}

func TestRouteClearRejectsUnsupportedProfile(t *testing.T) {
	executor, _ := NewExecutor(config.NewLogger("error"), testDefinition(), &actionMock{})
	if err := executor.ConfigureRouteClear(RouteClearSingleTarget, 66, 84, &routeCombatActionMock{}); err == nil {
		t.Fatal("unsupported profile was accepted")
	}
}

func TestMercenaryResourceHealsAfterPlayerPriority(t *testing.T) {
	actions := &actionMock{}
	e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
	state, now := profileState(), time.Now()
	state.Mercenary = world.Mercenary{
		HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true,
		UnitID: 7, NPCID: 271, HP: 44, MaxHP: 90,
	}
	state.Items = []world.Item{{UnitID: 91, Type: "hpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 0}}

	if got := e.TickResources(state, ResourceContext{AllowMercenary: true}, now); got.Status != StatusAction || got.BeltSlot != 1 {
		t.Fatalf("merc heal=%+v", got)
	}
	if len(actions.belts) != 0 || len(actions.mercBelts) != 1 || actions.mercBelts[0] != 1 {
		t.Fatalf("belts=%v mercBelts=%v", actions.belts, actions.mercBelts)
	}
	state.Items = nil
	if got := e.TickResources(state, ResourceContext{AllowMercenary: true}, now.Add(100*time.Millisecond)); got.Status != StatusComplete {
		t.Fatalf("confirmed=%+v", got)
	}
}

func TestMercenaryResourceSkipsExactThresholdAndDeadUnknown(t *testing.T) {
	actions := &actionMock{}
	e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
	state := profileState()
	state.Items = []world.Item{{UnitID: 91, Type: "hpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 0}}

	state.Mercenary = world.Mercenary{HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true, UnitID: 7, HP: 45, MaxHP: 90}
	if got := e.TickResources(state, ResourceContext{AllowMercenary: true}, time.Now()); got.Status != StatusComplete || len(actions.mercBelts) != 0 {
		t.Fatalf("exact 50%% drank: %+v mercBelts=%v", got, actions.mercBelts)
	}

	state.Mercenary = world.Mercenary{HiredKnown: true, Hired: true, Dead: true, UnitID: 7}
	if got := e.TickResources(state, ResourceContext{AllowMercenary: true}, time.Now()); len(actions.mercBelts) != 0 {
		t.Fatalf("dead drank: %+v", got)
	}

	state.Mercenary = world.Mercenary{}
	if got := e.TickResources(state, ResourceContext{AllowMercenary: true}, time.Now()); len(actions.mercBelts) != 0 {
		t.Fatalf("unknown drank: %+v", got)
	}

	state.Mercenary = world.Mercenary{HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true, UnitID: 7, HP: 44, MaxHP: 90}
	if got := e.TickResources(state, ResourceContext{}, time.Now()); len(actions.mercBelts) != 0 {
		t.Fatalf("disallowed context drank: %+v", got)
	}
}

func TestMercenaryResourceYieldsToPlayerHealing(t *testing.T) {
	actions := &actionMock{}
	e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
	state, now := profileState(), time.Now()
	state.Player.HP = 50
	state.Mercenary = world.Mercenary{
		HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true,
		UnitID: 7, HP: 10, MaxHP: 90,
	}
	state.Items = []world.Item{{UnitID: 91, Type: "hpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 0}}
	if got := e.TickResources(state, ResourceContext{AllowMercenary: true}, now); got.Status != StatusAction || got.Resource != ResourceHealing {
		t.Fatalf("player priority=%+v", got)
	}
	if len(actions.belts) != 1 || len(actions.mercBelts) != 0 {
		t.Fatalf("belts=%v mercBelts=%v", actions.belts, actions.mercBelts)
	}
}

func TestMercenaryResourceRejectsRejuvenationFallback(t *testing.T) {
	actions := &actionMock{}
	e, _ := NewExecutor(config.NewLogger("error"), testDefinition(), actions)
	state := profileState()
	state.Mercenary = world.Mercenary{
		HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true,
		UnitID: 7, HP: 10, MaxHP: 90,
	}
	state.Items = []world.Item{{UnitID: 99, Type: "rpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 0}}
	got := e.TickResources(state, ResourceContext{AllowMercenary: true}, time.Now())
	if got.Reason != "mercenary_potion_unavailable" || len(actions.mercBelts) != 0 {
		t.Fatalf("rejuv fallback=%+v mercBelts=%v", got, actions.mercBelts)
	}
}
