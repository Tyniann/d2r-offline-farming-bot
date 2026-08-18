package app

import (
	"reflect"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

func TestApplyBeltLayoutToResourcesRemapsColumnsAndMerc(t *testing.T) {
	base := config.ProfileResourcesConfig{
		Healing:      config.ResourceRuleConfig{UseBelowPercent: 65, BeltSlots: []int{1}, CooldownMs: 4000},
		Mana:         config.ResourceRuleConfig{UseBelowPercent: 35, BeltSlots: []int{2, 3}, CooldownMs: 4000},
		Rejuvenation: config.ResourceRuleConfig{UseBelowPercent: 35, BeltSlots: []int{4}, CooldownMs: 1500},
		Mercenary:    config.MercenaryResourceConfig{UseBelowPercent: 50, BeltSlots: []int{1}, CooldownMs: 4000},
	}
	got, err := ApplyBeltLayoutToResources(base, OperatorBeltLayout{
		Slot1: beltPotionHealing,
		Slot2: beltPotionHealing,
		Slot3: beltPotionRejuvenation,
		Slot4: beltPotionRejuvenation,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !reflect.DeepEqual(got.Healing.BeltSlots, []int{1, 2}) {
		t.Fatalf("healing slots=%v", got.Healing.BeltSlots)
	}
	if len(got.Mana.BeltSlots) != 0 {
		t.Fatalf("mana slots=%v, want empty", got.Mana.BeltSlots)
	}
	if !reflect.DeepEqual(got.Rejuvenation.BeltSlots, []int{3, 4}) {
		t.Fatalf("rejuv slots=%v", got.Rejuvenation.BeltSlots)
	}
	if !reflect.DeepEqual(got.Mercenary.BeltSlots, []int{1}) {
		t.Fatalf("merc slots=%v", got.Mercenary.BeltSlots)
	}
	if base.Mana.BeltSlots[0] != 2 {
		t.Fatalf("base resources mutated: %v", base.Mana.BeltSlots)
	}
}

func TestValidateOperatorBeltLayoutRejectsPartial(t *testing.T) {
	if err := validateOperatorBeltLayout(OperatorBeltLayout{Slot1: beltPotionHealing}); err == nil {
		t.Fatal("expected partial layout error")
	}
	if err := validateOperatorBeltLayout(OperatorBeltLayout{}); err != nil {
		t.Fatalf("empty layout: %v", err)
	}
}

func TestBeltLayoutFromResources(t *testing.T) {
	layout, ok := BeltLayoutFromResources(config.ProfileResourcesConfig{
		Healing:      config.ResourceRuleConfig{BeltSlots: []int{1}},
		Mana:         config.ResourceRuleConfig{BeltSlots: []int{2, 3}},
		Rejuvenation: config.ResourceRuleConfig{BeltSlots: []int{4}},
	})
	if !ok || layout != (OperatorBeltLayout{Slot1: "healing", Slot2: "mana", Slot3: "mana", Slot4: "rejuvenation"}) {
		t.Fatalf("layout=%+v ok=%t", layout, ok)
	}
}
