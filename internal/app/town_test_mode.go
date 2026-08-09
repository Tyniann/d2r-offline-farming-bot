package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const townTestTimeout = 60 * time.Second
const townPurchaseSettle = 500 * time.Millisecond

type townTestController interface {
	inputController
	pathing.InputDriver
	town.ShopInput
}

type townNPCClickerAdapter struct{ clicker *pathing.EntityClicker }

func (a townNPCClickerAdapter) TickNPC(state world.State, target town.NPCClickTarget, maxDistance float64) (town.NPCClickResult, error) {
	result, err := a.clicker.Tick(state, pathing.ClickTarget{UnitID: target.UnitID, UnitType: world.HoverUnitTypeMonster, Position: target.Position, Name: target.Name}, maxDistance)
	return town.NPCClickResult{Clicked: result.Status == pathing.ClickHit, Done: result.Done, Reason: string(result.Status)}, err
}

func (a townNPCClickerAdapter) Reset() { a.clicker.Reset() }

// RunTownTest executes one isolated Town interaction acceptance flow.
func (rt *Runtime) RunTownTest(spec string) error {
	switch strings.ToLower(strings.TrimSpace(spec)) {
	case "akara-shop":
		return rt.runAkaraShopTownTest()
	case "item-services:mephisto":
		return rt.runItemServicesTownTest()
	case "mercenary-heal":
		return rt.runMercenaryHealTownTest()
	case "mercenary-revive":
		return rt.runMercenaryReviveTownTest()
	default:
		return fmt.Errorf("town test: unsupported spec %q", spec)
	}
}

