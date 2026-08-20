// Package replay records versioned, interpreted runtime traces for deterministic
// headless diagnosis. It never reads process memory and owns no OS input sender.
package replay

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// SchemaVersion is the current runtime-trace bundle schema.
	SchemaVersion = 1
	// BundleExtension is the only filename suffix managed by trace retention.
	BundleExtension = ".trace.gz"
)

// Bundle is one compressed, self-contained runtime decision trace.
type Bundle struct {
	SchemaVersion   int              `json:"schema_version"`
	Metadata        Metadata         `json:"metadata"`
	Contract        ContractSnapshot `json:"contract"`
	Checkpoints     []Checkpoint     `json:"checkpoints,omitempty"`
	Frames          []Frame          `json:"frames"`
	FramesTruncated bool             `json:"frames_truncated,omitempty"`
	Terminal        Terminal         `json:"terminal"`
}

// Metadata identifies the producer without exposing local filesystem state.
type Metadata struct {
	BotVersion string    `json:"bot_version"`
	Commit     string    `json:"commit"`
	Label      string    `json:"label"`
	CapturedAt time.Time `json:"captured_at"`
}

// ContractSnapshot freezes decision-relevant run, profile, route, and loadout
// contracts. Values are sanitized before they enter a bundle.
type ContractSnapshot struct {
	RunID        string         `json:"run_id"`
	Phase        string         `json:"phase,omitempty"`
	ProfileID    string         `json:"profile_id"`
	Character    string         `json:"character,omitempty"`
	Difficulty   string         `json:"difficulty,omitempty"`
	GameVersion  string         `json:"game_version,omitempty"`
	Dependencies []string       `json:"dependencies,omitempty"`
	Definition   map[string]any `json:"definition"`
	Route        map[string]any `json:"route,omitempty"`
	Policy       map[string]any `json:"policy,omitempty"`
	Loadout      map[string]any `json:"loadout,omitempty"`
	Tuning       map[string]any `json:"tuning,omitempty"`
}

// RuntimeGates captures read-only input authorization state observed before a tick.
type RuntimeGates struct {
	InputEnabled bool `json:"input_enabled"`
	Paused       bool `json:"paused"`
	Stopped      bool `json:"stopped"`
	WindowBound  bool `json:"window_bound"`
}

// TickState is the task state before or after one productive decision tick.
type TickState struct {
	Step    string `json:"step,omitempty"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
	Active  bool   `json:"active"`
}

// Frame contains one normalized World snapshot and every observed dependency
// result and semantic input intent produced while deciding that tick.
type Frame struct {
	Tick         uint64           `json:"tick"`
	ElapsedNS    int64            `json:"elapsed_ns"`
	SnapshotAtNS int64            `json:"snapshot_at_ns"`
	Generation   uint64           `json:"generation"`
	Gates        RuntimeGates     `json:"gates"`
	Before       TickState        `json:"before"`
	World        WorldFrame       `json:"world"`
	Dependencies []DependencyCall `json:"dependencies,omitempty"`
	Intents      []Intent         `json:"intents,omitempty"`
	After        TickState        `json:"after"`
}

// DependencyCall is one ordered interaction with a non-pure runtime dependency.
type DependencyCall struct {
	Sequence uint64         `json:"sequence"`
	Name     string         `json:"name"`
	Args     map[string]any `json:"args,omitempty"`
	Result   map[string]any `json:"result,omitempty"`
	Error    string         `json:"error,omitempty"`
}

// Intent is one semantic request that may have produced gameplay input. It
// records a request or a dependency-confirmed action, never raw key events.
type Intent struct {
	Sequence uint64         `json:"sequence"`
	Name     string         `json:"name"`
	Params   map[string]any `json:"params,omitempty"`
	Outcome  string         `json:"outcome,omitempty"`
}

// Checkpoint retains compact step transitions even when detailed frames fall
// out of the bounded ring.
type Checkpoint struct {
	Tick      uint64 `json:"tick"`
	ElapsedNS int64  `json:"elapsed_ns"`
	Step      string `json:"step"`
	Outcome   string `json:"outcome"`
	Reason    string `json:"reason,omitempty"`
}

// Terminal is the recorded run result used as the replay acceptance target.
type Terminal struct {
	Step    string `json:"step"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
}

