package town

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// TownLayoutFingerprint binds route assets to one rolled Rogue Encampment preset.
// Hash identity is translation-independent; Stash coordinates remain separate
// as the absolute playback origin for the current game.
type TownLayoutFingerprint struct {
	Version        int    `json:"version"`
	Hash           string `json:"hash"`
	StashX         uint32 `json:"stash_x"`
	StashY         uint32 `json:"stash_y"`
	WaypointDeltaX int64  `json:"waypoint_delta_x"`
	WaypointDeltaY int64  `json:"waypoint_delta_y"`
	AkaraDeltaX    int64  `json:"akara_delta_x"`
	AkaraDeltaY    int64  `json:"akara_delta_y"`
	CainDeltaX     int64  `json:"cain_delta_x"`
	CainDeltaY     int64  `json:"cain_delta_y"`
	CharsiDeltaX   int64  `json:"charsi_delta_x"`
	CharsiDeltaY   int64  `json:"charsi_delta_y"`
}

// InspectTownLayout derives a translation-independent preset identity from Memory anchors.
// Stash and Waypoint are the only authoritative anchors because NPC enumeration
// is region-dependent; NPC deltas are diagnostics and never affect the hash.
func InspectTownLayout(state world.State) (TownLayoutFingerprint, Reason) {
	if !state.Valid || state.Phase != world.GamePhaseInGame || state.Area.ID != world.RogueEncampment {
		return TownLayoutFingerprint{}, ReasonTownLayoutUnavailable
	}
	stash, stashCount := uniqueTownObject(state, world.ObjectKindPersonalStash)
	waypoint, waypointCount := uniqueTownObject(state, world.ObjectKindWaypoint)
	if stashCount != 1 || waypointCount != 1 {
		return TownLayoutFingerprint{}, ReasonTownLayoutUnavailable
	}
	dx := int64(waypoint.Position.X) - int64(stash.Position.X)
	dy := int64(waypoint.Position.Y) - int64(stash.Position.Y)
	if dx == 0 && dy == 0 {
		return TownLayoutFingerprint{}, ReasonTownLayoutUnavailable
	}
	akaraDX, akaraDY := optionalNPCDelta(state, world.Akara, stash.Position)
	cainDX, cainDY := optionalNPCDelta(state, world.DeckardCain, stash.Position)
	charsiDX, charsiDY := optionalNPCDelta(state, world.Charsi, stash.Position)
	canonical := fmt.Sprintf("v1|stash=0,0|waypoint=%d,%d", dx, dy)
	sum := sha256.Sum256([]byte(canonical))
	return TownLayoutFingerprint{Version: 1, Hash: hex.EncodeToString(sum[:]), StashX: stash.Position.X, StashY: stash.Position.Y, WaypointDeltaX: dx, WaypointDeltaY: dy, AkaraDeltaX: akaraDX, AkaraDeltaY: akaraDY, CainDeltaX: cainDX, CainDeltaY: cainDY, CharsiDeltaX: charsiDX, CharsiDeltaY: charsiDY}, ""
}

func uniqueTownNPC(state world.State, npcID uint32) (world.Monster, int) {
	var found world.Monster
	count := 0
	for _, monster := range state.Monsters {
		if monster.NPCID == npcID {
			found = monster
			count++
		}
	}
	return found, count
}

func optionalNPCDelta(state world.State, npcID uint32, stash world.Position) (int64, int64) {
	npc, count := uniqueTownNPC(state, npcID)
	if count != 1 {
		return 0, 0
	}
	return relativeTo(stash, npc.Position)
}

func relativeTo(origin, target world.Position) (int64, int64) {
	return int64(target.X) - int64(origin.X), int64(target.Y) - int64(origin.Y)
}

func uniqueTownObject(state world.State, kind world.ObjectKind) (world.Object, int) {
	var found world.Object
	count := 0
	for _, object := range state.Objects {
		if object.Kind == kind {
			found = object
			count++
		}
	}
	return found, count
}
