# Countess-Run

## Überblick

Der Countess-Run enthält aktuell zwei explizite Travel-Phasen:

```powershell
go run ./cmd/d2rbot --run countess --probe --verbose
go run ./cmd/d2rbot --run countess --phase travel-entry --probe --verbose
go run ./cmd/d2rbot --run countess --phase play-route --probe --verbose
go run ./cmd/d2rbot --run countess --phase boss --probe --verbose
go run ./cmd/d2rbot --run countess --phase loot-and-return --probe --verbose
go run ./cmd/d2rbot --run countess --phase stash-personal --verbose
```

`--run countess` ohne `--phase` ist ab Phase 5.8 der vollständige Countess-Run: Act-1-Town-Waypoint -> Black Marsh -> Forgotten Tower -> Tower Cellar Level 5 -> Countess-Kill -> Loot-Pickup -> Town Portal -> Portal-Eintritt -> Rogue Encampment -> Personal Stash -> `complete`.

`travel-entry` führt vom Rogue Encampment über den Act-1-Waypoint nach `Black Marsh`.
`play-route` nutzt diesen Prefix weiter und delegiert anschließend bis `Tower Cellar Level 5` an die über das Character-/Run-Assignment ausgewählte Aufnahme.

**Status Phase 10.2:** Der Erfolgspfad verwendet die gemeinsame `runPipeline`. Countess-spezifische Daten liegen in der Registry-Definition und im ID-basierten Config-Eintrag. Der frühere produktive Explorer-/Cellar-Fallback ist entfernt.

## Verhalten

- Die Phasennamen sind generisch und gehören zum gemeinsamen CLI-Vertrag. Countess und Mephisto führen dieselben Phasen mit ihren jeweiligen Definitionsdaten aus.
- `--phase boss` ist CLI-only, startet nur in `Tower Cellar Level 5`, enthält keinen Travel-Prefix und castet kein Town Portal.
- `--phase loot-and-return` ist CLI-only, startet nur in `Tower Cellar Level 5`, enthält keinen Travel- oder Kill-Prefix und castet nach der Loot-Phase Town Portal.
- `--run countess` ohne Phase startet den Full Run und verlangt zu Beginn `Rogue Encampment`; andere Gebiete schlagen mit `not_act1_town` fehl.
- Der Travel-Flow nutzt Area-IDs für Entscheidungen; Area-Namen dienen nur Logs.
- Input-Schritte laufen nur bei `Valid && Phase=in_game`.
- `wait_entry_area` darf für beide Travel-Phasen und den Full Run als Non-Input-Step während Loading/invalid Snapshots weitergetickt werden.
- `play-route` kann nach Prozessstart nur aus Act-1-Town, vom verifizierten Black-Marsh-Routenstart oder bereits auf Cellar 5 fortgesetzt werden. Mittlere Route-Areas sind kein zulässiges Resume.

## Full Run (Phase 5.6)

Der Full Run verwendet die gemeinsame Pipeline direkt:

```text
precheck -> acquire_town_waypoint -> open_waypoint -> select_run_waypoint -> wait_entry_area
-> play_bound_route -> acquire_boss -> engage_boss -> reposition_for_loot -> wait_for_drops -> scan_loot
-> pick_loot -> cast_town_portal -> enter_town_portal -> wait_origin_town
-> open_personal_stash -> stash_items -> close_personal_stash -> complete
```

`cast_town_portal` nutzt den konfigurierten `town_portal`-Skill und castet client-relativ auf die Fenstermitte (`ClientWidth/2`, `ClientHeight/2`). Ab Phase 5.7 wartet `enter_town_portal` auf das aus lokalem D2R-`objects.txt` generierte Portal-Objekt und klickt es ausschließlich nach Hover-Bestätigung. `wait_origin_town` bestätigt die Ankunft im Rogue Encampment. Ab Phase 5.8 folgen Personal-Stash-Navigation, geschützte Transfers und bestätigtes Schließen; erst danach loggt der Full Run `completion=personal_stash_complete`.

Die isolierten Phasen bleiben bewusst als Testoberflächen erhalten: Travel-Phasen enden am jeweiligen Zielgebiet, `boss` endet nach defensiver Kill-Bestätigung ohne Portal, `loot-and-return` prüft nur Loot und Portal nach einem manuellen oder vorherigen Kill.

