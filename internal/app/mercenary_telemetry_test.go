package app

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestObserveMercenaryDeathEmitsOnceOnAliveToDead(t *testing.T) {
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
	rt.observeMercenaryDeath(prev, cur)
	rt.observeMercenaryDeath(cur, cur)

	if err := recorder.Close(); err != nil {
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
