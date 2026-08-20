package app

import (
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type recordingChestInput struct {
	preparationInputMock
	x, y int
}

func (m *recordingChestInput) MoveTo(x, y int) error {
	m.x, m.y = x, y
	return m.preparationInputMock.MoveTo(x, y)
}

func TestChestOperateAdapterAimsAtObjectBodyNotGroundTile(t *testing.T) {
	in := &recordingChestInput{}
	cfg := pathing.DefaultConfig()
	adapter := newChestOperateAdapter(config.NewLogger("error"), in, cfg)
	player := world.Position{X: 100, Y: 100}
	target := world.Object{
		ID: world.JungleChest2ID, Kind: world.ObjectKindSuperChest, UnitID: 183,
		Position: world.Position{X: 103, Y: 103}, Name: "Super Chest",
		Mode: world.ObjectModeClosed, ModeKnown: true,
	}
	state := world.State{
		At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.LowerKurast),
		Player: world.Player{Position: player}, Objects: []world.Object{target},
	}
	if first := adapter.Tick(state, target, 8); first.Status != tasks.ChestOperatePending || in.moves != 1 {
		t.Fatalf("aim tick=%+v moves=%d", first, in.moves)
	}
	win := input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}
	bodyX, bodyY, ok := pathing.ProjectHoverProbe(cfg.Projector(), player, target.Position, win, cfg.Click, 0)
	if !ok {
		t.Fatal("body projection failed")
	}
	groundCfg := cfg.Click
	groundCfg.AnchorOffsetTiles = 0
	groundX, groundY, ok := pathing.ProjectHoverProbe(cfg.Projector(), player, target.Position, win, groundCfg, 0)
	if !ok {
		t.Fatal("ground projection failed")
	}
	if in.x != bodyX || in.y != bodyY {
		t.Fatalf("probe = (%d,%d), want object-body (%d,%d)", in.x, in.y, bodyX, bodyY)
	}
	if in.x == groundX && in.y == groundY {
		t.Fatalf("probe landed on ground tile (%d,%d) instead of object body", groundX, groundY)
	}
}

func TestChestOperateAdapterClicksOnHoverConfirm(t *testing.T) {
	in := &preparationInputMock{}
	adapter := newChestOperateAdapter(config.NewLogger("error"), in, pathing.DefaultConfig())
	now := time.Now()
	target := world.Object{
		ID: world.JungleChest2ID, Kind: world.ObjectKindSuperChest, UnitID: 183,
		Position: world.Position{X: 103, Y: 103}, Name: "Super Chest",
		Mode: world.ObjectModeClosed, ModeKnown: true,
	}
	state := world.State{
		At: now, Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.LowerKurast),
		Player:  world.Player{Position: world.Position{X: 100, Y: 100}},
		Objects: []world.Object{target},
	}
	first := adapter.Tick(state, target, 8)
	if first.Status != tasks.ChestOperatePending || first.Done || in.moves != 1 || in.clicks != 0 {
		t.Fatalf("aim tick=%+v moves/clicks=%d/%d", first, in.moves, in.clicks)
	}
	state.At = now.Add(100 * time.Millisecond)
	state.Hover = world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeObject, UnitID: 183}
	second := adapter.Tick(state, target, 8)
	if second.Status != tasks.ChestOperateClicked || !second.Done || in.clicks != 1 {
		t.Fatalf("confirmed tick=%+v clicks=%d", second, in.clicks)
	}
}

func TestChestOperateAdapterReportsMonsterHoverAfterProbe(t *testing.T) {
	in := &preparationInputMock{}
	adapter := newChestOperateAdapter(config.NewLogger("error"), in, pathing.DefaultConfig())
	target := world.Object{
		ID: world.JungleChest2ID, Kind: world.ObjectKindSuperChest, UnitID: 183,
		Position: world.Position{X: 103, Y: 103}, Name: "Super Chest",
		Mode: world.ObjectModeClosed, ModeKnown: true,
	}
	state := world.State{
		At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.LowerKurast),
		Player: world.Player{Position: world.Position{X: 100, Y: 100}}, Objects: []world.Object{target},
	}
	if first := adapter.Tick(state, target, 8); first.Status != tasks.ChestOperatePending {
		t.Fatalf("first tick = %+v", first)
	}
	state.Hover = world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeMonster, UnitID: 42}
	second := adapter.Tick(state, target, 8)
	if second.Status != tasks.ChestOperatePending || second.BlockerUnitID != 42 {
		t.Fatalf("monster blocker tick = %+v, want blocker 42", second)
	}
}
