package tasks

import (
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
)

// Deps holds shared runtime dependencies injected into task runs.
type Deps struct {
	Input   Input
	Pathing Navigator
}

// Input is the subset of input.Controller used by task runs.
type Input interface {
	Status() input.Status
	Bound() bool
}

// Navigator is the subset of pathing.Navigator used by task runs.
type Navigator interface {
	Ready() bool
}

var (
	_ Navigator = (*pathing.Navigator)(nil)
	_ Input     = (*input.Controller)(nil)
)
