package app

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const reasonMercenaryDiedDuringRun = "mercenary_died_during_run"

// observeMercenaryDeath emits one `mercenary_died` event on Alive→Dead edges.
// Last known HP% comes from the previous snapshot when vitals were known.
func (rt *Runtime) observeMercenaryDeath(prev, cur world.State) {
	if rt == nil || rt.Telemetry == nil {
		return
	}
	if !prev.Valid || !cur.Valid || cur.Phase != world.GamePhaseInGame {
		return
	}
	if !mercenaryDiedEdge(prev, cur) {
		return
	}
	event := telemetry.Event{
		Event:      telemetry.MercenaryDied,
		MercUnitID: cur.Mercenary.UnitID,
		AreaID:     uint32(cur.Area.ID),
	}
	if prev.Mercenary.VitalsKnown && prev.Mercenary.MaxHP > 0 {
		event.HPPercent = prev.Mercenary.HPPercent()
	}
	if err := rt.Telemetry.Emit(event); err != nil {
		rt.Log.Error("mercenary death telemetry failed", "error", err)
	}
	rt.Log.Info("mercenary died",
		"merc_unit_id", cur.Mercenary.UnitID,
		"last_hp_percent", event.HPPercent,
		"area_id", cur.Area.ID,
	)
}

// abortRunOnMercenaryDeath stops offensive state before terminalizing the open
// productive step. Queue ownership then performs the normal controlled Town
// return; same-point Cow continuation is deliberately not attempted.
func (rt *Runtime) abortRunOnMercenaryDeath(prev, cur world.State) error {
	if rt == nil || !rt.productiveRunActive || rt.Tasks == nil || rt.Tasks.Terminal() {
		return nil
	}
	if !mercenaryDiedEdge(prev, cur) {
		return nil
	}
	if rt.taskDeps.Combat == nil {
		return fmt.Errorf("mercenary death: combat stop is not wired")
	}
	if err := rt.taskDeps.Combat.StopAttack(); err != nil {
		return fmt.Errorf("mercenary death stop attack: %w", err)
	}
	if err := rt.Tasks.AbortOpenStep(reasonMercenaryDiedDuringRun); err != nil {
		return fmt.Errorf("mercenary death abort run: %w", err)
	}
	return nil
}

func mercenaryDiedEdge(prev, cur world.State) bool {
	return prev.Valid && cur.Valid && cur.Phase == world.GamePhaseInGame &&
		prev.Mercenary.HiredKnown && prev.Mercenary.Hired && prev.Mercenary.Alive && !prev.Mercenary.Dead &&
		cur.Mercenary.HiredKnown && cur.Mercenary.Hired && cur.Mercenary.Dead
}
