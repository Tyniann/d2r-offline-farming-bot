package app

import (
	"errors"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type recordingCombatInput struct {
	mockInput
	castCalls      int
	selectCalls    int
	moveCalls      int
	clickCalls     []input.MouseButton
	modifiedClicks []modifiedClick
	holds          []modifiedClick
	releaseCalls   int
	holdActive     bool
	pressedKeys    []string
	lastSkill      uint16
}

type modifiedClick struct {
	clientX, clientY int
	modifier         string
	button           input.MouseButton
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

func (r *recordingCombatInput) ClickAtWithModifier(clientX, clientY int, modifier string, button input.MouseButton) error {
	r.modifiedClicks = append(r.modifiedClicks, modifiedClick{clientX: clientX, clientY: clientY, modifier: modifier, button: button})
	r.lastClientX = clientX
	r.lastClientY = clientY
	return nil
}

func (r *recordingCombatInput) HoldAt(clientX, clientY int, button input.MouseButton) error {
	r.holds = append(r.holds, modifiedClick{clientX: clientX, clientY: clientY, button: button})
	r.lastClientX = clientX
	r.lastClientY = clientY
	r.holdActive = true
	return nil
}

func (r *recordingCombatInput) ReleaseModifierHold() error {
	r.releaseCalls++
	r.holdActive = false
	return nil
}

func (r *recordingCombatInput) ModifierHoldActive() bool {
	return r.holdActive
}

func (r *recordingCombatInput) MoveTo(clientX, clientY int) error {
	r.moveCalls++
	r.lastClientX = clientX
	r.lastClientY = clientY
	return r.mockInput.MoveTo(clientX, clientY)
}

func (r *recordingCombatInput) PressKey(key string) error {
	r.pressedKeys = append(r.pressedKeys, key)
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

	if sent, err := adapter.CastAttackAtWorld(now, memory.SkillBoneSpear, player, target); err != nil || sent {
		t.Fatalf("selection sent=%t err=%v, want no attack click", sent, err)
	}
	if sent, err := adapter.CastAttackAtWorld(now.Add(100*time.Millisecond), memory.SkillBoneSpear, player, target); err != nil || sent {
		t.Fatalf("throttled confirmation sent=%t err=%v, want no attack click", sent, err)
	}
	if in.selectCalls != 1 || len(in.clickCalls) != 0 {
		t.Fatalf("selectCalls=%d clickCalls=%v before confirmation, want 1/0", in.selectCalls, in.clickCalls)
	}
	player.RightSkillID = memory.SkillBoneSpear
	if sent, err := adapter.CastAttackAtWorld(now.Add(400*time.Millisecond), memory.SkillBoneSpear, player, target); err != nil || !sent {
		t.Fatalf("confirmed attack sent=%t err=%v, want click", sent, err)
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
	adapter.selector.timeout = 300 * time.Millisecond
	player := world.Player{Position: world.Position{X: 100, Y: 100}, LeftSkillID: memory.SkillBoneSpear, RightSkillID: memory.SkillBonePrison}
	now := time.Now()
	if sent, err := adapter.CastAttackAtWorld(now, memory.SkillBoneSpear, player, world.Position{X: 105, Y: 100}); err != nil || sent {
		t.Fatalf("selection sent=%t err=%v, want no attack click", sent, err)
	}
	if _, err := adapter.CastAttackAtWorld(now.Add(400*time.Millisecond), memory.SkillBoneSpear, player, world.Position{X: 105, Y: 100}); err == nil {
		t.Fatal("CastAttackAtWorld error = nil, want unconfirmed right-skill failure")
	}
	if len(in.clickCalls) != 0 {
		t.Fatalf("clickCalls=%v, want no click for left-bound F8", in.clickCalls)
	}
}

func TestCombatAdapterClicksOnlyHoverConfirmedLivingMonster(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillBoneSpear: {SkillID: memory.SkillBoneSpear, SelectKey: "f8", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), 350*time.Millisecond)
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillBoneSpear}
	target := world.Monster{NPCID: 131, UnitID: 42, Position: world.Position{X: 105, Y: 100}}
	now := time.Now()

	if cast, err := adapter.CastAttackAtMonster(now, memory.SkillBoneSpear, player, target); err != nil || cast.Sent || !cast.AimRequested {
		t.Fatalf("initial aim result=%+v err=%v, want no click before hover proof", cast, err)
	}
	if len(in.clickCalls) != 0 || adapter.pendingTargetUnitID != target.UnitID {
		t.Fatalf("clicks=%v pending_target=%d, want aim only for %d", in.clickCalls, adapter.pendingTargetUnitID, target.UnitID)
	}

	target.IsHovered = true
	if cast, err := adapter.CastAttackAtMonster(now.Add(100*time.Millisecond), memory.SkillBoneSpear, player, target); err != nil || !cast.Sent || cast.TargetingMode != profile.MonsterTargetingHoverConfirmed {
		t.Fatalf("hover-confirmed cast result=%+v err=%v, want click", cast, err)
	}
	if len(in.clickCalls) != 1 || in.clickCalls[0] != input.MouseRight {
		t.Fatalf("clicks=%v, want one confirmed right-click", in.clickCalls)
	}

	nearer := world.Monster{NPCID: 56, UnitID: 43, Position: world.Position{X: 102, Y: 100}}
	if cast, err := adapter.CastAttackAtMonster(now.Add(500*time.Millisecond), memory.SkillBoneSpear, player, nearer); err != nil || cast.Sent {
		t.Fatalf("changed target result=%+v err=%v, want fresh aim before click", cast, err)
	}
	if len(in.clickCalls) != 1 || adapter.pendingTargetUnitID != nearer.UnitID {
		t.Fatalf("clicks=%v pending_target=%d, want retarget without stale click", in.clickCalls, adapter.pendingTargetUnitID)
	}
}

func TestCombatAdapterSkillSelectionDoesNotAuthorizeTargetHover(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillBoneSpear: {SkillID: memory.SkillBoneSpear, SelectKey: "f8", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), 350*time.Millisecond)
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillTeleport}
	target := world.Monster{NPCID: world.Nihlathak, UnitID: 42, Position: world.Position{X: 105, Y: 100}}

	cast, err := adapter.CastAttackAtMonster(time.Now(), memory.SkillBoneSpear, player, target)
	if err != nil || cast.Sent || cast.AimRequested {
		t.Fatalf("skill selection result=%+v err=%v, want neither cast nor aim authorization", cast, err)
	}
}

