# Pathing (Teleport-Navigation)

## Überblick

Phase 4.3: Bewegung **ausschließlich per Teleport** nach dem Koolo-Modell — eine feste, spieler-zentrierte isometrische Projektion liefert grobe Bildschirmkoordinaten; **Präzision** für Entity-Klicks kommt aus einem geschlossenen **Hover-Feedback-Loop** aus dem Speicher. Zielerreichung wird primär über den Area-Wechsel (`world.State.Area.ID`) erkannt; Stuck-Detection bricht festgefahrene Navigation ab.

**Kein Memory-Camera-Ansatz:** Der frühere Plan, die Viewport-Origin per Reverse Engineering aus dem Speicher zu lesen, ist verworfen. Keine öffentliche Referenz (d2go, MapAssist, Koolo) liest eine Kamera aus dem Speicher, und manuelles RE pro D2R-Patch ist für dieses Projekt nicht leistbar. Koolo beweist, dass es ohne geht.

## Ort im Code

- **Paket:** `internal/pathing/`
- **Einstieg:** `app.New()` → `pathing.NewNavigator(log, pathing.Deps{Input, Bindings, Config})`; CLI `--pathing-test` in [`internal/app/pathing_test_mode.go`](../../internal/app/pathing_test_mode.go)
- **Wichtige Dateien:**
  - `transform.go` — `Projector`-Interface, `RelativeProjector` (Koolo-Formel)
  - `click.go` — `EntityClicker` (Spiral + Hover-Feedback-Loop)
  - `teleport.go` — `TeleportMover` via `CastSkillAt(Teleport)` aus YAML-Bindings
  - `stuck.go` — `StuckDetector` (Positions-/Area-Fortschritt, Timeout)
  - `explore.go` — `ExplorePlanner` (bearing-first, entity nur nah)
  - `navigator.go` — `Navigator`-State-Machine (`Start`/`Tick`/`Active`/`LastResult`/`Reset`)
  - `types.go` — `Goal`, `GoalKind`, `NavStatus`, `NavResult`, `NavTickResult`
  - `config.go` — `pathing.Config` mit `Validate()` und Defaults
  - `internal/memory/hover.go` — `HoverState`, 12-Byte-Hover-Read
  - `internal/world/hover.go` — `HoverInfo`, `Matches(unitType, unitID)`
  - `internal/world/area_parse.go` — `ParseAreaSpec` für CLI-Specs
- **Config:** `configs/config.example.yaml` → Abschnitt `pathing:`

## Funktionalität

### Relative Projektion (Koolo-Modell)

```text
dx = targetX − playerX;  dy = targetY − playerY   (signed)
clientX = playable_center_x·W + (dx − dy)·tile_width
clientY = playable_center_y·H + (dx + dy)·tile_height
```

Defaults `19.8` / `9.9` und `playable_center 0.5/0.52` sind für **1280×720 windowed** kalibriert. Bei abweichender Client-Größe loggt der Bot beim Fenster-Bind eine Warnung (kein Hard-Fail).

### Hover-Feedback-Loop (EntityClicker)

Die Projektion liefert nur den **Startpunkt** — geklickt wird erst nach Bestätigung aus dem Speicher:

1. Distanz-Gate: Ziel weiter als `max_entrance_click_distance` → `too_far` (Navigator nähert erst an)
2. Projektion von `target.Position − anchor_offset_tiles` (Klickpunkt = sichtbarer Körper, nicht Boden-Tile)
3. Spiral-Offset für Versuch N (archimedische Spirale, `spiral_step` Grad pro Versuch)
4. `MoveTo(clientX, clientY)`
5. Nächster Tick: `state.Hover` matcht **UnitID und UnitType** → `Click(left)` → `hit`
6. Nach `max_hover_attempts` ohne Match → `hover_not_found` — **kein blinder Klick**

Der Hover-Offset kommt — wie UnitTable und UI — per **Signature-Scan** (d2go-Pattern) aus `ScanProbeOffsets`. Schlägt der Scan fehl, bleibt `Hover=0`: Lesen ist fail-open (`HoverState` leer), Klicken fail-closed (nie ohne Bestätigung).

### Hover-Datenfluss

`memory.HoverState` (12-Byte-Buffer bei `moduleBase+Hover`: uint16 `is_hovered`, `+0x04` `unit_type`, `+0x08` `unit_id`) → `Snapshot.Hover` → `world.State.Hover` (`HoverInfo`). Entities (Objects/Entrances/Monsters) tragen zusätzlich `IsHovered`, gematcht über UnitID **und** UnitType (1=monster, 2=object, 5=entrance — wie d2go).

