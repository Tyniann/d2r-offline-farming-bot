package town

import (
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestEvaluateMercenaryTownDemandScenarios(t *testing.T) {
	policy := MercenaryPolicy{Enabled: true, ThresholdPercent: 50}
	cases := []struct {
		name       string
		policy     MercenaryPolicy
		merc       world.Mercenary
		wantHeal   bool
		wantRevive bool
		wantFail   Reason
	}{
		{name: "disabled", policy: MercenaryPolicy{}, merc: world.Mercenary{HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true, HP: 10, MaxHP: 100}},
		{name: "healthy", policy: policy, merc: world.Mercenary{HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true, HP: 90, MaxHP: 100}},
		{name: "exact threshold", policy: policy, merc: world.Mercenary{HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true, HP: 50, MaxHP: 100}},
		{name: "heal below", policy: policy, merc: world.Mercenary{HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true, HP: 49, MaxHP: 100}, wantHeal: true},
		{name: "dead revive", policy: policy, merc: world.Mercenary{HiredKnown: true, Hired: true, Dead: true}, wantRevive: true},
		{name: "not hired", policy: policy, merc: world.Mercenary{HiredKnown: true, Hired: false}, wantFail: ReasonMercenaryNotHired},
		{name: "unknown hired", policy: policy, merc: world.Mercenary{}, wantFail: ReasonMercenaryStateInvalid},
		{name: "alive without vitals", policy: policy, merc: world.Mercenary{HiredKnown: true, Hired: true, Alive: true}, wantFail: ReasonMercenaryStateInvalid},
		{name: "contradictory dead alive", policy: policy, merc: world.Mercenary{HiredKnown: true, Hired: true, Alive: true, Dead: true}, wantFail: ReasonMercenaryStateInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			heal, revive, fail := EvaluateMercenaryTownDemand(tc.policy, tc.merc)
			if heal != tc.wantHeal || revive != tc.wantRevive || fail != tc.wantFail {
				t.Fatalf("got heal=%v revive=%v fail=%q, want %v/%v/%q", heal, revive, fail, tc.wantHeal, tc.wantRevive, tc.wantFail)
			}
		})
	}
}

func TestEvaluateMercenaryPreflightRejectsDeadAtStart(t *testing.T) {
	policy := MercenaryPolicy{Enabled: true, ThresholdPercent: 50}
	if got := EvaluateMercenaryPreflight(policy, world.Mercenary{HiredKnown: true, Hired: true, Dead: true}); got != ReasonMercenaryDeadAtStart {
		t.Fatalf("preflight = %q", got)
	}
	if got := EvaluateMercenaryPreflight(policy, world.Mercenary{HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true, HP: 10, MaxHP: 100}); got != "" {
		t.Fatalf("injured living merc must pass preflight, got %q", got)
	}
}

func TestPlannerOrdersMercServicesBetweenCainAndAkara(t *testing.T) {
	planner, err := NewPlanner(validTownConfig())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := DemandSnapshot{Demand: Demand{Identify: true, MercenaryRevive: true, Potions: true}}
	plan, reason := planner.Plan(Origin{Act: OriginAct1, Anchor: AnchorStash}, snapshot, NextRunTarget{})
	if reason != "" {
		t.Fatal(reason)
	}
	want := []Service{ServiceIdentify, ServiceMercenaryRevive, ServicePotions}
	if len(plan.Steps) != len(want) {
		t.Fatalf("steps = %+v", plan.Steps)
	}
	for i, service := range want {
		if plan.Steps[i].Service != service {
			t.Fatalf("step %d = %s, want %s", i, plan.Steps[i].Service, service)
		}
	}
	_, sequence, _, reason := planner.GraphAnchorSequence(plan)
	if reason != "" {
		t.Fatal(reason)
	}
	if len(sequence) != 3 || sequence[0] != AnchorCain || sequence[1] != AnchorKashya || sequence[2] != AnchorAkara {
		t.Fatalf("sequence = %v", sequence)
	}
}

func TestMenuSelectorHomeDownEnterPerTick(t *testing.T) {
	in := &menuInputMock{}
	selector := NewMenuSelector(in, time.Second)
	state := world.State{Valid: true, At: time.Now(), UI: world.UIState{NPCInteractOpen: true}}
	for i, wantKey := range []string{"home", "down", "enter"} {
		result := selector.Tick(state)
		if result.Status != InteractionAction {
			t.Fatalf("tick %d status = %s reason=%s", i, result.Status, result.Reason)
		}
		if len(in.keys) != i+1 || in.keys[i] != wantKey {
			t.Fatalf("keys = %v after tick %d", in.keys, i)
		}
	}
	result := selector.Tick(state)
	if result.Status != InteractionComplete || !result.Done {
		t.Fatalf("final = %+v", result)
	}
}

type menuInputMock struct{ keys []string }

func (m *menuInputMock) MoveTo(int, int) error                      { return nil }
func (m *menuInputMock) Click(input.MouseButton) error              { return nil }
func (m *menuInputMock) ClickWithModifier(string, input.MouseButton) error {
	return nil
}
func (m *menuInputMock) PressKey(key string) error {
	m.keys = append(m.keys, key)
	return nil
}