func TestCombatAdapterSearchesVisibleBodyWithoutBlindClick(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillBoneSpear: {SkillID: memory.SkillBoneSpear, SelectKey: "f8", CastButton: input.MouseRight},
	}}
	cfg := pathing.DefaultConfig()
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, cfg, 350*time.Millisecond)
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillBoneSpear}
	target := world.Monster{NPCID: world.ArcaneSpecter, UnitID: 46, Position: world.Position{X: 105, Y: 100}}
	now := time.Now()

	if cast, err := adapter.CastAttackAtMonster(now, memory.SkillBoneSpear, player, target); err != nil || cast.Sent {
		t.Fatalf("first probe result=%+v err=%v", cast, err)
	}
	firstX, firstY := in.lastClientX, in.lastClientY
	if cast, err := adapter.CastAttackAtMonster(now.Add(100*time.Millisecond), memory.SkillBoneSpear, player, target); err != nil || cast.Sent {
		t.Fatalf("second probe result=%+v err=%v", cast, err)
	}
	if in.lastClientX == firstX && in.lastClientY == firstY {
		t.Fatalf("monster hover search repeated (%d,%d)", firstX, firstY)
	}
	if len(in.clickCalls) != 0 {
		t.Fatalf("unconfirmed hover search clicked: %v", in.clickCalls)
	}
	target.IsHovered = true
	if cast, err := adapter.CastAttackAtMonster(now.Add(400*time.Millisecond), memory.SkillBoneSpear, player, target); err != nil || !cast.Sent || cast.TargetingMode != profile.MonsterTargetingHoverConfirmed {
		t.Fatalf("confirmed hover result=%+v err=%v", cast, err)
	}
	if len(in.clickCalls) != 1 || in.clickCalls[0] != input.MouseRight {
		t.Fatalf("confirmed hover clicks=%v", in.clickCalls)
	}
}