// WorldFrame is the interpreted decision surface consumed by task code. It
// deliberately excludes pointers, raw memory values, item raw stats, and save data.
type WorldFrame struct {
	Phase           string           `json:"phase"`
	Valid           bool             `json:"valid"`
	Reason          string           `json:"reason,omitempty"`
	AreaID          uint32           `json:"area_id,omitempty"`
	Player          PlayerFrame      `json:"player"`
	Mercenary       MercenaryFrame   `json:"mercenary"`
	Identity        IdentityFrame    `json:"identity"`
	UI              UIFrame          `json:"ui"`
	Hover           HoverFrame       `json:"hover"`
	Objects         []EntityFrame    `json:"objects,omitempty"`
	Entrances       []EntityFrame    `json:"entrances,omitempty"`
	Monsters        []MonsterFrame   `json:"monsters,omitempty"`
	CowCorpses      []CowCorpseFrame `json:"cow_corpses,omitempty"`
	Items           []ItemFrame      `json:"items,omitempty"`
	MonsterCoverage MonsterCoverage  `json:"monster_coverage"`
	Evidence        map[string]bool  `json:"evidence,omitempty"`
}

// PlayerFrame is the normalized player decision state.
type PlayerFrame struct {
	X                      uint32   `json:"x"`
	Y                      uint32   `json:"y"`
	HP                     uint32   `json:"hp"`
	MaxHP                  uint32   `json:"max_hp"`
	Mana                   uint32   `json:"mana"`
	MaxMana                uint32   `json:"max_mana"`
	Gold                   uint32   `json:"gold,omitempty"`
	PrivateStashGold       uint32   `json:"private_stash_gold,omitempty"`
	GoldKnown              bool     `json:"gold_known"`
	PrivateStashGoldKnown  bool     `json:"private_stash_gold_known"`
	LeftSkillID            uint16   `json:"left_skill_id"`
	RightSkillID           uint16   `json:"right_skill_id"`
	ActiveWeaponSet        string   `json:"active_weapon_set,omitempty"`
	WeaponSetAvailable     bool     `json:"weapon_set_available"`
	SkillsKnown            []uint16 `json:"skills_known,omitempty"`
	SkillsComplete         bool     `json:"skills_complete"`
	SkillsIncompleteReason string   `json:"skills_incomplete_reason,omitempty"`
}

// MercenaryFrame is the fail-closed interpreted hireling state.
type MercenaryFrame struct {
	HiredKnown  bool   `json:"hired_known"`
	Hired       bool   `json:"hired"`
	Alive       bool   `json:"alive"`
	Dead        bool   `json:"dead"`
	VitalsKnown bool   `json:"vitals_known"`
	UnitID      uint32 `json:"unit_id,omitempty"`
	NPCID       uint32 `json:"npc_id,omitempty"`
	HP          uint32 `json:"hp,omitempty"`
	MaxHP       uint32 `json:"max_hp,omitempty"`
}

// IdentityFrame omits the diagnostic-only map seed.
type IdentityFrame struct {
	Valid         bool   `json:"valid"`
	CharacterName string `json:"character_name,omitempty"`
	Class         string `json:"class,omitempty"`
}

// UIFrame is the read-only UI safety projection.
type UIFrame struct {
	InventoryOpen   bool `json:"inventory_open"`
	NPCInteractOpen bool `json:"npc_interact_open"`
	NPCShopOpen     bool `json:"npc_shop_open"`
	WaypointOpen    bool `json:"waypoint_open"`
	StashOpen       bool `json:"stash_open"`
	QuitMenuOpen    bool `json:"quit_menu_open"`
	CubeOpen        bool `json:"cube_open"`
	CubeOpenKnown   bool `json:"cube_open_known"`
}

// HoverFrame is the semantic cursor evidence for the current snapshot.
type HoverFrame struct {
	Hovered  bool   `json:"hovered"`
	UnitType string `json:"unit_type,omitempty"`
	UnitID   uint32 `json:"unit_id,omitempty"`
}

// EntityFrame represents an interpreted object or entrance.
type EntityFrame struct {
	Kind      string `json:"kind"`
	ID        uint32 `json:"id"`
	UnitID    uint32 `json:"unit_id"`
	X         uint32 `json:"x"`
	Y         uint32 `json:"y"`
	Hovered   bool   `json:"hovered"`
	Mode      uint32 `json:"mode,omitempty"`
	ModeKnown bool   `json:"mode_known,omitempty"`
}

// MonsterFrame represents one interpreted living monster.
type MonsterFrame struct {
	NPCID    uint32 `json:"npc_id"`
	UnitID   uint32 `json:"unit_id"`
	X        uint32 `json:"x"`
	Y        uint32 `json:"y"`
	TypeFlag uint8  `json:"type_flag"`
	Hovered  bool   `json:"hovered"`
}

