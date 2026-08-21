package input

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const chatOpenSettle = 200 * time.Millisecond

type runeSender interface {
	TypeRune(r rune) error
}

// SendChatCommand opens D2R chat, types an allowlisted command, and submits it.
// The only accepted command is `/players 1` through `/players 8`. Chat text uses
// layout-mapped virtual keys, not Unicode, because D2R ignores `KEYEVENTF_UNICODE`.
func (c *Controller) SendChatCommand(command string) error {
	if err := validateOfflinePlayersCommand(command); err != nil {
		return err
	}
	return c.withGameplayAction(func() error {
		if err := c.pressKey("enter", "chat_open"); err != nil {
			return err
		}
		c.timings.sleep(chatOpenSettle)
		if err := c.typeText(command, "chat_body"); err != nil {
			return err
		}
		if err := c.pressKey("enter", "chat_send"); err != nil {
			return err
		}
		return nil
	})
}

func validateOfflinePlayersCommand(command string) error {
	switch command {
	case "/players 1", "/players 2", "/players 3", "/players 4",
		"/players 5", "/players 6", "/players 7", "/players 8":
		return nil
	default:
		return fmt.Errorf("chat command %q: %w", command, ErrInvalidKey)
	}
}

func (c *Controller) typeText(text, reason string) error {
	if err := c.actionGuard("keyboard", "type", reason, "text", text); err != nil {
		return err
	}
	sender, ok := c.keys.(runeSender)
	if !ok {
		return fmt.Errorf("type text: %w", ErrUnsupportedPlatform)
	}
	if err := validateTypeText(text); err != nil {
		return err
	}

	c.keyMu.Lock()
	defer c.keyMu.Unlock()

	for _, r := range text {
		delay := c.timings.delay(c.keyboard.KeyDelayMsMin, c.keyboard.KeyDelayMsMax)
		c.log.Debug("input rune down", "rune", string(r))
		if err := sender.TypeRune(r); err != nil {
			return err
		}
		c.timings.sleep(delay)
	}
	c.logAllowedAction("keyboard", "type", reason, "text", text)
	return nil
}

func validateTypeText(text string) error {
	if text == "" || !utf8.ValidString(text) {
		return fmt.Errorf("type text: %w", ErrInvalidKey)
	}
	for _, r := range text {
		if r == '/' || r == ' ' || unicode.IsLetter(r) && r <= unicode.MaxASCII || unicode.IsDigit(r) && r <= unicode.MaxASCII {
			continue
		}
		return fmt.Errorf("type text %q: %w", text, ErrInvalidKey)
	}
	if strings.Contains(text, "\n") || strings.Contains(text, "\r") {
		return fmt.Errorf("type text: %w", ErrInvalidKey)
	}
	return nil
}