func TestCombatAdapterUsesPlayableProjectionAfterHoverAttemptLimit(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillBoneSpear: {SkillID: memory.SkillBoneSpear, SelectKey: "f8", CastButton: input.MouseRight},
	}}
	cfg := pathing.DefaultConfig()
	cfg.Click.MaxHoverAttempts = 3
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, cfg, 350*time.Millisecond)
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillBoneSpear}
	target := world.Monster{NPCID: world.HellBovine, UnitID: 46, Position: world.Position{X: 105, Y: 100}}
	now := time.Now()

	for attempt := 0; attempt < cfg.Click.MaxHoverAttempts; attempt++ {
		if cast, err := adapter.CastAttackAtMonster(now.Add(time.Duration(attempt)*100*time.Millisecond), memory.SkillBoneSpear, player, target); err != nil || cast.Sent {
			t.Fatalf("probe %d result=%+v err=%v", attempt+1, cast, err)
		}
	}
	cast, err := adapter.CastAttackAtMonster(now.Add(400*time.Millisecond), memory.SkillBoneSpear, player, target)
	if err != nil || !cast.Sent || cast.TargetingMode != profile.MonsterTargetingWorldProjected {
		t.Fatalf("projected fallback result=%+v err=%v", cast, err)
	}
	if in.moveCalls != cfg.Click.MaxHoverAttempts+1 || len(in.clickCalls) != 1 {
		t.Fatalf("moves=%d clicks=%v", in.moveCalls, in.clickCalls)
	}
}

func TestCombatAdapterCapsMonsterHoverSearchBelowStaticEntityBudget(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillBoneSpear: {SkillID: memory.SkillBoneSpear, SelectKey: "f8", CastButton: input.MouseRight},
	}}
	cfg := pathing.DefaultConfig()
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, cfg, 350*time.Millisecond)
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillBoneSpear}
	target := world.Monster{NPCID: world.HellBovine, UnitID: 47, Position: world.Position{X: 105, Y: 100}}
	now := time.Now()

	for attempt := 0; attempt < combatMonsterMaxHoverAttempts; attempt++ {
		if cast, err := adapter.CastAttackAtMonster(now.Add(time.Duration(attempt)*100*time.Millisecond), memory.SkillBoneSpear, player, target); err != nil || cast.Sent {
			t.Fatalf("probe %d result=%+v err=%v", attempt+1, cast, err)
		}
	}
	cast, err := adapter.CastAttackAtMonster(now.Add(time.Second), memory.SkillBoneSpear, player, target)
	if err != nil || !cast.Sent || cast.TargetingMode != profile.MonsterTargetingWorldProjected {
		t.Fatalf("capped search result=%+v err=%v", cast, err)
	}
	if in.moveCalls != combatMonsterMaxHoverAttempts+1 || adapter.hoverProbe.MaxHoverAttempts != combatMonsterMaxHoverAttempts {
		t.Fatalf("moves=%d adapter limit=%d", in.moveCalls, adapter.hoverProbe.MaxHoverAttempts)
	}
}

func TestCombatAdapterReportsOffscreenMonsterWithoutMovingOrClicking(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillBoneSpear: {SkillID: memory.SkillBoneSpear, SelectKey: "f8", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), 350*time.Millisecond)
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillBoneSpear}
	target := world.Monster{NPCID: world.ArcaneGhoulLord, UnitID: 83, Position: world.Position{X: 140, Y: 100}}

	cast, err := adapter.CastAttackAtMonster(time.Now(), memory.SkillBoneSpear, player, target)
	if cast.Sent || !errors.Is(err, profile.ErrRouteClearTargetUnprojectable) {
		t.Fatalf("offscreen target result=%+v err=%v", cast, err)
	}
	if in.moveCalls != 0 || len(in.clickCalls) != 0 {
		t.Fatalf("offscreen target moves=%d clicks=%v", in.moveCalls, in.clickCalls)
	}
}

func TestCombatAdapterTeleportTowardClampsBelowPlayableHUD(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Millisecond)
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillTeleport}
	sent, err := adapter.TeleportToward(time.Now(), player, world.Position{X: 120, Y: 120}, 0)
	if err != nil || !sent {
		t.Fatalf("sent=%t err=%v", sent, err)
	}
	win, ok := in.Window()
	if !ok {
		t.Fatal("expected bound test window")
	}
	_, maxY := pathing.ClampTeleportClientPoint(0, win.ClientHeight, win)
	if in.lastClientY != maxY {
		t.Fatalf("clientY=%d, want playable clamp %d (74%% of %d)", in.lastClientY, maxY, win.ClientHeight)
	}
}

func TestCombatAdapterTeleportTowardKeepsDesiredDistance(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Millisecond)
	sent, err := adapter.TeleportToward(time.Now(), world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillTeleport}, world.Position{X: 200, Y: 100}, 22)
	if err != nil {
		t.Fatal(err)
	}
	if !sent || in.moveCalls != 1 || len(in.clickCalls) != 1 || in.selectCalls != 0 {
		t.Fatalf("sent=%t moves=%d clicks=%v selects=%d, want confirmed teleport click without F-key", sent, in.moveCalls, in.clickCalls, in.selectCalls)
	}
}

