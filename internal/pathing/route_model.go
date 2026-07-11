package pathing

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// RouteVersion is the only route schema version supported by Phase 6.
	RouteVersion           = 1
	maxRoutePointStepTiles = 100.0
)

var (
	routeIDPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,63}$`)
	routeTagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
)

// Route is a versioned, run-independent navigation recording.
type Route struct {
	Version   int            `yaml:"version"`
	ID        string         `yaml:"id"`
	Name      string         `yaml:"name"`
	Kind      RouteKind      `yaml:"kind"`
	Tags      []string       `yaml:"tags,omitempty"`
	Binding   RouteBinding   `yaml:"binding"`
	Recording RouteRecording `yaml:"recording"`
	Playback  RoutePlayback  `yaml:"playback"`
	Segments  []RouteSegment `yaml:"segments"`
}

// RouteKind identifies the semantic content of a route file.
type RouteKind string

const (
	// RouteKindNavigation contains only navigation segments.
	RouteKindNavigation RouteKind = "navigation"
)

// RouteDifficulty is the operator-facing offline difficulty label.
type RouteDifficulty string

const (
	// RouteDifficultyNormal labels Normal recordings.
	RouteDifficultyNormal RouteDifficulty = "normal"
	// RouteDifficultyNightmare labels Nightmare recordings.
	RouteDifficultyNightmare RouteDifficulty = "nightmare"
	// RouteDifficultyHell labels Hell recordings.
	RouteDifficultyHell RouteDifficulty = "hell"
)

// RouteBinding binds a recording to a confirmed character, game version, and layout.
type RouteBinding struct {
	CharacterName     string                 `yaml:"character_name"`
	CharacterClass    string                 `yaml:"character_class"`
	Difficulty        RouteDifficulty        `yaml:"difficulty"`
	MapSeed           *uint32                `yaml:"map_seed,omitempty"`
	GameVersion       string                 `yaml:"game_version"`
	ProfileID         string                 `yaml:"profile_id,omitempty"`
	LayoutFingerprint RouteLayoutFingerprint `yaml:"layout_fingerprint"`
}

// RouteLayoutFingerprint stores the authoritative pre-input layout proof.
type RouteLayoutFingerprint struct {
	Version     int          `yaml:"version"`
	AreaID      world.AreaID `yaml:"area_id"`
	AnchorCount int          `yaml:"anchor_count"`
	Hash        string       `yaml:"hash"`
}

// RouteRecording describes how and when samples were captured.
type RouteRecording struct {
	RecordedAt          time.Time `yaml:"recorded_at"`
	SampleDistanceTiles float64   `yaml:"sample_distance_tiles"`
}

// RoutePlayback contains conservative playback safety limits.
type RoutePlayback struct {
	WaypointToleranceTiles float64 `yaml:"waypoint_tolerance_tiles"`
	MaxDriftTiles          float64 `yaml:"max_drift_tiles"`
	MaxLocalCorrections    int     `yaml:"max_local_corrections"`
	SegmentTimeoutMs       int     `yaml:"segment_timeout_ms"`
	TransitionTimeoutMs    int     `yaml:"transition_timeout_ms"`
}

// RouteMovement identifies the single movement method used by a segment.
type RouteMovement string

const (
	// RouteMovementTeleport replays points with teleport movement.
	RouteMovementTeleport RouteMovement = "teleport"
	// RouteMovementWalk replays points with walking movement.
	RouteMovementWalk RouteMovement = "walk"
)

// RouteSegment is one ordered movement sequence followed by an area transition.
type RouteSegment struct {
	ID         string          `yaml:"id"`
	FromAreaID world.AreaID    `yaml:"from_area_id"`
	ToAreaID   world.AreaID    `yaml:"to_area_id"`
	Movement   RouteMovement   `yaml:"movement"`
	Points     []RoutePoint    `yaml:"points"`
	Transition RouteTransition `yaml:"transition"`
}

// RoutePoint is a persistent world-coordinate sample.
type RoutePoint struct {
	X uint32 `yaml:"x"`
	Y uint32 `yaml:"y"`
}

// RouteTransition defines the Memory-confirmed transition after a segment.
type RouteTransition struct {
	Type         string `yaml:"type"`
	EntranceKind string `yaml:"entrance_kind"`
}

// Validate checks all Route Contract v1 fields and cross-segment invariants.
func (r Route) Validate() error {
	if r.Version != RouteVersion {
		return fmt.Errorf("version: got %d, want %d", r.Version, RouteVersion)
	}
	if err := ValidateRouteID(r.ID); err != nil {
		return err
	}
	if n := len([]rune(strings.TrimSpace(r.Name))); n < 1 || n > 80 {
		return fmt.Errorf("name: length %d outside 1..80", n)
	}
	if r.Kind != RouteKindNavigation {
		return fmt.Errorf("kind: unsupported value %q", r.Kind)
	}
	if err := validateRouteTags(r.Tags); err != nil {
		return err
	}
	if err := r.Binding.validate(); err != nil {
		return fmt.Errorf("binding.%w", err)
	}
	_, recordedOffset := r.Recording.RecordedAt.Zone()
	if r.Recording.RecordedAt.IsZero() || recordedOffset != 0 {
		return fmt.Errorf("recording.recorded_at: required UTC RFC3339 timestamp")
	}
	if !finitePositive(r.Recording.SampleDistanceTiles) || r.Recording.SampleDistanceTiles > 20 {
		return fmt.Errorf("recording.sample_distance_tiles: must be > 0 and <= 20")
	}
	if err := r.Playback.validate(); err != nil {
		return fmt.Errorf("playback.%w", err)
	}
	if len(r.Segments) == 0 {
		return fmt.Errorf("segments: must not be empty")
	}
	seen := make(map[string]struct{}, len(r.Segments))
	for i, segment := range r.Segments {
		if err := segment.validate(); err != nil {
			return fmt.Errorf("segments[%d].%w", i, err)
		}
		if _, exists := seen[segment.ID]; exists {
			return fmt.Errorf("segments[%d].id: duplicate %q", i, segment.ID)
		}
		seen[segment.ID] = struct{}{}
		if i > 0 && r.Segments[i-1].ToAreaID != segment.FromAreaID {
			return fmt.Errorf("segments[%d].from_area_id: got %d, previous to_area_id is %d", i, segment.FromAreaID, r.Segments[i-1].ToAreaID)
		}
	}
	if r.Binding.LayoutFingerprint.AreaID != r.Segments[0].FromAreaID {
		return fmt.Errorf("binding.layout_fingerprint.area_id: got %d, first segment starts in %d", r.Binding.LayoutFingerprint.AreaID, r.Segments[0].FromAreaID)
	}
	return nil
}

// ValidateRouteID checks the stable lower-kebab-case Route Contract identifier.
func ValidateRouteID(id string) error {
	if !routeIDPattern.MatchString(id) {
		return fmt.Errorf("id: %q does not match %s", id, routeIDPattern)
	}
	return nil
}

func validateRouteTags(tags []string) error {
	if len(tags) > 16 {
		return fmt.Errorf("tags: count %d exceeds 16", len(tags))
	}
	seen := make(map[string]struct{}, len(tags))
	for i, tag := range tags {
		if !routeTagPattern.MatchString(tag) {
			return fmt.Errorf("tags[%d]: invalid lower-kebab-case value %q", i, tag)
		}
		if _, exists := seen[tag]; exists {
			return fmt.Errorf("tags[%d]: duplicate %q", i, tag)
		}
		seen[tag] = struct{}{}
	}
	return nil
}

func (b RouteBinding) validate() error {
	if strings.TrimSpace(b.CharacterName) == "" || len([]rune(b.CharacterName)) > 32 {
		return fmt.Errorf("character_name: required with at most 32 characters")
	}
	if _, ok := parseCharacterClass(b.CharacterClass); !ok {
		return fmt.Errorf("character_class: unsupported value %q", b.CharacterClass)
	}
	switch b.Difficulty {
	case RouteDifficultyNormal, RouteDifficultyNightmare, RouteDifficultyHell:
	default:
		return fmt.Errorf("difficulty: unsupported value %q", b.Difficulty)
	}
	if strings.TrimSpace(b.GameVersion) == "" {
		return fmt.Errorf("game_version: required")
	}
	if b.LayoutFingerprint.Version != layoutFingerprintVersion || b.LayoutFingerprint.AreaID == 0 || b.LayoutFingerprint.AnchorCount < 1 || !isSHA256(b.LayoutFingerprint.Hash) {
		return fmt.Errorf("layout_fingerprint: invalid version, area, anchors, or hash")
	}
	return nil
}

func (p RoutePlayback) validate() error {
	if !finitePositive(p.WaypointToleranceTiles) {
		return fmt.Errorf("waypoint_tolerance_tiles: must be positive")
	}
	if !finitePositive(p.MaxDriftTiles) || p.MaxDriftTiles <= p.WaypointToleranceTiles {
		return fmt.Errorf("max_drift_tiles: must exceed waypoint_tolerance_tiles")
	}
	if p.MaxLocalCorrections < 0 || p.MaxLocalCorrections > 20 {
		return fmt.Errorf("max_local_corrections: must be within 0..20")
	}
	if p.SegmentTimeoutMs <= 0 || p.TransitionTimeoutMs <= 0 {
		return fmt.Errorf("timeouts: must be positive")
	}
	return nil
}

func (s RouteSegment) validate() error {
	if !routeIDPattern.MatchString(s.ID) {
		return fmt.Errorf("id: invalid value %q", s.ID)
	}
	if s.FromAreaID == 0 || s.ToAreaID == 0 || s.FromAreaID == s.ToAreaID {
		return fmt.Errorf("area_ids: must be non-zero and different")
	}
	if s.Movement != RouteMovementTeleport && s.Movement != RouteMovementWalk {
		return fmt.Errorf("movement: unsupported value %q", s.Movement)
	}
	if len(s.Points) == 0 {
		return fmt.Errorf("points: must not be empty")
	}
	for i, point := range s.Points {
		if point.X == 0 || point.Y == 0 {
			return fmt.Errorf("points[%d]: coordinates must be non-zero", i)
		}
		if i > 0 && routePointDistance(s.Points[i-1], point) > maxRoutePointStepTiles {
			return fmt.Errorf("points[%d]: step exceeds %.0f tiles", i, maxRoutePointStepTiles)
		}
	}
	if s.Transition.Type != "entrance" || strings.TrimSpace(s.Transition.EntranceKind) == "" {
		return fmt.Errorf("transition: type must be entrance with entrance_kind")
	}
	return nil
}

func finitePositive(v float64) bool { return v > 0 && !math.IsInf(v, 0) && !math.IsNaN(v) }

func routePointDistance(a, b RoutePoint) float64 {
	return math.Hypot(float64(a.X)-float64(b.X), float64(a.Y)-float64(b.Y))
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

func parseCharacterClass(value string) (world.CharacterClass, bool) {
	for class := world.CharacterClassAmazon; class <= world.CharacterClassAssassin; class++ {
		if class.String() == value {
			return class, true
		}
	}
	return 0, false
}