func (rt *Runtime) runAkaraShopTownTest() error {
	if !rt.Input.Status().Enabled {
		return fmt.Errorf("town test requires input.enabled=true")
	}
	ctrl, ok := rt.Input.(townTestController)
	if !ok {
		return fmt.Errorf("town test: controller lacks click or modified-click support")
	}
	if err := validateAkaraBulkProfile(rt.Config); err != nil {
		return err
	}
	window, ok := ctrl.Window()
	if ok && (window.ClientWidth != 1280 || window.ClientHeight != 720) {
		return fmt.Errorf("town test requires 1280x720, got %dx%d", window.ClientWidth, window.ClientHeight)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		if detachErr := rt.Process.Detach(); detachErr != nil {
			rt.Log.Warn("process detach failed", "error", detachErr)
		}
	}()
	defer rt.Input.Unbind()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	defer rt.stopHotkeys(cancel)
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	if readyErr := rt.waitPathingTestReady(ctx, state, hotkeys, ticker, time.Now().Add(rt.attachTimeoutOrDefault(60*time.Second)), cancel, true); readyErr != nil {
		return readyErr
	}
	if ctx.Err() != nil || rt.Input.Status().Stopped {
		return nil
	}
	window, ok = ctrl.Window()
	if !ok {
		return fmt.Errorf("town test: input window not bound")
	}
	if window.ClientWidth != 1280 || window.ClientHeight != 720 {
		return fmt.Errorf("town test requires 1280x720, got %dx%d", window.ClientWidth, window.ClientHeight)
	}

	pathCfg := mapPathingConfig(rt.Config.Pathing)
	clickCfg := pathCfg.Click
	clickCfg.AnchorOffsetTiles = 0
	clicker := pathing.NewEntityClicker(rt.Log, ctrl, pathCfg.Projector(), clickCfg)
	npc := town.NewNPCInteractor(townNPCClickerAdapter{clicker: clicker}, world.Akara, 15, 8*time.Second)
	shop := town.NewShopOpener(ctrl, 8*time.Second)
	buyers := []*town.VendorBuyer{
		town.NewVendorBuyer(ctrl, town.VendorRequest{Type: "hpot", Mode: town.BuyModeBulk}),
		town.NewVendorBuyer(ctrl, town.VendorRequest{Type: "mpot", Mode: town.BuyModeBulk}),
		town.NewVendorBuyer(ctrl, town.VendorRequest{Code: "tsc", Mode: town.BuyModeBulk}),
	}
	labels := []string{"healing", "mana", "town_portal_scroll"}
	phase := 0
	buyerIndex := 0
	purchaseSettleUntil := time.Time{}
	escapeSent := false
	deadline := time.Now().Add(townTestTimeout)
	profileID, err := rt.resolvedCombatProfileID()
	if err != nil {
		return err
	}
	rt.Log.Info("town Akara acceptance started", "profile", profileID, "bulk_items", labels)
	for time.Now().Before(deadline) {
		current, stop, tickErr := rt.pathingTestTick(ctx, state, hotkeys, ticker, cancel)
		if tickErr != nil {
			return tickErr
		}
		if stop {
			return nil
		}
		if !current.Valid || current.Area.ID != world.RogueEncampment {
			continue
		}
		switch phase {
		case 0:
			result := npc.Tick(current)
			if err := logTownInteraction(rt, "npc", result); err != nil {
				return err
			}
			if result.Done {
				phase = 1
				rt.Log.Info("town NPC dialog confirmed", "npc", "Akara", "unit_id", result.UnitID)
			}
		case 1:
			result := shop.Tick(current)
			if err := logTownInteraction(rt, "shop", result); err != nil {
				return err
			}
			if result.Done {
				if !hasInventoryCode(current, "tbk") {
					return fmt.Errorf("town test: Tome of Town Portal not found in personal inventory")
				}
				phase = 2
				rt.Log.Info("town shop confirmed", "npc", "Akara")
			}
		case 2:
			if buyerIndex >= len(buyers) {
				phase = 3
				continue
			}
			if time.Now().Before(purchaseSettleUntil) {
				continue
			}
			result := buyers[buyerIndex].Tick(current)
			if result.Action == "vendor_buy_bulk" {
				rt.Log.Info("town bulk purchase action", "resource", labels[buyerIndex], "vendor_item_unit_id", result.UnitID, "mode", "bulk")
				purchaseSettleUntil = time.Now().Add(townPurchaseSettle)
			}
			if err := logTownInteraction(rt, "buy_"+labels[buyerIndex], result); err != nil {
				return err
			}
			if result.Done {
				rt.Log.Info("town vendor item action complete", "resource", labels[buyerIndex], "vendor_item_unit_id", result.UnitID)
				buyerIndex++
			}
		case 3:
			if !escapeSent {
				if err := ctrl.PressKey("esc"); err != nil {
					return fmt.Errorf("town test close shop: %w", err)
				}
				escapeSent = true
				rt.Log.Info("town shop close requested")
				continue
			}
			if !current.UI.NPCShopOpen && !current.UI.NPCInteractOpen {
				rt.Log.Info("town Akara acceptance completed", "bulk_actions", len(buyers), "outcome", "success")
				return nil
			}
		}
	}
	return fmt.Errorf("town test timeout after %s", townTestTimeout)
}

