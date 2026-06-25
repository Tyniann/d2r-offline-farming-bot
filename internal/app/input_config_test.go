package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
)

func TestMapInputConfig(t *testing.T) {
	cfg := config.InputConfig{
		KeyDelayMsMin: 5,
		KeyDelayMsMax: 15,
		ComboHoldMs:   100,
		Skills: config.SkillKeys{
			Slot1: "f1", Slot2: "f2", Slot3: "f3", Slot4: "f4",
			Slot5: "f5", Slot6: "f6", Slot7: "f7", Slot8: "",
		},
		Belt: config.BeltKeys{
			Slot1: "1", Slot2: "2", Slot3: "3", Slot4: "4",
		},
		TownPortal: "f8",
	}

	got := mapInputConfig(cfg)
	want := input.KeyboardConfig{
		KeyDelayMsMin: 5,
		KeyDelayMsMax: 15,
		ComboHoldMs:   100,
		Skills:        [8]string{"f1", "f2", "f3", "f4", "f5", "f6", "f7", ""},
		Belt:          [4]string{"1", "2", "3", "4"},
		TownPortal:    "f8",
	}
	if got != want {
		t.Fatalf("mapInputConfig() = %+v, want %+v", got, want)
	}
}

func TestMapSafetyConfig(t *testing.T) {
	cfg := config.InputConfig{
		Enabled:     true,
		PauseHotkey: "pause",
		StopHotkey:  "f11",
	}
	got := mapSafetyConfig(cfg)
	want := input.SafetyConfig{Enabled: true, PauseHotkey: "pause", StopHotkey: "f11"}
	if got != want {
		t.Fatalf("mapSafetyConfig() = %+v, want %+v", got, want)
	}
}
