package world

import (
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

// StorageTab identifies an active stash section in bot state, not a numeric game ID.
type StorageTab string

const (
	// StorageTabBasic identifies the Personal/Shared section as a navigation start.
	StorageTabBasic StorageTab = "basic"
	// StorageTabMaterials identifies the materials section as a navigation start.
	StorageTabMaterials StorageTab = "materials"
	// StorageTabGems selects the shared gem/skull collection.
	StorageTabGems StorageTab = "gems"
	// StorageTabRunes selects the shared rune collection.
	StorageTabRunes StorageTab = "runes"
)

// MaterialCount is a collection count with explicit availability.
type MaterialCount struct {
	Code  string
	Count int
	Known bool
}

// CollectionState binds current material counts to an opaque live storage
// instance and snapshot. Tab evidence comes from the named stash UI widgets.
// The embedded stash Cube is available when the personal inventory contains a
// Cube; it does not have an independent UI-open state.
type CollectionState struct {
	ScopeID    string
	ScopeKnown bool
	Generation uint64
	ObservedAt time.Time
	Counts     []MaterialCount
	Tab        StorageTab
	TabKnown   bool
}

// CollectionAvailable reports whether the storage belongs to this valid,
// current In-Game snapshot. Stale and closed-stash values cannot authorize input.
func (s State) CollectionAvailable() bool {
	c := s.Collection
	return s.Valid && s.Phase == GamePhaseInGame && s.UI.StashOpen && c.ScopeKnown && c.ScopeID != "" && s.Generation != 0 && c.Generation == s.Generation && c.ObservedAt.Equal(s.At)
}

// CollectionCount returns a known, unique current count. Absent slots, duplicate
// entries and invalid values stay unknown; zero is known only on a read entry.
func (s State) CollectionCount(code string) (int, bool) {
	if !s.CollectionAvailable() || code == "" {
		return 0, false
	}
	value, matches := 0, 0
	for _, entry := range s.Collection.Counts {
		if entry.Code != code {
			continue
		}
		matches++
		if !entry.Known || entry.Count < 0 || entry.Count > 99 {
			return 0, false
		}
		value = entry.Count
	}
	return value, matches == 1
}

func mapCollection(snap memory.Snapshot) CollectionState {
	raw := snap.Collection
	if !snap.Valid || snap.Phase != memory.GamePhaseInGame || !snap.UI.StashOpen || snap.Generation == 0 || !raw.Available || raw.ScopeID == "" || raw.Generation != snap.Generation {
		return CollectionState{}
	}
	result := CollectionState{ScopeID: raw.ScopeID, ScopeKnown: true, Generation: snap.Generation, ObservedAt: snap.At}
	if raw.TabKnown && (raw.Tab == string(StorageTabBasic) || raw.Tab == string(StorageTabMaterials) || raw.Tab == string(StorageTabGems) || raw.Tab == string(StorageTabRunes)) {
		result.Tab, result.TabKnown = StorageTab(raw.Tab), true
	}
	seen := make(map[string]bool)
	for _, entry := range raw.Counts {
		// Gate 25.1 matched gems/skulls; Gate 25.4 matched the same source to
		// the Rune screenshot (El=7, Eld=6). Types come from the local CASC catalog.
		switch LookupItemType(entry.TxtFileNo) {
		case "gema", "gemr", "gems", "gemt", "geme", "gemd", "gemz", "rune":
		default:
			continue
		}
		code := LookupItemCode(entry.TxtFileNo)
		if code == "" || seen[code] || entry.Count > 99 || entry.UnitID == 0 {
			return CollectionState{}
		}
		seen[code] = true
		result.Counts = append(result.Counts, MaterialCount{Code: code, Count: int(entry.Count), Known: true})
	}
	return result
}
