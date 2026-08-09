package app

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	cowProbeDefaultTimeout   = 20 * time.Second
	cowProbeSchemaVersion    = 2
	cowProbeMaxSamples       = 2000
	cowProbeUIResearchStride = 10
)

var cowProbeLabelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type cowProbeArtifact struct {
	SchemaVersion int              `json:"schema_version"`
	CapturedAt    time.Time        `json:"captured_at"`
	Label         string           `json:"label"`
	GameVersion   string           `json:"game_version"`
	SampleCount   int              `json:"sample_count"`
	Samples       []cowProbeSample `json:"samples"`
	Notes         []string         `json:"notes"`
}

type cowProbeSample struct {
	At                  time.Time               `json:"at"`
	Phase               string                  `json:"phase"`
	Valid               bool                    `json:"valid"`
	Reason              string                  `json:"reason,omitempty"`
	AreaID              uint32                  `json:"area_id"`
	PlayerX             uint32                  `json:"player_x"`
	PlayerY             uint32                  `json:"player_y"`
	LeftSkillID         uint16                  `json:"left_skill_id"`
	RightSkillID        uint16                  `json:"right_skill_id"`
	Objects             []world.Object          `json:"objects"`
	Items               []cowProbeItem          `json:"items"`
	CowEvidenceComplete bool                    `json:"cow_evidence_complete"`
	Cows                []memory.CowRawEvidence `json:"cows"`
	UI                  world.UIState           `json:"ui"`
	UIBufferAnchor      int                     `json:"ui_buffer_anchor"`
	UIBufferHex         string                  `json:"ui_buffer_hex,omitempty"`
	UIBufferError       string                  `json:"ui_buffer_error,omitempty"`
	UIResearchAnchor    int                     `json:"ui_research_anchor,omitempty"`
	UIResearchHex       string                  `json:"ui_research_hex,omitempty"`
	UIResearchError     string                  `json:"ui_research_error,omitempty"`
}

type cowProbeItem struct {
	TxtFileNo uint32             `json:"txt_file_no"`
	UnitID    uint32             `json:"unit_id"`
	Code      string             `json:"code"`
	Name      string             `json:"name"`
	Location  world.ItemLocation `json:"location"`
	GridX     int                `json:"grid_x"`
	GridY     int                `json:"grid_y"`
	Width     int                `json:"width"`
	Height    int                `json:"height"`
}

func validateCowProbeLabel(label string) error {
	if !cowProbeLabelPattern.MatchString(label) {
		return fmt.Errorf("--cow-probe label must match %s", cowProbeLabelPattern.String())
	}
	return nil
}

// RunCowProbe captures a bounded read-only Phase-20.0 evidence series. The
// mode never selects a run and never sends keyboard or mouse input.
func (rt *Runtime) RunCowProbe(label string) error {
	if err := validateCowProbeLabel(label); err != nil {
		return err
	}
	if rt.UIProbe == nil {
		return fmt.Errorf("cow probe: UI capture reader unavailable")
	}
	timeout := time.Duration(rt.Options.CowProbeTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = cowProbeDefaultTimeout
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

	pollInterval := time.Duration(max(1, rt.Config.Runtime.PollIntervalMs)) * time.Millisecond
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	state := &runState{}
	samples := make([]cowProbeSample, 0, min(cowProbeMaxSamples, int(timeout/pollInterval)))
	rt.Log.Info("cow probe started", "label", label, "timeout", timeout.String(), "input", "disabled")

	for len(samples) < cowProbeMaxSamples {
		select {
		case <-ctx.Done():
			if len(samples) == 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("cow probe timeout after %s with no in-game samples", timeout)
			}
			return rt.publishCowProbe(label, samples)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("cow probe poll: %w", err)
			}
			if !state.attached || !rt.lastSnapshot.Valid || rt.lastSnapshot.Phase != memory.GamePhaseInGame {
				continue
			}
			sample := buildCowProbeSample(rt.lastSnapshot, rt.World.Current())
			capture, captureErr := rt.UIProbe.CaptureUIBuffer()
			if captureErr != nil {
				sample.UIBufferError = captureErr.Error()
			} else {
				sample.UIBufferAnchor = capture.Anchor
				sample.UIBufferHex = hex.EncodeToString(capture.Bytes)
			}
			// Wide UI research is intentionally limited to Cube-labelled probes
			// and sampled at a lower cadence to keep evidence bounded.
			if strings.HasPrefix(label, "cube-") && len(samples)%cowProbeUIResearchStride == 0 {
				research, researchErr := rt.UIProbe.CaptureUIResearchBuffer()
				if researchErr != nil {
					sample.UIResearchError = researchErr.Error()
				} else {
					sample.UIResearchAnchor = research.Anchor
					sample.UIResearchHex = hex.EncodeToString(research.Bytes)
				}
			}
			samples = append(samples, sample)
			rt.Log.Info("cow probe sample",
				"n", len(samples), "area_id", sample.AreaID,
				"objects", len(sample.Objects), "items", len(sample.Items),
				"cows", len(sample.Cows), "cow_evidence_complete", sample.CowEvidenceComplete,
				"left_skill_id", sample.LeftSkillID, "right_skill_id", sample.RightSkillID,
			)
		}
	}
	return rt.publishCowProbe(label, samples)
}

