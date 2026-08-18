package app

import (
	"fmt"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

const (
	beltPotionHealing      = "healing"
	beltPotionMana         = "mana"
	beltPotionRejuvenation = "rejuvenation"
)

// OperatorBeltLayout assigns one potion type to each belt column 1..4.
type OperatorBeltLayout struct {
	Slot1 string `yaml:"slot_1,omitempty" json:"slot_1,omitempty"`
	Slot2 string `yaml:"slot_2,omitempty" json:"slot_2,omitempty"`
	Slot3 string `yaml:"slot_3,omitempty" json:"slot_3,omitempty"`
	Slot4 string `yaml:"slot_4,omitempty" json:"slot_4,omitempty"`
}

// beltLayoutConfigured reports whether every column has an explicit potion type.
func beltLayoutConfigured(layout OperatorBeltLayout) bool {
	return strings.TrimSpace(layout.Slot1) != "" &&
		strings.TrimSpace(layout.Slot2) != "" &&
		strings.TrimSpace(layout.Slot3) != "" &&
		strings.TrimSpace(layout.Slot4) != ""
}

func beltLayoutPartial(layout OperatorBeltLayout) bool {
	slots := []string{layout.Slot1, layout.Slot2, layout.Slot3, layout.Slot4}
	any, all := false, true
	for _, slot := range slots {
		if strings.TrimSpace(slot) == "" {
			all = false
			continue
		}
		any = true
	}
	return any && !all
}

func validateOperatorBeltLayout(layout OperatorBeltLayout) error {
	if !beltLayoutConfigured(layout) {
		if beltLayoutPartial(layout) {
			return fmt.Errorf("belt_layout must set all four slots or none")
		}
		return nil
	}
	for index, raw := range []string{layout.Slot1, layout.Slot2, layout.Slot3, layout.Slot4} {
		kind := strings.ToLower(strings.TrimSpace(raw))
		if kind != raw {
			return fmt.Errorf("belt_layout slot_%d must be canonical lowercase", index+1)
		}
		if !isBeltPotionKind(kind) {
			return fmt.Errorf("belt_layout slot_%d must be healing, mana, or rejuvenation", index+1)
		}
	}
	return nil
}

func isBeltPotionKind(kind string) bool {
	switch kind {
	case beltPotionHealing, beltPotionMana, beltPotionRejuvenation:
		return true
	default:
		return false
	}
}

func beltLayoutHasKind(layout OperatorBeltLayout, kind string) bool {
	for _, raw := range []string{layout.Slot1, layout.Slot2, layout.Slot3, layout.Slot4} {
		if strings.EqualFold(strings.TrimSpace(raw), kind) {
			return true
		}
	}
	return false
}

// BeltLayoutFromResources derives the operator layout from combat-profile belt columns.
func BeltLayoutFromResources(resources config.ProfileResourcesConfig) (OperatorBeltLayout, bool) {
	assigned := map[int]string{}
	for kind, slots := range map[string][]int{
		beltPotionHealing:      resources.Healing.BeltSlots,
		beltPotionMana:         resources.Mana.BeltSlots,
		beltPotionRejuvenation: resources.Rejuvenation.BeltSlots,
	} {
		for _, slot := range slots {
			if slot < 1 || slot > 4 {
				return OperatorBeltLayout{}, false
			}
			if previous := assigned[slot]; previous != "" && previous != kind {
				return OperatorBeltLayout{}, false
			}
			assigned[slot] = kind
		}
	}
	if len(assigned) != 4 {
		return OperatorBeltLayout{}, false
	}
	return OperatorBeltLayout{
		Slot1: assigned[1],
		Slot2: assigned[2],
		Slot3: assigned[3],
		Slot4: assigned[4],
	}, true
}

// EffectiveBeltLayout returns the operator override or the combat-profile default.
func EffectiveBeltLayout(layout OperatorBeltLayout, resources config.ProfileResourcesConfig) OperatorBeltLayout {
	if beltLayoutConfigured(layout) {
		return layout
	}
	if defaults, ok := BeltLayoutFromResources(resources); ok {
		return defaults
	}
	return OperatorBeltLayout{
		Slot1: beltPotionHealing,
		Slot2: beltPotionMana,
		Slot3: beltPotionMana,
		Slot4: beltPotionRejuvenation,
	}
}

// ApplyBeltLayoutToResources remaps healing/mana/rejuvenation columns from a complete layout.
// Mercenary keeps healing potions and follows the first healing column.
func ApplyBeltLayoutToResources(resources config.ProfileResourcesConfig, layout OperatorBeltLayout) (config.ProfileResourcesConfig, error) {
	if err := validateOperatorBeltLayout(layout); err != nil {
		return config.ProfileResourcesConfig{}, err
	}
	if !beltLayoutConfigured(layout) {
		return cloneProfileResources(resources), nil
	}
	out := cloneProfileResources(resources)
	out.Healing.BeltSlots = nil
	out.Mana.BeltSlots = nil
	out.Rejuvenation.BeltSlots = nil
	for _, slot := range []struct {
		index int
		kind  string
	}{
		{1, layout.Slot1}, {2, layout.Slot2}, {3, layout.Slot3}, {4, layout.Slot4},
	} {
		switch strings.ToLower(strings.TrimSpace(slot.kind)) {
		case beltPotionHealing:
			out.Healing.BeltSlots = append(out.Healing.BeltSlots, slot.index)
		case beltPotionMana:
			out.Mana.BeltSlots = append(out.Mana.BeltSlots, slot.index)
		case beltPotionRejuvenation:
			out.Rejuvenation.BeltSlots = append(out.Rejuvenation.BeltSlots, slot.index)
		}
	}
	if enabled, rule := out.Mercenary.Resolve(); enabled {
		if len(out.Healing.BeltSlots) == 0 {
			return config.ProfileResourcesConfig{}, fmt.Errorf("belt_layout needs at least one healing column for mercenary potions")
		}
		rule.BeltSlots = []int{out.Healing.BeltSlots[0]}
		out.Mercenary.BeltSlots = append([]int(nil), rule.BeltSlots...)
		out.Mercenary.UseBelowPercent = rule.UseBelowPercent
		out.Mercenary.CooldownMs = rule.CooldownMs
	}
	return out, nil
}

func cloneProfileResources(resources config.ProfileResourcesConfig) config.ProfileResourcesConfig {
	out := resources
	out.Healing.BeltSlots = append([]int(nil), resources.Healing.BeltSlots...)
	out.Mana.BeltSlots = append([]int(nil), resources.Mana.BeltSlots...)
	out.Rejuvenation.BeltSlots = append([]int(nil), resources.Rejuvenation.BeltSlots...)
	out.Mercenary.BeltSlots = append([]int(nil), resources.Mercenary.BeltSlots...)
	if resources.Mercenary.Enabled != nil {
		enabled := *resources.Mercenary.Enabled
		out.Mercenary.Enabled = &enabled
	}
	return out
}

func cloneProfilesConfig(profiles config.ProfilesConfig) config.ProfilesConfig {
	if profiles == nil {
		return nil
	}
	clone := make(config.ProfilesConfig, len(profiles))
	for id, profile := range profiles {
		profile.Resources = cloneProfileResources(profile.Resources)
		clone[id] = profile
	}
	return clone
}

func applyLoadoutBeltLayout(profiles config.ProfilesConfig, loadout *CharacterLoadoutSnapshot) error {
	if loadout == nil || profiles == nil {
		return nil
	}
	profileID := strings.TrimSpace(loadout.ProfileID)
	profile, ok := profiles[profileID]
	if !ok {
		return fmt.Errorf("combat profile %q unavailable for belt layout", profileID)
	}
	resources, err := ApplyBeltLayoutToResources(profile.Resources, loadout.BeltLayout)
	if err != nil {
		return err
	}
	profile.Resources = resources
	profiles[profileID] = profile
	return nil
}
