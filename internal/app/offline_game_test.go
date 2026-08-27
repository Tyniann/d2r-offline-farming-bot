package app

import (
	"context"
	"image"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type offlineSelectionMock struct {
	window input.WindowInfo
	movedX int
	movedY int
	clicks int
}

func (m *offlineSelectionMock) Bind(uint32) error                           { return nil }
func (m *offlineSelectionMock) Unbind()                                     {}
func (m *offlineSelectionMock) Bound() bool                                 { return true }
func (m *offlineSelectionMock) Ready() bool                                 { return true }
func (m *offlineSelectionMock) Status() input.Status                        { return input.Status{Enabled: true} }
func (m *offlineSelectionMock) CastBelt(input.BeltBindingSource, int) error { return nil }
func (m *offlineSelectionMock) CastBeltWithModifier(input.BeltBindingSource, string, int) error {
	return nil
}
func (m *offlineSelectionMock) CastSkillAt(input.BindingSource, uint16, int, int) error { return nil }
func (m *offlineSelectionMock) SelectSkill(input.BindingSource, uint16) error           { return nil }
func (m *offlineSelectionMock) MoveTo(x, y int) error                                   { m.movedX, m.movedY = x, y; return nil }
func (m *offlineSelectionMock) Click(input.MouseButton) error                           { m.clicks++; return nil }
func (m *offlineSelectionMock) ClickWithModifier(string, input.MouseButton) error       { return nil }
func (m *offlineSelectionMock) ClickAtWithModifier(x, y int, _ string, _ input.MouseButton) error {
	m.movedX, m.movedY = x, y
	m.clicks++
	return nil
}
func (m *offlineSelectionMock) PressKey(string) error            { return nil }
func (m *offlineSelectionMock) SendChatCommand(string) error     { return nil }
func (m *offlineSelectionMock) Focus() error                     { return nil }
func (m *offlineSelectionMock) Window() (input.WindowInfo, bool) { return m.window, true }
func (m *offlineSelectionMock) TogglePause(string) bool          { return false }
func (m *offlineSelectionMock) Stop(string)                      {}
func (m *offlineSelectionMock) ListenHotkeys(context.Context, chan<- input.HotkeyEvent, chan<- error) {
}
func (m *offlineSelectionMock) CaptureClient() (*image.RGBA, error) {
	return image.NewRGBA(image.Rect(0, 0, 1280, 720)), nil
}

func TestSelectOfflineDifficulty(t *testing.T) {
	mock := &offlineSelectionMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	if err := selectOfflineDifficulty(mock, offlineDifficultyNightmare); err != nil {
		t.Fatal(err)
	}
	if mock.movedX != 640 || mock.movedY != 355 || mock.clicks != 1 {
		t.Fatalf("selection = (%d,%d) clicks=%d", mock.movedX, mock.movedY, mock.clicks)
	}
}

func TestSelectOfflineDifficultyRejectsResolution(t *testing.T) {
	mock := &offlineSelectionMock{window: input.WindowInfo{ClientWidth: 1920, ClientHeight: 1080}}
	if err := selectOfflineDifficulty(mock, offlineDifficultyHell); err == nil {
		t.Fatal("expected resolution error")
	}
}

func TestInstalledOfflineAnchorsResolveBelowAbsoluteConfigDirectory(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{LoadedFrom: filepath.Join(root, "configs", "config.yaml")}
	paths := []string{
		cfg.ResolvePath(filepath.Join("ui", "characters", "mrbones-selected.png")),
		cfg.ResolvePath(filepath.Join("ui", "character-play.png")),
		cfg.ResolvePath(filepath.Join("ui", "difficulty-dialog.png")),
	}
	for _, path := range paths {
		if !filepath.IsAbs(path) || filepath.Dir(path) == "." {
			t.Fatalf("installed anchor path is not absolute: %q", path)
		}
		if rel, err := filepath.Rel(filepath.Join(root, "configs"), path); err != nil || rel == ".." || filepath.IsAbs(rel) {
			t.Fatalf("installed anchor escaped config directory: path=%q rel=%q err=%v", path, rel, err)
		}
	}
}

func TestOfflineStartMachineCompletesAfterStableIdentity(t *testing.T) {
	now := time.Now()
	machine := &offlineStartMachine{character: "MrBones"}
	menu := world.State{Phase: world.GamePhaseMenu}
	for i := 0; i < offlineExitStableTicks; i++ {
		tickAt := now.Add(time.Duration(i) * time.Millisecond)
		if i == offlineExitStableTicks-1 {
			tickAt = now.Add(offlineCharacterSettleDelay)
		}
		action, _, err := machine.tick(tickAt, menu)
		if err != nil {
			t.Fatal(err)
		}
		if i == offlineExitStableTicks-1 && action != offlineStartVerifyCharacter {
			t.Fatalf("action = %d", action)
		}
	}
	machine.advance(offlineStartAwaitDifficulty, now)
	for i := 0; i < offlineExitStableTicks; i++ {
		action, _, err := machine.tick(now.Add(time.Duration(i)*time.Millisecond), menu)
		if err != nil {
			t.Fatal(err)
		}
		if i == offlineExitStableTicks-1 && action != offlineStartVerifyDifficulty {
			t.Fatalf("action = %d", action)
		}
	}
	machine.advance(offlineStartAwaitGame, now)
	inGame := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.Area{ID: world.RogueEncampment}, Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones"}}
	for i := 0; i < offlineExitStableTicks; i++ {
		_, done, err := machine.tick(now.Add(time.Duration(i)*time.Millisecond), inGame)
		if err != nil {
			t.Fatal(err)
		}
		if done != (i == offlineExitStableTicks-1) {
			t.Fatalf("tick %d done = %t", i, done)
		}
	}
}

func TestOfflineStartMachineAcceptsSupportedForeignTownForNormalization(t *testing.T) {
	now := time.Now()
	machine := &offlineStartMachine{stage: offlineStartAwaitGame, character: "MrHammer"}
	inGame := world.State{
		Valid:    true,
		Phase:    world.GamePhaseInGame,
		Area:     world.LookupArea(world.KurastDocks),
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrHammer"},
	}
	for i := 0; i < offlineExitStableTicks; i++ {
		_, done, err := machine.tick(now.Add(time.Duration(i)*time.Millisecond), inGame)
		if err != nil {
			t.Fatal(err)
		}
		if done != (i == offlineExitStableTicks-1) {
			t.Fatalf("tick %d done = %t", i, done)
		}
	}
}

func TestOfflineStartMachineRejectsUnsupportedFreshGameArea(t *testing.T) {
	machine := &offlineStartMachine{stage: offlineStartAwaitGame, character: "MrHammer"}
	state := world.State{
		Valid:    true,
		Phase:    world.GamePhaseInGame,
		Area:     world.LookupArea(world.BloodMoor),
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrHammer"},
	}
	if _, _, err := machine.tick(time.Now(), state); err == nil {
		t.Fatal("unsupported fresh game area was accepted")
	}
}

func TestOfflineStartMachineRejectsWrongCharacterClass(t *testing.T) {
	now := time.Now()
	machine := &offlineStartMachine{stage: offlineStartAwaitGame, character: "MrBones", expectedClass: world.CharacterClassNecromancer, verifyClass: true}
	state := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.Area{ID: world.RogueEncampment}, Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassPaladin}}
	if _, _, err := machine.tick(now, state); err == nil {
		t.Fatal("wrong character class was accepted")
	}
}

func TestOfflineStartMachineTimesOutWithoutMenu(t *testing.T) {
	now := time.Now()
	machine := &offlineStartMachine{}
	_, _, _ = machine.tick(now, world.State{})
	if _, _, err := machine.tick(now.Add(offlineStartTimeout), world.State{}); err == nil {
		t.Fatal("offline start did not time out")
	}
}
