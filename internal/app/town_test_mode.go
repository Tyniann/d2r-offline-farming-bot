package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
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

// RunTownTest executes one isolated Phase-9 Town interaction acceptance flow.
func (rt *Runtime) RunTownTest(spec string) error {
	if strings.ToLower(strings.TrimSpace(spec)) != "akara-shop" {
		return fmt.Errorf("town test: unsupported spec %q", spec)
	}
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
	defer rt.Process.Detach()
	defer rt.Input.Unbind()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	defer rt.stopHotkeys(cancel)
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	if err := rt.waitPathingTestReady(ctx, state, hotkeys, ticker, time.Now().Add(rt.attachTimeoutOrDefault(60*time.Second)), cancel, true); err != nil {
		return err
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
	rt.Log.Info("town Akara acceptance started", "profile", rt.Config.Runs.Countess.Combat.Profile, "bulk_items", labels)
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
	profileCfg, ok := cfg.Profiles[cfg.Runs.Countess.Combat.Profile]
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