func TestCombatAdapterReportsThrottledTeleportWithoutInput(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Second)
	now := time.Now()
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillTeleport}
	sent, err := adapter.TeleportToward(now, player, world.Position{X: 120, Y: 100}, 0)
	if err != nil || !sent {
		t.Fatalf("first teleport sent=%t err=%v", sent, err)
	}
	sent, err = adapter.TeleportToward(now.Add(100*time.Millisecond), player, world.Position{X: 120, Y: 100}, 0)
	if err != nil || sent || in.moveCalls != 1 || len(in.clickCalls) != 1 {
		t.Fatalf("throttled teleport sent=%t err=%v moves=%d clicks=%v, want no second input", sent, err, in.moveCalls, in.clickCalls)
	}
}

func TestCombatAdapterTeleportSelectsBeforeClick(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f7", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Millisecond)
	now := time.Now()
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillBoneSpear}
	sent, err := adapter.TeleportToward(now, player, world.Position{X: 140, Y: 100}, 0)
	if err != nil || sent || in.selectCalls != 1 || len(in.clickCalls) != 0 {
		t.Fatalf("select phase sent=%t err=%v selects=%d clicks=%v", sent, err, in.selectCalls, in.clickCalls)
	}
	player.RightSkillID = memory.SkillTeleport
	sent, err = adapter.TeleportToward(now.Add(time.Millisecond), player, world.Position{X: 140, Y: 100}, 0)
	if err != nil || !sent || len(in.clickCalls) != 1 || in.selectCalls != 1 {
		t.Fatalf("confirm phase sent=%t err=%v selects=%d clicks=%v", sent, err, in.selectCalls, in.clickCalls)
	}
}

func TestCombatAdapterForceMovesWithConfiguredTownBinding(t *testing.T) {
	in := &recordingCombatInput{}
	cfg := pathing.DefaultConfig()
	cfg.TownWalk.ForceMoveKey = "e"
	adapter := newCombatAdapter(config.NewLogger("error"), in, configBindingSource{}, cfg, time.Millisecond)
	target := world.Position{X: 112, Y: 100}
	sent, err := adapter.ForceMoveToward(time.Now(), world.Position{X: 100, Y: 100}, target)
	if err != nil || !sent {
		t.Fatalf("force move sent=%t err=%v", sent, err)
	}
	if in.moveCalls != 1 || len(in.pressedKeys) != 1 || in.pressedKeys[0] != "e" {
		t.Fatalf("moves=%d keys=%v", in.moveCalls, in.pressedKeys)
	}
}

func TestCombatAdapterResetClearsPendingSelection(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		memory.SkillBoneSpear: {SkillID: memory.SkillBoneSpear, SelectKey: "f8", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Millisecond)
	_, _ = adapter.selector.EnsureAndCast(memory.SkillBoneSpear, memory.SkillTeleport, time.Now(), func() error { return nil })
	if adapter.selector.pending == 0 {
		t.Fatal("expected pending selection before reset")
	}
	adapter.Reset()
	if adapter.selector.pending != 0 {
		t.Fatalf("pending=%d, want reset", adapter.selector.pending)
	}
}

func hammerdinCombatBindings() configBindingSource {
	hammer := memory.MustSkillID("blessed_hammer")
	concentration := memory.MustSkillID("concentration")
	return configBindingSource{skills: map[uint16]input.SkillCast{
		hammer:               {SkillID: hammer, SelectKey: "f1", CastButton: input.MouseLeft},
		concentration:        {SkillID: concentration, SelectKey: "f2", CastButton: input.MouseRight},
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f3", CastButton: input.MouseRight},
	}}
}

func hammerdinBoss(hovered bool) world.Monster {
	return world.Monster{UnitID: 42, NPCID: 242, Position: world.Position{X: 105, Y: 100}, IsHovered: hovered}
}

