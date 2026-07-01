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
	}

	got := mapInputConfig(cfg)
	want := input.KeyboardConfig{
		KeyDelayMsMin: 5,
		KeyDelayMsMax: 15,
		ComboHoldMs:   100,
	}
	if got != want {
		t.Fatalf("mapInputConfig() = %+v, want %+v", got, want)
	}
}
