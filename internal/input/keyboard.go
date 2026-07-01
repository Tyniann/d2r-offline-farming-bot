package input

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
)

// Key is a normalized keyboard key identifier used by [KeySender].
type Key string

// KeyboardConfig holds timing settings for keyboard actions.
type KeyboardConfig struct {
	KeyDelayMsMin int
	KeyDelayMsMax int
	ComboHoldMs   int
}

// DefaultKeyboardConfig returns standard keyboard timing defaults.
func DefaultKeyboardConfig() KeyboardConfig {
	return KeyboardConfig{
		KeyDelayMsMin: 10,
		KeyDelayMsMax: 40,
		ComboHoldMs:   200,
	}
}

// KeySender sends low-level keyboard down/up events to the OS.
type KeySender interface {
	KeyDown(key Key) error
	KeyUp(key Key) error
}

type keyTimings struct {
	sleep func(time.Duration)
	delay func(minMs, maxMs int) time.Duration
}

func defaultKeyTimings() keyTimings {
	return keyTimings{
		sleep: time.Sleep,
		delay: randomDelay,
	}
}

func randomDelay(minMs, maxMs int) time.Duration {
	if minMs >= maxMs {
		return time.Duration(minMs) * time.Millisecond
	}
	return time.Duration(minMs+rand.IntN(maxMs-minMs+1)) * time.Millisecond
}

// supportedKeys maps normalized key names to virtual-key codes (Windows VK_*).
var supportedKeys = map[string]uint16{
	"0": 0x30, "1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34,
	"5": 0x35, "6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39,
	"a": 0x41, "b": 0x42, "c": 0x43, "d": 0x44, "e": 0x45,
	"f": 0x46, "g": 0x47, "h": 0x48, "i": 0x49, "j": 0x4A,
	"k": 0x4B, "l": 0x4C, "m": 0x4D, "n": 0x4E, "o": 0x4F,
	"p": 0x50, "q": 0x51, "r": 0x52, "s": 0x53, "t": 0x54,
	"u": 0x55, "v": 0x56, "w": 0x57, "x": 0x58, "y": 0x59, "z": 0x5A,
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73,
	"f5": 0x74, "f6": 0x75, "f7": 0x76, "f8": 0x77,
	"f9": 0x78, "f10": 0x79, "f11": 0x7A, "f12": 0x7B,
	"shift": 0xA0, "ctrl": 0xA2, "alt": 0xA4,
	"esc": 0x1B, "enter": 0x0D, "space": 0x20, "tab": 0x09,
	",": 0xBC, ".": 0xBE, "-": 0xBD, "]": 0xDD,
	"pause": 0x13,
}

// NormalizeKey trims whitespace, lowercases, and validates a key string.
func NormalizeKey(raw string) (Key, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "", fmt.Errorf("normalize key: %w", ErrInvalidKey)
	}
	if _, ok := supportedKeys[normalized]; !ok {
		return "", fmt.Errorf("normalize key %q: %w", raw, ErrInvalidKey)
	}
	return Key(normalized), nil
}

// ValidateKeyStrings checks that every non-empty key string is supported.
func ValidateKeyStrings(keys ...string) error {
	for _, k := range keys {
		if k == "" {
			continue
		}
		if _, err := NormalizeKey(k); err != nil {
			return err
		}
	}
	return nil
}

func virtualKey(key Key) (uint16, bool) {
	vk, ok := supportedKeys[string(key)]
	return vk, ok
}

// KeyDown sends a key-down event for the given key string.
func (c *Controller) KeyDown(key string) error {
	k, err := NormalizeKey(key)
	if err != nil {
		return err
	}
	return c.keyDown(k, "direct_call")
}

// KeyUp sends a key-up event for the given key string.
func (c *Controller) KeyUp(key string) error {
	k, err := NormalizeKey(key)
	if err != nil {
		return err
	}
	return c.keyUp(k, "direct_call")
}

