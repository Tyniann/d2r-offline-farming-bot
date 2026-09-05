package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	cowWirtMaxClicks          = 2
	cowWirtSpawnSettle        = 2 * time.Second
	cowWirtInteractionTimeout = 12 * time.Second
	cowWirtLegSpawnDistance   = 8
	cowWirtClickDistance      = 8
	cowTomeVerifySnapshots    = 3
	// cowAkaraClickDistance is the live-NPC close-up used before hover. The
	// recorded Akara graph endpoint can leave her near the 15-tile click gate.
	cowAkaraClickDistance = 8
	// cowAkaraInteractDistance is hover-click slack after the close-up so one
	// wander step during probing does not abort as `npc_too_far`.
	cowAkaraInteractDistance = 15
	cowAkaraCloseTimeout     = 8 * time.Second
)

type cowWirtApproach interface {
	Active() bool
	Start(pathing.Goal) error
	Tick(context.Context, world.State) pathing.NavTickResult
	Reset()
}

// cowSetupAdapter composes existing click, Town graph, and vendor primitives
// without exposing them as a general quest or crafting framework.
type cowSetupAdapter struct {
	log        *slog.Logger
	controller townPreparationController
	pathCfg    pathing.Config
	approach   *townPreparationAdapter

	wirtClicker     *pathing.EntityClicker
	wirtApproach    cowWirtApproach
	wirtApproaching bool
	wirtStartedAt   time.Time
	wirtClickedAt   time.Time
	wirtClicks      int
	wirtUnitID      uint32
	wirtPosition    world.Position
	wirtInitialLegs map[uint32]bool
	wirtLegUnitID   uint32

	tomeStage        string
	tomeExisting     map[uint32]bool
	tomeNPC          *town.NPCInteractor
	tomeShop         *town.ShopOpener
	tomeBuyer        *town.VendorBuyer
	tomeUnitID       uint32
	tomeVerifyUnitID uint32
	tomeVerifyTicks  int
	tomeCloseSent    bool

	tomeCloseStarted  time.Time
	tomeCloseLastMove time.Time
	tomeCloseLastPos  world.Position
	tomeCloseProgress time.Time
}

func newCowSetupAdapterWithProfile(log *slog.Logger, controller townPreparationController, navigator *pathing.Navigator, pathCfg pathing.Config, cfg *config.Config, runID string, profileID string, layoutPin *townLayoutPin, trace town.ExecutorTelemetry) (*cowSetupAdapter, error) {
	approach, err := newTownPreparationAdapterWithProfile(log, controller, pathCfg, cfg, runID, profileID, layoutPin, trace, false)
	if err != nil {
		return nil, err
	}
	approach.thresholds = town.Thresholds{}
	approach.startAnchor = town.AnchorPortalArrival
	approach.targetAnchor = town.AnchorAkara
	clickCfg := pathCfg.Click
	clickCfg.AnchorOffsetTiles = 0
	return &cowSetupAdapter{
		log: log.With("component", "cow_setup"), controller: controller, pathCfg: pathCfg, approach: approach,
		wirtClicker: pathing.NewEntityClicker(log, controller, pathCfg.Projector(), clickCfg), wirtApproach: navigator,
	}, nil
}

