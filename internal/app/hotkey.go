package app

import (
	"context"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
)

func (rt *Runtime) handleHotkeyEvent(ev input.HotkeyEvent, cancel context.CancelFunc) {
	switch ev.Action {
	case input.HotkeyActionPause:
		if rt.pauseHotkeyHandler != nil {
			if err := rt.pauseHotkeyHandler(); err != nil {
				rt.Log.Warn("pause-after-run hotkey rejected", "error", err)
			} else {
				rt.Log.Info("pause-after-run hotkey accepted")
			}
			return
		}
		rt.Input.TogglePause("hotkey")
	case input.HotkeyActionStopAfterRun:
		if rt.stopAfterRunHotkeyHandler == nil {
			rt.Log.Warn("stop-after-run hotkey ignored outside an active queue")
			return
		}
		if err := rt.stopAfterRunHotkeyHandler(); err != nil {
			rt.Log.Warn("stop-after-run hotkey rejected", "error", err)
		} else {
			rt.Log.Info("stop-after-run hotkey accepted")
		}
	case input.HotkeyActionStop:
		rt.Input.Stop("hotkey")
		cancel()
	}
}

func (rt *Runtime) setPauseHotkeyHandler(handler func() error) {
	rt.pauseHotkeyHandler = handler
}

func (rt *Runtime) setStopAfterRunHotkeyHandler(handler func() error) {
	rt.stopAfterRunHotkeyHandler = handler
}