func (rt *Runtime) runItemServicesTownTest() error {
	if !rt.Input.Status().Enabled {
		return fmt.Errorf("town item-service test requires input.enabled=true")
	}
	ctrl, ok := rt.Input.(townTestController)
	if !ok {
		return fmt.Errorf("town item-service test: controller lacks click or modified-click support")
	}
	runCfg, configured := rt.Config.Runs.Run("mephisto")
	if !configured {
		return fmt.Errorf("town item-service test: Mephisto run config unavailable")
	}
	effective, err := rt.PickitAssignments.Resolve(rt.Config.Session.Character, tasks.RunIDMephisto)
	if err != nil {
		return fmt.Errorf("town item-service test: Mephisto pickit assignment unavailable: %w", err)
	}
	pickup := effective.All
	lock, err := loot.NewInventoryLock(rt.Loot.InventoryLock().Grid())
	if err != nil {
		return fmt.Errorf("town item-service test inventory lock: %w", err)
	}
	trace, err := telemetry.New(rt.Config.Telemetry.Directory, "town-item-services", "mephisto")
	if err != nil {
		return fmt.Errorf("town item-service test telemetry: %w", err)
	}
	defer trace.Close()
	adapter, err := newTownPreparationAdapter(rt.Log, ctrl, mapPathingConfig(rt.Config.Pathing), rt.Config, "mephisto", runCfg, &townLayoutPin{}, townTelemetryAdapter{emitter: trace}, true)
	if err != nil {
		return err
	}
	adapter.thresholds = town.Thresholds{}
	adapter.setItemPolicies(loot.NewFilter(rt.Log, lock, pickup), rt.Config.Loot.Stash)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		if detachErr := rt.Process.Detach(); detachErr != nil {
			rt.Log.Warn("process detach failed", "error", detachErr)
		}
	}()
	defer rt.Input.Unbind()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	defer rt.stopHotkeys(cancel)
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	if readyErr := rt.waitPathingTestReady(ctx, state, hotkeys, ticker, time.Now().Add(rt.attachTimeoutOrDefault(60*time.Second)), cancel, true); readyErr != nil {
		return readyErr
	}
	initial := rt.World.Current()
	orders, reason := adapter.planItemServiceOrders(initial)
	if reason != "" {
		return fmt.Errorf("town item-service test preflight failed: %s", reason)
	}
	if err := validateItemServicesAcceptanceOrders(orders); err != nil {
		return err
	}
	expectedOrder := "Akara"
	if len(orders) == 2 {
		expectedOrder = "Cain -> Akara"
	}
	candidate := orders[len(orders)-1]
	deadline := time.Now().Add(2 * townTestTimeout)
	rt.Log.Info("town item-service acceptance started", "run", "mephisto", "expected_order", expectedOrder, "candidate_unit_id", candidate.UnitID, "candidate_code", candidate.Code)
	for time.Now().Before(deadline) {
		current, stop, tickErr := rt.pathingTestTick(ctx, state, hotkeys, ticker, cancel)
		if tickErr != nil {
			return tickErr
		}
		if stop {
			return nil
		}
		if !current.Valid || current.Area.ID != world.RogueEncampment {
			continue
		}
		result := adapter.Tick(ctx, current)
		if !result.Done {
			continue
		}
		if result.Status != "complete" {
			return fmt.Errorf("town item-service test failed: %s", result.Reason)
		}
		rt.Log.Info("town item-service acceptance completed", "outcome", "success")
		return nil
	}
	return fmt.Errorf("town item-service test timeout after %s", 2*townTestTimeout)
}

func validateItemServicesAcceptanceOrders(orders []town.ItemServiceOrder) error {
	if len(orders) == 1 && orders[0].Kind == town.ItemServiceSell && orders[0].UnitID != 0 {
		// A candidate identified by an earlier failed acceptance attempt remains
		// useful: resume at Akara instead of forcing the operator to farm again.
		return nil
	}
	if len(orders) != 2 || orders[0].Kind != town.ItemServiceIdentify || orders[1].Kind != town.ItemServiceSell || orders[0].UnitID == 0 || orders[0].UnitID != orders[1].UnitID || orders[0].Code != orders[1].Code {
		return fmt.Errorf("town item-service test requires exactly one unlocked Exceptional/Elite Set or Unique sell candidate; no input was sent")
	}
	return nil
}

func logTownInteraction(rt *Runtime, stage string, result town.InteractionResult) error {
	if result.Status == town.InteractionFailed {
		return fmt.Errorf("town test %s failed: %s", stage, result.Reason)
	}
	return nil
}

func hasInventoryCode(state world.State, code string) bool {
	for _, item := range state.InventoryItems() {
		if item.Code == code {
			return true
		}
	}
	return false
}