## Countess-Loot (Phase 5.6)

`loot-and-return` und der Full Run nutzen dieselben Loot-Schritte:

Für Phase-13-Gate B dient `loot-and-return` zusätzlich als isolierter GUI-bis-Loot-Nachweis: Das unzugeordnete Profil `phase13-live-acceptance` matcht ausschließlich einen billigen Pfeilköcher (`aqv`), den keine bestehende Countess-Regel abfängt. Nach synthetischer Core-Vorschau, revisionsgebundenem Profil-Save und expliziter Countess-Zuordnung prüft der Lauf Ground-Erkennung, hover-bestätigten Pickup, Portalrückkehr und Memory-bestätigten Personal-Stash-Transfer ohne Travel- oder Bosskampf. Profil-/Regel-ID, Aktion und Revisionen müssen zwischen Dashboard, Core-Log und Run-JSONL übereinstimmen.

Nur für diese ausdrücklich routefreie Phase werden Farming-Route-Reasons wie `route_lifecycle_unavailable` beim CLI-Preflight ausgeblendet. Full Run, `travel-entry`, `play-route`, `boss`, `stash-personal` und `town-ready` behalten ihr bisheriges Route-Gate unverändert. Profil-, Loot-, Portal-, Belt-, Town-, Telemetrie- und Input-Sicherheitsprüfungen bleiben auch für `loot-and-return` verpflichtend.

| Step | Verhalten |
|------|-----------|
| `wait_for_drops` | verlangt drei aufeinanderfolgende gültige `in_game`-Snapshots in `Tower Cellar Level 5`; Drops sind nicht erforderlich |
| `scan_loot` | führt einen stateless Scan über `rt.Loot.Decide(state)` aus und wählt den nächsten Pickit-/Inventory-tauglichen Kandidaten |
| `pick_loot` | startet genau einen stateful Pickup-Executor pro Kandidat und tickt ihn bis zu einem terminalen Ergebnis |

Während aller Loot-Schritte führt ein gültiger Snapshot außerhalb von `Tower Cellar Level 5` zu `unexpected_area`. Invalid-/Loading-Snapshots werden nicht als Input-Ticks ausgeführt und laufen in den äußeren Step-Timeout.

Pickup-Ergebnisse mit Item-/World-Ursache (`monster_nearby`, `hover_not_found`, `target_lost`, `target_unstable`, `too_far`, `pickup_failed`) werden für die aktuelle `pick_loot`-Phase per `UnitID` übersprungen und danach erneut gescannt. Harte Verdrahtungs- oder Projektionsfehler (`input_blocked`, `projection_failed`, ein vom Loot-Adapter gemeldetes `invalid_world`) beenden den Run als Fehler. Inventory-Full/No-Fit entsteht im Scan als fehlender Pickup-Kandidat, nicht als Pickup-Executor-Fehler.

Nach bestätigtem Countess-Kill bewahrt die State Machine ihre letzte Memory-bestätigte Weltposition auf. `reposition_for_loot` versucht höchstens dreimal, sich dieser Position zu nähern. Nur ein tatsächlich gesendeter Teleport verbraucht einen Versuch; ein durch das Input-Throttling unterdrückter Aufruf zählt nicht. Zwischen zwei Versuchen müssen mindestens 500 ms und ein neuerer Memory-Snapshot liegen. Bleibt die bestätigte Ankunft innerhalb von vier Tiles auch nach dem letzten Versuch aus, beginnt die Drop-Stabilisierung trotzdem: Die aktuellen Item-Positionen sind für den anschließenden Pickup genauer als die historische Bossposition.