### Gebietswechsel — zwei Mechanismen

| Übergang | Beispiel | Strategie | Präzision |
|----------|----------|-----------|-----------|
| **Outdoor-Grenze** | Blood Moor → Black Marsh | Bearing Richtung Rand; Erfolg = `Area.ID` | grob (Projektion genügt) |
| **Town-Ausgang** | Rogue Encampment → Blood Moor | Bearing zum Ausgang | grob |
| **Entrance-Unit** | Marsh → Tower (ID 10), Catacombs Up/Down | annähern bis `distance ≤ max_entrance_click_distance`, dann EntityClicker | präzise via Hover |
| **Waypoint-Object** | Town → Marsh (Object-Klick) | `NearestObject(Waypoint)` → EntityClicker | präzise via Hover |

### Navigator-State-Machine

`NavStatus`: `idle` → `moving`/`exploring` → `clicking` → `arrived` | `stuck` | `failed`.
`NavResult.Reason`: leer (Erfolg) | `stuck` | `hover_not_found` | `cancelled` | `invalid_goal` | `projection_failed`.

- **Explore bearing-first:** rotierende Kompass-Richtungen (`bearing_count`, Default 8) mit `step_distance_tiles`; Area-Wechsel resettet den Bearing-Index
- **Cast-Auswertung mit Settle-Zeit:** Nach einem Teleport-Cast wartet der Navigator bis zu 700 ms (`teleportSettleTimeout`) auf die Positionsänderung im Speicher (Cast-Animation ist FCR-abhängig). Fortschritt ≥ `stuck_progress_tiles` wird sofort bestätigt (schnelles Cast-Chaining); erst nach Ablauf der Settle-Zeit ohne Bewegung gilt der Cast als blockiert und die Richtung rotiert. Ohne diese Wartezeit würde jeder Cast fälschlich als blockiert gewertet und der Bot teleportiert im Kreis
- **Entity-Modus** nur, wenn `goal.ViaEntrance` gesetzt ist und die Entrance näher als `max_entrance_click_distance` liegt; sichtbare, aber noch zu entfernte Entrances werden als `entity_approach` priorisiert und per Teleport angenähert
- **Stuck:** weder Positions-Delta ≥ `stuck_progress_tiles` noch Area-Wechsel innerhalb `stuck_timeout_ms` → `reason=stuck`; das Log enthält `player_mana` (Teleport ohne Mana erkennbar)
- Ticks bei `Phase != in_game` (Loading) werden übersprungen; Pausen zählen nicht als Stuck-Zeit
- Teleport-Casts sind über `move_interval_ms` gedrosselt

## Datenmodell

### YAML-Config (`pathing:`)

```yaml
pathing:
  stuck_timeout_ms: 8000
  stuck_progress_tiles: 3
  move_interval_ms: 250
  arrival_distance: 15
  projection:
    playable_center_x: 0.5
    playable_center_y: 0.52
    tile_width: 19.8
    tile_height: 9.9
  click:
    max_hover_attempts: 15
    spiral_step: 40
    anchor_offset_tiles: 2
  explore:
    bearing_count: 8
    step_distance_tiles: 25
    max_entrance_click_distance: 15
```

Fehlende Keys erhalten die obigen Defaults (`applyDefaults`); ungültige Werte brechen den Start ab (`validate` in config + `pathing.Config.Validate()`).

### Skill-Bindings (Abhängigkeit Phase 3.6)

Teleport wird über `input.bindings.skills.teleport` (`key` + `button`) gecastet — keine Kalibrierung, keine Sorc-Hardcodierung. `BindingsPrecheck` erzwingt beim Start: Teleport konfiguriert und `button: right` (Hard-Stop), Town Portal nur Warnung. Für `hover:watch` (read-only) entfällt der Precheck.

## Operator / CLI

```powershell
# Kalibrier-Hilfe: einmaliger Teleport auf Weltkoordinate, loggt client_x/y
go run ./cmd/d2rbot --pathing-test teleport:5000,5000

# Slice-2-Validierung: Hover-Wechsel beim manuellen Mausführen loggen (read-only)
go run ./cmd/d2rbot --pathing-test hover:watch

# Stufe A: Bearing-Explore bis Area-Wechsel
go run ./cmd/d2rbot --pathing-test move-area:black_marsh

# Stufe B: Hover-Loop-Klick auf Waypoint/Entrance
go run ./cmd/d2rbot --pathing-test click-entity:waypoint
go run ./cmd/d2rbot --pathing-test click-entity:entrance

# Act-1-Town-Graph: layoutgebundene Kante aufnehmen und wiedergeben
go run ./cmd/d2rbot --pathing-test record-town-edge:stash-waypoint
go run ./cmd/d2rbot --pathing-test play-town-graph:stash,waypoint

# Phase 5.5: Hover-confirmed Pickup eines Pickit-Kandidaten
go run ./cmd/d2rbot --pathing-test pickup:item --probe --verbose
```

