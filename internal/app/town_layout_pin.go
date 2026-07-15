package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	townLayoutTestPinVersion = 1
	townLayoutTestPinMaxAge  = 10 * time.Minute
)

// townLayoutPin keeps one game-scoped Town preset usable while regional object
// enumeration temporarily unloads Stash or Waypoint. It may bridge missing
// anchors, but never a game-identity change or a newly observed mismatch.
type townLayoutPin struct {
	fingerprint town.TownLayoutFingerprint
	identity    world.GameIdentity
}

func (p *townLayoutPin) Resolve(state world.State) (town.TownLayoutFingerprint, town.Reason, bool) {
	observed, reason := town.InspectTownLayout(state)
	if reason == "" {
		// Reappearing anchors revalidate both preset hash and absolute origin.
		// A changed origin inside the same identity is not silently translated.
		if p.fingerprint.Hash != "" && (p.fingerprint.Hash != observed.Hash || p.fingerprint.StashX != observed.StashX || p.fingerprint.StashY != observed.StashY) {
			return town.TownLayoutFingerprint{}, town.ReasonTownLayoutMismatch, true
		}
		p.fingerprint = observed
		p.identity = state.Identity
		return observed, "", true
	}
	if p.fingerprint.Hash == "" || !sameTownLayoutIdentity(p.identity, state.Identity) || !state.Valid || state.Phase != world.GamePhaseInGame || state.Area.ID != world.RogueEncampment {
		return town.TownLayoutFingerprint{}, reason, false
	}
	return p.fingerprint, "", false
}

// Seed exists only to restore a short-lived diagnostic playback pin after a
// separate CLI process. Productive sessions establish their pin from Memory.
func (p *townLayoutPin) Seed(fingerprint town.TownLayoutFingerprint, identity world.GameIdentity) error {
	if len(fingerprint.Hash) != 64 || fingerprint.StashX == 0 || fingerprint.StashY == 0 || !identity.Valid || strings.TrimSpace(identity.CharacterName) == "" {
		return fmt.Errorf("invalid Town layout pin")
	}
	p.fingerprint = fingerprint
	p.identity = identity
	return nil
}

func (p *townLayoutPin) Reset() {
	p.fingerprint = town.TownLayoutFingerprint{}
	p.identity = world.GameIdentity{}
}

func sameTownLayoutIdentity(a, b world.GameIdentity) bool {
	// MapSeed scopes the pin to the current offline game; character and class
	// prevent an otherwise coincidental seed match from crossing profiles.
	return a.Valid && b.Valid && strings.EqualFold(a.CharacterName, b.CharacterName) && a.Class == b.Class && a.MapSeed == b.MapSeed
}

// townLayoutTestPinFile is a diagnostic bridge, not a persistent route binding.
// Process, identity, age, and fingerprint are all revalidated before seeding.
type townLayoutTestPinFile struct {
	Version        int                        `json:"version"`
	ObservedAt     time.Time                  `json:"observed_at"`
	ProcessID      uint32                     `json:"process_id"`
	CharacterName  string                     `json:"character_name"`
	CharacterClass world.CharacterClass       `json:"character_class"`
	MapSeed        uint32                     `json:"map_seed"`
	Fingerprint    town.TownLayoutFingerprint `json:"fingerprint"`
}

func saveTownLayoutTestPin(path string, pid uint32, state world.State, fingerprint town.TownLayoutFingerprint, now time.Time) error {
	if pid == 0 || !state.Identity.Valid {
		return fmt.Errorf("town layout test pin requires a live process and valid game identity")
	}
	file := townLayoutTestPinFile{
		Version: townLayoutTestPinVersion, ObservedAt: now.UTC(), ProcessID: pid,
		CharacterName: state.Identity.CharacterName, CharacterClass: state.Identity.Class, MapSeed: state.Identity.MapSeed,
		Fingerprint: fingerprint,
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal Town layout test pin: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create Town diagnostics directory: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write Town layout test pin: %w", err)
	}
	return nil
}

func loadTownLayoutTestPin(path string, pid uint32, state world.State, now time.Time) (town.TownLayoutFingerprint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return town.TownLayoutFingerprint{}, err
	}
	var file townLayoutTestPinFile
	if err := json.Unmarshal(data, &file); err != nil {
		return town.TownLayoutFingerprint{}, fmt.Errorf("parse Town layout test pin: %w", err)
	}
	age := now.Sub(file.ObservedAt)
	if file.Version != townLayoutTestPinVersion || file.ProcessID != pid || age < 0 || age > townLayoutTestPinMaxAge {
		return town.TownLayoutFingerprint{}, fmt.Errorf("town layout test pin is stale or belongs to another process")
	}
	identity := world.GameIdentity{Valid: true, CharacterName: file.CharacterName, Class: file.CharacterClass, MapSeed: file.MapSeed}
	if !sameTownLayoutIdentity(identity, state.Identity) {
		return town.TownLayoutFingerprint{}, fmt.Errorf("town layout test pin identity mismatch")
	}
	if len(file.Fingerprint.Hash) != 64 || file.Fingerprint.StashX == 0 || file.Fingerprint.StashY == 0 {
		return town.TownLayoutFingerprint{}, fmt.Errorf("town layout test pin fingerprint is invalid")
	}
	return file.Fingerprint, nil
}