// CowCorpseFrame is current-generation Corpse Explosion evidence.
type CowCorpseFrame struct {
	NPCID            uint32 `json:"npc_id"`
	UnitID           uint32 `json:"unit_id"`
	X                uint32 `json:"x"`
	Y                uint32 `json:"y"`
	TypeFlag         uint8  `json:"type_flag"`
	Consumed         bool   `json:"consumed"`
	ConsumptionKnown bool   `json:"consumption_known"`
}

// ItemFrame contains semantic item identity and placement only.
type ItemFrame struct {
	TxtFileNo        uint32 `json:"txt_file_no"`
	UnitID           uint32 `json:"unit_id"`
	Code             string `json:"code"`
	Quality          string `json:"quality"`
	IdentityKind     string `json:"identity_kind,omitempty"`
	IdentityKey      string `json:"identity_key,omitempty"`
	IdentityValid    bool   `json:"identity_valid"`
	Location         string `json:"location"`
	OwnerID          uint32 `json:"owner_id,omitempty"`
	PlayerOwned      bool   `json:"player_owned"`
	Page             int    `json:"page,omitempty"`
	GridX            int    `json:"grid_x,omitempty"`
	GridY            int    `json:"grid_y,omitempty"`
	Width            int    `json:"width,omitempty"`
	Height           int    `json:"height,omitempty"`
	X                uint32 `json:"x,omitempty"`
	Y                uint32 `json:"y,omitempty"`
	Identified       bool   `json:"identified"`
	Ethereal         bool   `json:"ethereal"`
	Hovered          bool   `json:"hovered"`
	Sockets          int    `json:"sockets,omitempty"`
	SocketsAvailable bool   `json:"sockets_available"`
	Socketed         bool   `json:"socketed"`
	Quantity         int    `json:"quantity,omitempty"`
	QuantityKnown    bool   `json:"quantity_known,omitempty"`
}

// MonsterCoverage preserves the local enumeration completeness contract.
type MonsterCoverage struct {
	EligibleCount int     `json:"eligible_count"`
	Truncated     bool    `json:"truncated"`
	RadiusTiles   float64 `json:"radius_tiles"`
}

