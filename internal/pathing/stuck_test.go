package pathing

import (
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func stuckState(area world.AreaID, x, y uint32) world.State {
	return world.State{
		Valid:  true,
		Phase:  world.GamePhaseInGame,
		Area:   world.LookupArea(area),
		Player: world.Player{Position: world.Position{X: x, Y: y}},
	}
}

func TestStuckDetectorNoProgress(t *testing.T) {
	d := NewStuckDetector(8*time.Second, 3)
	base := time.Now()

	if d.Update(base, stuckState(world.BloodMoor, 5000, 5000)) {
		t.Fatal("first update must not report stuck")
	}
	if d.Update(base.Add(4*time.Second), stuckState(world.BloodMoor, 5001, 5000)) {
		t.Fatal("stuck before timeout elapsed")
	}
	if !d.Update(base.Add(9*time.Second), stuckState(world.BloodMoor, 5001, 5001)) {
		t.Fatal("expected stuck after timeout without progress")
	}
}

func TestStuckDetectorProgressResetsTimer(t *testing.T) {
	d := NewStuckDetector(8*time.Second, 3)
	base := time.Now()

	d.Update(base, stuckState(world.BloodMoor, 5000, 5000))
	// Movement >= progress tiles resets the timer.
	if d.Update(base.Add(6*time.Second), stuckState(world.BloodMoor, 5010, 5000)) {
		t.Fatal("progress must not report stuck")
	}
	if d.Update(base.Add(13*time.Second), stuckState(world.BloodMoor, 5010, 5000)) {
		t.Fatal("timer must restart after progress")
	}
	if !d.Update(base.Add(15*time.Second), stuckState(world.BloodMoor, 5010, 5000)) {
		t.Fatal("expected stuck after fresh timeout")
	}
}

func TestStuckDetectorAreaChangeResetsTimer(t *testing.T) {
	d := NewStuckDetector(8*time.Second, 3)
	base := time.Now()

	d.Update(base, stuckState(world.BloodMoor, 5000, 5000))
	if d.Update(base.Add(7*time.Second), stuckState(world.ColdPlains, 5000, 5000)) {
		t.Fatal("area change must count as progress")
	}
	if d.Update(base.Add(14*time.Second), stuckState(world.ColdPlains, 5000, 5000)) {
		t.Fatal("timer must restart after area change")
	}
}
