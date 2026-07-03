# Countess-Run

Phase 4.4 ergaenzt den ersten echten End-to-End-Travel-Schritt fuer den Countess-MVP:

```powershell
go run ./cmd/d2rbot --run countess --phase travel-marsh --probe --verbose
```

Der Bot startet in `Rogue Encampment`, laeuft bei Bedarf vom Spawn-/Stash-Bereich zum Act-1-Waypoint, klickt den Waypoint per Hover-Feedback-Loop, waehlt im Waypoint-Menue `Black Marsh` per fester client-relativer Koordinate und wartet anschliessend auf `Area.ID == world.BlackMarsh`.

## Verhalten

- `--phase travel-marsh` ist CLI-only und nur mit dem Countess-Run gueltig.
- `--run countess` ohne Phase behaelt den bisherigen Phase-4.1-Stub bei.
- Der Travel-Flow nutzt Area-IDs (`world.RogueEncampment`, `world.BlackMarsh`) fuer Entscheidungen; Area-Namen dienen nur Logs.
- Input-Schritte laufen nur bei `Valid && Phase=in_game`. Nur `wait_black_marsh` darf als Non-Input-Step waehrend Loading/invalid Snapshots weitergetickt werden.

## Waypoint-Interaktion

Es wird kein D2R-Waypoint-Hotkey vorausgesetzt. Der Ablauf folgt dem Koolo-Modell:

1. `acquire_town_waypoint`: Wenn der Waypoint noch nicht enumeriert ist, laeuft `pathing.TownWalker` per Force Move (`e`) entlang einer Act-1-Preset-Route Richtung nordoestlichem Waypoint.
2. `open_waypoint`: `pathing.WaypointActions.TickTownWaypoint` klickt den naechsten `ObjectKindWaypoint` erst nach Hover-Bestaetigung.
3. `select_black_marsh`: nach kurzer Menue-Settle-Zeit klickt `SelectBlackMarsh` die konfigurierte UI-Position.
4. `wait_black_marsh`: Erfolg bei `world.BlackMarsh`, sonst Timeout ueber `runs.step_timeout_ms`.

Die Default-UI-Koordinate ist fuer 1280x720 windowed kalibriert:

```yaml
pathing:
  town_walk:
    force_move_key: e
    route_file: configs/routes/act1-town-waypoint.yaml
    move_interval_ms: 650
    settle_timeout_ms: 350
    stuck_timeout_ms: 3500
    arrival_distance_tiles: 8
  waypoint:
    max_click_distance: 15
  waypoint_ui:
    black_marsh_x: 200
    black_marsh_y: 342
```

Bei abweichender Fenstergroesse loggt der Bot eine separate Warnung fuer fixe Waypoint-UI-Koordinaten.

## Preconditions

- `input.enabled: true`
- D2R windowed 1280x720 oder kalibrierte `pathing.waypoint_ui`-Koordinaten
- Charakter steht in `Rogue Encampment`, entweder nahe Waypoint oder am Spawn-/Stash-Bereich
- Force Move ist in D2R auf `pathing.town_walk.force_move_key` gebunden (Default `e`)
- Black-Marsh-Waypoint ist fuer Charakter und Schwierigkeit freigeschaltet

## Manuelle Validierung

```powershell
go run ./cmd/d2rbot --pathing-test play-town-route:act1-waypoint --probe --verbose
go run ./cmd/d2rbot --run countess --phase travel-marsh --probe --verbose
```

Falls die Preset-Route nicht zum lokalen Setup passt, kann eine Override-Route aufgezeichnet werden:

```powershell
go run ./cmd/d2rbot --pathing-test record-town-route:act1-waypoint --probe --verbose
```

## Grenzen

- Town-Walk ist nur fuer Rogue Encampment / Act 1 vorgesehen.
- Keine OCR-/Bild-Erkennung des Waypoint-Menues.
- Kein Pickit, Kampf oder Tower-Travel in Phase 4.5.