// NormalizeWorld builds a defensive, pointer-free trace projection.
func NormalizeWorld(state world.State) WorldFrame {
	skills := make([]uint16, 0, len(state.Player.SkillsKnown))
	for skillID, known := range state.Player.SkillsKnown {
		if known {
			skills = append(skills, skillID)
		}
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i] < skills[j] })
	activeWeaponSet := ""
	if state.Player.ActiveWeaponSet.Available {
		activeWeaponSet = state.Player.ActiveWeaponSet.Set.String()
	}
	frame := WorldFrame{
		Phase: state.Phase.String(), Valid: state.Valid, Reason: state.Reason,
		AreaID:          uint32(state.Area.ID),
		Player:          PlayerFrame{X: state.Player.Position.X, Y: state.Player.Position.Y, HP: state.Player.HP, MaxHP: state.Player.MaxHP, Mana: state.Player.Mana, MaxMana: state.Player.MaxMana, Gold: state.Player.Gold, PrivateStashGold: state.Player.PrivateStashGold, GoldKnown: state.Player.GoldKnown, PrivateStashGoldKnown: state.Player.PrivateStashGoldKnown, LeftSkillID: state.Player.LeftSkillID, RightSkillID: state.Player.RightSkillID, ActiveWeaponSet: activeWeaponSet, WeaponSetAvailable: state.Player.ActiveWeaponSet.Available, SkillsKnown: skills, SkillsComplete: state.Player.SkillsComplete, SkillsIncompleteReason: state.Player.SkillsIncompleteReason},
		Mercenary:       MercenaryFrame{HiredKnown: state.Mercenary.HiredKnown, Hired: state.Mercenary.Hired, Alive: state.Mercenary.Alive, Dead: state.Mercenary.Dead, VitalsKnown: state.Mercenary.VitalsKnown, UnitID: state.Mercenary.UnitID, NPCID: state.Mercenary.NPCID, HP: state.Mercenary.HP, MaxHP: state.Mercenary.MaxHP},
		Identity:        IdentityFrame{Valid: state.Identity.Valid, CharacterName: state.Identity.CharacterName, Class: state.Identity.Class.String()},
		UI:              UIFrame{InventoryOpen: state.UI.InventoryOpen, NPCInteractOpen: state.UI.NPCInteractOpen, NPCShopOpen: state.UI.NPCShopOpen, WaypointOpen: state.UI.WaypointOpen, StashOpen: state.UI.StashOpen, QuitMenuOpen: state.UI.QuitMenuOpen, CubeOpen: state.UI.CubeOpen, CubeOpenKnown: state.UI.CubeOpenKnown},
		Hover:           HoverFrame{Hovered: state.Hover.IsHovered, UnitType: state.Hover.UnitType.String(), UnitID: state.Hover.UnitID},
		MonsterCoverage: MonsterCoverage{EligibleCount: state.MonsterCoverage.EligibleMonsterCount, Truncated: state.MonsterCoverage.MonstersTruncated, RadiusTiles: state.MonsterCoverage.MonsterCoverageRadiusTiles},
		Evidence:        map[string]bool{"cow_corpses_complete": state.CowCorpsesComplete},
	}
	for _, object := range state.Objects {
		frame.Objects = append(frame.Objects, EntityFrame{Kind: object.Kind.String(), ID: object.ID, UnitID: object.UnitID, X: object.Position.X, Y: object.Position.Y, Hovered: object.IsHovered, Mode: object.Mode, ModeKnown: object.ModeKnown})
	}
	for _, entrance := range state.Entrances {
		frame.Entrances = append(frame.Entrances, EntityFrame{Kind: entrance.Kind.String(), ID: entrance.ID, UnitID: entrance.UnitID, X: entrance.Position.X, Y: entrance.Position.Y, Hovered: entrance.IsHovered})
	}
	for _, monster := range state.Monsters {
		frame.Monsters = append(frame.Monsters, MonsterFrame{NPCID: monster.NPCID, UnitID: monster.UnitID, X: monster.Position.X, Y: monster.Position.Y, TypeFlag: monster.MonsterTypeFlag, Hovered: monster.IsHovered})
	}
	for _, corpse := range state.CowCorpses {
		frame.CowCorpses = append(frame.CowCorpses, CowCorpseFrame{NPCID: corpse.NPCID, UnitID: corpse.UnitID, X: corpse.Position.X, Y: corpse.Position.Y, TypeFlag: corpse.MonsterTypeFlag, Consumed: corpse.Consumed, ConsumptionKnown: corpse.ConsumptionKnown})
	}
	for _, item := range state.Items {
		frame.Items = append(frame.Items, ItemFrame{TxtFileNo: item.TxtFileNo, UnitID: item.UnitID, Code: item.Code, Quality: item.Quality.String(), IdentityKind: string(item.IdentityKind), IdentityKey: item.IdentityKey, IdentityValid: item.IdentityValid, Location: item.Location.String(), OwnerID: item.OwnerID, PlayerOwned: item.PlayerOwned, Page: item.Page, GridX: item.GridX, GridY: item.GridY, Width: item.Width, Height: item.Height, X: item.Position.X, Y: item.Position.Y, Identified: item.Identified, Ethereal: item.Ethereal, Hovered: item.IsHovered, Sockets: item.Sockets, SocketsAvailable: item.SocketsAvailable, Socketed: item.Socketed, Quantity: item.Quantity, QuantityKnown: item.QuantityKnown})
	}
	return frame
}

// Validate checks structural and ordering invariants without executing a replay.
func (b Bundle) Validate() error {
	if b.SchemaVersion != SchemaVersion {
		return fmt.Errorf("runtime trace schema version %d is unsupported", b.SchemaVersion)
	}
	if strings.TrimSpace(b.Contract.RunID) == "" {
		return fmt.Errorf("runtime trace run_id is required")
	}
	if len(b.Frames) == 0 {
		return fmt.Errorf("runtime trace requires at least one frame")
	}
	var previousTick uint64
	var previousElapsed int64 = -1
	for index, frame := range b.Frames {
		if index > 0 && frame.Tick <= previousTick {
			return fmt.Errorf("runtime trace frame tick %d is not strictly increasing", frame.Tick)
		}
		if frame.ElapsedNS < previousElapsed {
			return fmt.Errorf("runtime trace frame tick %d moves monotonic time backwards", frame.Tick)
		}
		previousTick, previousElapsed = frame.Tick, frame.ElapsedNS
	}
	encoded, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("encode runtime trace for safety validation: %w", err)
	}
	if err := ValidateSafeJSON(encoded); err != nil {
		return err
	}
	return nil
}