func buildCowProbeSample(snap memory.Snapshot, state world.State) cowProbeSample {
	objects := make([]world.Object, 0)
	for _, object := range state.Objects {
		if object.Kind == world.ObjectKindPermanentPortal || object.Kind == world.ObjectKindWirtsBody {
			objects = append(objects, object)
		}
	}
	items := make([]cowProbeItem, 0)
	for _, item := range state.Items {
		if item.Location != world.ItemLocationInventory && item.Location != world.ItemLocationCube && item.Code != "leg" && item.Code != "box" && item.Code != "tbk" {
			continue
		}
		items = append(items, cowProbeItem{
			TxtFileNo: item.TxtFileNo, UnitID: item.UnitID, Code: item.Code, Name: item.Name,
			Location: item.Location, GridX: item.GridX, GridY: item.GridY, Width: item.Width, Height: item.Height,
		})
	}
	return cowProbeSample{
		At: snap.At, Phase: state.Phase.String(), Valid: state.Valid, Reason: state.Reason,
		AreaID: uint32(state.Area.ID), PlayerX: state.Player.Position.X, PlayerY: state.Player.Position.Y,
		LeftSkillID: snap.PlayerSkills.LeftSkill, RightSkillID: snap.PlayerSkills.RightSkill,
		Objects: objects, Items: items, CowEvidenceComplete: snap.CowEvidenceComplete,
		Cows: slices.Clone(snap.CowEvidence), UI: state.UI,
	}
}

func (rt *Runtime) publishCowProbe(label string, samples []cowProbeSample) error {
	artifact := cowProbeArtifact{
		SchemaVersion: cowProbeSchemaVersion,
		CapturedAt:    time.Now().UTC(), Label: label, GameVersion: rt.Config.Memory.GameVersion,
		SampleCount: len(samples), Samples: samples,
		Notes: []string{
			"Read-only Gate 20.0 capture; this mode never sends keyboard or mouse input.",
			"Cow corpse evidence is captured directly in the existing monster walk and is diagnostic only.",
			"A disappearing unit is not by itself an authorized corpse or proof of Corpse Explosion consumption.",
			"UI buffer bytes remain candidates until repeated open/closed captures isolate one stable Cube-open bit.",
			"Cube-labelled probes include a throttled wider UI research window; those bytes remain non-authoritative until cross-state validation.",
			"Cow StateWindowHex is raw StatsListEx-relative evidence and remains non-authoritative until a repeated CE transition isolates the consumed-corpse state.",
		},
	}
	path, err := saveCowProbeArtifact(filepath.Join("diagnostics", "cows"), artifact)
	if err != nil {
		return err
	}
	rt.Log.Info("cow probe written", "path", path, "label", label, "samples", len(samples))
	return nil
}

func saveCowProbeArtifact(directory string, artifact cowProbeArtifact) (string, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create cow probe directory: %w", err)
	}
	name := fmt.Sprintf("%s-%s.json", artifact.CapturedAt.Format("20060102T150405.000000000Z"), artifact.Label)
	path := filepath.Join(directory, name)
	tmp, err := os.CreateTemp(directory, ".cow-probe-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary cow artifact: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("encode cow artifact: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("flush cow artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close cow artifact: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("publish cow artifact: %w", err)
	}
	return path, nil
}