func validateAkaraBulkProfile(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("town test: config unavailable")
	}
	if _, configured := cfg.Runs.Run(string(tasks.RunIDCountess)); !configured {
		return fmt.Errorf("town test: Countess run config unavailable")
	}
	profileID, err := resolveActiveCombatProfileID(cfg, nil, cfg.Session.Character)
	if err != nil {
		return fmt.Errorf("town test: %w", err)
	}
	profileCfg, ok := cfg.Profiles[profileID]
	if !ok {
		return fmt.Errorf("town test: active combat profile unavailable")
	}
	slots := map[int]string{}
	for resource, list := range map[string][]int{"healing": profileCfg.Resources.Healing.BeltSlots, "mana": profileCfg.Resources.Mana.BeltSlots, "rejuvenation": profileCfg.Resources.Rejuvenation.BeltSlots} {
		for _, slot := range list {
			if previous := slots[slot]; previous != "" {
				return fmt.Errorf("town test: belt slot %d assigned to %s and %s", slot, previous, resource)
			}
			slots[slot] = resource
		}
	}
	for slot := 1; slot <= 4; slot++ {
		if slots[slot] == "" {
			return fmt.Errorf("town test: belt slot %d is unassigned; bulk purchase blocked", slot)
		}
	}
	if slots[4] != "rejuvenation" {
		return fmt.Errorf("town test: slot 4 must remain rejuvenation-protected")
	}
	return nil
}

func (rt *Runtime) runMercenaryHealTownTest() error {
	return rt.runMercenaryTownServiceTest(town.ServiceMercenaryHeal, town.AnchorAkara, world.Akara, "mercenary-heal")
}

func (rt *Runtime) runMercenaryReviveTownTest() error {
	return rt.runMercenaryTownServiceTest(town.ServiceMercenaryRevive, town.AnchorKashya, world.Kashya, "mercenary-revive")
}

func (rt *Runtime) runMercenaryTownServiceTest(service town.Service, anchor town.Anchor, npcID uint32, label string) error {
	if !rt.Input.Status().Enabled {
		return fmt.Errorf("town %s test requires input.enabled=true", label)
	}
	ctrl, ok := rt.Input.(townTestController)
	if !ok {
		return fmt.Errorf("town %s test: controller lacks click support", label)
	}
	runCfg, configured := rt.Config.Runs.Run(string(tasks.RunIDCountess))
	if !configured {
		return fmt.Errorf("town %s test: Countess run config unavailable", label)
	}
	trace, err := telemetry.New(rt.Config.Telemetry.Directory, "town-"+label, string(tasks.RunIDCountess))
	if err != nil {
		return fmt.Errorf("town %s test telemetry: %w", label, err)
	}
	defer trace.Close()
	adapter, err := newTownPreparationAdapter(rt.Log, ctrl, mapPathingConfig(rt.Config.Pathing), rt.Config, string(tasks.RunIDCountess), runCfg, &townLayoutPin{}, townTelemetryAdapter{emitter: trace}, true)
	if err != nil {
		return err
	}
	handler := newTownPreparationStepHandler(adapter, nil, nil, nil)
	handler.anchor = anchor
	handler.stage = "interact"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		if detachErr := rt.Process.Detach(); detachErr != nil {
			rt.Log.Warn("process detach failed", "error", detachErr)
		}
	}()
	defer rt.Input.Unbind()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	defer rt.stopHotkeys(cancel)
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	if readyErr := rt.waitPathingTestReady(ctx, state, hotkeys, ticker, time.Now().Add(rt.attachTimeoutOrDefault(60*time.Second)), cancel, true); readyErr != nil {
		return readyErr
	}
	deadline := time.Now().Add(townTestTimeout)
	rt.Log.Info("town mercenary acceptance started", "spec", label, "npc_id", npcID, "anchor", anchor)
	step := town.PlanStep{Phase: town.PlanPhaseServices, Kind: town.StepService, Service: service, Act: town.OriginAct1}
	for time.Now().Before(deadline) {
		current, stop, tickErr := rt.pathingTestTick(ctx, state, hotkeys, ticker, cancel)
		if tickErr != nil {
			return tickErr
		}
		if stop {
			return nil
		}
		if !current.Valid || current.Area.ID != world.RogueEncampment {
			continue
		}
		if _, found := current.FindNPC(npcID); !found {
			return fmt.Errorf("town %s test: stand within range of %s before starting", label, anchor)
		}
		result := handler.Tick(ctx, step, current)
		if err := logTownInteraction(rt, label, result); err != nil {
			return err
		}
		if result.Status == town.InteractionComplete && result.Done {
			rt.Log.Info("town mercenary acceptance completed", "spec", label, "outcome", "success")
			return nil
		}
	}
	return fmt.Errorf("town %s test timeout after %s", label, townTestTimeout)
}
