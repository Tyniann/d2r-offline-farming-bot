package app

import (
	"context"
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
	mercenaryProbeDefaultTimeout = 45 * time.Second
	mercenaryProbeSampleCount    = 8
	mercenaryProbeSchemaVersion  = 1
)

var mercenaryProbeLabelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Known Gate-18.0 capture labels. Custom labels remain allowed for extra research.
const (
	MercenaryProbeNotHired       = "not-hired"
	MercenaryProbeAliveHealthy   = "alive-healthy"
	MercenaryProbeAliveInjured   = "alive-injured"
	MercenaryProbeDead           = "dead"
	MercenaryProbeAreaTransition = "area-transition"
)

type hirelingEvidenceReader interface {
	CollectHirelingEvidence() ([]memory.HirelingRawEvidence, error)
}

type mercenaryProbeArtifact struct {
	SchemaVersion   int                    `json:"schema_version"`
	CapturedAt      time.Time              `json:"captured_at"`
	Label           string                 `json:"label"`
	GameVersion     string                 `json:"game_version"`
	SampleCount     int                    `json:"sample_count"`
	HirelingClasses []mercenaryProbeClass  `json:"hireling_classes"`
	Samples         []mercenaryProbeSample `json:"samples"`
	Notes           []string               `json:"notes"`
}

type mercenaryProbeClass struct {
	NPCID uint32 `json:"npc_id"`
	Name  string `json:"name"`
}

type mercenaryProbeSample struct {
	At                   time.Time                    `json:"at"`
	Phase                string                       `json:"phase"`
	Valid                bool                         `json:"valid"`
	Reason               string                       `json:"reason,omitempty"`
	AreaID               uint32                       `json:"area_id"`
	PlayerHP             uint32                       `json:"player_hp"`
	PlayerMaxHP          uint32                       `json:"player_max_hp"`
	UI                   world.UIState                `json:"ui"`
	Mercenary            world.Mercenary              `json:"mercenary"`
	MonsterCount         int                          `json:"monster_count"`
	EligibleMonsterCount int                          `json:"eligible_monster_count"`
	HostileHirelingCount int                          `json:"hostile_hireling_count"`
	HirelingCount        int                          `json:"hireling_count"`
	Hirelings            []memory.HirelingRawEvidence `json:"hirelings"`
	CollectError         string                       `json:"collect_error,omitempty"`
}

func validateMercenaryProbeLabel(label string) error {
	if !mercenaryProbeLabelPattern.MatchString(label) {
		return fmt.Errorf("--mercenary-probe label must match %s", mercenaryProbeLabelPattern.String())
	}
	return nil
}

// RunMercenaryProbe captures a named, read-only series of raw hireling evidence
// beside the simultaneously mapped productive World Mercenary state. It never
// sends input.
func (rt *Runtime) RunMercenaryProbe(label string) error {
	if err := validateMercenaryProbeLabel(label); err != nil {
		return err
	}
	if rt.HirelingProbe == nil {
		return fmt.Errorf("mercenary probe: hireling reader unavailable")
	}
	timeout := time.Duration(rt.Options.MercenaryProbeTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = mercenaryProbeDefaultTimeout
	}
	if label == MercenaryProbeAreaTransition && timeout < 90*time.Second {
		timeout = 90 * time.Second
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
	samples := make([]mercenaryProbeSample, 0, mercenaryProbeSampleCount)
	target := mercenaryProbeSampleCount
	if label == MercenaryProbeAreaTransition {
		target = 24
	}

	rt.Log.Info("mercenary probe started",
		"label", label,
		"timeout", timeout.String(),
		"samples", target,
		"input", "disabled",
	)

	for len(samples) < target {
		select {
		case <-ctx.Done():
			if len(samples) == 0 {
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return fmt.Errorf("mercenary probe timeout after %s with 0/%d samples", timeout, target)
				}
				return nil
			}
			return rt.publishMercenaryProbe(label, samples)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("mercenary probe poll: %w", err)
			}
			if !state.attached {
				continue
			}
			cur := rt.World.Current()
			if cur.At.IsZero() {
				continue
			}
			if label != MercenaryProbeAreaTransition && (!cur.Valid || cur.Phase != world.GamePhaseInGame) {
				continue
			}
			sample := mercenaryProbeSample{
				At:                   cur.At,
				Phase:                cur.Phase.String(),
				Valid:                cur.Valid,
				Reason:               cur.Reason,
				AreaID:               uint32(cur.Area.ID),
				PlayerHP:             cur.Player.HP,
				PlayerMaxHP:          cur.Player.MaxHP,
				UI:                   cur.UI,
				Mercenary:            cur.Mercenary,
				MonsterCount:         len(cur.Monsters),
				EligibleMonsterCount: cur.MonsterCoverage.EligibleMonsterCount,
				HostileHirelingCount: countHostileHirelings(cur.Monsters),
			}
			hirelings, collectErr := rt.HirelingProbe.CollectHirelingEvidence()
			if collectErr != nil {
				sample.CollectError = collectErr.Error()
			} else {
				sample.Hirelings = hirelings
				sample.HirelingCount = len(hirelings)
			}
			samples = append(samples, sample)
			rt.Log.Info("mercenary probe sample",
				"n", len(samples),
				"phase", sample.Phase,
				"area_id", sample.AreaID,
				"hired_known", sample.Mercenary.HiredKnown,
				"hired", sample.Mercenary.Hired,
				"alive", sample.Mercenary.Alive,
				"dead", sample.Mercenary.Dead,
				"vitals_known", sample.Mercenary.VitalsKnown,
				"mercenary_hp", sample.Mercenary.HP,
				"mercenary_max_hp", sample.Mercenary.MaxHP,
				"mercenary_hp_pct", sample.Mercenary.HPPercent(),
				"hostile_hireling_count", sample.HostileHirelingCount,
				"hireling_count", sample.HirelingCount,
				"npc_interact", sample.UI.NPCInteractOpen,
			)
		}
	}
	return rt.publishMercenaryProbe(label, samples)
}

