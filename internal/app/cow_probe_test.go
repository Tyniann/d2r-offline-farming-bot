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

func TestValidateCowProbeLabel(t *testing.T) {
	for _, label := range []string{"stony-tristram", "wirts-body", "rogue-cow-portal", "cow-life-death-ce", "cube-open"} {
		if err := validateCowProbeLabel(label); err != nil {
			t.Fatalf("label %q: %v", label, err)
		}
	}
	for _, label := range []string{"", "Cow", "has space", "../escape"} {
		if err := validateCowProbeLabel(label); err == nil {
			t.Fatalf("label %q should fail", label)
		}
	}
}

func TestBuildCowProbeSampleKeepsOnlyPhase20ObjectsAndItems(t *testing.T) {
	at := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	snap := memory.Snapshot{
		At: at, Valid: true, Phase: memory.GamePhaseInGame,
		PlayerSkills:        memory.PlayerSkills{LeftSkill: memory.SkillAttack, RightSkill: memory.SkillCorpseExplosion},
		CowEvidenceComplete: true,
		CowEvidence:         []memory.CowRawEvidence{{NPCID: world.HellBovine, UnitID: 7, Corpse: 1, PosX: 10, PosY: 11}},
	}
	state := world.State{
		At: at, Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.MooMooFarm),
		Player: world.Player{Position: world.Position{X: 10, Y: 20}},
		Objects: []world.Object{
			{Kind: world.ObjectKindWaypoint, ID: 119, UnitID: 1},
			{Kind: world.ObjectKindPermanentPortal, ID: world.PermanentPortalID, UnitID: 2},
			{Kind: world.ObjectKindWirtsBody, ID: world.WirtsBodyID, UnitID: 3},
		},
		Items: []world.Item{{Code: "box", UnitID: 4}, {Code: "tbk", UnitID: 5}, {Code: "r01", UnitID: 6}},
	}
	got := buildCowProbeSample(snap, state)
	if len(got.Objects) != 2 || len(got.Items) != 2 || len(got.Cows) != 1 {
		t.Fatalf("sample = %+v", got)
	}
	if got.RightSkillID != memory.SkillCorpseExplosion || !got.CowEvidenceComplete {
		t.Fatalf("skill/coverage = %+v", got)
	}
}

func TestSaveCowProbeArtifactPublishesJSON(t *testing.T) {
	artifact := cowProbeArtifact{
		SchemaVersion: 1, CapturedAt: time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
		Label: "cube-open", GameVersion: "3.2.92777", SampleCount: 1,
		Samples: []cowProbeSample{{At: time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC), Valid: true, AreaID: 1, UIBufferHex: "0001"}},
	}
	path, err := saveCowProbeArtifact(t.TempDir(), artifact)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got cowProbeArtifact
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Label != artifact.Label || got.SampleCount != 1 || filepath.Ext(path) != ".json" {
		t.Fatalf("artifact = %+v path=%q", got, path)
	}
}
