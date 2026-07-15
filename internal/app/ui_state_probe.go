package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	uiStateProbeDefaultTimeout = 30 * time.Second
	uiStateProbeSampleCount    = 12
	uiStateProbeSchemaVersion  = 1
)

var uiStateProbeLabelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type uiBufferCaptureReader interface {
	CaptureUIBuffer() (memory.UIBufferCapture, error)
}

type uiStateProbeArtifact struct {
	SchemaVersion   int                  `json:"schema_version"`
	CapturedAt      time.Time            `json:"captured_at"`
	Label           string               `json:"label"`
	GameVersion     string               `json:"game_version"`
	Phase           string               `json:"phase"`
	WorldValid      bool                 `json:"world_valid"`
	WorldReason     string               `json:"world_reason,omitempty"`
	InventoryOpen   bool                 `json:"inventory_open"`
	StashOpen       bool                 `json:"stash_open"`
	QuitMenuOpen    bool                 `json:"quit_menu_open"`
	BufferSize      int                  `json:"buffer_size"`
	AnchorIndex     int                  `json:"anchor_index"`
	SampleCount     int                  `json:"sample_count"`
	StableHash      string               `json:"stable_hash"`
	StableNonZero   []uiStateProbeByte   `json:"stable_non_zero"`
	VolatileOffsets []uiStateProbeOffset `json:"volatile_offsets"`
	RawHexSamples   []string             `json:"raw_hex_samples"`
}

type uiStateProbeByte struct {
	Index          int   `json:"index"`
	RelativeOffset int   `json:"relative_offset"`
	Value          uint8 `json:"value"`
}

type uiStateProbeOffset struct {
	Index          int `json:"index"`
	RelativeOffset int `json:"relative_offset"`
}

func validateUIStateProbeLabel(label string) error {
	if !uiStateProbeLabelPattern.MatchString(label) {
		return fmt.Errorf("--ui-state-probe label must match %s", uiStateProbeLabelPattern.String())
	}
	return nil
}

// RunUIStateProbe captures a named, read-only sample set of the D2R UI buffer
// for manual Phase-7.1 state research. It never invokes an input action.
func (rt *Runtime) RunUIStateProbe(label string) error {
	if err := validateUIStateProbeLabel(label); err != nil {
		return err
	}
	if rt.UIProbe == nil {
		return fmt.Errorf("UI-state probe: capture reader unavailable")
	}

	timeout := time.Duration(rt.Options.UIStateProbeTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = uiStateProbeDefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		if detachErr := rt.Process.Detach(); detachErr != nil {
			rt.Log.Warn("process detach failed", "error", detachErr)
		}
	}()
	defer rt.Input.Unbind()

	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	captures := make([]memory.UIBufferCapture, 0, uiStateProbeSampleCount)

	rt.Log.Info("read-only UI-state capture waiting",
		"label", label,
		"samples", uiStateProbeSampleCount,
		"timeout_ms", timeout.Milliseconds(),
	)
	for len(captures) < uiStateProbeSampleCount {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("UI-state probe timeout after %s with %d/%d samples", timeout, len(captures), uiStateProbeSampleCount)
			}
			return nil
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("UI-state probe poll: %w", err)
			}
			if !state.attached {
				continue
			}
			capture, err := rt.UIProbe.CaptureUIBuffer()
			if err != nil {
				rt.Log.Debug("read-only UI-state sample unavailable", "label", label, "error", err)
				continue
			}
			if len(captures) > 0 && (capture.Anchor != captures[0].Anchor || len(capture.Bytes) != len(captures[0].Bytes)) {
				return fmt.Errorf("UI-state probe buffer shape changed during capture")
			}
			captures = append(captures, capture)
		}
	}

	artifact, err := buildUIStateProbeArtifact(label, rt.Config.Memory.GameVersion, rt.World.Current(), captures)
	if err != nil {
		return err
	}
	path, err := saveUIStateProbeArtifact(filepath.Join("diagnostics", "ui-states"), artifact)
	if err != nil {
		return err
	}
	rt.Log.Info("read-only UI-state capture published",
		"label", label,
		"path", path,
		"phase", artifact.Phase,
		"stable_hash", artifact.StableHash,
		"stable_non_zero", len(artifact.StableNonZero),
		"volatile_offsets", len(artifact.VolatileOffsets),
	)
	return nil
}

