package app

import (
	"fmt"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

type configBindingSource struct {
	skills map[uint16]input.SkillCast
	belt   [4]string
}

func newConfigBindingSource(cfg config.InputBindingsConfig) (configBindingSource, error) {
	out := configBindingSource{
		skills: make(map[uint16]input.SkillCast, len(cfg.Skills)),
		belt: [4]string{
			cfg.Belt.Slot1,
			cfg.Belt.Slot2,
			cfg.Belt.Slot3,
			cfg.Belt.Slot4,
		},
	}
	for rawName, binding := range cfg.Skills {
		skillID, err := memory.ParseSkillTestName(rawName)
		if err != nil {
			return configBindingSource{}, fmt.Errorf("bindings.skills.%s: %w", rawName, err)
		}
		out.skills[skillID] = input.SkillCast{
			SkillID:    skillID,
			SelectKey:  strings.ToLower(strings.TrimSpace(binding.Key)),
			CastButton: input.MouseButton(strings.ToLower(strings.TrimSpace(binding.Button))),
		}
	}
	return out, nil
}

func (s configBindingSource) Resolve(skillID uint16) (input.SkillCast, error) {
	cast, ok := s.skills[skillID]
	if !ok || cast.SelectKey == "" {
		return input.SkillCast{}, fmt.Errorf("skill %s(%d): %w", memory.SkillName(skillID), skillID, input.ErrUnconfiguredSlot)
	}
	return cast, nil
}

func (s configBindingSource) BeltKeyName(slot int) (string, error) {
	if slot < 1 || slot > 4 {
		return "", fmt.Errorf("belt slot %d: %w", slot, input.ErrInvalidSlot)
	}
	return s.belt[slot-1], nil
}

func (s configBindingSource) TownPortalSkillID() (uint16, bool) {
	if _, ok := s.skills[memory.SkillTownPortal]; ok {
		return memory.SkillTownPortal, true
	}
	return 0, false
}