// PressKey sends a key-down, waits a configured random delay, then key-up.
func (c *Controller) PressKey(key string) error {
	k, err := NormalizeKey(key)
	if err != nil {
		return err
	}
	return c.pressKey(k, "direct_call")
}

// PressCombo presses keys in order, holds for combo_hold_ms, then releases in reverse order.
func (c *Controller) PressCombo(keys ...string) error {
	if len(keys) == 0 {
		return fmt.Errorf("press combo: %w", ErrInvalidKey)
	}

	normalized := make([]Key, len(keys))
	for i, raw := range keys {
		k, err := NormalizeKey(raw)
		if err != nil {
			return err
		}
		normalized[i] = k
	}
	return c.pressCombo(normalized, "combo")
}

func (c *Controller) keyDown(k Key, reason string) error {
	if err := c.actionGuard("keyboard", "down", reason, "key", k); err != nil {
		return err
	}

	c.keyMu.Lock()
	defer c.keyMu.Unlock()

	c.log.Debug("input key down", "key", k)
	if err := c.keys.KeyDown(k); err != nil {
		return err
	}
	c.logAllowedAction("keyboard", "down", reason, "key", k)
	return nil
}

func (c *Controller) keyUp(k Key, reason string) error {
	if err := c.actionGuard("keyboard", "up", reason, "key", k); err != nil {
		return err
	}

	c.keyMu.Lock()
	defer c.keyMu.Unlock()

	c.log.Debug("input key up", "key", k)
	if err := c.keys.KeyUp(k); err != nil {
		return err
	}
	c.logAllowedAction("keyboard", "up", reason, "key", k)
	return nil
}

func (c *Controller) pressKey(k Key, reason string) error {
	if err := c.actionGuard("keyboard", "press", reason, "key", k); err != nil {
		return err
	}

	c.keyMu.Lock()
	defer c.keyMu.Unlock()

	delay := c.timings.delay(c.keyboard.KeyDelayMsMin, c.keyboard.KeyDelayMsMax)

	c.log.Debug("input key down", "key", k)
	if err := c.keys.KeyDown(k); err != nil {
		return err
	}
	c.timings.sleep(delay)
	c.log.Debug("input key up", "key", k)
	if err := c.keys.KeyUp(k); err != nil {
		return err
	}

	c.logAllowedAction("keyboard", "press", reason, "key", k, "delay_ms", delay.Milliseconds())
	return nil
}

func (c *Controller) pressCombo(normalized []Key, reason string) error {
	if err := c.actionGuard("keyboard", "combo", reason, "keys", keysToStrings(normalized)); err != nil {
		return err
	}

	c.keyMu.Lock()
	defer c.keyMu.Unlock()

	pressed := make([]Key, 0, len(normalized))
	for _, k := range normalized {
		c.log.Debug("input key down", "key", k)
		if err := c.keys.KeyDown(k); err != nil {
			c.releasePressedKeys(pressed)
			return err
		}
		pressed = append(pressed, k)
	}

	hold := time.Duration(c.keyboard.ComboHoldMs) * time.Millisecond
	c.timings.sleep(hold)

	for i := len(pressed) - 1; i >= 0; i-- {
		k := pressed[i]
		c.log.Debug("input key up", "key", k)
		if err := c.keys.KeyUp(k); err != nil {
			c.releasePressedKeys(pressed[:i])
			return err
		}
	}

	c.logAllowedAction("keyboard", "combo", reason, "keys", keysToStrings(normalized), "delay_ms", c.keyboard.ComboHoldMs)
	return nil
}

func keysToStrings(keys []Key) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = string(k)
	}
	return out
}

func (c *Controller) releasePressedKeys(pressed []Key) {
	for i := len(pressed) - 1; i >= 0; i-- {
		k := pressed[i]
		if err := c.keys.KeyUp(k); err != nil {
			c.log.Warn("input key cleanup failed", "key", k, "error", err)
		}
	}
}
