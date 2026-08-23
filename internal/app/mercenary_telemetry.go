package app

import (
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const reasonMercenaryDiedDuringRun = "mercenary_died_during_run"

const (
	mercenaryDeathConfirmationSnapshots = 3
	mercenaryDeathTransitionSettle      = 3 * time.Second
)

type mercenaryDeathDecision string

const (
	mercenaryDeathStable    mercenaryDeathDecision = "stable"
	mercenaryDeathPending   mercenaryDeathDecision = "pending"
	mercenaryDeathConfirmed mercenaryDeathDecision = "confirmed"
)

type mercenaryDeathObservation struct {
	Decision  mercenaryDeathDecision
	UnitID    uint32
	AreaID    world.AreaID
	HPPercent uint8
}

// mercenaryDeathGuard confirms Dead evidence across fresh World snapshots.
// An area transition additionally needs the destination to remain stable for
// the normal load-fade settle because Memory may briefly combine old hireling
// state with the new area. Alive evidence always cancels a candidate.
type mercenaryDeathGuard struct {
	lastAliveSet        bool
	lastAliveUnitID     uint32
	lastAliveAreaID     world.AreaID
	lastAliveHPPercent  uint8
	candidateSet        bool
	candidateAreaID     world.AreaID
	candidateStartedAt  time.Time
	candidateSnapshots  int
	candidateGeneration uint64
	candidateSnapshotAt time.Time
	candidateTransition bool
	confirmed           bool
}

func (g *mercenaryDeathGuard) reset() {
	*g = mercenaryDeathGuard{}
}

func (g *mercenaryDeathGuard) observe(cur world.State) mercenaryDeathObservation {
	if !cur.Valid || cur.Phase != world.GamePhaseInGame {
		return g.currentDecision()
	}
	merc := cur.Mercenary
	if merc.HiredKnown && merc.Hired && merc.Alive && !merc.Dead {
		g.lastAliveSet = true
		g.lastAliveUnitID = merc.UnitID
		g.lastAliveAreaID = cur.Area.ID
		g.lastAliveHPPercent = 0
		if merc.VitalsKnown && merc.MaxHP > 0 {
			g.lastAliveHPPercent = merc.HPPercent()
		}
		g.clearCandidate()
		return mercenaryDeathObservation{Decision: mercenaryDeathStable}
	}
	if !merc.HiredKnown || !merc.Hired || !merc.Dead || !g.lastAliveSet || g.confirmed {
		return g.currentDecision()
	}

	if !g.candidateSet || g.candidateAreaID != cur.Area.ID {
		g.candidateSet = true
		g.candidateAreaID = cur.Area.ID
		g.candidateStartedAt = cur.At
		g.candidateSnapshots = 0
		g.candidateGeneration = 0
		g.candidateSnapshotAt = time.Time{}
		g.candidateTransition = cur.Area.ID != g.lastAliveAreaID
	}
	if g.freshSnapshot(cur) {
		g.candidateSnapshots++
		g.candidateGeneration = cur.Generation
		g.candidateSnapshotAt = cur.At
	}

	observation := mercenaryDeathObservation{
		Decision:  mercenaryDeathPending,
		UnitID:    g.lastAliveUnitID,
		AreaID:    cur.Area.ID,
		HPPercent: g.lastAliveHPPercent,
	}
	if g.candidateSnapshots < mercenaryDeathConfirmationSnapshots {
		return observation
	}
	if g.candidateTransition && (g.candidateStartedAt.IsZero() || cur.At.Sub(g.candidateStartedAt) < mercenaryDeathTransitionSettle) {
		return observation
	}
	g.confirmed = true
	observation.Decision = mercenaryDeathConfirmed
	return observation
}

func (g *mercenaryDeathGuard) freshSnapshot(cur world.State) bool {
	if cur.Generation != 0 {
		return cur.Generation != g.candidateGeneration
	}
	return !cur.At.IsZero() && cur.At != g.candidateSnapshotAt
}

func (g *mercenaryDeathGuard) currentDecision() mercenaryDeathObservation {
	if g.candidateSet && !g.confirmed {
		return mercenaryDeathObservation{
			Decision: mercenaryDeathPending, UnitID: g.lastAliveUnitID,
			AreaID: g.candidateAreaID, HPPercent: g.lastAliveHPPercent,
		}
	}
	return mercenaryDeathObservation{Decision: mercenaryDeathStable}
}

func (g *mercenaryDeathGuard) clearCandidate() {
	g.candidateSet = false
	g.candidateAreaID = world.None
	g.candidateStartedAt = time.Time{}
	g.candidateSnapshots = 0
	g.candidateGeneration = 0
	g.candidateSnapshotAt = time.Time{}
	g.candidateTransition = false
	g.confirmed = false
}

// observeMercenaryDeath emits one `mercenary_died` event after confirmation.
func (rt *Runtime) observeMercenaryDeath(observation mercenaryDeathObservation) {
	if rt == nil || rt.Telemetry == nil {
		return
	}
	if observation.Decision != mercenaryDeathConfirmed {
		return
	}
	event := telemetry.Event{
		Event:      telemetry.MercenaryDied,
		MercUnitID: observation.UnitID,
		AreaID:     uint32(observation.AreaID),
		HPPercent:  observation.HPPercent,
	}
	if err := rt.Telemetry.Emit(event); err != nil {
		rt.Log.Error("mercenary death telemetry failed", "error", err)
	}
	rt.Log.Info("mercenary died",
		"merc_unit_id", observation.UnitID,
		"last_hp_percent", event.HPPercent,
		"area_id", observation.AreaID,
	)
}

// holdRunForMercenaryDeath prevents offensive or task input while a Dead read
// is being confirmed. Confirmation terminalizes the productive step so queue
// ownership can perform the controlled return.
func (rt *Runtime) holdRunForMercenaryDeath(observation mercenaryDeathObservation) error {
	if rt == nil || !rt.productiveRunActive || observation.Decision == mercenaryDeathStable || rt.Tasks == nil || rt.Tasks.Terminal() {
		return nil
	}
	if rt.taskDeps.Combat == nil {
		return fmt.Errorf("mercenary death: combat stop is not wired")
	}
	if err := rt.taskDeps.Combat.StopAttack(); err != nil {
		return fmt.Errorf("mercenary death stop attack: %w", err)
	}
	if observation.Decision != mercenaryDeathConfirmed {
		return nil
	}
	if err := rt.Tasks.AbortOpenStep(reasonMercenaryDiedDuringRun); err != nil {
		return fmt.Errorf("mercenary death abort run: %w", err)
	}
	return nil
}
