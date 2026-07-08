package loot

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestPickupRequiresExactStability(t *testing.T) {
	clicker := &fakePickupClicker{}
	exec := NewPickupExecutor(testLootLogger(), testPickupConfig(), clicker, testPickupTarget())
	now := time.Unix(1, 0)

	res := exec.Tick(testPickupState(testGroundItem()), now)
	if res.Done || clicker.calls != 0 {
		t.Fatalf("first tick = %+v calls=%d, want pending without click", res, clicker.calls)
	}

	changed := testGroundItem()
	changed.Position = world.Position{X: 101, Y: 100}
	res = exec.Tick(testPickupState(changed), now.Add(100*time.Millisecond))
	if !res.Done || res.Status != PickupTargetUnstable {
		t.Fatalf("unstable tick = %+v, want target_unstable", res)
	}
}

func TestPickupInvalidWorldCannotSucceed(t *testing.T) {
	exec := NewPickupExecutor(testLootLogger(), testPickupConfig(), &fakePickupClicker{}, testPickupTarget())
	res := exec.Tick(world.State{Valid: false}, time.Unix(1, 0))
	if !res.Done || res.Status != PickupInvalidWorld {
		t.Fatalf("invalid tick = %+v, want invalid_world", res)
	}
}

func TestPickupHoverNotFoundRetriesAndFails(t *testing.T) {
	cfg := testPickupConfig()
	cfg.MaxRetries = 2
	clicker := &fakePickupClicker{results: []PickupClickResult{
		{Status: PickupClickHoverNotFound, Attempt: 3, Done: true},
		{Status: PickupClickHoverNotFound, Attempt: 3, Done: true},
	}}
	exec := NewPickupExecutor(testLootLogger(), cfg, clicker, testPickupTarget())
	now := time.Unix(1, 0)

	res := exec.Tick(testPickupState(testGroundItem()), now)
	if res.Done {
		t.Fatalf("tick 1 = %+v, want pending", res)
	}
	res = exec.Tick(testPickupState(testGroundItem()), now.Add(100*time.Millisecond))
	if res.Done {
		t.Fatalf("tick 2 = %+v, want retry pending", res)
	}
	res = exec.Tick(testPickupState(testGroundItem()), now.Add(200*time.Millisecond))
	if res.Done {
		t.Fatalf("tick 3 = %+v, want restabilize pending", res)
	}
	res = exec.Tick(testPickupState(testGroundItem()), now.Add(300*time.Millisecond))
	if !res.Done || res.Status != PickupHoverNotFound {
		t.Fatalf("tick 4 = %+v, want hover_not_found", res)
	}
	if clicker.calls != 2 {
		t.Fatalf("clicker calls=%d, want 2", clicker.calls)
	}
}

func TestPickupVerifyGroundDisappearance(t *testing.T) {
	cfg := testPickupConfig()
	cfg.VerifyTicks = 2
	clicker := &fakePickupClicker{results: []PickupClickResult{{Status: PickupClickHit, Attempt: 1, Done: true}}}
	exec := NewPickupExecutor(testLootLogger(), cfg, clicker, testPickupTarget())
	now := time.Unix(1, 0)

	exec.Tick(testPickupState(testGroundItem()), now)
	res := exec.Tick(testPickupState(testGroundItem()), now.Add(100*time.Millisecond))
	if res.Done {
		t.Fatalf("click tick = %+v, want verify pending", res)
	}
	res = exec.Tick(testPickupState(), now.Add(200*time.Millisecond))
	if res.Done {
		t.Fatalf("first verify = %+v, want pending", res)
	}
	res = exec.Tick(testPickupState(), now.Add(300*time.Millisecond))
	if !res.Done || res.Status != PickupPickedUp {
		t.Fatalf("second verify = %+v, want picked_up", res)
	}
}

func TestPickupVerifyInventoryTransitionRequiresExactTarget(t *testing.T) {
	cfg := testPickupConfig()
	cfg.VerifyTicks = 1
	clicker := &fakePickupClicker{results: []PickupClickResult{{Status: PickupClickHit, Done: true}}}
	exec := NewPickupExecutor(testLootLogger(), cfg, clicker, testPickupTarget())
	now := time.Unix(1, 0)

	exec.Tick(testPickupState(testGroundItem()), now)
	exec.Tick(testPickupState(testGroundItem()), now.Add(100*time.Millisecond))

	wrong := testInventoryItem()
	wrong.TxtFileNo = 999
	res := exec.Tick(testPickupState(wrong), now.Add(200*time.Millisecond))
	if res.Done {
		t.Fatalf("wrong inventory = %+v, want pending until timeout", res)
	}

	res = exec.Tick(testPickupState(testInventoryItem()), now.Add(300*time.Millisecond))
	if !res.Done || res.Status != PickupPickedUp {
		t.Fatalf("exact inventory = %+v, want picked_up", res)
	}
}

