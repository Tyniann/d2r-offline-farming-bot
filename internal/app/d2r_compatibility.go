package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

// D2RCompatibilityState beschreibt den einzigen autoritativen Versionsgate-Zustand.
type D2RCompatibilityState string

const (
	// D2RCompatibilityNotDetected bedeutet, dass keine gebundene D2R-Instanz vorliegt.
	D2RCompatibilityNotDetected D2RCompatibilityState = "not_detected"
	// D2RCompatibilityCompatible erlaubt nach allen exakten Versionsprüfungen Inputpfade.
	D2RCompatibilityCompatible D2RCompatibilityState = "compatible"
	// D2RCompatibilityIncompatible blockiert einen lesbaren, aber nicht unterstützten Vertrag.
	D2RCompatibilityIncompatible D2RCompatibilityState = "incompatible"
	// D2RCompatibilityUnreadable blockiert eine nicht sicher lesbare Prozess- oder Versionsbindung.
	D2RCompatibilityUnreadable D2RCompatibilityState = "unreadable"
)

// D2RCompatibilitySnapshot ist die pfadfreie Projektion des Versionsgates.
type D2RCompatibilitySnapshot struct {
	State             D2RCompatibilityState
	Reason            Phase15ReasonCode
	SupportedVersion  string
	ExpectedVersion   string
	OffsetVersion     string
	ActualVersion     string
	PrivilegeMismatch bool
}

type d2rCompatibilityContract struct {
	supportedVersion string
	expectedVersion  string
	offsetVersion    string
}

func (c d2rCompatibilityContract) evaluate(status process.Status) D2RCompatibilitySnapshot {
	result := D2RCompatibilitySnapshot{
		State: D2RCompatibilityNotDetected, Reason: Phase15ReasonD2RVersionNotDetected,
		SupportedVersion:  strings.TrimSpace(c.supportedVersion),
		ExpectedVersion:   strings.TrimSpace(c.expectedVersion),
		OffsetVersion:     strings.TrimSpace(c.offsetVersion),
		ActualVersion:     strings.TrimSpace(status.FileVersion),
		PrivilegeMismatch: status.PrivilegeMismatch,
	}
	if result.SupportedVersion == "" || result.ExpectedVersion == "" || result.OffsetVersion == "" ||
		result.ExpectedVersion != result.OffsetVersion || result.OffsetVersion != result.SupportedVersion {
		result.State = D2RCompatibilityIncompatible
		result.Reason = Phase15ReasonOffsetVersionMismatch
		return result
	}
	if status.PrivilegeMismatch {
		result.State = D2RCompatibilityUnreadable
		result.Reason = Phase15ReasonPrivilegeMismatch
		return result
	}
	if status.State == process.StateLost {
		return result
	}
	if status.VersionError != "" {
		result.State = D2RCompatibilityUnreadable
		result.Reason = Phase15ReasonD2RVersionUnreadable
		return result
	}
	if status.State != process.StateAttached {
		return result
	}
	if result.ActualVersion == "" {
		result.State = D2RCompatibilityUnreadable
		result.Reason = Phase15ReasonD2RVersionUnreadable
		return result
	}
	if result.ActualVersion != result.SupportedVersion || result.ActualVersion != result.ExpectedVersion || result.ActualVersion != result.OffsetVersion {
		result.State = D2RCompatibilityIncompatible
		result.Reason = Phase15ReasonD2RVersionUnsupported
		return result
	}
	result.State = D2RCompatibilityCompatible
	result.Reason = ""
	return result
}

// CompatibilitySnapshot liefert den aktuellen pfadfreien Versionsgate-Zustand.
func (rt *Runtime) CompatibilitySnapshot() D2RCompatibilitySnapshot {
	if rt == nil || rt.Process == nil {
		return D2RCompatibilitySnapshot{State: D2RCompatibilityNotDetected, Reason: Phase15ReasonD2RVersionNotDetected}
	}
	return rt.compatibility.evaluate(rt.Process.Status())
}

func (rt *Runtime) requireCompatible() error {
	snapshot := rt.CompatibilitySnapshot()
	if snapshot.State != D2RCompatibilityCompatible {
		return fmt.Errorf("%s: D2R compatibility is %s", snapshot.Reason, snapshot.State)
	}
	return nil
}

// waitForCompatibility performs only process discovery and version reads. It
// intentionally runs before registering hotkeys or binding/focusing a window.
func (rt *Runtime) waitForCompatibility(ctx context.Context) error {
	started := time.Now()
	interval := time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond
	if interval <= 0 || interval > 250*time.Millisecond {
		interval = 100 * time.Millisecond
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if rt.Process.Status().State != process.StateAttached {
			if err := rt.Process.Attach(ctx); err != nil {
				snapshot := rt.CompatibilitySnapshot()
				if snapshot.State == D2RCompatibilityUnreadable || snapshot.State == D2RCompatibilityIncompatible {
					return rt.requireCompatible()
				}
				if !process.IsRetryable(err) && !errors.Is(err, process.ErrAlreadyAttached) {
					return fmt.Errorf("compatibility process attach: %w", err)
				}
			}
		}
		if rt.Process.Status().State == process.StateAttached {
			if err := rt.requireCompatible(); err != nil {
				return err
			}
			rt.processPreAttached.Store(true)
			return nil
		}
		if timeout := rt.Config.Process.AttachTimeoutMs; timeout > 0 && time.Since(started) >= time.Duration(timeout)*time.Millisecond {
			return fmt.Errorf("%s: attach timeout waiting for compatibility", Phase15ReasonD2RVersionNotDetected)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