func (a *cowSetupAdapter) TickWirt(ctx context.Context, state world.State) tasks.CowSetupActionResult {
	if a == nil || a.controller == nil || !state.Valid || state.Phase != world.GamePhaseInGame || state.Area.ID != world.Tristram {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_wirt_state_invalid"}
	}
	now := state.At
	if now.IsZero() {
		now = time.Now()
	}
	if a.wirtStartedAt.IsZero() {
		a.wirtStartedAt = now
		a.wirtInitialLegs = make(map[uint32]bool)
		for _, item := range state.GroundItems() {
			if item.Code == "leg" {
				a.wirtInitialLegs[item.UnitID] = true
			}
		}
	}
	if now.Sub(a.wirtStartedAt) >= cowWirtInteractionTimeout {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_wirt_interaction_failed"}
	}

	if a.wirtClicks > 0 {
		legs := a.newWirtLegs(state)
		if len(legs) > 1 {
			return tasks.CowSetupActionResult{Done: true, Reason: "cow_leg_spawn_ambiguous"}
		}
		if len(legs) == 1 {
			a.wirtLegUnitID = legs[0].UnitID
			a.log.Info("Wirt's Leg gebunden", "wirt_unit_id", a.wirtUnitID, "leg_unit_id", a.wirtLegUnitID, "clicks", a.wirtClicks)
			return tasks.CowSetupActionResult{Done: true, UnitID: a.wirtLegUnitID}
		}
		if now.Sub(a.wirtClickedAt) < cowWirtSpawnSettle {
			return tasks.CowSetupActionResult{}
		}
		if a.wirtClicks >= cowWirtMaxClicks {
			return tasks.CowSetupActionResult{Done: true, Reason: "cow_leg_spawn_failed"}
		}
		a.wirtClicker.Reset()
	}

	wirt, ok := nearestWirt(state)
	if !ok || wirt.UnitID == 0 {
		return tasks.CowSetupActionResult{}
	}
	if a.wirtUnitID != 0 && a.wirtUnitID != wirt.UnitID {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_wirt_pin_changed"}
	}
	a.wirtUnitID, a.wirtPosition = wirt.UnitID, wirt.Position
	if a.wirtApproaching {
		result := a.wirtApproach.Tick(ctx, state)
		if !result.Done {
			return tasks.CowSetupActionResult{}
		}
		a.wirtApproaching = false
		if result.Status != pathing.NavArrived {
			return tasks.CowSetupActionResult{Done: true, Reason: "cow_wirt_approach_failed"}
		}
		// Require one newer snapshot after arrival before projecting the body.
		return tasks.CowSetupActionResult{}
	}
	if world.Distance(state.Player.Position, wirt.Position) > cowWirtClickDistance {
		if a.wirtApproach == nil || a.wirtApproach.Active() {
			return tasks.CowSetupActionResult{Done: true, Reason: "cow_wirt_approach_failed"}
		}
		a.wirtClicker.Reset()
		if err := a.wirtApproach.Start(pathing.Goal{
			Kind: pathing.GoalKindMoveToPosition, TargetPos: wirt.Position, ArrivalDistance: cowWirtClickDistance,
		}); err != nil {
			return tasks.CowSetupActionResult{Done: true, Reason: "cow_wirt_approach_failed"}
		}
		a.wirtApproaching = true
		a.log.Info("Wirts Körper wird vor dem Öffnen angenähert",
			"unit_id", wirt.UnitID, "distance", world.Distance(state.Player.Position, wirt.Position))
		return tasks.CowSetupActionResult{}
	}
	result, err := a.wirtClicker.Tick(state, pathing.ClickTarget{UnitID: wirt.UnitID, UnitType: world.HoverUnitTypeObject, Position: wirt.Position, Name: wirt.Name}, cowWirtClickDistance)
	if err != nil {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_wirt_input_failed"}
	}
	if result.Status == pathing.ClickHit {
		a.wirtClicks++
		a.wirtClickedAt = now
		a.log.Info("Wirts Körper bestätigt angeklickt", "unit_id", wirt.UnitID, "attempt", a.wirtClicks, "hover_attempts", result.Attempt)
		return tasks.CowSetupActionResult{}
	}
	if result.Done {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_wirt_hover_failed"}
	}
	return tasks.CowSetupActionResult{}
}

func (a *cowSetupAdapter) newWirtLegs(state world.State) []world.Item {
	legs := make([]world.Item, 0, 1)
	for _, item := range state.GroundItems() {
		if item.Code != "leg" || a.wirtInitialLegs[item.UnitID] || world.Distance(item.Position, a.wirtPosition) > cowWirtLegSpawnDistance {
			continue
		}
		legs = append(legs, item)
	}
	return legs
}

func nearestWirt(state world.State) (world.Object, bool) {
	var selected world.Object
	best := 0.0
	for _, object := range state.Objects {
		if object.Kind != world.ObjectKindWirtsBody || object.ID != world.WirtsBodyID {
			continue
		}
		distance := world.Distance(state.Player.Position, object.Position)
		if selected.UnitID == 0 || distance < best {
			selected, best = object, distance
		}
	}
	return selected, selected.UnitID != 0
}

