package pathing

import (
	"errors"
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

var (
	// ErrGameIdentityUnavailable blocks routes without a confirmed character.
	ErrGameIdentityUnavailable = errors.New("game identity unavailable")
	// ErrRouteCharacterMismatch blocks routes recorded for another character.
	ErrRouteCharacterMismatch = errors.New("route character mismatch")
	// ErrRouteGameVersionMismatch blocks routes recorded against another game version.
	ErrRouteGameVersionMismatch = errors.New("route game version mismatch")
	// ErrRouteLayoutUnverified blocks routes when no authoritative layout proof exists.
	ErrRouteLayoutUnverified = errors.New("route layout unverified")
	// ErrRouteLayoutMismatch blocks routes when the active layout differs.
	ErrRouteLayoutMismatch = errors.New("route layout mismatch")
	// ErrRouteStartMismatch blocks routes outside their first segment area or tolerance.
	ErrRouteStartMismatch = errors.New("route start mismatch")
)

// RoutePrecheckInput contains authoritative runtime data checked before route input.
type RoutePrecheckInput struct {
	Identity    world.GameIdentity
	GameVersion string
	Layout      LayoutFingerprint
	World       world.State
}

// ValidateRoutePrecheck applies fail-closed identity, version, layout, and start checks.
func ValidateRoutePrecheck(route Route, input RoutePrecheckInput) error {
	if err := route.Validate(); err != nil {
		return fmt.Errorf("route invalid: %w", err)
	}
	if !input.Identity.Valid {
		return ErrGameIdentityUnavailable
	}
	class, ok := parseCharacterClass(route.Binding.CharacterClass)
	if !ok || input.Identity.CharacterName != route.Binding.CharacterName || input.Identity.Class != class {
		return fmt.Errorf("%w: active=%s/%s route=%s/%s", ErrRouteCharacterMismatch, input.Identity.CharacterName, input.Identity.Class.String(), route.Binding.CharacterName, route.Binding.CharacterClass)
	}
	if input.GameVersion == "" || input.GameVersion != route.Binding.GameVersion {
		return fmt.Errorf("%w: active=%q route=%q", ErrRouteGameVersionMismatch, input.GameVersion, route.Binding.GameVersion)
	}
	if input.Layout.Hash == "" || input.Layout.AnchorCount < 1 {
		return ErrRouteLayoutUnverified
	}
	expected := route.Binding.LayoutFingerprint
	if input.Layout.Version != expected.Version || input.Layout.AreaID != expected.AreaID || input.Layout.Hash != expected.Hash {
		return fmt.Errorf("%w: active=%s route=%s", ErrRouteLayoutMismatch, input.Layout.Hash, expected.Hash)
	}
	if !input.World.Valid || input.World.Phase != world.GamePhaseInGame || input.World.Area.ID != route.Segments[0].FromAreaID {
		return fmt.Errorf("%w: expected area %d", ErrRouteStartMismatch, route.Segments[0].FromAreaID)
	}
	start := route.Segments[0].Points[0]
	distance := world.Distance(input.World.Player.Position, world.Position{X: start.X, Y: start.Y})
	if distance > route.Playback.MaxDriftTiles {
		return fmt.Errorf("%w: distance %.1f exceeds %.1f", ErrRouteStartMismatch, distance, route.Playback.MaxDriftTiles)
	}
	return nil
}

// ValidateRouteSegmentStart checks an isolated diagnostic segment start.
// Segment zero must additionally pass [ValidateRoutePrecheck] with a layout proof.
func ValidateRouteSegmentStart(route Route, segmentIndex int, identity world.GameIdentity, gameVersion string, state world.State) error {
	if err := route.Validate(); err != nil {
		return fmt.Errorf("route invalid: %w", err)
	}
	if segmentIndex < 0 || segmentIndex >= len(route.Segments) {
		return fmt.Errorf("segment index %d out of range", segmentIndex)
	}
	if !identity.Valid {
		return ErrGameIdentityUnavailable
	}
	class, _ := parseCharacterClass(route.Binding.CharacterClass)
	if identity.CharacterName != route.Binding.CharacterName || identity.Class != class {
		return ErrRouteCharacterMismatch
	}
	if gameVersion == "" || gameVersion != route.Binding.GameVersion {
		return ErrRouteGameVersionMismatch
	}
	segment := route.Segments[segmentIndex]
	if !state.Valid || state.Phase != world.GamePhaseInGame || state.Area.ID != segment.FromAreaID {
		return ErrRouteStartMismatch
	}
	start := routePointPosition(segment.Points[0])
	if world.Distance(state.Player.Position, start) > route.Playback.MaxDriftTiles {
		return ErrRouteStartMismatch
	}
	return nil
}
