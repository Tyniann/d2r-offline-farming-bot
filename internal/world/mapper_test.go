package world

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func validSnapshot() memory.Snapshot {
	return memory.Snapshot{
		At:      time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
		Valid:   true,
		Phase:   memory.GamePhaseInGame,
		AreaID:  uint32(BlackMarsh),
		PosX:    1234,
		PosY:    5678,
		HP:      100,
		MaxHP:   125,
		Mana:    50,
		MaxMana: 75,
	}
}

func TestFromSnapshotValid(t *testing.T) {
	snap := validSnapshot()
	state := FromSnapshot(snap)

	if !state.Valid {
		t.Fatal("Valid = false, want true")
	}
	if state.Reason != "" {
		t.Fatalf("Reason = %q, want empty", state.Reason)
	}
	if state.Phase != GamePhaseInGame {
		t.Fatalf("Phase = %v, want GamePhaseInGame", state.Phase)
	}
	if state.Objects == nil || state.Entrances == nil || state.Monsters == nil {
		t.Fatal("valid snapshot should have non-nil entity slices")
	}
	if !state.At.Equal(snap.At) {
		t.Fatalf("At = %v, want %v", state.At, snap.At)
	}
	if state.Area.Name != "Black Marsh" {
		t.Fatalf("Area.Name = %q, want Black Marsh", state.Area.Name)
	}
	if state.Area.Act != Act1 {
		t.Fatalf("Area.Act = %v, want Act1", state.Area.Act)
	}
	if state.Area.Kind != AreaKindOutdoor {
		t.Fatalf("Area.Kind = %v, want AreaKindOutdoor", state.Area.Kind)
	}
	if state.Player.Position.X != snap.PosX || state.Player.Position.Y != snap.PosY {
		t.Fatalf("Position = (%d,%d), want (%d,%d)",
			state.Player.Position.X, state.Player.Position.Y, snap.PosX, snap.PosY)
	}
	if state.Player.HP != snap.HP || state.Player.MaxHP != snap.MaxHP {
		t.Fatalf("HP = %d/%d, want %d/%d", state.Player.HP, state.Player.MaxHP, snap.HP, snap.MaxHP)
	}
	if state.Player.Mana != snap.Mana || state.Player.MaxMana != snap.MaxMana {
		t.Fatalf("Mana = %d/%d, want %d/%d", state.Player.Mana, state.Player.MaxMana, snap.Mana, snap.MaxMana)
	}
}

func TestFromSnapshotGamePhaseFromSnapshot(t *testing.T) {
	snap := validSnapshot()
	snap.Phase = memory.GamePhaseInGame
	state := FromSnapshot(snap)
	if state.Phase != GamePhaseInGame {
		t.Fatalf("Phase = %v, want GamePhaseInGame", state.Phase)
	}
}

func TestFromSnapshotEffectiveMaxValues(t *testing.T) {
	snap := validSnapshot()
	snap.HP = 100
	snap.MaxHP = 125
	snap.Mana = 50
	snap.MaxMana = 75

	state := FromSnapshot(snap)
	if state.Player.HP != 100 || state.Player.MaxHP != 125 {
		t.Fatalf("HP = %d/%d, want 100/125", state.Player.HP, state.Player.MaxHP)
	}
	if state.Player.Mana != 50 || state.Player.MaxMana != 75 {
		t.Fatalf("Mana = %d/%d, want 50/75", state.Player.Mana, state.Player.MaxMana)
	}
	if state.Player.HPPercent() != 80 {
		t.Fatalf("HPPercent = %d, want 80", state.Player.HPPercent())
	}
	if state.Player.ManaPercent() != 66 {
		t.Fatalf("ManaPercent = %d, want 66", state.Player.ManaPercent())
	}
}

func TestFromSnapshotUnknownArea(t *testing.T) {
	snap := validSnapshot()
	snap.AreaID = 9999

	state := FromSnapshot(snap)
	if !state.Valid {
		t.Fatal("Valid = false, want true for unknown area")
	}
	if state.Area.Name != "Unknown Area 9999" {
		t.Fatalf("Area.Name = %q, want Unknown Area 9999", state.Area.Name)
	}
	if state.Area.Kind != AreaKindUnknown {
		t.Fatalf("Area.Kind = %v, want AreaKindUnknown", state.Area.Kind)
	}
	if state.Player.HP != snap.HP {
		t.Fatalf("Player.HP = %d, want %d", state.Player.HP, snap.HP)
	}
}

