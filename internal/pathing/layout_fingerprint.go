package pathing

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const layoutFingerprintVersion = 1

var (
	// ErrLayoutStateInvalid indicates that no valid in-game state was supplied.
	ErrLayoutStateInvalid = errors.New("layout fingerprint state invalid")
	// ErrLayoutAnchorsUnavailable indicates that the current snapshot has no stable anchors.
	ErrLayoutAnchorsUnavailable = errors.New("layout fingerprint anchors unavailable")
)

// LayoutFingerprint identifies an observed local layout without volatile UnitIDs.
type LayoutFingerprint struct {
	Version     int
	AreaID      world.AreaID
	PlayerX     uint32
	PlayerY     uint32
	AnchorCount int
	Hash        string
	// Anchors lists the sorted canonical strings that produced Hash.
	// They are diagnostic only and are not persisted on system-egress contracts.
	Anchors []string
}

// BuildLayoutFingerprint canonicalizes stable objects and entrances around the player.
// It is intended for a pre-input route anchor, not for identifying an entire unseen map.
func BuildLayoutFingerprint(state world.State) (LayoutFingerprint, error) {
	if !state.Valid || state.Phase != world.GamePhaseInGame || state.Area.ID == 0 {
		return LayoutFingerprint{}, ErrLayoutStateInvalid
	}
	anchors := stableLayoutAnchors(state)
	if len(anchors) == 0 {
		return LayoutFingerprint{}, ErrLayoutAnchorsUnavailable
	}
	sort.Strings(anchors)
	canonical := fmt.Sprintf("v=%d|area=%d|", layoutFingerprintVersion, state.Area.ID)
	for _, anchor := range anchors {
		canonical += anchor + ";"
	}
	digest := sha256.Sum256([]byte(canonical))
	return LayoutFingerprint{
		Version:     layoutFingerprintVersion,
		AreaID:      state.Area.ID,
		PlayerX:     state.Player.Position.X,
		PlayerY:     state.Player.Position.Y,
		AnchorCount: len(anchors),
		Hash:        hex.EncodeToString(digest[:]),
		Anchors:     anchors,
	}, nil
}

func stableLayoutAnchors(state world.State) []string {
	anchors := make([]string, 0, len(state.Objects)+len(state.Entrances))
	for _, object := range state.Objects {
		switch object.Kind {
		case world.ObjectKindWaypoint, world.ObjectKindGoodChest, world.ObjectKindPersonalStash:
			anchors = append(anchors, fmt.Sprintf("o:%d:%d,%d", object.ID, object.Position.X, object.Position.Y))
		}
	}
	for _, entrance := range state.Entrances {
		anchors = append(anchors, fmt.Sprintf("e:%d:%d:%d,%d", entrance.ID, entrance.Kind, entrance.Position.X, entrance.Position.Y))
	}
	return anchors
}
