package app

import (
	"context"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
)

func (rt *Runtime) handleHotkeyEvent(ev input.HotkeyEvent, cancel context.CancelFunc) {
	switch ev.Action {
	case input.HotkeyActionPause:
		rt.Input.TogglePause("hotkey")
	case input.HotkeyActionStop:
		rt.Input.Stop("hotkey")
		cancel()
	}
}
