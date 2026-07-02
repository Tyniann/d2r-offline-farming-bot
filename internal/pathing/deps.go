package pathing

import (
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
)

// InputDriver is the subset of [input.Controller] used by pathing components.
// All calls are client-relative to the bound D2R window and honor the input
// safety guards (enabled/paused/stopped).
type InputDriver interface {
	MoveTo(clientX, clientY int) error
	Click(button input.MouseButton) error
	CastSkillAt(src input.BindingSource, skillID uint16, clientX, clientY int) error
	Window() (input.WindowInfo, bool)
}

// Deps wires the navigator to input, YAML skill bindings, and tuning config.
type Deps struct {
	Input    InputDriver
	Bindings input.BindingSource
	Config   Config
}

var _ InputDriver = (*input.Controller)(nil)
