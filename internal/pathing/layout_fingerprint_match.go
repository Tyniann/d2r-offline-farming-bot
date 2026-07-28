package pathing

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// SystemEgressLayoutAnchorToleranceTiles absorbs small Memory coordinate jitter
// on the same stable Town object. Live Lut Gholein evidence showed Personal Stash
// Y drifting by three tiles between an accepted Egress binding and a later game.
const SystemEgressLayoutAnchorToleranceTiles = 4.0

// MatchSystemEgressLayout compares a live fingerprint to a persisted system-egress binding.
// When boundAnchors is non-empty, identity and coordinate tolerance are authoritative and
// Hash is ignored. Legacy files without anchors keep exact Hash equality.
func MatchSystemEgressLayout(live LayoutFingerprint, version int, areaID world.AreaID, anchorCount int, hash string, boundAnchors []string) error {
	if live.Version != version || live.AreaID != areaID {
		return fmt.Errorf("%w: version/area live=%d/%d bound=%d/%d", ErrRouteLayoutMismatch, live.Version, live.AreaID, version, areaID)
	}
	if len(boundAnchors) > 0 {
		if live.AnchorCount != len(boundAnchors) || anchorCount != len(boundAnchors) {
			return fmt.Errorf("%w: anchor_count live=%d bound=%d anchors=%d", ErrRouteLayoutMismatch, live.AnchorCount, anchorCount, len(boundAnchors))
		}
		if err := matchLayoutAnchorsWithin(live.Anchors, boundAnchors, SystemEgressLayoutAnchorToleranceTiles); err != nil {
			return fmt.Errorf("%w: %v", ErrRouteLayoutMismatch, err)
		}
		return nil
	}
	if live.AnchorCount != anchorCount || live.Hash != hash {
		return fmt.Errorf("%w: active=%s route=%s", ErrRouteLayoutMismatch, live.Hash, hash)
	}
	return nil
}

type layoutAnchor struct {
	key string
	x   uint32
	y   uint32
}

func matchLayoutAnchorsWithin(live, expected []string, maxTileDelta float64) error {
	if len(live) != len(expected) {
		return fmt.Errorf("anchor count live=%d expected=%d", len(live), len(expected))
	}
	liveByKey := make(map[string]layoutAnchor, len(live))
	for _, raw := range live {
		anchor, err := parseLayoutAnchor(raw)
		if err != nil {
			return err
		}
		if _, exists := liveByKey[anchor.key]; exists {
			return fmt.Errorf("duplicate live anchor %s", anchor.key)
		}
		liveByKey[anchor.key] = anchor
	}
	for _, raw := range expected {
		want, err := parseLayoutAnchor(raw)
		if err != nil {
			return err
		}
		got, ok := liveByKey[want.key]
		if !ok {
			return fmt.Errorf("missing live anchor %s", want.key)
		}
		distance := world.Distance(world.Position{X: got.x, Y: got.y}, world.Position{X: want.x, Y: want.y})
		if distance > maxTileDelta {
			return fmt.Errorf("anchor %s distance %.1f exceeds %.1f", want.key, distance, maxTileDelta)
		}
	}
	return nil
}

func parseLayoutAnchor(raw string) (layoutAnchor, error) {
	parts := strings.Split(raw, ":")
	switch {
	case len(parts) == 3 && parts[0] == "o":
		x, y, err := parseAnchorXY(parts[2])
		if err != nil {
			return layoutAnchor{}, fmt.Errorf("object anchor %q: %w", raw, err)
		}
		return layoutAnchor{key: "o:" + parts[1], x: x, y: y}, nil
	case len(parts) == 4 && parts[0] == "e":
		x, y, err := parseAnchorXY(parts[3])
		if err != nil {
			return layoutAnchor{}, fmt.Errorf("entrance anchor %q: %w", raw, err)
		}
		return layoutAnchor{key: "e:" + parts[1] + ":" + parts[2], x: x, y: y}, nil
	default:
		return layoutAnchor{}, fmt.Errorf("unsupported layout anchor %q", raw)
	}
}

func parseAnchorXY(value string) (uint32, uint32, error) {
	coords := strings.Split(value, ",")
	if len(coords) != 2 {
		return 0, 0, fmt.Errorf("expected x,y got %q", value)
	}
	x, err := strconv.ParseUint(coords[0], 10, 32)
	if err != nil {
		return 0, 0, err
	}
	y, err := strconv.ParseUint(coords[1], 10, 32)
	if err != nil {
		return 0, 0, err
	}
	return uint32(x), uint32(y), nil
}