func (a *cowSetupAdapter) TickTome(ctx context.Context, state world.State) tasks.CowSetupActionResult {
	if a == nil || a.controller == nil || !state.Valid || state.Phase != world.GamePhaseInGame || state.Area.ID != world.RogueEncampment {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_tome_state_invalid"}
	}
	if a.tomeStage == "" {
		a.tomeStage = "approach"
	}
	switch a.tomeStage {
	case "approach":
		result := a.approach.Tick(ctx, state)
		if !result.Done {
			return tasks.CowSetupActionResult{}
		}
		if result.Status != "complete" {
			return tasks.CowSetupActionResult{Done: true, Reason: "cow_akara_approach_failed"}
		}
		a.tomeExisting = inventoryItemIDsByCode(state, "tbk")
		if len(a.tomeExisting) == 0 {
			return tasks.CowSetupActionResult{Done: true, Reason: tasks.CowReasonReturnPortalUnavailable}
		}
		a.tomeStage = "approach_npc"
		return a.tickAkaraClose(state)
	case "approach_npc":
		return a.tickAkaraClose(state)
	case "npc":
		result := a.tomeNPC.Tick(state)
		if result.Status == town.InteractionComplete {
			a.tomeShop = town.NewShopOpener(a.controller, 8*time.Second)
			a.tomeStage = "shop"
			return tasks.CowSetupActionResult{}
		}
		if result.Done && result.Status == town.InteractionFailed {
			a.log.Warn("Akara-Interaktion fehlgeschlagen", "reason", result.Reason, "unit_id", result.UnitID)
			return tasks.CowSetupActionResult{Done: true, Reason: "cow_akara_interaction_failed"}
		}
		return tasks.CowSetupActionResult{}
	case "shop":
		result := a.tomeShop.Tick(state)
		if result.Status == town.InteractionComplete {
			a.tomeBuyer = town.NewVendorBuyer(a.controller, town.VendorRequest{Code: "tbk", Mode: town.BuyModeSingle})
			a.tomeStage = "buy"
			return tasks.CowSetupActionResult{}
		}
		if result.Done && result.Status == town.InteractionFailed {
			return tasks.CowSetupActionResult{Done: true, Reason: "cow_tome_shop_failed"}
		}
		return tasks.CowSetupActionResult{}
	case "buy":
		result := a.tomeBuyer.Tick(state)
		if result.Status == town.InteractionAction && result.Action == "vendor_buy_single" {
			a.tomeStage = "verify"
			return tasks.CowSetupActionResult{}
		}
		if result.Done && result.Status == town.InteractionFailed {
			return tasks.CowSetupActionResult{Done: true, Reason: "cow_tome_purchase_failed"}
		}
		return tasks.CowSetupActionResult{}
	case "verify":
		for unitID := range a.tomeExisting {
			item, ok := state.FindItemByUnitID(unitID)
			if !ok || item.Code != "tbk" || item.Location != world.ItemLocationInventory {
				return tasks.CowSetupActionResult{Done: true, Reason: "cow_operational_tome_changed"}
			}
		}
		current := inventoryItemIDsByCode(state, "tbk")
		newIDs := make([]uint32, 0, 1)
		for unitID := range current {
			if !a.tomeExisting[unitID] {
				newIDs = append(newIDs, unitID)
			}
		}
		if len(newIDs) > 1 {
			return tasks.CowSetupActionResult{Done: true, Reason: "cow_tome_purchase_ambiguous"}
		}
		if len(newIDs) == 0 {
			a.tomeVerifyTicks = 0
			return tasks.CowSetupActionResult{}
		}
		if a.tomeVerifyUnitID != newIDs[0] {
			a.tomeVerifyUnitID, a.tomeVerifyTicks = newIDs[0], 1
			return tasks.CowSetupActionResult{}
		}
		a.tomeVerifyTicks++
		if a.tomeVerifyTicks < cowTomeVerifySnapshots {
			return tasks.CowSetupActionResult{}
		}
		a.tomeUnitID = newIDs[0]
		a.tomeStage = "close"
		return tasks.CowSetupActionResult{}
	case "close":
		if !state.UI.NPCShopOpen && !state.UI.NPCInteractOpen {
			a.log.Info("Cow-Setup-Tome gebunden", "new_tome_unit_id", a.tomeUnitID, "existing_tome_count", len(a.tomeExisting))
			return tasks.CowSetupActionResult{Done: true, UnitID: a.tomeUnitID}
		}
		if !a.tomeCloseSent {
			if err := a.controller.PressKey("esc"); err != nil {
				a.log.Error("Cow-Setup-Tome: Händlerfenster konnte nicht geschlossen werden", "error", err)
				return tasks.CowSetupActionResult{Done: true, Reason: "cow_tome_shop_close_failed"}
			}
			a.tomeCloseSent = true
		}
		return tasks.CowSetupActionResult{}
	default:
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_tome_state_invalid"}
	}
}