Für jeden ausgewählten Pickit-Kandidaten wird dessen `UnitID` vor Input erneut im aktuellen World-State aufgelöst. Liegt das Item außerhalb von `loot.pickup.max_distance_tiles`, folgen nach denselben Snapshot-/Zeitgrenzen höchstens drei Teleports direkt zu seiner aktuellen Memory-Position. Erst danach übernimmt der vorhandene hover-bestätigte Pickup-Executor. Ist das Item weiterhin zu weit entfernt, wird es mit `too_far` für diese Loot-Phase übersprungen; Input-, Projektions- und ungültige World-Fehler bleiben terminal. Dadurch kann ein ungünstig liegender Drop gezielt erreicht werden, ohne blinde Teleport- oder Klickschleifen zu erzeugen. Andere Run-Definitionen erhalten die Post-Kill-Repositionierung nur nach ausdrücklichem Opt-in.

Endet der Pickup mit `hover_not_found` oder `pickup_failed`, folgt einmalig pro `UnitID` ein distanzignorierender Teleport auf die aktuelle Item-Position (z. B. aus Bone Prison), danach genau ein erneuter Pickup-Versuch. Scheitert auch der, bleibt das bisherige Skip-Verhalten.

## Safety-Potions (Phase 4.7)

Vor jedem normalen Task-Tick prüft der Runner den aktuellen HP-Stand. Die Safety greift nur bei gültigem `in_game`-Snapshot und `MaxHP > 0`.

| Bedingung | Aktion |
|-----------|--------|
| `HPPercent() <= 35` | Belt Slot 4: Full Rejuvenation Potion |
| `HPPercent() <= 65` | Belt Slot 1: Heiltrank |

Nach einem Safety-Cast endet der aktuelle Poll-Tick sofort, damit pro Tick nur eine echte Input-Aktion passiert. Der Guard ist auf 1500 ms gedrosselt. Falls die Run-Actions nicht verdrahtet sind, läuft der normale Step weiter; falls ein vorhandener Belt-Cast fehlschlägt, endet der Run mit `safety_potion_failed`.

Feste MVP-Belegung:

- Slot 1 = Heiltränke
- Slot 2 und 3 = Manatränke
- Slot 4 = Full Rejuvenation Potions

Diese Belegung soll später durch den Nutzer konfigurierbar werden. Mana-Potions werden in 4.7 noch nicht automatisch verbraucht.

## Waypoint-Interaktion

Es wird kein D2R-Waypoint-Hotkey vorausgesetzt. Der Ablauf folgt dem Koolo-Modell:

1. `acquire_town_waypoint`: Wenn der Waypoint noch nicht enumeriert ist, läuft `pathing.TownWalker` per Force Move (`e`) entlang der für den konfigurierten Schwierigkeitsgrad aufgezeichneten Act-1-Route Richtung Waypoint.
2. `open_waypoint`: `pathing.WaypointActions.TickTownWaypoint` klickt den nächsten `ObjectKindWaypoint` erst nach Hover-Bestätigung.
3. `select_run_waypoint`: nach bestätigtem `WaypointOpen` wählt der gemeinsame Executor den registrierten Act-1-Tab und nach 200 ms die Black-Marsh-Zeile. Jeder Tick erzeugt höchstens einen Klick.
4. `wait_entry_area`: Erfolg bei `world.BlackMarsh`; eine andere bestätigte Area oder `runs.step_timeout_ms` beendet ohne weiteren Klick.

Die Default-UI-Koordinate ist für 1280x720 windowed kalibriert:

```yaml
pathing:
  town_walk:
    force_move_key: e
    move_interval_ms: 650
    settle_timeout_ms: 350
    stuck_timeout_ms: 3500
    arrival_distance_tiles: 8
  waypoint:
    max_click_distance: 15
```

Die Waypoint-Zielregistry ist hart auf 1280×720 gebunden; eine abweichende Fenstergröße stoppt vor UI-Input. Die frühere `pathing.waypoint_ui`-Konfiguration existiert nicht mehr.

Jeder Schwierigkeitsgrad benötigt eine eigene Aufzeichnung, weil Offline-Map-Layouts und damit die Waypoint-Position voneinander abweichen können. Fehlt die ausgewählte Datei oder ist sie ungültig, endet `acquire_town_waypoint` mit `town_route_missing`; der Bot verwendet niemals stillschweigend eine Route aus einer anderen Schwierigkeit.

## Tower-Traversal

`play-route` hängt nach `wait_entry_area` den generischen Route-Schritt an:

