package app

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type offlineSelectionMock struct {
	window input.WindowInfo
	movedX int
	movedY int
	clicks int
}

func (m *offlineSelectionMock) Bind(uint32) error                                       { return nil }
func (m *offlineSelectionMock) Unbind()                                                 {}
func (m *offlineSelectionMock) Bound() bool                                             { return true }
func (m *offlineSelectionMock) Ready() bool                                             { return true }
func (m *offlineSelectionMock) Status() input.Status                                    { return input.Status{Enabled: true} }
func (m *offlineSelectionMock) CastBelt(input.BeltBindingSource, int) error             { return nil }
func (m *offlineSelectionMock) CastSkillAt(input.BindingSource, uint16, int, int) error { return nil }
func (m *offlineSelectionMock) MoveTo(x, y int) error                                   { m.movedX, m.movedY = x, y; return nil }
func (m *offlineSelectionMock) ClickWithModifier(string, input.MouseButton) error       { return nil }
func (m *offlineSelectionMock) PressKey(string) error                                   { return nil }
func (m *offlineSelectionMock) Focus() error                                            { return nil }
func (m *offlineSelectionMock) Window() (input.WindowInfo, bool)                        { return m.window, true }
func (m *offlineSelectionMock) TogglePause(string) bool                                 { return false }
func (m *offlineSelectionMock) Stop(string)                                             {}
func (m *offlineSelectionMock) ListenHotkeys(context.Context, chan<- input.HotkeyEvent, chan<- error) {
}
func (m *offlineSelectionMock) Click(input.MouseButton) error { m.clicks++; return nil }
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

func TestOfflineStartMachineCompletesAfterStableIdentity(t *testing.T) {
	now := time.Now()
	machine := &offlineStartMachine{character: "MrBones"}
	menu := world.State{Phase: world.GamePhaseMenu}
	for i := 0; i < offlineExitStableTicks; i++ {
		action, _, err := machine.tick(now.Add(time.Duration(i)*time.Millisecond), menu)
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