func TestFromSnapshotValidAreaIDZero(t *testing.T) {
	snap := validSnapshot()
	snap.AreaID = 0

	state := FromSnapshot(snap)
	if !state.Valid {
		t.Fatal("Valid = false, want true")
	}
	if state.Phase != GamePhaseInGame {
		t.Fatalf("Phase = %v, want GamePhaseInGame", state.Phase)
	}
	if state.Area.Name != "Unknown Area 0" {
		t.Fatalf("Area.Name = %q, want Unknown Area 0", state.Area.Name)
	}
	if state.Area.Act != ActUnknown {
		t.Fatalf("Area.Act = %v, want ActUnknown", state.Area.Act)
	}
	if state.Area.Kind != AreaKindUnknown {
		t.Fatalf("Area.Kind = %v, want AreaKindUnknown", state.Area.Kind)
	}
	if state.Player.HP != snap.HP || state.Player.Position.X != snap.PosX {
		t.Fatal("Player should remain populated for valid snapshot with AreaID 0")
	}
}

func TestFromSnapshotNamedConstantWithoutCatalogName(t *testing.T) {
	snap := validSnapshot()
	snap.AreaID = uint32(MapsAncientTemple)

	state := FromSnapshot(snap)
	if !state.Valid {
		t.Fatal("Valid = false, want true for unnamed catalog constant")
	}
	if state.Area.Name != "Unknown Area 137" {
		t.Fatalf("Area.Name = %q, want Unknown Area 137", state.Area.Name)
	}
	if state.Area.Kind != AreaKindUnknown {
		t.Fatalf("Area.Kind = %v, want AreaKindUnknown", state.Area.Kind)
	}
	if state.Area.Act != Act5 {
		t.Fatalf("Area.Act = %v, want Act5", state.Area.Act)
	}
}

func TestFromSnapshotInvalid(t *testing.T) {
	at := time.Date(2026, 6, 25, 13, 0, 0, 0, time.UTC)
	snap := memory.Snapshot{
		At:     at,
		Valid:  false,
		Reason: memory.ReasonStatsUnavailable,
		AreaID: 6,
		HP:     999,
	}

	state := FromSnapshot(snap)
	if state.Valid {
		t.Fatal("Valid = true, want false")
	}
	if !state.At.Equal(at) {
		t.Fatalf("At = %v, want %v", state.At, at)
	}
	if state.Reason != memory.ReasonStatsUnavailable {
		t.Fatalf("Reason = %q, want %q", state.Reason, memory.ReasonStatsUnavailable)
	}
	if state.Phase != GamePhaseUnknown {
		t.Fatalf("Phase = %v, want GamePhaseUnknown", state.Phase)
	}
	if state.Area != (Area{}) {
		t.Fatalf("Area = %+v, want zero value", state.Area)
	}
	if state.Player != (Player{}) {
		t.Fatalf("Player = %+v, want zero value", state.Player)
	}
}

func TestFromSnapshotInvalidNotInGameMapsMenu(t *testing.T) {
	snap := memory.Snapshot{
		At:     time.Now(),
		Valid:  false,
		Reason: memory.ReasonNotInGame,
		Phase:  memory.GamePhaseMenu,
	}

	state := FromSnapshot(snap)
	if state.Phase != GamePhaseMenu {
		t.Fatalf("Phase = %v, want GamePhaseMenu", state.Phase)
	}
	if state.Objects == nil || state.Entrances == nil || state.Monsters == nil {
		t.Fatal("invalid snapshot should have non-nil empty entity slices")
	}
}

func TestFromSnapshotInvalidEmptyReason(t *testing.T) {
	snap := memory.Snapshot{
		At:    time.Now(),
		Valid: false,
	}

	state := FromSnapshot(snap)
	if state.Valid {
		t.Fatal("Valid = true, want false")
	}
	if state.Reason != "" {
		t.Fatalf("Reason = %q, want empty (no synthetic reason)", state.Reason)
	}
	if state.Phase != GamePhaseUnknown {
		t.Fatalf("Phase = %v, want GamePhaseUnknown", state.Phase)
	}
}