| Step | Zielgebiet | Entrance |
|------|------------|----------|
| `play_bound_route` | `world.TowerCellarLevel5` | Assignment-Route für `(character, countess)`; der generische Player bestätigt Punkte und Transitions |

Der Adapter lädt die Route per stabiler ID, führt Character-/Versions-/Layout-/Start-Precheck aus und tickt denselben `pathing.RoutePlayer` wie der isolierte CLI-Modus.

Die Traversierung verwendet keinen Bearing-Explore-Fallback. Fehlt die Route oder weicht der aktive Zustand ab, endet der Run fail-closed.

Countess bindet die run-unabhängige Route ausschließlich über das atomische Assignment-Manifest. Datei-, Segment- und Transition-Details bleiben außerhalb der Task-State-Machine.

Transitions nutzen die in der Aufnahme gespeicherte Semantik. Der Forgotten-Tower-Vorraum bleibt konservativ `unknown`; alle Cellar-Abgänge verwenden `tower_cellar_down`. Der Handler pinnt jeweils eine passende Laufzeit-Unit und bestätigt Erfolg ausschließlich über die erwartete Ziel-Area.

## Countess-Kill (Phase 4.6)

`boss` ist eine getrennte Cellar-5-Phase:

| Step | Verhalten |
|------|-----------|
| `precheck` | verlangt `Valid`, `Phase=in_game`, `Tower Cellar Level 5` und verdrahtete Combat-Actions |
| `acquire_boss` | sucht zuerst `DarkStalker` als Super-Unique, danach irgendein Super-Unique nur in Cellar 5 |
| `engage_boss` | hält das gespeicherte Target per `UnitID`, repositioniert bei Distanz und castet `Bone Spear` |

Wenn Countess noch nicht sichtbar ist, startet der Bot genau einmal ein `GoalKindMoveToPosition` auf einen Suchanker: zuerst die `Good Chest`, sonst die sichtbare `tower_cellar_down`-Entrance in Cellar 5. Er klickt weder Chest noch Entrance an, führt keinen Hover-Klick aus und hebt keinen Loot auf. Nach Ankunft wartet der Step bis zum Timeout, falls Countess weiter unsichtbar bleibt.

Der MVP-Kampf ist fest auf `necro_bone_spear` begrenzt. Der Task ruft Combat pro Tick auf; der App-Adapter drosselt echte Casts über `runs.definitions.countess.combat.attack_interval_ms`. Bei Abstand größer `reposition_distance_tiles` teleportiert der Bot in Richtung Countess, mit `engage_distance_tiles` als gewünschtem Restabstand. Sonst castet er den aufgelösten Skill `bone_spear`.

Kill-Erfolg wird defensiv gezählt: Erst wenn eine gespeicherte Countess-`UnitID` in gültigen Cellar-5-Snapshots `kill_confirm_ticks` mal in Folge nicht mehr als living monster erscheint, endet die Phase erfolgreich. Ungültige Snapshots, Loading und Area-Wechsel zählen nicht als Tod; ein Area-Wechsel aus Cellar 5 während des Kampfes schlägt mit `unexpected_area` fehl.

```yaml
runs:
  definitions:
    countess:
      combat:
        profile: necro_bone_spear
        attack_skill: bone_spear
        attack_interval_ms: 350
        engage_distance_tiles: 22
        reposition_distance_tiles: 32
        kill_confirm_ticks: 3
```

## Preconditions

- `input.enabled: true`
- D2R windowed 1280×720
- Charakter steht in `Rogue Encampment`, entweder nahe Waypoint oder am Spawn-/Stash-Bereich
- Force Move ist in D2R auf `pathing.town_walk.force_move_key` gebunden (Default `e`)
- Teleport ist in `input.bindings.skills.teleport` konfiguriert; das bleibt in 5.6 ein globaler Runtime-Precheck für aktive Input-Runs, auch wenn eine isolierte Phase fachlich keinen Teleport nutzt
- Für `--run countess`: `input.bindings.skills.bone_spear`, `input.bindings.skills.town_portal`, `input.bindings.belt.slot_1` und `input.bindings.belt.slot_4` sind konfiguriert
- Für `boss`: `input.bindings.skills.bone_spear` ist konfiguriert; die Phase castet kein Portal
- Für `loot-and-return`: `input.bindings.skills.teleport`, `input.bindings.skills.town_portal`, `input.bindings.belt.slot_1` und `input.bindings.belt.slot_4` sind konfiguriert; Bone Spear ist nicht erforderlich
- Black-Marsh-Waypoint ist für Charakter und Schwierigkeit freigeschaltet

