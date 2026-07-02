package pathing

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

// mockInput records pathing input actions for assertions.
type mockInput struct {
	window   input.WindowInfo
	unbound  bool
	moves    [][2]int
	clicks   []input.MouseButton
	casts    [][2]int
	moveErr  error
	clickErr error
	castErr  error
}

func newMockInput() *mockInput {
	return &mockInput{
		window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720},
	}
}

func (m *mockInput) MoveTo(clientX, clientY int) error {
	if m.moveErr != nil {
		return m.moveErr
	}
	m.moves = append(m.moves, [2]int{clientX, clientY})
	return nil
}

func (m *mockInput) Click(button input.MouseButton) error {
	if m.clickErr != nil {
		return m.clickErr
	}
	m.clicks = append(m.clicks, button)
	return nil
}

func (m *mockInput) CastSkillAt(_ input.BindingSource, skillID uint16, clientX, clientY int) error {
	if m.castErr != nil {
		return m.castErr
	}
	if skillID != memory.SkillTeleport {
		return fmt.Errorf("unexpected skill %d", skillID)
	}
	m.casts = append(m.casts, [2]int{clientX, clientY})
	return nil
}

func (m *mockInput) Window() (input.WindowInfo, bool) {
	if m.unbound {
		return input.WindowInfo{}, false
	}
	return m.window, true
}

// mockBindings resolves every skill to a fixed right-button cast.
type mockBindings struct{}

func (mockBindings) Resolve(skillID uint16) (input.SkillCast, error) {
	return input.SkillCast{SkillID: skillID, SelectKey: "f7", CastButton: input.MouseRight}, nil
}
