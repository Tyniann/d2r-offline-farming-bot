package app

import (
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// observeMercenaryDeath emits one `mercenary_died` event on Alive→Dead edges.
// Last known HP% comes from the previous snapshot when vitals were known.
func (rt *Runtime) observeMercenaryDeath(prev, cur world.State) {
	if rt == nil || rt.Telemetry == nil {
		return
	}
	if !prev.Valid || !cur.Valid || cur.Phase != world.GamePhaseInGame {
		return
	}
	wasAlive := prev.Mercenary.HiredKnown && prev.Mercenary.Hired && prev.Mercenary.Alive && !prev.Mercenary.Dead
	nowDead := cur.Mercenary.HiredKnown && cur.Mercenary.Hired && cur.Mercenary.Dead
	if !wasAlive || !nowDead {
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
