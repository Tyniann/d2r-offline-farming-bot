package hammerdin

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
)

func TestMephistoStrategyContract(t *testing.T) {
	strategy := NewMephistoFactory()()
	if strategy.ProfileID() != "paladin_hammerdin" || strategy.RunID() != "mephisto" {
		t.Fatalf("strategy identity = %s/%s", strategy.ProfileID(), strategy.RunID())
	}
	want := []string{"teleport", "town_portal", "blessed_hammer", "concentration", "holy_shield"}
	got := strategy.RequiredSkills()
	if len(got) != len(want) {
		t.Fatalf("required skills = %v", got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("required skills = %v", got)
		}
	}
	if err := strategy.Configure(&profile.Executor{}, memory.MustSkillID("blessed_hammer"), nil); err != nil {
		t.Fatal(err)
	}
	if err := strategy.Configure(&profile.Executor{}, memory.MustSkillID("bone_spear"), nil); err == nil {
		t.Fatal("wrong standard attack accepted")
	}
}
