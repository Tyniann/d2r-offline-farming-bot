package app

import (
	"context"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
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
func (m *offlineSelectionMock) Window() (input.WindowInfo, bool)                        { return m.window, true }
func (m *offlineSelectionMock) TogglePause(string) bool                                 { return false }
func (m *offlineSelectionMock) Stop(string)                                             {}
func (m *offlineSelectionMock) ListenHotkeys(context.Context, chan<- input.HotkeyEvent, chan<- error) {
}
func (m *offlineSelectionMock) Click(input.MouseButton) error { m.clicks++; return nil }

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
