package pathing

import (
	"fmt"
	"sort"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// WaypointTargetID identifies a registered, Memory-verified waypoint destination.
// An ID alone never authorizes UI input; the waypoint executor must resolve it
// through its resolution-bound action registry before selecting a destination.
type WaypointTargetID string

const (
	// WaypointTargetBlackMarsh selects the Act-1 Black Marsh destination.
	WaypointTargetBlackMarsh WaypointTargetID = "black_marsh"
	// WaypointTargetDuranceOfHateLevel2 selects the Act-3 Durance Level 2 destination.
	WaypointTargetDuranceOfHateLevel2 WaypointTargetID = "durance_of_hate_level_2"
	// WaypointTargetRogueEncampment selects the Act-1 hub destination.
	WaypointTargetRogueEncampment WaypointTargetID = "rogue_encampment"
)

const (
	waypointClientWidth  = 1280
	waypointClientHeight = 720
	waypointTabY         = 148
	waypointAct1TabX     = 159
	waypointAct3TabX     = 273
	waypointRowX         = 200
	waypointTabSettle    = 200 * time.Millisecond
)

// WaypointTargetAction is one immutable, resolution-bound waypoint menu action.
// Coordinates are client-relative and never authorize input without the
// Memory-confirmed waypoint menu gate.
type WaypointTargetAction struct {
	ID             WaypointTargetID `json:"id"`
	Name           string           `json:"name"`
	Act            int              `json:"act"`
	TabX           int              `json:"tab_x"`
	TabY           int              `json:"tab_y"`
	RowX           int              `json:"row_x"`
	RowY           int              `json:"row_y"`
	ClientWidth    int              `json:"client_width"`
	ClientHeight   int              `json:"client_height"`
	SettleMs       int64            `json:"settle_ms"`
	ExpectedAreaID world.AreaID     `json:"expected_area_id"`
}

// WaypointTargetRegistry stores validated waypoint actions by stable target ID.
type WaypointTargetRegistry struct {
	actions map[WaypointTargetID]WaypointTargetAction
	ids     []WaypointTargetID
}

// NewWaypointTargetRegistry validates actions and rejects duplicate IDs.
func NewWaypointTargetRegistry(actions ...WaypointTargetAction) (*WaypointTargetRegistry, error) {
	r := &WaypointTargetRegistry{actions: make(map[WaypointTargetID]WaypointTargetAction, len(actions))}
	for i, action := range actions {
		if action.ID == "" || action.Name == "" || action.Act < 1 || action.Act > 5 || action.ExpectedAreaID == world.None {
			return nil, fmt.Errorf("waypoint action[%d] has incomplete identity", i)
		}
		if action.ClientWidth <= 0 || action.ClientHeight <= 0 || action.TabX < 0 || action.TabY < 0 || action.RowX < 0 || action.RowY < 0 || action.SettleMs <= 0 {
			return nil, fmt.Errorf("waypoint action[%d] has invalid geometry or settle", i)
		}
		if _, exists := r.actions[action.ID]; exists {
			return nil, fmt.Errorf("waypoint action[%d] duplicates target %q", i, action.ID)
		}
		r.actions[action.ID] = action
		r.ids = append(r.ids, action.ID)
	}
	sort.Slice(r.ids, func(i, j int) bool { return r.ids[i] < r.ids[j] })
	return r, nil
}

// DefaultWaypointTargetRegistry returns the calibrated 1280×720 Phase-10 targets.
func DefaultWaypointTargetRegistry() *WaypointTargetRegistry {
	registry, err := NewWaypointTargetRegistry(
		waypointTargetAction(WaypointTargetBlackMarsh, "Black Marsh", 1, 5, world.BlackMarsh),
		waypointTargetAction(WaypointTargetDuranceOfHateLevel2, "Durance of Hate Level 2", 3, 9, world.DuranceOfHateLevel2),
		waypointTargetAction(WaypointTargetRogueEncampment, "Rogue Encampment", 1, 1, world.RogueEncampment),
	)
	if err != nil {
		panic(fmt.Sprintf("invalid built-in waypoint registry: %v", err))
	}
	return registry
}

func waypointTargetAction(id WaypointTargetID, name string, act, row int, area world.AreaID) WaypointTargetAction {
	tabX := waypointAct1TabX
	if act == 3 {
		tabX = waypointAct3TabX
	}
	return WaypointTargetAction{
		ID: id, Name: name, Act: act, TabX: tabX, TabY: waypointTabY,
		RowX: waypointRowX, RowY: 158 + (row-1)*41 + 20,
		ClientWidth: waypointClientWidth, ClientHeight: waypointClientHeight,
		SettleMs: waypointTabSettle.Milliseconds(), ExpectedAreaID: area,
	}
}

// Action returns a registered target action.
func (r *WaypointTargetRegistry) Action(id WaypointTargetID) (WaypointTargetAction, bool) {
	if r == nil {
		return WaypointTargetAction{}, false
	}
	action, ok := r.actions[id]
	return action, ok
}

// Actions returns all registered actions in stable ID order.
func (r *WaypointTargetRegistry) Actions() []WaypointTargetAction {
	if r == nil {
		return nil
	}
	actions := make([]WaypointTargetAction, 0, len(r.ids))
	for _, id := range r.ids {
		actions = append(actions, r.actions[id])
	}
	return actions
}