Ab Phase 5.7 verwendet `pathing.TownPortalActions` einen eigenen fail-closed Entity-Clicker für den Eintritt in ein selbst gecastetes Town Portal. Die Portal-ID stammt aus dem lokalen `objects.txt`-Generator; der Baustein wartet `pathing.town_portal.appear_timeout_ms` auf das Objekt. Nach dem ersten Fund müssen dieselbe UnitID und Position 500 ms stabil bleiben, damit die während der Öffnungsanimation noch nicht interaktive Portalfläche nicht unnötig Hover-Budget verbraucht. Ein Unit- oder Positionswechsel startet dieses Gate neu. Erst danach gelten die unveränderten Hover-Versuche; `max_click_distance` bleibt verpflichtend und nur eine passende Object-`UnitID` im Hover-Buffer erlaubt den Klick. Das kurze Gate begrenzt die Standzeit unter Gegnerdruck, insbesondere auf Hell.

Ab Phase 5.8 läuft `pathing.PersonalStashActions` in Rogue Encampment ohne Teleport zum lokal generierten `Bank`-Objekt. Zwei relative Detour-Anker am Memory-Stash umgehen die live validierte Town-Geometrie. Vor dem Hover-Loop bestätigt eine über `town_walk.settle_timeout_ms` stabile Memory-Position, dass der letzte Force-Move beendet ist; Bewegung setzt dieses Gate zurück. Der finale Linksklick erfolgt ausschließlich nach Object-Hover-Bestätigung. Fixed-coordinate Stash-Aktionen sind hart auf exakt `1280×720` begrenzt.

Der Act-1-Town-Walker erhält ausschließlich bereits validierte Punkte einer Graph-Kante. Auswahl und Laden erfolgen über den aktuellen `TownLayoutFingerprint`; unbekannte Layouts oder fehlende Varianten brechen vor Bewegungsinput ab. Character und Difficulty wählen keine Town-Route aus.

- `--pathing-test-timeout-ms` (Default 120000) begrenzt die Testdauer
- `--pathing-test` ist mutual exclusive mit `--run` und `--input-test`
- `input.enabled: true` erforderlich (außer `hover:watch`)
- Ready-Wait wie im Input-Test: attached, Fenster gebunden, `Valid && Phase=in_game`
- Stop/Pause-Hotkeys und Process-Lost-Behandlung wie im Input-Test
- Area-Spec: Katalogname (`black_marsh`, `Black Marsh`) oder numerische ID (`6`)

### Generische Waypoint-Ziele (Phase 10.7)

`WaypointTargetRegistry` registriert die auf 1280×720 gebundenen Ziele (Black Marsh, Durance of Hate Level 2, Arcane Sanctuary, Halls of Pain, Rogue Encampment). Jede Aktion enthält Akt-Tab, Zielzeile, 200-ms-Settle und erwartete Ziel-Area. Tab- und Zeilenklicks erfordern zusätzlich zu `world.UIState.WaypointOpen` eine post-open-Evidenz aus unserem eigenen hover-bestätigten Waypoint-Objektklick: mindestens 500 ms sowie ein Positions-Settle (`pathing.town_walk.settle_timeout_ms`). Sticky `WaypointOpen` allein reicht nicht — auf dem aktuellen Build bleibt das Bit nach Reisen oft gesetzt, obwohl das Panel geschlossen ist; UI-Koordinaten würden sonst in die Welt fallen und die Figur am Waypoint vorbeischieben. `beginStep` setzt Waypoint-State beim Eintritt in `select_run_waypoint` / `select_hub_waypoint` deshalb nicht zurück. Ein Tick führt höchstens einen Klick aus; nach dem Zielklick wird niemals erneut geklickt. Falsche Auflösung, unbekanntes Ziel, fehlendes UI-Gate, falsche Ziel-Area und Timeout enden fail-closed.

Die frühere konfigurierbare Black-Marsh-Sonderaktion wurde vollständig entfernt. Die autoritative read-only Kalibrierung ist abrufbar mit:

```powershell
go run ./cmd/d2rbot --config configs/config.yaml --waypoint-targets-inspect
```

Sie liefert für 1280×720: Act-1-Tab `(159,148)`, Act-3-Tab `(273,148)`, Black Marsh `(200,342)`, Durance Level 2 `(200,506)` und Rogue Encampment `(200,178)`. Der Befehl attachiert keinen Prozess, registriert keine Hotkeys und sendet keinen Input.

### Kalibrierung

1. `--pathing-test teleport:TX,TY` mit bekannter Zielkoordinate ausführen
2. Log `pathing test teleport cast` (`client_x/y`) mit der tatsächlichen Landeposition (`pos_x_delta/pos_y_delta`) vergleichen
3. `pathing.projection.playable_center_x/y` bzw. `tile_width`/`tile_height` anpassen
4. Empfehlung: D2R **1280×720 windowed**, Fenster nicht resizen

### Manuelle Validierung (Stufe A + B)

| Szenario | Mechanismus | Erwartung |
|----------|-------------|-----------|
| `hover:watch` über Waypoint | Hover-Read | Log zeigt UnitID des enumerierten Waypoint-Objects |
| `move-area:black_marsh` | bearing | `status=arrived`, Area Black Marsh (**Stufe A**) |
| `click-entity:waypoint` in Town | Hover-Loop | Klick erst nach Hover-Match (**Stufe B**) |
| Entity zu weit | Distanz-Gate | `too_far`, kein Klickversuch |
| Hover nie bestätigt | Fail-closed | `reason=hover_not_found`, kein blinder Klick |
| Ecke/Stuck | Stuck-Detector | `reason=stuck`, Log mit `player_mana` |
| Loading | Phase-Gate | kein Pathing-Tick |

## Abhängigkeiten

- [Input Controller](input-controller.md) — `MoveTo`, `Click`, `CastSkillAt`, Fenster-Geometrie, YAML-Bindings, Safety
- [State Probe](state-probe.md) — Hover-Read, Entity-Enumeration, GamePhase
- [World Model](world-model.md) — `State`, `NearestObject`/`NearestEntrance`, `Distance`, Area-Katalog
- Recherche: Koolo (`GameCoordsToScreenCords`, `helper.Spiral`), d2go (`HoveredData`, Hover-Pattern) — keine Runtime-Dependency

## Grenzen (explizit nicht in 4.3)

- **Bearing-Explore ist ungerichtet (WIP):** Der Planner kennt die Richtung des Zielgebiets nicht — er hält eine Kompass-Richtung, bis sie blockiert, und rotiert dann weiter. Für lange Strecken (z. B. Blood Moor → Black Marsh über mehrere Zonen) ist das funktional, aber langsam und ineffizient
- **Tower-Dungeon-Traversal bleibt im Phase-5-Stand best-effort:** Der generische Navigator nutzt Entrance-Priorisierung und Hover-Feedback, bleibt auf zufälligen Tower-Layouts aber störanfällig. Phase 6 implementiert deshalb run-unabhängiges Route Recording und Playback mit Countess als erstem Use Case; der Navigator wird dort zur Hilfslogik für lokale Korrekturen und Interaktionen.
- **Outdoor-Zonenübergänge werden nicht gezielt angesteuert (Testlauf 2026-07-02):** Bearing-Explore deckt den Kartenrand ab, teleportiert aber nie gezielt **in** den schmalen Übergangsbereich zwischen zwei Outdoor-Zonen hinein. Die verbindliche Abhilfe ist Phase 6: Der Spieler zeichnet den Run einmal manuell auf, der Bot spielt die Route ab. Entrance-Übergänge (Tower, Treppen) bleiben über den Hover-Loop verifiziert. Koolos Alternative (lokaler Map-Server aus D2-LoD-1.13c-DLLs + Seed-Read) bleibt zurückgestellt — Begründung im Backlog unter „Verworfene Alternative: Map-Server-Navigation“.
- Countess-Run-Travel-Steps (Phase 4.4+), Route-Cache ([`docs/backlog.md`](../backlog.md))
- Allgemeines Laufen ohne Teleport ausserhalb des Act-1-Town-Walkers, Klassen-Erkennung, Pickit
- Perspective-Mode; Memory-Camera (verworfen)

## Verwandte Features

- [Task Runner](task-runner.md) — nutzt `tasks.Navigator` (Start/Tick/Active) ab Phase 4.4
- [Input Controller](input-controller.md) — führt alle Maus-/Tastatur-Aktionen aus

---
*Zuletzt aktualisiert: 2026-07-31*