## Manuelle Validierung

```powershell
go run ./cmd/d2rbot --pathing-test play-town-graph:stash,waypoint --probe --verbose
go run ./cmd/d2rbot --pathing-test click-entity:entrance --probe --verbose
go run ./cmd/d2rbot --pathing-test inspect:entrances --probe --verbose --pathing-test-timeout-ms 30000
go run ./cmd/d2rbot --run countess --phase travel-entry --probe --verbose
go run ./cmd/d2rbot --run countess --phase play-route --probe --verbose
go run ./cmd/d2rbot --run countess --phase boss --probe --verbose
go run ./cmd/d2rbot --run countess --phase loot-and-return --probe --verbose
```

`click-entity:entrance` prüft nur Hover-/Klickmechanik auf der nächsten Entrance-Unit. Für die Tower-Annahmen müssen die verbose World-Logs zusätzlich zeigen, dass nahe Tower/Stairs die erwarteten Entrance-Kinds und Namen erscheinen.
`inspect:entrances` bewegt oder klickt nicht. Der Operator kann den Charakter manuell im `Forgotten Tower`-Vorraum oder an einem Durchgang positionieren; der Bot loggt Spielerposition, Hover-State und alle sichtbaren Entrance-IDs mit Unit-ID, Kind, Position, Delta und Distanz.
Jeder CLI-Lauf schreibt denselben Logstream zusätzlich in eine timestamped Datei unter `logs/`, damit verbose Testläufe vollständig nachträglich auswertbar bleiben.

Phase 5.7 wurde am 10.07.2026 mit `loot-and-return` live validiert: hover-bestätigter Tir-Runen-Pickup, Portal-Erkennung aus dem lokalen `objects.txt`-Katalog, hover-bestätigter Portal-Klick und verifizierte Ankunft im Rogue Encampment endeten mit `outcome=success`.

Phase 5.8 wurde am selben Tag isoliert live validiert: Town-Portalbereich → relative Stash-Detour → Hover-Klick → UI-Bestätigung sowie sechs einzeln Memory-bestätigte Ctrl+LMB-Transfers mit anschließend bestätigtem `Esc` endeten jeweils mit `outcome=success`.

Ab Phase 5.10 erzeugt jeder aktive Countess-Lauf vor dem ersten Input eine eigene fail-closed JSONL-Datei unter `telemetry.directory`. Ein I/O-Fehler beendet den Run mit `telemetry_failed`.

Falls für das aktuell erkannte Town-Layout noch eine Graph-Kante fehlt, kann sie aufgezeichnet werden:

```powershell
go run ./cmd/d2rbot --pathing-test record-town-edge:stash-waypoint --probe --verbose
```

Der Recorder bindet die Aufnahme an den read-only aus Stash und Waypoint ermittelten `TownLayoutFingerprint`. Der Graphplayer und der produktive Run akzeptieren ausschließlich eine exakt passende Variante; Difficulty und Charakter sind keine Town-Routenschlüssel.

## Grenzen

- Town-Walk ist nur für Rogue Encampment / Act 1 vorgesehen.
- Keine OCR-/Bild-Erkennung des Waypoint-Menüs.
- Keine Shared-Stash-, Sell- oder Identify-Automation. Phase 5.8 automatisiert ausschließlich den Personal Stash für aktuelle Pickit-MVP-Typen.
- `boss` nutzt keine Curses, Summons, Bone Prison, Potion-Logik oder Good-Chest-Interaktion.
- Kein robuster Tower-Solver: zufällige Tower-Layouts bleiben die größte Unsicherheit des Phase-5-Stands. Phase 6 zieht Run Recording und Playback deshalb als direkte Nachfolgephase vor und ersetzt die globale Explorer-Traversierung im produktiven Countess-Run.

---
*Zuletzt aktualisiert: 2026-07-27*
