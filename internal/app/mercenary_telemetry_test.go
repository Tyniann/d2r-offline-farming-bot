package app

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestObserveMercenaryDeathEmitsConfirmedObservation(t *testing.T) {
	recorder, err := telemetry.New(t.TempDir(), "summoner", "")
	if err != nil {
		t.Fatal(err)
	}
	path := recorder.Path()
	rt := &Runtime{Log: config.NewLogger("error"), Telemetry: recorder}

	prev := world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.Area{ID: world.ArcaneSanctuary},
		Mercenary: world.Mercenary{
			HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true,
			UnitID: 9, HP: 40, MaxHP: 100,
		},
	}
	cur := prev
	cur.Mercenary = world.Mercenary{HiredKnown: true, Hired: true, Dead: true, UnitID: 9}
	observation := mercenaryDeathObservation{
		Decision:  mercenaryDeathConfirmed,
		UnitID:    cur.Mercenary.UnitID,
		AreaID:    cur.Area.ID,
		HPPercent: prev.Mercenary.HPPercent(),
	}
	rt.observeMercenaryDeath(observation)
	rt.observeMercenaryDeath(mercenaryDeathObservation{})

	if err = recorder.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var died []telemetry.Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event telemetry.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		if event.Event == telemetry.MercenaryDied {
			died = append(died, event)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(died) != 1 || died[0].HPPercent != 40 || died[0].MercUnitID != 9 {
		t.Fatalf("died=%+v", died)
	}
}

func TestMercenaryDeathGuardRejectsTransientAreaTransitionRead(t *testing.T) {
	base := time.Unix(100, 0).UTC()
	guard := mercenaryDeathGuard{}

	if got := guard.observe(mercenaryState(1, base, world.RogueEncampment, true)); got.Decision != mercenaryDeathStable {
		t.Fatalf("alive observation = %+v", got)
	}
	if got := guard.observe(mercenaryState(2, base.Add(100*time.Millisecond), world.LowerKurast, false)); got.Decision != mercenaryDeathPending {
		t.Fatalf("first destination dead observation = %+v, want pending", got)
	}
	if got := guard.observe(mercenaryState(3, base.Add(200*time.Millisecond), world.LowerKurast, true)); got.Decision != mercenaryDeathStable {
		t.Fatalf("recovered destination observation = %+v, want stable", got)
	}
}

func TestMercenaryDeathGuardConfirmsStableAreaTransitionDeath(t *testing.T) {
	base := time.Unix(200, 0).UTC()
	guard := mercenaryDeathGuard{}
	guard.observe(mercenaryState(1, base, world.RogueEncampment, true))

	for index, elapsed := range []time.Duration{100 * time.Millisecond, time.Second, 3100 * time.Millisecond} {
		got := guard.observe(mercenaryState(uint64(index+2), base.Add(elapsed), world.LowerKurast, false))
		want := mercenaryDeathPending
		if index == 2 {
			want = mercenaryDeathConfirmed
		}
		if got.Decision != want {
			t.Fatalf("observation %d = %+v, want %q", index+1, got, want)
		}
	}
}

func TestMercenaryDeathGuardConfirmsThreeFreshReadsInStableArea(t *testing.T) {
	base := time.Unix(300, 0).UTC()
	guard := mercenaryDeathGuard{}
	guard.observe(mercenaryState(1, base, world.ArcaneSanctuary, true))

	for generation := uint64(2); generation <= 4; generation++ {
		got := guard.observe(mercenaryState(generation, base.Add(time.Duration(generation)*100*time.Millisecond), world.ArcaneSanctuary, false))
		want := mercenaryDeathPending
		if generation == 4 {
			want = mercenaryDeathConfirmed
		}
		if got.Decision != want {
			t.Fatalf("generation %d = %+v, want %q", generation, got, want)
		}
	}
}

func mercenaryState(generation uint64, at time.Time, area world.AreaID, alive bool) world.State {
	mercenary := world.Mercenary{HiredKnown: true, Hired: true, UnitID: 9}
	if alive {
		mercenary.Alive = true
		mercenary.VitalsKnown = true
		mercenary.HP = 40
		mercenary.MaxHP = 100
	} else {
		mercenary.Dead = true
	}
	return world.State{
		Valid:      true,
		Phase:      world.GamePhaseInGame,
		Generation: generation,
		At:         at,
		Area:       world.LookupArea(area),
		Mercenary:  mercenary,
	}
}