func TestCombatAdapterHoldsBlessedHammerOnHoveredMonster(t *testing.T) {
	in := &recordingCombatInput{}
	adapter := newCombatAdapter(config.NewLogger("error"), in, hammerdinCombatBindings(), pathing.DefaultConfig(), 350*time.Millisecond)
	hammer := memory.MustSkillID("blessed_hammer")
	player := world.Player{
		Position:     world.Position{X: 100, Y: 100},
		LeftSkillID:  hammer,
		RightSkillID: memory.MustSkillID("concentration"),
	}
	boss := hammerdinBoss(true)

	result, err := adapter.HoldStandardAttack(time.Now(), hammer, player, boss)
	if err != nil || !result.Sent || result.TargetingMode != profile.MonsterTargetingHoverConfirmed {
		t.Fatalf("hammer hold=%+v err=%v, want hover-confirmed LMB", result, err)
	}
	if len(in.clickCalls) != 0 || len(in.modifiedClicks) != 0 {
		t.Fatalf("clicks=%v modified=%v, want none", in.clickCalls, in.modifiedClicks)
	}
	if len(in.holds) != 1 {
		t.Fatalf("holds=%v, want one LMB hold", in.holds)
	}
	got := in.holds[0]
	if got.clientX == 640 && got.clientY == 360 {
		t.Fatalf("hold=%+v, want monster projection not client center", got)
	}
	if got.modifier != "" || got.button != input.MouseLeft {
		t.Fatalf("hold=%+v, want left without modifier", got)
	}
	again, err := adapter.HoldStandardAttack(time.Now().Add(time.Second), hammer, player, boss)
	if err != nil || again.Sent || len(in.holds) != 1 {
		t.Fatalf("second hold sent=%t err=%v holds=%d, want no-op", again.Sent, err, len(in.holds))
	}
	if err := adapter.StopAttack(); err != nil || in.releaseCalls != 1 || in.holdActive {
		t.Fatalf("release calls=%d active=%t err=%v", in.releaseCalls, in.holdActive, err)
	}
}

func TestCombatAdapterAimsAtMonsterBeforeHammerHold(t *testing.T) {
	in := &recordingCombatInput{}
	adapter := newCombatAdapter(config.NewLogger("error"), in, hammerdinCombatBindings(), pathing.DefaultConfig(), 350*time.Millisecond)
	hammer := memory.MustSkillID("blessed_hammer")
	player := world.Player{
		Position:     world.Position{X: 100, Y: 100},
		LeftSkillID:  hammer,
		RightSkillID: memory.MustSkillID("concentration"),
	}
	boss := hammerdinBoss(false)

	aimed, err := adapter.HoldStandardAttack(time.Now(), hammer, player, boss)
	if err != nil || aimed.Sent || !aimed.AimRequested || len(in.holds) != 0 || in.moveCalls != 1 {
		t.Fatalf("aim=%+v err=%v holds=%d moves=%d, want aim move and no hold", aimed, err, len(in.holds), in.moveCalls)
	}

	overlay := hammerdinBoss(true)
	overlay.UnitID = 99
	held, err := adapter.HoldStandardAttack(time.Now().Add(400*time.Millisecond), hammer, player, overlay)
	if err != nil || !held.Sent || len(in.holds) != 1 {
		t.Fatalf("overlay hold=%+v err=%v holds=%d", held, err, len(in.holds))
	}
	if in.holds[0].clientX == 640 && in.holds[0].clientY == 360 {
		t.Fatal("overlay hold used client center")
	}
}

func TestCombatAdapterReconfirmsConcentrationAfterTeleportBeforeHammer(t *testing.T) {
	in := &recordingCombatInput{}
	adapter := newCombatAdapter(config.NewLogger("error"), in, hammerdinCombatBindings(), pathing.DefaultConfig(), 350*time.Millisecond)
	hammer := memory.MustSkillID("blessed_hammer")
	concentration := memory.MustSkillID("concentration")
	now := time.Now()
	player := world.Player{
		Position:     world.Position{X: 100, Y: 100},
		LeftSkillID:  hammer,
		RightSkillID: memory.SkillTeleport,
	}

	boss := hammerdinBoss(false)
	sent, err := adapter.HoldStandardAttack(now, hammer, player, boss)
	if err != nil || sent.Sent {
		t.Fatalf("concentration select sent=%t err=%v, want no hammer", sent.Sent, err)
	}
	if in.selectCalls != 1 || in.lastSkill != concentration || in.moveCalls != 1 || len(in.holds) != 0 || len(in.clickCalls) != 0 {
		t.Fatalf("selects=%d last=%d moves=%d holds=%v clicks=%v, want Concentration and overlapped aim", in.selectCalls, in.lastSkill, in.moveCalls, in.holds, in.clickCalls)
	}

	player.RightSkillID = concentration
	boss.IsHovered = true
	sent, err = adapter.HoldStandardAttack(now.Add(400*time.Millisecond), hammer, player, boss)
	if err != nil || !sent.Sent {
		t.Fatalf("hammer after aura sent=%t err=%v", sent.Sent, err)
	}
	if len(in.holds) != 1 || in.holds[0].button != input.MouseLeft || in.holds[0].modifier != "" || (in.holds[0].clientX == 640 && in.holds[0].clientY == 360) {
		t.Fatalf("holds=%v, want one LMB hold on the monster after Concentration", in.holds)
	}
}

