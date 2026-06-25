package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

func TestProcessServiceImplementsProcessAccess(t *testing.T) {
	var _ memory.ProcessAccess = (*process.Service)(nil)
}
