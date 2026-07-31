package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestValidateMercenaryProbeLabel(t *testing.T) {
	for _, label := range []string{
		MercenaryProbeNotHired,
		MercenaryProbeAliveHealthy,
		MercenaryProbeAliveInjured,
		MercenaryProbeDead,
		MercenaryProbeAreaTransition,
		"custom-check",
	} {
		if err := validateMercenaryProbeLabel(label); err != nil {
			t.Fatalf("label %q: %v", label, err)
		}
	}
	for _, label := range []string{"", "Bad", "has space", "UPPER"} {
		if err := validateMercenaryProbeLabel(label); err == nil {
			t.Fatalf("label %q should fail", label)
		}
	}
}

func TestCountHostileHirelings(t *testing.T) {
	monsters := []world.Monster{
		{NPCID: 56, UnitID: 1},
		{NPCID: memory.HirelingClassRogueScout, UnitID: 2},
		{NPCID: memory.HirelingClassDesertMercenary, UnitID: 3},
	}
	if got := countHostileHirelings(monsters); got != 2 {
		t.Fatalf("countHostileHirelings() = %d, want 2", got)
	}
}

func TestSaveMercenaryProbeArtifact(t *testing.T) {
	dir := t.TempDir()
	life := int32(8192)
	artifact := mercenaryProbeArtifact{
		SchemaVersion: 1,
		CapturedAt:    time.Date(2026, 7, 30, 20, 0, 0, 0, time.UTC),
		Label:         MercenaryProbeAliveInjured,
		GameVersion:   "3.2.92777",
		SampleCount:   1,
		HirelingClasses: []mercenaryProbeClass{
			{NPCID: memory.HirelingClassDesertMercenary, Name: "desert_mercenary"},
		},
		Samples: []mercenaryProbeSample{{
			At:    time.Date(2026, 7, 30, 20, 0, 0, 0, time.UTC),
			Phase: "in_game", Valid: true, AreaID: 1,
			PlayerHP: 1000, PlayerMaxHP: 1000,
			UI: world.UIState{},
			Mercenary: world.Mercenary{
				HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true,
				UnitID: 7, NPCID: memory.HirelingClassDesertMercenary, HP: 10, MaxHP: 90,
			},
			HirelingCount: 1,
			Hirelings: []memory.HirelingRawEvidence{{
				UnitID: 7, NPCID: memory.HirelingClassDesertMercenary,
				ClassName: "desert_mercenary", Corpse: 0, Mode: 1, ModeKnown: true,
				ActiveLifeRaw: &life,
			}},
		}},
		Notes: []string{"test"},
	}
	path, err := saveMercenaryProbeArtifact(dir, artifact)
	if err != nil {
		t.Fatalf("saveMercenaryProbeArtifact: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var got mercenaryProbeArtifact
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Label != MercenaryProbeAliveInjured || got.SampleCount != 1 || len(got.Samples) != 1 {
		t.Fatalf("artifact = %+v", got)
	}
	if got.Samples[0].HirelingCount != 1 || got.Samples[0].Hirelings[0].NPCID != memory.HirelingClassDesertMercenary {
		t.Fatalf("sample = %+v", got.Samples[0])
	}
	if !got.Samples[0].Mercenary.Alive || got.Samples[0].Mercenary.HP != 10 {
		t.Fatalf("semantic Mercenary = %+v", got.Samples[0].Mercenary)
	}
	if filepath.Ext(path) != ".json" {
		t.Fatalf("path = %q", path)
	}
}
