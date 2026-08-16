package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	weaponSetProbeLabel           = "primary-secondary"
	weaponSetProbeDefaultTimeout  = 2 * time.Minute
	weaponSetProbeStableSnapshots = 3
	weaponSetProbeSchemaVersion   = 2
)

type weaponSetProbeArtifact struct {
	SchemaVersion       int                    `json:"schema_version"`
	CapturedAt          time.Time              `json:"captured_at"`
	Label               string                 `json:"label"`
	GameVersion         string                 `json:"game_version"`
	Passed              bool                   `json:"passed"`
	SampleCount         int                    `json:"sample_count"`
	StableTransitions   int                    `json:"stable_transitions"`
	StableConfirmations []string               `json:"stable_confirmations"`
	Samples             []weaponSetProbeSample `json:"samples"`
	Notes               []string               `json:"notes"`
}

type weaponSetProbeSample struct {
	At             time.Time `json:"at"`
	Generation     uint64    `json:"generation"`
	Phase          string    `json:"phase"`
	Valid          bool      `json:"valid"`
	Reason         string    `json:"reason,omitempty"`
	Available      bool      `json:"available"`
	Set            string    `json:"set,omitempty"`
	SkillsComplete bool      `json:"skills_complete"`
	BattleOrders   bool      `json:"battle_orders"`
	BattleCommand  bool      `json:"battle_command"`
	StableTicks    int       `json:"stable_ticks"`
	Confirmed      bool      `json:"confirmed"`
}

type weaponSetProbeStability struct {
	candidate     world.WeaponSet
	hasCandidate  bool
	stableTicks   int
	confirmations []string
	transitions   int
}

func validateWeaponSetProbeLabel(label string) error {
	if label != weaponSetProbeLabel {
		return fmt.Errorf("--weapon-set-probe must be %q", weaponSetProbeLabel)
	}
	return nil
}

func (s *weaponSetProbeStability) observe(active world.WeaponSetState) (stableTicks int, confirmed bool) {
	if !active.Available {
		s.hasCandidate = false
		s.stableTicks = 0
		return 0, false
	}
	if !s.hasCandidate || s.candidate != active.Set {
		s.candidate = active.Set
		s.hasCandidate = true
		s.stableTicks = 1
	} else {
		s.stableTicks++
	}
	if s.stableTicks != weaponSetProbeStableSnapshots {
		return s.stableTicks, false
	}
	label := active.Set.String()
	if len(s.confirmations) > 0 && s.confirmations[len(s.confirmations)-1] == label {
		return s.stableTicks, false
	}
	if len(s.confirmations) > 0 {
		s.transitions++
	}
	s.confirmations = append(s.confirmations, label)
	return s.stableTicks, true
}

func (s *weaponSetProbeStability) passed() bool {
	if s.transitions < 2 || len(s.confirmations) < 3 {
		return false
	}
	seenPrimary, seenSecondary := false, false
	for _, confirmation := range s.confirmations {
		seenPrimary = seenPrimary || confirmation == world.WeaponSetPrimary.String()
		seenSecondary = seenSecondary || confirmation == world.WeaponSetSecondary.String()
	}
	return seenPrimary && seenSecondary
}

