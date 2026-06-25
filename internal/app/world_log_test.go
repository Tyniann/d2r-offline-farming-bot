package app

import (
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func validWorldState(hp uint32) world.State {
	return world.FromSnapshot(validSnapshot(hp))
}

func TestWorldLogIsHeartbeat(t *testing.T) {
	if worldLogIsHeartbeat(time.Time{}, worldHeartbeat) {
		t.Fatal("zero lastLog must not be heartbeat (first invalid log uses Info)")
	}
	if !worldLogIsHeartbeat(time.Now().Add(-6*time.Second), worldHeartbeat) {
		t.Fatal("expected heartbeat after interval elapsed")
	}
	if worldLogIsHeartbeat(time.Now(), worldHeartbeat) {
		t.Fatal("unexpected heartbeat before interval elapsed")
	}
}

func TestWorldShouldLogOnValueChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(90)
	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on HP change")
	}
}

func TestWorldShouldLogManaChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Player.Mana = 40

	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on mana change")
	}
}

func TestWorldShouldLogMaxHPChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Player.MaxHP = 120

	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on max HP change")
	}
}

func TestWorldShouldNotLogOnlyPositionChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Player.Position.X++
	cur.Player.Position.Y++

	if worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("unexpected log for position-only change within heartbeat")
	}
}

func TestWorldShouldLogPositionChangeInVerbose(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Player.Position.X++
	cur.Player.Position.Y++

	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, true) {
		t.Fatal("expected log for position-only change in verbose mode")
	}
}

func TestWorldShouldLogAreaChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Area = world.LookupArea(world.AreaID(prev.Area.ID + 1))

	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on area change")
	}
}

func TestWorldShouldLogPhaseChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Phase = world.GamePhaseLoading

	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on phase change")
	}
}

func TestWorldShouldLogHeartbeat(t *testing.T) {
	st := validWorldState(100)
	last := time.Now().Add(-6 * time.Second)
	if !worldShouldLog(st, st, last, worldHeartbeat, false, false) {
		t.Fatal("expected log after heartbeat interval")
	}
}

func TestWorldShouldNotLogUnchanged(t *testing.T) {
	st := validWorldState(100)
	last := time.Now()
	if worldShouldLog(st, st, last, worldHeartbeat, false, false) {
		t.Fatal("unexpected log for unchanged state within heartbeat")
	}
}

func TestWorldShouldIgnoreAtChange(t *testing.T) {
	prev := validWorldState(100)
	cur := prev
	cur.At = prev.At.Add(time.Minute)
	if worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("At change alone must not trigger log")
	}
}

func TestWorldShouldLogInvalidReasonChange(t *testing.T) {
	prev := world.State{Valid: false, Reason: memory.ReasonNotInGame}
	cur := world.State{Valid: false, Reason: memory.ReasonStatsUnavailable}
	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log when invalid reason changes")
	}
}

func TestWorldShouldNotLogSameInvalidReason(t *testing.T) {
	prev := world.State{Valid: false, Reason: memory.ReasonNotInGame}
	cur := world.State{Valid: false, Reason: memory.ReasonNotInGame}
	if worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("unexpected log for same invalid reason within heartbeat")
	}
}

func TestWorldShouldLogInvalidOnHeartbeat(t *testing.T) {
	prev := world.State{Valid: false, Reason: memory.ReasonNotInGame}
	cur := world.State{Valid: false, Reason: memory.ReasonNotInGame}
	last := time.Now().Add(-6 * time.Second)
	if !worldShouldLog(prev, cur, last, worldHeartbeat, false, false) {
		t.Fatal("expected heartbeat log for unchanged invalid state")
	}
}

func TestWorldShouldNotLogPositionOnlyOnHeartbeat(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Player.Position.X++
	cur.Player.Position.Y++
	last := time.Now().Add(-6 * time.Second)

	if worldShouldLog(prev, cur, last, worldHeartbeat, false, false) {
		t.Fatal("unexpected heartbeat Info log for position-only change without verbose")
	}
	if !worldShouldLog(prev, cur, last, worldHeartbeat, false, true) {
		t.Fatal("expected heartbeat log for position-only change in verbose mode")
	}
}

func TestWorldShouldLogOnForce(t *testing.T) {
	st := validWorldState(100)
	if !worldShouldLog(st, st, time.Now(), worldHeartbeat, true, false) {
		t.Fatal("expected log when force=true after re-attach")
	}
}

func TestWorldShouldLogValidToInvalid(t *testing.T) {
	prev := validWorldState(100)
	cur := world.State{Valid: false, Reason: memory.ReasonNotInGame}
	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log when state becomes invalid")
	}
}

func TestWorldLogStateAfterProcessLostAndReattach(t *testing.T) {
	var lastLogged world.State
	var lastLog time.Time

	st := validWorldState(100)
	if !worldShouldLog(lastLogged, st, lastLog, worldHeartbeat, true, false) {
		t.Fatal("expected forced log after re-attach")
	}
}

func TestWorldLogAttrsValidState(t *testing.T) {
	st := validWorldState(100)
	attrs := worldLogAttrs(st)
	if len(attrs) == 0 {
		t.Fatal("expected attrs for valid state")
	}
	found := map[string]bool{}
	for _, a := range attrs {
		found[a.Key] = true
	}
	for _, key := range []string{"phase", "area_name", "area_id", "act", "area_kind", "hp", "max_hp", "hp_pct", "mana", "max_mana", "mana_pct", "pos_x", "pos_y"} {
		if !found[key] {
			t.Fatalf("missing log attr %q", key)
		}
	}
}

func TestWorldLogAttrsInvalidState(t *testing.T) {
	st := world.State{Valid: false, Reason: "test_reason"}
	attrs := worldLogAttrs(st)
	if len(attrs) != 1 || attrs[0].Key != "reason" {
		t.Fatalf("unexpected invalid attrs: %+v", attrs)
	}
}