func buildUIStateProbeArtifact(label, gameVersion string, state world.State, captures []memory.UIBufferCapture) (uiStateProbeArtifact, error) {
	return buildUIStateProbeArtifactValues(label, gameVersion, state.Phase.String(), state.Valid, state.Reason, state.UI.InventoryOpen, state.UI.StashOpen, state.UI.QuitMenuOpen, captures)
}

func buildUIStateProbeArtifactValues(label, gameVersion, phase string, valid bool, reason string, inventoryOpen, stashOpen, quitMenuOpen bool, captures []memory.UIBufferCapture) (uiStateProbeArtifact, error) {
	if len(captures) == 0 || len(captures[0].Bytes) == 0 {
		return uiStateProbeArtifact{}, fmt.Errorf("build UI-state artifact: no samples")
	}
	width := len(captures[0].Bytes)
	anchor := captures[0].Anchor
	stable := make([]byte, width)
	stableMask := make([]byte, width)
	stableNonZero := make([]uiStateProbeByte, 0)
	volatile := make([]uiStateProbeOffset, 0)
	raw := make([]string, 0, len(captures))
	for _, capture := range captures {
		if len(capture.Bytes) != width || capture.Anchor != anchor {
			return uiStateProbeArtifact{}, fmt.Errorf("build UI-state artifact: inconsistent sample shape")
		}
		raw = append(raw, hex.EncodeToString(capture.Bytes))
	}
	for index := 0; index < width; index++ {
		value := captures[0].Bytes[index]
		isStable := true
		for sample := 1; sample < len(captures); sample++ {
			if captures[sample].Bytes[index] != value {
				isStable = false
				break
			}
		}
		if !isStable {
			volatile = append(volatile, uiStateProbeOffset{Index: index, RelativeOffset: index - anchor})
			continue
		}
		stable[index] = value
		stableMask[index] = 1
		if value != 0 {
			stableNonZero = append(stableNonZero, uiStateProbeByte{Index: index, RelativeOffset: index - anchor, Value: value})
		}
	}
	hashInput := append(stableMask, stable...)
	hash := sha256.Sum256(hashInput)
	return uiStateProbeArtifact{
		SchemaVersion:   uiStateProbeSchemaVersion,
		CapturedAt:      captures[len(captures)-1].At.UTC(),
		Label:           label,
		GameVersion:     gameVersion,
		Phase:           phase,
		WorldValid:      valid,
		WorldReason:     reason,
		InventoryOpen:   inventoryOpen,
		StashOpen:       stashOpen,
		QuitMenuOpen:    quitMenuOpen,
		BufferSize:      width,
		AnchorIndex:     anchor,
		SampleCount:     len(captures),
		StableHash:      hex.EncodeToString(hash[:]),
		StableNonZero:   stableNonZero,
		VolatileOffsets: volatile,
		RawHexSamples:   raw,
	}, nil
}

func saveUIStateProbeArtifact(directory string, artifact uiStateProbeArtifact) (string, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create UI-state probe directory: %w", err)
	}
	name := fmt.Sprintf("%s-%s.json", artifact.CapturedAt.Format("20060102T150405.000000000Z"), artifact.Label)
	path := filepath.Join(directory, name)
	tmp, err := os.CreateTemp(directory, ".ui-state-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary UI-state artifact: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("encode UI-state artifact: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("flush UI-state artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close UI-state artifact: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("publish UI-state artifact: %w", err)
	}
	return path, nil
}
