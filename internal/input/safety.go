package input

import (
	"fmt"
	"log/slog"
)

// SafetyConfig holds operator safety settings for real OS input and global hotkeys.
type SafetyConfig struct {
	Enabled               bool
	PauseHotkey           string
	StopAfterRunHotkey    string
	RecordingFinishHotkey string
	StopHotkey            string
}

// Status reports the current runtime safety state of the input controller.
type Status struct {
	Enabled bool
	Paused  bool
	Stopped bool
}

// Status returns a snapshot of the current safety state.
func (c *Controller) Status() Status {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	return Status{
		Enabled: c.enabled,
		Paused:  c.paused,
		Stopped: c.stopped,
	}
}

// SetEnabled toggles whether real keyboard and mouse actions are permitted.
func (c *Controller) SetEnabled(enabled bool) {
	c.stateMu.Lock()
	c.enabled = enabled
	c.stateMu.Unlock()
}

// Pause blocks real input actions until [Controller.Resume] or [Controller.TogglePause].
func (c *Controller) Pause(reason string) {
	c.stateMu.Lock()
	if c.stopped {
		c.log.Debug("input pause ignored", "reason", reason, "cause", "stopped")
		c.stateMu.Unlock()
		return
	}
	c.paused = true
	c.log.Info("input safety state changed", "paused", true, "reason", reason)
	c.stateMu.Unlock()
}

// Resume clears a pause set by [Controller.Pause] or [Controller.TogglePause].
func (c *Controller) Resume(reason string) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.stopped {
		c.log.Debug("input resume ignored", "reason", reason, "cause", "stopped")
		return
	}
	c.paused = false
	c.log.Info("input safety state changed", "paused", false, "reason", reason)
}

// TogglePause flips the pause state and returns the new paused value.
func (c *Controller) TogglePause(reason string) bool {
	c.stateMu.Lock()
	if c.stopped {
		c.log.Debug("input toggle pause ignored", "reason", reason, "cause", "stopped")
		paused := c.paused
		c.stateMu.Unlock()
		return paused
	}
	c.paused = !c.paused
	paused := c.paused
	c.log.Info("input safety state changed", "paused", paused, "reason", reason)
	c.stateMu.Unlock()
	return paused
}

// Stop marks the controller as terminally stopped; pause/resume become no-ops afterward.
func (c *Controller) Stop(reason string) {
	c.stateMu.Lock()
	if c.stopped {
		c.stateMu.Unlock()
		return
	}
	c.stopped = true
	c.paused = false
	c.log.Info("input safety stop requested", "reason", reason)
	c.stateMu.Unlock()
}

func (c *Controller) actionGuard(kind, action, reason string, attrs ...any) error {
	c.stateMu.Lock()
	stopped := c.stopped
	disabled := !c.enabled
	paused := c.paused
	c.stateMu.Unlock()

	if stopped {
		err := fmt.Errorf("%s %s: %w", kind, action, ErrInputStopped)
		c.logInputAction(kind, action, reason, false, "stopped", err, attrs...)
		return err
	}
	if disabled {
		err := fmt.Errorf("%s %s: %w", kind, action, ErrInputDisabled)
		c.logInputAction(kind, action, reason, false, "disabled", err, attrs...)
		return err
	}
	if paused {
		err := fmt.Errorf("%s %s: %w", kind, action, ErrInputPaused)
		c.logInputAction(kind, action, reason, false, "paused", err, attrs...)
		return err
	}
	return nil
}

func (c *Controller) logInputAction(kind, action, reason string, allowed bool, blockedBy string, err error, attrs ...any) {
	args := []any{
		"kind", kind,
		"action", action,
		"reason", reason,
		"allowed", allowed,
	}
	args = append(args, attrs...)
	if !allowed {
		args = append(args, "blocked_by", blockedBy, "error", err)
	}
	c.log.Info("input action", args...)
}

func (c *Controller) logAllowedAction(kind, action, reason string, attrs ...any) {
	args := []any{
		"kind", kind,
		"action", action,
		"reason", reason,
		"allowed", true,
	}
	args = append(args, attrs...)
	c.log.Info("input action", args...)
}

func normalizeHotkeyBindings(cfg SafetyConfig) (HotkeyBindings, error) {
	if cfg.RecordingFinishHotkey == "" {
		cfg.RecordingFinishHotkey = "f9"
	}
	pause, err := NormalizeKey(cfg.PauseHotkey)
	if err != nil {
		return HotkeyBindings{}, fmt.Errorf("pause hotkey: %w", err)
	}
	stopAfterRun, err := NormalizeKey(cfg.StopAfterRunHotkey)
	if err != nil {
		return HotkeyBindings{}, fmt.Errorf("stop-after-run hotkey: %w", err)
	}
	recordingFinish, err := NormalizeKey(cfg.RecordingFinishHotkey)
	if err != nil {
		return HotkeyBindings{}, fmt.Errorf("recording-finish hotkey: %w", err)
	}
	stop, err := NormalizeKey(cfg.StopHotkey)
	if err != nil {
		return HotkeyBindings{}, fmt.Errorf("stop hotkey: %w", err)
	}
	return HotkeyBindings{Pause: pause, RecordingFinish: recordingFinish, StopAfterRun: stopAfterRun, Stop: stop}, nil
}

func logSafetyConfigured(log *slog.Logger, cfg SafetyConfig) {
	log.Info("input safety configured",
		"enabled", cfg.Enabled,
		"pause_hotkey", cfg.PauseHotkey,
		"stop_after_run_hotkey", cfg.StopAfterRunHotkey,
		"recording_finish_hotkey", cfg.RecordingFinishHotkey,
		"stop_hotkey", cfg.StopHotkey,
	)
}
