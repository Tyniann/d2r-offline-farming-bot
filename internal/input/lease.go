package input

// withGameplayAction serializes one complete bot-owned keyboard/mouse
// transaction. Callers validate arguments before entering when those errors
// must take precedence, then re-check safety and window state inside fn.
func (c *Controller) withGameplayAction(fn func() error) error {
	c.gameplayMu.Lock()
	defer c.gameplayMu.Unlock()
	return fn()
}
