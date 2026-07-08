# Hover-Confirmed Item Pickup

## Überblick

Phase 5.5 führt einen sicheren Pickup-Baustein für Ground-Items ein. Der Bot wählt ein Pickit-/Inventory-geprüftes Item, friert dessen Identität ein und klickt erst, wenn der D2R-Hover-Buffer genau dieses Item bestätigt.

Der Baustein ist noch nicht in den Countess-Full-Run integriert. Die Live-Validierung läuft isoliert über `--pathing-test pickup:item`.

## Ort im Code

- **Paket:** `internal/loot/`
- **Einstieg:** `loot.NewPickupExecutor`
- **Wichtige Dateien:** `internal/loot/pickup.go`, `internal/app/pathing_test_mode.go`
- **Config:** `loot.pickup` in `configs/config.example.yaml`

## Funktionalität

### Zielauswahl

Der Testmodus ruft `loot.Filter.Decide(state)` auf und berücksichtigt nur Decisions mit `Stage=pick_candidate`, `Kind=pick_candidate` und `CanFit=true`. Danach wird das aktuelle Ground-Item per `UnitID` aus `state.GroundItems()` gelesen.

Wenn mehrere Kandidaten existieren, gewinnt der nächste Kandidat zur Spielerposition; bei gleicher Distanz gewinnt die niedrigere `UnitID`. Danach bleibt der Executor auf genau diese `UnitID`, `TxtFileNo`, `Code` und `Position` gelockt. Retries wählen kein anderes Item.

Wenn kein Kandidat existiert, loggt der Testmodus `loot pickup test: no candidate` und beendet sich ohne Eingabe.

### Stabilität

Vor jedem Click-Zyklus verlangt der Executor zwei konsekutive gültige In-Game-Snapshots mit:

- gleicher `Area.ID`
- gleicher `UnitID`
- gleicher `TxtFileNo`
- gleichem `Code`
- `Location=ground`
- gleicher `Position`

Ungültige Snapshots oder Loading-Snapshots werden nie als Pickup-Erfolg gewertet.

### Hover-Click

Der eigentliche Klick läuft über den vorhandenen `pathing.EntityClicker`, aber hinter einem loot-lokalen Interface. Der Adapter setzt `UnitType=item` und mappt `pathing`-Ergebnisse auf loot-eigene Statuswerte.

Ein Klick passiert nur nach `Hover.UnitType == item` und `Hover.UnitID == target.UnitID`. Ohne Hover-Match endet der Zyklus mit `hover_not_found`; es gibt keinen Blind-Klick.

### Verify

Nach einem bestätigten Klick beginnt die Verify-Phase. `verify_timeout_ms` startet direkt nach `ClickHit`.

Erfolg benötigt `verify_ticks` konsekutive gültige In-Game-Snapshots mit demselben terminalen Befund:

- das Ziel-Item ist nicht mehr als Ground-Item vorhanden, oder
- `FindItemByUnitID(target.UnitID)` findet exakt `TxtFileNo`, `Location=inventory`, `PlayerOwned=true`, `Page=0`.

Ein falsch gemappter Inventory-Eintrag mit gleicher `UnitID`, aber anderer `TxtFileNo` zählt nicht als Erfolg.

### Abbruchbedingungen

Vor jedem Retry werden Stabilität, Distanz und Monster-Nähe erneut geprüft.

Terminale Statuswerte:

| Status | Bedeutung |
|--------|-----------|
| `picked_up` | Item verschwand vom Boden oder wurde im persönlichen Inventar bestätigt |
| `target_unstable` | Ziel-Unit existiert, aber Identität oder Position passt nicht mehr |
| `target_lost` | Ziel fehlt vor einem bestätigten Klick in einem gültigen In-Game-Snapshot |
| `too_far` | Ziel ist weiter entfernt als `loot.pickup.max_distance_tiles` |
| `hover_not_found` | Hover-Match wurde nach den erlaubten Click-Versuchen nicht gefunden |
| `projection_failed` | Projektion liefert keinen sicheren Client-Punkt |
| `pickup_failed` | Verify blieb bis zum Timeout ohne Erfolg |
| `monster_nearby` | Ein enumerierter lebender Monster-Eintrag ist zu nah am Spieler |
| `invalid_world` | Snapshot ist außerhalb tolerierter Wartefenster ungültig oder nicht in-game |
| `input_blocked` | Eingabe ist pausiert, gestoppt, ungebunden oder durch Safety blockiert |

Monster-Nähe nutzt das aktuelle World Model: jeder enumerierte `world.Monster` innerhalb der Distanz zählt. Hostility- oder Dead-Flags existieren in dieser Phase noch nicht.

## Datenmodell

```yaml
loot:
  pickup:
    max_retries: 3
    max_distance_tiles: 8
    verify_ticks: 3
    verify_timeout_ms: 1500
    monster_abort_distance_tiles: 12
```

`max_retries` zählt Executor-Zyklen. Jeder Zyklus darf intern bis zu `pathing.click.max_hover_attempts` Mauspositionen probieren.

## Operator / CLI

```powershell
go run ./cmd/d2rbot --pathing-test pickup:item --probe --verbose
```

Voraussetzungen:

- `input.enabled: true`
- gültiger In-Game-Snapshot
- ein sichtbares Ground-Item, das Pickit matcht und laut Inventory-Lock in das Inventar passen kann
- keine Monster innerhalb von `monster_abort_distance_tiles`

Der Modus bewegt nicht, kämpft nicht und integriert sich noch nicht in `--run countess`.

## Abhängigkeiten

- [Loot Decision Pipeline](loot-decision-pipeline.md)
- [Pickit Engine](pickit-engine.md)
- [Inventory Model und Lock Grid](inventory-lock-grid.md)
- [Pathing](pathing.md)
- [World Model](world-model.md)

---
*Zuletzt aktualisiert: 2026-07-08*