func countHostileHirelings(monsters []world.Monster) int {
	count := 0
	for _, monster := range monsters {
		if memory.IsHirelingClassID(monster.NPCID) {
			count++
		}
	}
	return count
}

func (rt *Runtime) publishMercenaryProbe(label string, samples []mercenaryProbeSample) error {
	artifact := mercenaryProbeArtifact{
		SchemaVersion: mercenaryProbeSchemaVersion,
		CapturedAt:    time.Now().UTC(),
		Label:         label,
		GameVersion:   rt.Config.Memory.GameVersion,
		SampleCount:   len(samples),
		HirelingClasses: []mercenaryProbeClass{
			{NPCID: memory.HirelingClassRogueScout, Name: memory.HirelingClassName(memory.HirelingClassRogueScout)},
			{NPCID: memory.HirelingClassDesertMercenary, Name: memory.HirelingClassName(memory.HirelingClassDesertMercenary)},
			{NPCID: memory.HirelingClassEasternSorceror, Name: memory.HirelingClassName(memory.HirelingClassEasternSorceror)},
			{NPCID: memory.HirelingClassBarbarianA, Name: memory.HirelingClassName(memory.HirelingClassBarbarianA)},
			{NPCID: memory.HirelingClassBarbarianB, Name: memory.HirelingClassName(memory.HirelingClassBarbarianB)},
		},
		Samples: samples,
		Notes: []string{
			"Read-only Gate 18.0 capture. No Merc input, World.Mercenary mapping, or Town wiring.",
			"Hireling class IDs come from local hireling.txt Class column, not Koolo/d2go catalogs.",
			"Life fields expose raw values plus candidate shift8 and /32768 interpretations for live decoding.",
			"Absence of hireling units alone must not be treated as Dead or NotHired without contract evidence.",
		},
	}
	path, err := saveMercenaryProbeArtifact(filepath.Join("diagnostics", "mercenary"), artifact)
	if err != nil {
		return err
	}
	rt.Log.Info("mercenary probe written", "path", path, "label", label, "samples", len(samples))
	return nil
}

func saveMercenaryProbeArtifact(directory string, artifact mercenaryProbeArtifact) (string, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create mercenary probe directory: %w", err)
	}
	name := fmt.Sprintf("%s-%s.json", artifact.CapturedAt.Format("20060102T150405.000000000Z"), artifact.Label)
	path := filepath.Join(directory, name)
	tmp, err := os.CreateTemp(directory, ".mercenary-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary mercenary artifact: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("encode mercenary artifact: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("flush mercenary artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close mercenary artifact: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("publish mercenary artifact: %w", err)
	}
	return path, nil
}
