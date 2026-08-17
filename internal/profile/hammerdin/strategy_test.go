package hammerdin

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
)

func TestBossStrategyContract(t *testing.T) {
	wantSkills := []string{"teleport", "town_portal", "blessed_hammer", "concentration", "holy_shield"}
	for _, runID := range []string{"countess", "mephisto", "nihlathak"} {
		strategy := NewBossFactory(runID)()
		if strategy.ProfileID() != "paladin_hammerdin" || strategy.RunID() != runID {
			t.Fatalf("strategy identity = %s/%s", strategy.ProfileID(), strategy.RunID())
		}
		got := strategy.RequiredSkills()
		if len(got) != len(wantSkills) {
			t.Fatalf("%s required skills = %v", runID, got)
		}
		for index := range wantSkills {
			if got[index] != wantSkills[index] {
				t.Fatalf("%s required skills = %v", runID, got)
			}
		}
		if _, ok := strategy.(profile.SupportsRouteClear); ok {
			t.Fatalf("%s must not wire route clear", runID)
		}
		if err := strategy.Configure(&profile.Executor{}, memory.MustSkillID("blessed_hammer"), nil); err != nil {
			t.Fatal(err)
		}
		if err := strategy.Configure(&profile.Executor{}, memory.MustSkillID("bone_spear"), nil); err == nil {
			t.Fatalf("%s accepted wrong standard attack", runID)
		}
	}
}
