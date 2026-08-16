package app

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestValidateWeaponSetProbeLabel(t *testing.T) {
	if err := validateWeaponSetProbeLabel(weaponSetProbeLabel); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"", "primary", "secondary", "Primary-Secondary"} {
		if err := validateWeaponSetProbeLabel(label); err == nil {
			t.Fatalf("label %q should fail", label)
		}
	}
}

func TestWeaponSetProbeStabilityRequiresAlternatingFreshConfirmations(t *testing.T) {
	stability := &weaponSetProbeStability{}
	primary := world.WeaponSetState{Set: world.WeaponSetPrimary, Available: true}
	secondary := world.WeaponSetState{Set: world.WeaponSetSecondary, Available: true}
	for _, active := range []world.WeaponSetState{primary, primary, primary, primary, secondary, secondary, secondary, primary, primary, primary} {
		stability.observe(active)
	}
	if !stability.passed() || stability.transitions != 2 {
		t.Fatalf("stability = %+v, want passed with two transitions", stability)
	}
	want := []string{"primary", "secondary", "primary"}
	for i := range want {
		if stability.confirmations[i] != want[i] {
			t.Fatalf("confirmations = %v, want %v", stability.confirmations, want)
		}
	}
}

func TestWeaponSetProbeStabilityResetsOnUnavailable(t *testing.T) {
	stability := &weaponSetProbeStability{}
	primary := world.WeaponSetState{Set: world.WeaponSetPrimary, Available: true}
	stability.observe(primary)
	stability.observe(primary)
	if ticks, confirmed := stability.observe(world.WeaponSetState{}); ticks != 0 || confirmed {
		t.Fatalf("unavailable observation = ticks %d confirmed %v", ticks, confirmed)
	}
	if ticks, confirmed := stability.observe(primary); ticks != 1 || confirmed {
		t.Fatalf("fresh observation = ticks %d confirmed %v", ticks, confirmed)
	}
}

func TestSaveWeaponSetProbeArtifact(t *testing.T) {
	artifact := weaponSetProbeArtifact{
		SchemaVersion: weaponSetProbeSchemaVersion,
		CapturedAt:    time.Date(2026, 8, 14, 22, 58, 1, 0, time.UTC),
		Label:         weaponSetProbeLabel, GameVersion: "3.2.92777", Passed: true,
		SampleCount: 1, StableTransitions: 2, StableConfirmations: []string{"primary", "secondary", "primary"},
		Samples: []weaponSetProbeSample{{Generation: 3, Available: true, Set: "primary", SkillsComplete: true, StableTicks: 3, Confirmed: true}},
	}
	path, err := saveWeaponSetProbeArtifact(t.TempDir(), artifact)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got weaponSetProbeArtifact
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Passed || got.SchemaVersion != 2 || got.Samples[0].Generation != 3 {
		t.Fatalf("artifact = %+v", got)
	}
}
