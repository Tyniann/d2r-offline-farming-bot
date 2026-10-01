package input

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

type positionedTestKeys struct {
	events             *[]string
	downError, upError Key
	onDown             func(Key)
}

func (k positionedTestKeys) KeyDown(key Key) error {
	*k.events = append(*k.events, "down:"+string(key))
	if k.onDown != nil {
		k.onDown(key)
	}
	if key == k.downError {
		return errors.New("key down failed")
	}
	return nil
}
func (k positionedTestKeys) KeyUp(key Key) error {
	*k.events = append(*k.events, "up:"+string(key))
	if key == k.upError {
		return errors.New("key up failed")
	}
	return nil
}

type positionedTestMouse struct {
	events             *[]string
	downError, upError bool
}

func (m positionedTestMouse) MoveTo(x, y int) error {
	*m.events = append(*m.events, fmt.Sprintf("move:%d,%d", x, y))
	return nil
}
func (m positionedTestMouse) ButtonDown(b MouseButton) error {
	*m.events = append(*m.events, "mouse_down:"+string(b))
	if m.downError {
		return errors.New("mouse down failed")
	}
	return nil
}
func (m positionedTestMouse) ButtonUp(b MouseButton) error {
	*m.events = append(*m.events, "mouse_up:"+string(b))
	if m.upError {
		return errors.New("mouse up failed")
	}
	return nil
}

func TestPositionedClickCtrlShiftOrderAndPlainClick(t *testing.T) {
	for _, modified := range []bool{true, false} {
		t.Run(fmt.Sprint(modified), func(t *testing.T) {
			events := []string{}
			c := newForegroundTransactionController(positionedTestKeys{events: &events}, positionedTestMouse{events: &events})
			var err error
			if modified {
				err = c.ClickAtWithCtrlShift(400, 300, MouseLeft)
			} else {
				err = c.ClickAt(400, 300, MouseLeft)
			}
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"move:500,500", "mouse_down:left", "mouse_up:left"}
			if modified {
				want = []string{"move:500,500", "down:ctrl", "down:shift", "mouse_down:left", "mouse_up:left", "up:shift", "up:ctrl"}
			}
			if !slices.Equal(events, want) {
				t.Fatalf("events=%v want=%v", events, want)
			}
		})
	}
}

func TestPositionedClickPartialFailureReleasesBothKeys(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		down, up                 Key
		mouseDown, mouseUp, stop bool
	}{
		{"shift_down", "shift", "", false, false, false},
		{"mouse_down", "", "", true, false, false},
		{"mouse_up", "", "", false, true, true},
		{"shift_up", "", "shift", false, false, true},
		{"ctrl_up", "", "ctrl", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := []string{}
			c := newForegroundTransactionController(positionedTestKeys{events: &events, downError: tc.down, upError: tc.up}, positionedTestMouse{events: &events, downError: tc.mouseDown, upError: tc.mouseUp})
			if err := c.ClickAtWithCtrlShift(400, 300, MouseLeft); err == nil {
				t.Fatal("partial failure returned success")
			}
			if !slices.Contains(events, "up:shift") || !slices.Contains(events, "up:ctrl") {
				t.Fatalf("modifier left held: %v", events)
			}
			if tc.mouseDown && !slices.Contains(events, "mouse_up:left") {
				t.Fatal("mouse cleanup missing")
			}
			if c.Status().Stopped != tc.stop {
				t.Fatalf("stopped=%t want=%t", c.Status().Stopped, tc.stop)
			}
		})
	}
}

func TestPositionedClickRechecksFocusSafetyAndGeometry(t *testing.T) {
	for _, kind := range []string{"foreground", "focus_during_modifiers", "pause_during_modifiers", "paused", "stopped", "disabled", "resize"} {
		t.Run(kind, func(t *testing.T) {
			events := []string{}
			keys := &positionedTestKeys{events: &events}
			c := newForegroundTransactionController(keys, positionedTestMouse{events: &events})
			api := c.api.(*mockWindowAPI)
			switch kind {
			case "foreground":
				api.foreground = false
			case "focus_during_modifiers":
				keys.onDown = func(k Key) {
					if k == "shift" {
						api.foreground = false
					}
				}
			case "pause_during_modifiers":
				keys.onDown = func(k Key) {
					if k == "shift" {
						c.Pause("test")
					}
				}
			case "paused":
				c.Pause("test")
			case "stopped":
				c.Stop("test")
			case "disabled":
				c.enabled = false
			case "resize":
				api.area.ClientWidth--
			}
			if err := c.ClickAtWithCtrlShift(400, 300, MouseLeft); err == nil {
				t.Fatal("revoked input returned success")
			}
			if slices.Contains(events, "mouse_down:left") {
				t.Fatalf("clicked after revoke: %v", events)
			}
			if kind == "focus_during_modifiers" || kind == "pause_during_modifiers" {
				if !slices.Contains(events, "up:shift") || !slices.Contains(events, "up:ctrl") {
					t.Fatal("modifiers not released")
				}
			} else if len(events) != 0 {
				t.Fatalf("input before gate: %v", events)
			}
		})
	}
}