// RunWeaponSetProbe observes only fresh snapshots while the operator changes
// weapon sets manually. It never sends keyboard or mouse input.
func (rt *Runtime) RunWeaponSetProbe(label string) error {
	if err := validateWeaponSetProbeLabel(label); err != nil {
		return err
	}
	timeout := time.Duration(rt.Options.WeaponSetProbeTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = weaponSetProbeDefaultTimeout
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
	stability := &weaponSetProbeStability{}
	samples := make([]weaponSetProbeSample, 0, int(timeout/pollInterval))
	battleOrders := memory.MustSkillID("battle_orders")
	battleCommand := memory.MustSkillID("battle_command")

	rt.Log.Info("weapon set probe started",
		"label", label,
		"timeout", timeout.String(),
		"stable_snapshots", weaponSetProbeStableSnapshots,
		"input", "disabled",
		"instruction", "W im Spiel erst nach stabiler Bestätigung drücken; zweimal zwischen den Sets wechseln",
	)
	for {
		select {
		case <-ctx.Done():
			if len(samples) == 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("weapon set probe timeout after %s with no in-game samples", timeout)
			}
			return rt.publishWeaponSetProbe(label, samples, stability)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("weapon set probe poll: %w", err)
			}
			if !state.attached || !rt.lastSnapshot.Valid || rt.lastSnapshot.Phase != memory.GamePhaseInGame {
				continue
			}
			current := rt.World.Current()
			stableTicks, confirmed := stability.observe(current.Player.ActiveWeaponSet)
			sample := weaponSetProbeSample{
				At: rt.lastSnapshot.At, Generation: rt.lastSnapshot.Generation,
				Phase: current.Phase.String(), Valid: current.Valid, Reason: current.Reason,
				Available:      current.Player.ActiveWeaponSet.Available,
				SkillsComplete: rt.lastSnapshot.PlayerSkills.Complete,
				BattleOrders:   rt.lastSnapshot.PlayerSkills.SkillsKnown[battleOrders],
				BattleCommand:  rt.lastSnapshot.PlayerSkills.SkillsKnown[battleCommand],
				StableTicks:    stableTicks, Confirmed: confirmed,
			}
			if sample.Available {
				sample.Set = current.Player.ActiveWeaponSet.Set.String()
			}
			samples = append(samples, sample)
			if stableTicks == 1 || confirmed || !sample.Available {
				rt.Log.Info("weapon set probe sample",
					"generation", sample.Generation, "available", sample.Available, "set", sample.Set,
					"skills_complete", sample.SkillsComplete, "battle_orders", sample.BattleOrders,
					"battle_command", sample.BattleCommand, "stable_ticks", sample.StableTicks,
					"confirmed", sample.Confirmed, "stable_transitions", stability.transitions,
				)
			}
			if stability.passed() {
				return rt.publishWeaponSetProbe(label, samples, stability)
			}
		}
	}
}

func (rt *Runtime) publishWeaponSetProbe(label string, samples []weaponSetProbeSample, stability *weaponSetProbeStability) error {
	artifact := weaponSetProbeArtifact{
		SchemaVersion: weaponSetProbeSchemaVersion, CapturedAt: time.Now().UTC(), Label: label,
		GameVersion: rt.Config.Memory.GameVersion, Passed: stability.passed(), SampleCount: len(samples),
		StableTransitions: stability.transitions, StableConfirmations: append([]string(nil), stability.confirmations...), Samples: samples,
		Notes: []string{
			"Read-only Gate 22.3 capture; this mode never sends keyboard or mouse input.",
			"Available=false is unknown and is never interpreted as the primary weapon set.",
			"The explicit Hammerdin CTA contract maps an absent Battle Orders/Battle Command pair to primary and a present pair to secondary; partial or incomplete skill evidence is unavailable.",
			"A stable confirmation requires three consecutive fresh snapshot generations.",
			"Passing requires both sets and two alternating stable transitions caused manually by the operator.",
		},
	}
	path, err := saveWeaponSetProbeArtifact(rt.Config.ResolvePath(filepath.Join("diagnostics", "weapon-sets")), artifact)
	if err != nil {
		return err
	}
	rt.Log.Info("weapon set probe written", "path", path, "passed", artifact.Passed, "samples", len(samples), "stable_transitions", artifact.StableTransitions)
	if !artifact.Passed {
		return fmt.Errorf("weapon set probe did not confirm both sets across two stable transitions")
	}
	return nil
}

func saveWeaponSetProbeArtifact(directory string, artifact weaponSetProbeArtifact) (string, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create weapon set probe directory: %w", err)
	}
	name := fmt.Sprintf("%s-%s.json", artifact.CapturedAt.Format("20060102T150405.000000000Z"), artifact.Label)
	path := filepath.Join(directory, name)
	tmp, err := os.CreateTemp(directory, ".weapon-set-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary weapon set artifact: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("encode weapon set artifact: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("flush weapon set artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close weapon set artifact: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("publish weapon set artifact: %w", err)
	}
	return path, nil
}