func TestPickupVerifyTimeoutFails(t *testing.T) {
	cfg := testPickupConfig()
	cfg.VerifyTimeout = 200 * time.Millisecond
	clicker := &fakePickupClicker{results: []PickupClickResult{{Status: PickupClickHit, Done: true}}}
	exec := NewPickupExecutor(testLootLogger(), cfg, clicker, testPickupTarget())
	now := time.Unix(1, 0)

	exec.Tick(testPickupState(testGroundItem()), now)
	exec.Tick(testPickupState(testGroundItem()), now.Add(100*time.Millisecond))
	res := exec.Tick(testPickupState(testGroundItem()), now.Add(400*time.Millisecond))
	if !res.Done || res.Status != PickupFailed {
		t.Fatalf("verify timeout = %+v, want pickup_failed", res)
	}
}

func TestPickupDistanceAndMonsterAbort(t *testing.T) {
	far := testGroundItem()
	far.Position = world.Position{X: 500, Y: 500}
	target := testPickupTarget()
	target.Position = far.Position
	exec := NewPickupExecutor(testLootLogger(), testPickupConfig(), &fakePickupClicker{}, target)
	now := time.Unix(1, 0)

	res := exec.Tick(testPickupState(far), now)
	if !res.Done || res.Status != PickupTooFar {
		t.Fatalf("far target = %+v, want too_far", res)
	}

	st := testPickupState(testGroundItem())
	st.Monsters = []world.Monster{{UnitID: 7, Position: world.Position{X: 102, Y: 100}}}
	exec = NewPickupExecutor(testLootLogger(), testPickupConfig(), &fakePickupClicker{}, testPickupTarget())
	res = exec.Tick(st, now)
	if !res.Done || res.Status != PickupMonsterNearby {
		t.Fatalf("monster nearby = %+v, want monster_nearby", res)
	}
}

func TestPickupInputBlocked(t *testing.T) {
	clicker := &fakePickupClicker{err: errors.New("paused")}
	exec := NewPickupExecutor(testLootLogger(), testPickupConfig(), clicker, testPickupTarget())
	now := time.Unix(1, 0)

	exec.Tick(testPickupState(testGroundItem()), now)
	res := exec.Tick(testPickupState(testGroundItem()), now.Add(100*time.Millisecond))
	if !res.Done || res.Status != PickupInputBlocked {
		t.Fatalf("input error = %+v, want input_blocked", res)
	}
}

func TestSelectPickupCandidateNearestAndCurrentGroundOnly(t *testing.T) {
	st := testPickupState(
		world.Item{UnitID: 10, TxtFileNo: 1, Code: "r01", Name: "El Rune", Location: world.ItemLocationGround, Position: world.Position{X: 120, Y: 100}},
		world.Item{UnitID: 11, TxtFileNo: 2, Code: "r02", Name: "Eld Rune", Location: world.ItemLocationGround, Position: world.Position{X: 105, Y: 100}},
	)
	report := DecisionReport{Decisions: []ItemDecision{
		{UnitID: 10, Stage: DecisionStagePickCandidate, Kind: DecisionKindPickCandidate, CanFit: true},
		{UnitID: 11, Stage: DecisionStagePickCandidate, Kind: DecisionKindPickCandidate, CanFit: true},
		{UnitID: 12, Stage: DecisionStagePickCandidate, Kind: DecisionKindPickCandidate, CanFit: true},
	}}
	target, ok := SelectPickupCandidate(st, report)
	if !ok || target.UnitID != 11 {
		t.Fatalf("target = %+v ok=%t, want nearest unit 11", target, ok)
	}
}

func testPickupConfig() PickupConfig {
	return PickupConfig{
		MaxRetries:                1,
		MaxDistanceTiles:          8,
		VerifyTicks:               1,
		VerifyTimeout:             time.Second,
		MonsterAbortDistanceTiles: 12,
	}
}

func testPickupTarget() PickupTarget {
	return PickupTarget{
		UnitID:    1001,
		TxtFileNo: 1,
		Code:      "r01",
		Name:      "El Rune",
		Position:  world.Position{X: 104, Y: 100},
		AreaID:    world.TowerCellarLevel5,
	}
}

func testGroundItem() world.Item {
	return world.Item{
		UnitID:    1001,
		TxtFileNo: 1,
		Code:      "r01",
		Name:      "El Rune",
		Location:  world.ItemLocationGround,
		Position:  world.Position{X: 104, Y: 100},
	}
}

func testInventoryItem() world.Item {
	item := testGroundItem()
	item.Location = world.ItemLocationInventory
	item.PlayerOwned = true
	item.Page = 0
	return item
}

func testPickupState(items ...world.Item) world.State {
	return world.State{
		Valid:  true,
		Phase:  world.GamePhaseInGame,
		Area:   world.Area{ID: world.TowerCellarLevel5},
		Player: world.Player{Position: world.Position{X: 100, Y: 100}},
		Items:  items,
	}
}

func testLootLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakePickupClicker struct {
	results []PickupClickResult
	err     error
	calls   int
	resets  int
}

func (f *fakePickupClicker) Reset() {
	f.resets++
}

func (f *fakePickupClicker) Tick(world.State, PickupClickTarget, float64) (PickupClickResult, error) {
	if f.err != nil {
		return PickupClickResult{}, f.err
	}
	if f.calls >= len(f.results) {
		f.calls++
		return PickupClickResult{Status: PickupClickPending}, nil
	}
	res := f.results[f.calls]
	f.calls++
	return res, nil
}
