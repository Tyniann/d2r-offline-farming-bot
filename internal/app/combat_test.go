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
	castCalls   int
	selectCalls int
	clickCalls  []input.MouseButton
	lastSkill   uint16
}

func (r *recordingCombatInput) CastSkillAt(_ input.BindingSource, skillID uint16, _, _ int) error {
	r.castCalls++
	r.lastSkill = skillID
	return nil
}

func (r *recordingCombatInput) SelectSkill(_ input.BindingSource, skillID uint16) error {
	r.selectCalls++
	r.lastSkill = skillID
	return nil
}

func (r *recordingCombatInput) Click(button input.MouseButton) error {
	r.clickCalls = append(r.clickCalls, button)
	return nil
}

func TestCombatAdapterConfirmsRightSkillBeforePulsing(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillBoneSpear: {SkillID: memory.SkillBoneSpear, SelectKey: "f8", CastButton: input.MouseRight},
		memory.SkillTeleport:  {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), 350*time.Millisecond)
	now := time.Now()
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillBonePrison}
	target := world.Position{X: 105, Y: 100}

	if err := adapter.CastAttackAtWorld(now, memory.SkillBoneSpear, player, target); err != nil {
		t.Fatal(err)
	}
	if err := adapter.CastAttackAtWorld(now.Add(100*time.Millisecond), memory.SkillBoneSpear, player, target); err != nil {
		t.Fatal(err)
	}
	if in.selectCalls != 1 || len(in.clickCalls) != 0 {
		t.Fatalf("selectCalls=%d clickCalls=%v before confirmation, want 1/0", in.selectCalls, in.clickCalls)
	}
	player.RightSkillID = memory.SkillBoneSpear
	if err := adapter.CastAttackAtWorld(now.Add(400*time.Millisecond), memory.SkillBoneSpear, player, target); err != nil {
		t.Fatal(err)
	}
	if len(in.clickCalls) != 1 || in.clickCalls[0] != input.MouseRight {
		t.Fatalf("clickCalls=%v, want one confirmed right-click", in.clickCalls)
	}
}

func TestCombatAdapterFailsWhenRightSkillSelectionIsNotConfirmed(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillBoneSpear: {SkillID: memory.SkillBoneSpear, SelectKey: "f8", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), 350*time.Millisecond)
	player := world.Player{Position: world.Position{X: 100, Y: 100}, LeftSkillID: memory.SkillBoneSpear, RightSkillID: memory.SkillBonePrison}
	now := time.Now()
	if err := adapter.CastAttackAtWorld(now, memory.SkillBoneSpear, player, world.Position{X: 105, Y: 100}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.CastAttackAtWorld(now.Add(400*time.Millisecond), memory.SkillBoneSpear, player, world.Position{X: 105, Y: 100}); err == nil {
		t.Fatal("CastAttackAtWorld error = nil, want unconfirmed right-skill failure")
	}
	if len(in.clickCalls) != 0 {
		t.Fatalf("clickCalls=%v, want no click for left-bound F8", in.clickCalls)
	}
}

func TestCombatAdapterTeleportTowardKeepsDesiredDistance(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Millisecond)
	sent, err := adapter.TeleportToward(time.Now(), world.Position{X: 100, Y: 100}, world.Position{X: 200, Y: 100}, 22)
	if err != nil {
		t.Fatal(err)
	}
	if !sent || in.castCalls != 1 || in.lastSkill != memory.SkillTeleport {
		t.Fatalf("sent=%t castCalls=%d lastSkill=%d, want teleport cast", sent, in.castCalls, in.lastSkill)
	}
}

func TestCombatAdapterReportsThrottledTeleportWithoutInput(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Second)
	now := time.Now()
	sent, err := adapter.TeleportToward(now, world.Position{X: 100, Y: 100}, world.Position{X: 120, Y: 100}, 0)
	if err != nil || !sent {
		t.Fatalf("first teleport sent=%t err=%v", sent, err)
	}
	sent, err = adapter.TeleportToward(now.Add(100*time.Millisecond), world.Position{X: 100, Y: 100}, world.Position{X: 120, Y: 100}, 0)
	if err != nil || sent || in.castCalls != 1 {
		t.Fatalf("throttled teleport sent=%t err=%v casts=%d, want no second input", sent, err, in.castCalls)
	}
}

func TestCombatAdapterResetClearsPendingSelection(t *testing.T) {
	in := &recordingCombatInput{}
	adapter := newCombatAdapter(config.NewLogger("error"), in, configBindingSource{}, pathing.DefaultConfig(), time.Millisecond)
	adapter.pendingSkill = memory.SkillBoneSpear
	adapter.Reset()
	if adapter.pendingSkill != 0 {
		t.Fatalf("pendingSkill=%d, want reset", adapter.pendingSkill)
	}
}
