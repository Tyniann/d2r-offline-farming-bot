package app

import (
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type recordingCombatInput struct {
	mockInput
	castCalls int
	lastSkill uint16
}

func (r *recordingCombatInput) CastSkillAt(_ input.BindingSource, skillID uint16, _, _ int) error {
	r.castCalls++
	r.lastSkill = skillID
	return nil
}

func TestCombatAdapterThrottlesSkillCasts(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillBoneSpear: {SkillID: memory.SkillBoneSpear, SelectKey: "f8", CastButton: input.MouseLeft},
		memory.SkillTeleport:  {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), 350*time.Millisecond)
	now := time.Now()
	player := world.Position{X: 100, Y: 100}
	target := world.Position{X: 105, Y: 100}

	if err := adapter.CastSkillAtWorld(now, memory.SkillBoneSpear, player, target); err != nil {
		t.Fatal(err)
	}
	if err := adapter.CastSkillAtWorld(now.Add(100*time.Millisecond), memory.SkillBoneSpear, player, target); err != nil {
		t.Fatal(err)
	}
	if in.castCalls != 1 {
		t.Fatalf("castCalls after throttled tick = %d, want 1", in.castCalls)
	}
	if err := adapter.CastSkillAtWorld(now.Add(400*time.Millisecond), memory.SkillBoneSpear, player, target); err != nil {
		t.Fatal(err)
	}
	if in.castCalls != 2 || in.lastSkill != memory.SkillBoneSpear {
		t.Fatalf("castCalls=%d lastSkill=%d, want second Bone Spear cast", in.castCalls, in.lastSkill)
	}
}

func TestCombatAdapterTeleportTowardKeepsDesiredDistance(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Millisecond)
	err := adapter.TeleportToward(time.Now(), world.Position{X: 100, Y: 100}, world.Position{X: 200, Y: 100}, 22)
	if err != nil {
		t.Fatal(err)
	}
	if in.castCalls != 1 || in.lastSkill != memory.SkillTeleport {
		t.Fatalf("castCalls=%d lastSkill=%d, want teleport cast", in.castCalls, in.lastSkill)
	}
}
