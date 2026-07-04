package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestRunActionsCastBeltUsesInput(t *testing.T) {
	in := &mockInput{}
	actions := newRunActionsAdapter(config.NewLogger("error"), in, testBindings())

	if err := actions.CastBelt(4); err != nil {
		t.Fatal(err)
	}
	if len(in.castBeltCalls) != 1 || in.castBeltCalls[0] != 4 {
		t.Fatalf("CastBelt calls = %v, want [4]", in.castBeltCalls)
	}
}

func TestRunActionsCastTownPortalUsesClientCenter(t *testing.T) {
	in := &mockInput{}
	actions := newRunActionsAdapter(config.NewLogger("error"), in, testBindings())

	if err := actions.CastTownPortal(); err != nil {
		t.Fatal(err)
	}
	if len(in.castSkillCalls) != 1 || in.castSkillCalls[0] != memory.SkillTownPortal {
		t.Fatalf("CastSkill calls = %v, want town portal", in.castSkillCalls)
	}
	if in.lastClientX != 640 || in.lastClientY != 360 {
		t.Fatalf("client coords = %d,%d, want 640,360", in.lastClientX, in.lastClientY)
	}
}