func inventoryItemIDsByCode(state world.State, code string) map[uint32]bool {
	ids := make(map[uint32]bool)
	for _, item := range state.InventoryItems() {
		if item.Code == code {
			ids[item.UnitID] = true
		}
	}
	return ids
}

func (a *cowSetupAdapter) Reset() {
	if a == nil {
		return
	}
	if a.approach != nil {
		a.approach.Reset()
		a.approach.startAnchor = town.AnchorPortalArrival
		a.approach.targetAnchor = town.AnchorAkara
	}
	if a.wirtClicker != nil {
		a.wirtClicker.Reset()
	}
	if a.wirtApproaching && a.wirtApproach != nil {
		a.wirtApproach.Reset()
	}
	a.wirtApproaching = false
	a.wirtStartedAt, a.wirtClickedAt = time.Time{}, time.Time{}
	a.wirtClicks, a.wirtUnitID, a.wirtLegUnitID = 0, 0, 0
	a.wirtPosition = world.Position{}
	a.wirtInitialLegs = nil
	a.tomeStage = ""
	a.tomeExisting = nil
	a.tomeNPC, a.tomeShop, a.tomeBuyer = nil, nil, nil
	a.tomeUnitID, a.tomeVerifyUnitID, a.tomeVerifyTicks = 0, 0, 0
	a.tomeCloseSent = false
	a.tomeCloseStarted, a.tomeCloseLastMove, a.tomeCloseProgress = time.Time{}, time.Time{}, time.Time{}
	a.tomeCloseLastPos = world.Position{}
}

func (a *cowSetupAdapter) tickAkaraClose(state world.State) tasks.CowSetupActionResult {
	npc, ok := state.FindNPC(world.Akara)
	if !ok || npc.UnitID == 0 {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_akara_approach_failed"}
	}
	distance := world.Distance(state.Player.Position, npc.Position)
	if distance <= cowAkaraClickDistance {
		a.beginTomeNPC()
		a.tomeStage = "npc"
		return tasks.CowSetupActionResult{}
	}
	now := state.At
	if now.IsZero() {
		now = time.Now()
	}
	if a.tomeCloseStarted.IsZero() {
		a.tomeCloseStarted = now
		a.tomeCloseLastPos = state.Player.Position
		a.tomeCloseProgress = now
		a.log.Info("Akara wird vor dem Öffnen angenähert", "unit_id", npc.UnitID, "distance", distance)
	}
	if now.Sub(a.tomeCloseStarted) >= cowAkaraCloseTimeout {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_akara_approach_failed"}
	}
	if world.Distance(a.tomeCloseLastPos, state.Player.Position) >= 1 {
		a.tomeCloseProgress = now
		a.tomeCloseLastPos = state.Player.Position
	}
	if now.Sub(a.tomeCloseProgress) >= a.pathCfg.TownWalk.StuckTimeout {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_akara_approach_failed"}
	}
	if !a.tomeCloseLastMove.IsZero() && now.Sub(a.tomeCloseLastMove) < a.pathCfg.TownWalk.MoveInterval {
		return tasks.CowSetupActionResult{}
	}
	win, ok := a.controller.Window()
	if !ok {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_akara_approach_failed"}
	}
	clientX, clientY, ok := a.pathCfg.Projector().Project(state.Player.Position, npc.Position, win)
	if !ok {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_akara_approach_failed"}
	}
	if err := a.controller.MoveTo(clientX, clientY); err != nil {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_akara_approach_failed"}
	}
	if err := a.controller.PressKey(a.pathCfg.TownWalk.ForceMoveKey); err != nil {
		return tasks.CowSetupActionResult{Done: true, Reason: "cow_akara_approach_failed"}
	}
	a.tomeCloseLastMove = now
	return tasks.CowSetupActionResult{}
}

func (a *cowSetupAdapter) beginTomeNPC() {
	clickCfg := a.pathCfg.Click
	clickCfg.AnchorOffsetTiles = 0
	clicker := pathing.NewEntityClicker(a.log, a.controller, a.pathCfg.Projector(), clickCfg)
	a.tomeNPC = town.NewNPCInteractor(townNPCClickerAdapter{clicker: clicker}, world.Akara, cowAkaraInteractDistance, 8*time.Second)
}

var _ tasks.CowSetupActions = (*cowSetupAdapter)(nil)
var _ town.ShopInput = (townPreparationController)(nil)
