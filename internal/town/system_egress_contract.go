package town

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
	"gopkg.in/yaml.v3"
)

const (
	// SystemEgressSchemaVersion is the current global Egress file format.
	SystemEgressSchemaVersion = 1
	// SystemEgressFilename is the fixed filename below every act Egress root.
	SystemEgressFilename = "portal-waypoint.yaml"
)

// SystemEgressMovement identifies the movement allowed by a global Egress.
type SystemEgressMovement string

const (
	// SystemEgressMovementWalk is the only supported global Egress movement.
	SystemEgressMovementWalk SystemEgressMovement = "walk"
)

// SystemEgressLayoutFingerprint binds a system route to visible Town geometry.
type SystemEgressLayoutFingerprint struct {
	Version     int          `yaml:"version"`
	AreaID      world.AreaID `yaml:"area_id"`
	AnchorCount int          `yaml:"anchor_count"`
	Hash        string       `yaml:"hash"`
	// Anchors are the canonical layout lines captured at recording time.
	// When present, playback matches them with coordinate tolerance instead of Hash.
	Anchors []string `yaml:"anchors,omitempty"`
}

// SystemEgressContract defines one global portal-arrival-to-waypoint route.
// Character, class, difficulty and map seed are deliberately absent because
// they must never authorize or reject a system route.
type SystemEgressContract struct {
	Act                   OriginAct                     `yaml:"act"`
	TownArea              world.AreaID                  `yaml:"town_area_id"`
	GameVersion           string                        `yaml:"game_version"`
	LayoutFingerprint     SystemEgressLayoutFingerprint `yaml:"layout_fingerprint"`
	From                  Anchor                        `yaml:"from"`
	To                    Anchor                        `yaml:"to"`
	Movement              SystemEgressMovement          `yaml:"movement"`
	ArrivalToleranceTiles float64                       `yaml:"arrival_tolerance_tiles"`
}

// SystemEgressRoute is the complete global system route persisted outside the
// character-bound farming lifecycle.
type SystemEgressRoute struct {
	SchemaVersion       int                  `yaml:"schema_version"`
	Contract            SystemEgressContract `yaml:"contract"`
	SampleDistanceTiles float64              `yaml:"sample_distance_tiles"`
	Points              []SystemEgressPoint  `yaml:"points"`
}

// SystemEgressPoint is one immutable world-space walk sample.
type SystemEgressPoint struct {
	X uint32 `yaml:"x"`
	Y uint32 `yaml:"y"`
}

// Validate rejects unsupported acts, missing semantic anchors and non-walk movement.
func (c SystemEgressContract) Validate() error {
	if c.Act != OriginAct2 && c.Act != OriginAct3 && c.Act != OriginAct4 && c.Act != OriginAct5 {
		return fmt.Errorf("system egress act must be act2, act3, act4, or act5")
	}
	if c.TownArea == world.None || strings.TrimSpace(c.GameVersion) == "" {
		return fmt.Errorf("system egress town area and game version are required")
	}
	if c.LayoutFingerprint.Version == 0 || c.LayoutFingerprint.AreaID != c.TownArea || c.LayoutFingerprint.AnchorCount <= 0 || strings.TrimSpace(c.LayoutFingerprint.Hash) == "" {
		return fmt.Errorf("system egress layout fingerprint must bind the town area")
	}
	if len(c.LayoutFingerprint.Anchors) > 0 && len(c.LayoutFingerprint.Anchors) != c.LayoutFingerprint.AnchorCount {
		return fmt.Errorf("system egress layout fingerprint anchors must match anchor_count")
	}
	if c.From != AnchorPortalArrival || c.To != AnchorWaypoint || c.Movement != SystemEgressMovementWalk || c.ArrivalToleranceTiles <= 0 {
		return fmt.Errorf("system egress requires portal_arrival to waypoint walk movement and positive tolerance")
	}
	return nil
}

// Validate rejects malformed, cross-act and unplayable global Egress routes.
func (r SystemEgressRoute) Validate() error {
	if r.SchemaVersion != SystemEgressSchemaVersion {
		return fmt.Errorf("system egress schema_version must be %d", SystemEgressSchemaVersion)
	}
	if err := r.Contract.Validate(); err != nil {
		return err
	}
	wantArea, ok := TownAreaForAct(r.Contract.Act)
	if !ok || r.Contract.TownArea != wantArea {
		return fmt.Errorf("system egress town area does not match %s", r.Contract.Act)
	}
	if r.SampleDistanceTiles <= 0 || len(r.Points) < 2 {
		return fmt.Errorf("system egress requires a positive sample distance and at least two points")
	}
	return nil
}

// TownAreaForAct resolves the only supported town area for a foreign act.
func TownAreaForAct(act OriginAct) (world.AreaID, bool) {
	switch act {
	case OriginAct2:
		return world.LutGholein, true
	case OriginAct3:
		return world.KurastDocks, true
	case OriginAct4:
		return world.ThePandemoniumFortress, true
	case OriginAct5:
		return world.Harrogath, true
	default:
		return world.None, false
	}
}

// LoadSystemEgressRoute reads one strict global Egress YAML file.
func LoadSystemEgressRoute(path string) (SystemEgressRoute, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SystemEgressRoute{}, fmt.Errorf("read system egress %q: %w", path, err)
	}
	var route SystemEgressRoute
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&route); err != nil {
		return SystemEgressRoute{}, fmt.Errorf("parse system egress %q: %w", path, err)
	}
	if err := route.Validate(); err != nil {
		return SystemEgressRoute{}, fmt.Errorf("validate system egress %q: %w", path, err)
	}
	return route, nil
}

// SaveSystemEgressRoute atomically publishes one validated global Egress file.
func SaveSystemEgressRoute(path string, route SystemEgressRoute) error {
	if err := route.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(route)
	if err != nil {
		return fmt.Errorf("marshal system egress: %w", err)
	}
	if mkdirErr := os.MkdirAll(filepath.Dir(path), 0o755); mkdirErr != nil {
		return fmt.Errorf("create system egress directory: %w", mkdirErr)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".system-egress-*.tmp")
	if err != nil {
		return fmt.Errorf("create system egress temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write system egress temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync system egress temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close system egress temporary file: %w", err)
	}
	if err := replaceSystemEgressFile(temporaryPath, path); err != nil {
		return fmt.Errorf("publish system egress: %w", err)
	}
	return nil
}
