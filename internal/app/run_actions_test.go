package app

import (
	"errors"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestRunActionsCastBeltUsesInput(t *testing.T) {
	in := &mockInput{}
	actions := newRunActionsAdapter(config.NewLogger("error"), in, testBindings(), nil)

	if err := actions.CastBelt(4); err != nil {
		t.Fatal(err)
	}
	if len(in.castBeltCalls) != 1 || in.castBeltCalls[0] != 4 {
		t.Fatalf("CastBelt calls = %v, want [4]", in.castBeltCalls)
	}
}

func TestRunActionsCastTownPortalUsesRightSkillSelector(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := testBindings()
	combat := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Millisecond)
	actions := newRunActionsAdapter(config.NewLogger("error"), in, bindings, combat)
	now := time.Now()
	player := world.Player{RightSkillID: memory.SkillBoneSpear}
	err := actions.CastTownPortal(now, player)
	if !errors.Is(err, profile.ErrSkillSelectionPending) || in.selectCalls != 1 || len(in.clickCalls) != 0 {
		t.Fatalf("select phase err=%v selects=%d clicks=%v", err, in.selectCalls, in.clickCalls)
	}
	player.RightSkillID = memory.SkillTownPortal
	if err := actions.CastTownPortal(now.Add(time.Millisecond), player); err != nil {
		t.Fatal(err)
	}
	if len(in.clickCalls) != 1 || in.moveCalls != 1 {
		t.Fatalf("confirm phase moves=%d clicks=%v", in.moveCalls, in.clickCalls)
	}
}

func TestRunActionsCastTownPortalAcceptsSlingRightSkill(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := testBindings()
	combat := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Millisecond)
	actions := newRunActionsAdapter(config.NewLogger("error"), in, bindings, combat)
	player := world.Player{RightSkillID: memory.MustSkillID("townportal_o_skill")}
	if err := actions.CastTownPortal(time.Now(), player); err != nil {
		t.Fatal(err)
	}
	if in.selectCalls != 0 || len(in.clickCalls) != 1 {
		t.Fatalf("sling TP cast selects=%d clicks=%v", in.selectCalls, in.clickCalls)
	}
}