func testModel(t *testing.T) *Model {
	t.Helper()
	return NewModel(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestModelCurrentBeforeUpdate(t *testing.T) {
	m := testModel(t)
	state := m.Current()
	if state.Valid || state.Reason != "" || !state.At.IsZero() {
		t.Fatalf("Current before Update = %+v, want zero State", state)
	}
}

func TestModelUpdateStoresAndReturnsState(t *testing.T) {
	m := testModel(t)
	snap := validSnapshot()

	got := m.Update(snap)
	current := m.Current()

	if !got.Valid || !current.Valid {
		t.Fatal("Update should produce valid state")
	}
	if got.Area.Name != current.Area.Name {
		t.Fatalf("Update return Area = %q, Current Area = %q", got.Area.Name, current.Area.Name)
	}
	if got.Player.HP != current.Player.HP {
		t.Fatalf("Update return HP = %d, Current HP = %d", got.Player.HP, current.Player.HP)
	}
}

func TestModelUpdateReplacesPreviousState(t *testing.T) {
	m := testModel(t)

	first := m.Update(validSnapshot())
	secondSnap := validSnapshot()
	secondSnap.AreaID = uint32(ForgottenTower)
	secondSnap.HP = 42
	second := m.Update(secondSnap)

	if first.Area.Name == second.Area.Name {
		t.Fatal("second Update should replace area")
	}
	if second.Area.Name != "Forgotten Tower" {
		t.Fatalf("Area.Name = %q, want Forgotten Tower", second.Area.Name)
	}
	if m.Current().Player.HP != 42 {
		t.Fatalf("Current HP = %d, want 42", m.Current().Player.HP)
	}
}

func TestModelCurrentReturnsIndependentCopy(t *testing.T) {
	m := testModel(t)
	m.Update(validSnapshot())

	c1 := m.Current()
	c2 := m.Current()
	c1.Player.HP = 0

	if c2.Player.HP == 0 {
		t.Fatal("mutating c1 should not affect c2")
	}
	if m.Current().Player.HP != 100 {
		t.Fatalf("mutating copy should not affect model state, HP = %d", m.Current().Player.HP)
	}
}

func TestModelUpdateReturnMutationDoesNotAffectStoredState(t *testing.T) {
	m := testModel(t)
	got := m.Update(validSnapshot())
	got.Player.HP = 0

	if m.Current().Player.HP != 100 {
		t.Fatalf("mutating Update return should not affect stored state, HP = %d", m.Current().Player.HP)
	}
}

func TestModelReset(t *testing.T) {
	m := testModel(t)
	m.Update(validSnapshot())

	at := time.Now()
	got := m.Reset(at, "process_lost")
	cur := m.Current()

	if got.Valid || cur.Valid {
		t.Fatal("Reset should produce invalid state")
	}
	if got.Reason != "process_lost" || cur.Reason != "process_lost" {
		t.Fatalf("Reason = %q, want process_lost", cur.Reason)
	}
	if got.Phase != GamePhaseUnknown || cur.Phase != GamePhaseUnknown {
		t.Fatal("Reset should set GamePhaseUnknown")
	}
	if got.Area != (Area{}) || got.Player != (Player{}) {
		t.Fatalf("Reset return should zero Area/Player, got Area=%+v Player=%+v", got.Area, got.Player)
	}
	if cur.Area != (Area{}) || cur.Player != (Player{}) {
		t.Fatalf("Current after Reset should zero Area/Player, got Area=%+v Player=%+v", cur.Area, cur.Player)
	}
	if !got.At.Equal(at) {
		t.Fatal("Reset should preserve At timestamp")
	}
}

func TestMapPhaseAllValues(t *testing.T) {
	cases := []struct {
		in   memory.GamePhase
		want GamePhase
	}{
		{memory.GamePhaseUnknown, GamePhaseUnknown},
		{memory.GamePhaseMenu, GamePhaseMenu},
		{memory.GamePhaseLoading, GamePhaseLoading},
		{memory.GamePhaseInGame, GamePhaseInGame},
	}
	for _, tc := range cases {
		if got := mapPhase(tc.in); got != tc.want {
			t.Fatalf("mapPhase(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestFromSnapshotLoadingInvalid(t *testing.T) {
	snap := memory.Snapshot{
		Valid:  false,
		Phase:  memory.GamePhaseLoading,
		Reason: memory.ReasonPlayerPointerUnavailable,
	}
	state := FromSnapshot(snap)
	if state.Phase != GamePhaseLoading {
		t.Fatalf("Phase = %v, want loading", state.Phase)
	}
	if state.Area != (Area{}) || state.Player != (Player{}) {
		t.Fatal("invalid loading state should zero Area/Player")
	}
}