func TestCombatAdapterFailsWhenLeftSkillSelectionIsNotConfirmed(t *testing.T) {
	in := &recordingCombatInput{}
	adapter := newCombatAdapter(config.NewLogger("error"), in, hammerdinCombatBindings(), pathing.DefaultConfig(), 350*time.Millisecond)
	adapter.skills.timeout = 300 * time.Millisecond
	hammer := memory.MustSkillID("blessed_hammer")
	now := time.Now()
	player := world.Player{
		Position:     world.Position{X: 100, Y: 100},
		RightSkillID: memory.MustSkillID("concentration"),
	}

	boss := hammerdinBoss(true)
	sent, err := adapter.HoldStandardAttack(now, hammer, player, boss)
	if err != nil || sent.Sent {
		t.Fatalf("selection sent=%t err=%v, want no hammer hold", sent.Sent, err)
	}
	if _, err := adapter.HoldStandardAttack(now.Add(400*time.Millisecond), hammer, player, boss); err == nil {
		t.Fatal("HoldStandardAttack error = nil, want unconfirmed left-skill failure")
	}
	if len(in.holds) != 0 || len(in.clickCalls) != 0 {
		t.Fatalf("holds=%v clicks=%v, want none while LeftSkillID stays unconfirmed", in.holds, in.clickCalls)
	}
}

func TestCombatAdapterCastAttackAtMonsterHoldsLeftMouseBlessedHammer(t *testing.T) {
	in := &recordingCombatInput{}
	adapter := newCombatAdapter(config.NewLogger("error"), in, hammerdinCombatBindings(), pathing.DefaultConfig(), 350*time.Millisecond)
	hammer := memory.MustSkillID("blessed_hammer")
	player := world.Player{
		Position:     world.Position{X: 100, Y: 100},
		LeftSkillID:  hammer,
		RightSkillID: memory.MustSkillID("concentration"),
	}
	target := world.Monster{UnitID: 11, NPCID: world.ArcaneSpecter, Position: world.Position{X: 102, Y: 100}, IsHovered: true}

	result, err := adapter.CastAttackAtMonster(time.Now(), hammer, player, target)
	if err != nil || !result.Sent || result.TargetingMode != profile.MonsterTargetingHoverConfirmed {
		t.Fatalf("route-clear hammer=%+v err=%v, want LMB hold", result, err)
	}
	if len(in.holds) != 1 || in.holds[0].button != input.MouseLeft || len(in.clickCalls) != 0 {
		t.Fatalf("holds=%v clicks=%v, want one LMB hold", in.holds, in.clickCalls)
	}
}

func TestCombatAdapterRetargetsHammerHoldWhenUnitChanges(t *testing.T) {
	in := &recordingCombatInput{}
	adapter := newCombatAdapter(config.NewLogger("error"), in, hammerdinCombatBindings(), pathing.DefaultConfig(), 350*time.Millisecond)
	hammer := memory.MustSkillID("blessed_hammer")
	player := world.Player{
		Position:     world.Position{X: 100, Y: 100},
		LeftSkillID:  hammer,
		RightSkillID: memory.MustSkillID("concentration"),
	}
	first := world.Monster{UnitID: 11, NPCID: world.ArcaneSpecter, Position: world.Position{X: 102, Y: 100}, IsHovered: true}
	next := world.Monster{UnitID: 22, NPCID: world.ArcaneHellClan, Position: world.Position{X: 101, Y: 100}, IsHovered: true}
	now := time.Now()

	if result, err := adapter.HoldStandardAttack(now, hammer, player, first); err != nil || !result.Sent {
		t.Fatalf("first hold=%+v err=%v", result, err)
	}
	if result, err := adapter.HoldStandardAttack(now.Add(400*time.Millisecond), hammer, player, next); err != nil || !result.Sent {
		t.Fatalf("retarget hold=%+v err=%v", result, err)
	}
	if in.releaseCalls != 1 || len(in.holds) != 2 {
		t.Fatalf("releases=%d holds=%d, want release then second hold", in.releaseCalls, len(in.holds))
	}
}
