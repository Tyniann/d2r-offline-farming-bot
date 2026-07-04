# Countess-Run

## Überblick

Der Countess-Run enthält aktuell zwei explizite Travel-Phasen:

```powershell
go run ./cmd/d2rbot --run countess --phase travel-marsh --probe --verbose
go run ./cmd/d2rbot --run countess --phase travel-cellar5 --probe --verbose
go run ./cmd/d2rbot --run countess --phase kill-countess --probe --verbose
```

`travel-marsh` führt vom Rogue Encampment über den Act-1-Waypoint nach `Black Marsh`.
`travel-cellar5` nutzt diesen Prefix weiter, sucht anschließend den Forgotten Tower und traversiert best-effort bis `Tower Cellar Level 5`.

**Status Phase 4.5:** Der MVP ist abgeschlossen. `travel-cellar5` beweist den vollständigen Datenfluss Town-Waypoint → Black Marsh → Tower → Cellar-Ziele und ist als manuell validierbarer Startpunkt nutzbar. Der aktuelle generische Navigator ist aber ausdrücklich nicht zuverlässig genug, um zufällige Tower-Layouts stabil zu lösen. Dass er unterwegs mit `stuck`, `hover_not_found`, `projection_failed` oder `timeout` scheitert, ist für Phase 4.5 akzeptiert.

## Verhalten

- `--phase travel-marsh` und `--phase travel-cellar5` sind CLI-only und nur mit dem Countess-Run gültig.
- `--phase kill-countess` ist CLI-only, startet nur in `Tower Cellar Level 5` und enthält keinen Travel-Prefix.
- `--run countess` ohne Phase behält den bisherigen Phase-4.1-Stub bei.
- Der Travel-Flow nutzt Area-IDs für Entscheidungen; Area-Namen dienen nur Logs.
- Input-Schritte laufen nur bei `Valid && Phase=in_game`.
- `wait_black_marsh` darf für beide Travel-Phasen als Non-Input-Step während Loading/invalid Snapshots weitergetickt werden.
- `travel-cellar5` kann für manuelle Tests aus `Black Marsh`, `Forgotten Tower` oder einem Tower-Cellar-Level fortgesetzt werden und springt dann direkt zum passenden Traversal-Step.

## Waypoint-Interaktion

Es wird kein D2R-Waypoint-Hotkey vorausgesetzt. Der Ablauf folgt dem Koolo-Modell:

1. `acquire_town_waypoint`: Wenn der Waypoint noch nicht enumeriert ist, läuft `pathing.TownWalker` per Force Move (`e`) entlang einer Act-1-Preset-Route Richtung nordöstlichem Waypoint.
2. `open_waypoint`: `pathing.WaypointActions.TickTownWaypoint` klickt den nächsten `ObjectKindWaypoint` erst nach Hover-Bestätigung.
3. `select_black_marsh`: nach kurzer Menü-Settle-Zeit klickt `SelectBlackMarsh` die konfigurierte UI-Position.
4. `wait_black_marsh`: Erfolg bei `world.BlackMarsh`, sonst Timeout über `runs.step_timeout_ms`.

Die Default-UI-Koordinate ist für 1280x720 windowed kalibriert:

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

Bei abweichender Fenstergröße loggt der Bot eine separate Warnung für fixe Waypoint-UI-Koordinaten.

## Tower-Traversal

`travel-cellar5` hängt nach `wait_black_marsh` folgende Steps an:

| Step | Zielgebiet | Entrance |
|------|------------|----------|
| `find_tower` | `world.ForgottenTower` | `world.EntranceKindWildernessToTower` |
| `enter_cellar_1` | `world.TowerCellarLevel1` | unbekannte Vorraum-Entrance im `Forgotten Tower` |
| `enter_cellar_2` | `world.TowerCellarLevel2` | `world.EntranceKindTowerCellarDown` |
| `enter_cellar_3` | `world.TowerCellarLevel3` | `world.EntranceKindTowerCellarDown` |
| `enter_cellar_4` | `world.TowerCellarLevel4` | `world.EntranceKindTowerCellarDown` |
| `enter_cellar_5` | `world.TowerCellarLevel5` | `world.EntranceKindTowerCellarDown` |

Jeder Step startet ein `pathing.GoalKindMoveToArea` genau einmal und tickt danach den `pathing.Navigator`.
Wenn der aktuelle Snapshot bereits im Zielgebiet ist, schließt der Step ohne neue Eingabe ab.

Die Traversierung ist bewusst best-effort: Der aktuelle Explorer nutzt Bearing-Explore, nähert sichtbare passende Entrances an und übergibt nahe Entrances an den Hover-Feedback-Klick. Wenn ein direkter Annäherungs-Teleport auf eine sichtbare Entrance-Unit blockiert, versucht der Navigator als Fallback den Hover-Klick auf genau diese Unit ohne Distanz-Gate. Er merkt sich keine besuchten Räume, folgt keinen Wänden und nutzt noch keine `Left`-Map-Reading-Heuristik. Zufällige Tower-Layouts können deshalb weiterhin mit `stuck`, `hover_not_found`, `projection_failed` oder `timeout` scheitern.

Das ist kein offener 4.5-Blocker mehr. Der geplante robuste Weg ist eine spätere Recording-/Playback-Funktion: Der Spieler zeichnet eine vollständige Strecke einmal manuell auf, der Bot spielt diese Route deterministisch ab und nutzt den Navigator nur noch für kurze lokale Korrekturen, Hover-Checks und Area-Wechsel.

Wenn eine passende Entrance-Unit sichtbar ist, aber noch außerhalb der Klickdistanz liegt, priorisiert der Navigator sie als `entity_approach` und teleportiert auf sie zu. Verlässt `find_tower` versehentlich `Black Marsh` in ein anderes Gebiet, bricht der Step mit `unexpected_area` ab, statt im falschen Gebiet weiter zu explorieren.
Für `enter_cellar_1` gibt es einen engen Sonderfall: Der `Forgotten Tower`-Vorraum ist stabil, aber die sichtbare Durchbruch-Unit wird nicht von den bekannten Tower-/Stair-IDs abgedeckt. Die Probe enumeriert deshalb auch unbekannte Entrance-Units. Der Step bevorzugt im `Forgotten Tower` für `Tower Cellar Level 1` eine unbekannte Entrance-Unit und ignoriert die bekannten Back-/Surface-Entrances. Erfolg bleibt ausschließlich der Area-Wechsel nach `Tower Cellar Level 1`. Ab `Tower Cellar Level 1` nutzt der Bot die beobachteten Tower-Cellar-IDs `8` (up/source) und `9` (down/next level), damit sichtbare Down-Exits priorisiert werden.

## Countess-Kill (Phase 4.6)

`kill-countess` ist eine getrennte Cellar-5-Phase:

| Step | Verhalten |
|------|-----------|
| `precheck` | verlangt `Valid`, `Phase=in_game`, `Tower Cellar Level 5` und verdrahtete Combat-Actions |
| `locate_countess` | sucht zuerst `DarkStalker` als Super-Unique, danach irgendein Super-Unique nur in Cellar 5 |
| `engage_countess` | hält das gespeicherte Target per `UnitID`, repositioniert bei Distanz und castet `Bone Spear` |

Wenn Countess noch nicht sichtbar ist, startet der Bot genau einmal ein `GoalKindMoveToPosition` auf einen Suchanker: zuerst die `Good Chest`, sonst die sichtbare `tower_cellar_down`-Entrance in Cellar 5. Er klickt weder Chest noch Entrance an, führt keinen Hover-Klick aus und hebt keinen Loot auf. Nach Ankunft wartet der Step bis zum Timeout, falls Countess weiter unsichtbar bleibt.

Der MVP-Kampf ist fest auf `necro_bone_spear` begrenzt. Der Task ruft Combat pro Tick auf; der App-Adapter drosselt echte Casts über `runs.countess.combat.attack_interval_ms`. Bei Abstand größer `reposition_distance_tiles` teleportiert der Bot in Richtung Countess, mit `engage_distance_tiles` als gewünschtem Restabstand. Sonst castet er den aufgelösten Skill `bone_spear`.

Kill-Erfolg wird defensiv gezählt: Erst wenn eine gespeicherte Countess-`UnitID` in gültigen Cellar-5-Snapshots `kill_confirm_ticks` mal in Folge nicht mehr als living monster erscheint, endet die Phase erfolgreich. Ungültige Snapshots, Loading und Area-Wechsel zählen nicht als Tod; ein Area-Wechsel aus Cellar 5 während des Kampfes schlägt mit `unexpected_area` fehl.

```yaml
runs:
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
- D2R windowed 1280x720 oder kalibrierte `pathing.waypoint_ui`-Koordinaten
- Charakter steht in `Rogue Encampment`, entweder nahe Waypoint oder am Spawn-/Stash-Bereich
- Force Move ist in D2R auf `pathing.town_walk.force_move_key` gebunden (Default `e`)
- Teleport ist in `input.bindings.skills.teleport` konfiguriert
- Für `kill-countess`: `input.bindings.skills.bone_spear` ist konfiguriert
- Black-Marsh-Waypoint ist für Charakter und Schwierigkeit freigeschaltet

## Manuelle Validierung

```powershell
go run ./cmd/d2rbot --pathing-test play-town-route:act1-waypoint --probe --verbose
go run ./cmd/d2rbot --pathing-test click-entity:entrance --probe --verbose
go run ./cmd/d2rbot --pathing-test inspect:entrances --probe --verbose --pathing-test-timeout-ms 30000
go run ./cmd/d2rbot --run countess --phase travel-marsh --probe --verbose
go run ./cmd/d2rbot --run countess --phase travel-cellar5 --probe --verbose
go run ./cmd/d2rbot --run countess --phase kill-countess --probe --verbose
```

`click-entity:entrance` prüft nur Hover-/Klickmechanik auf der nächsten Entrance-Unit. Für die Tower-Annahmen müssen die verbose World-Logs zusätzlich zeigen, dass nahe Tower/Stairs die erwarteten Entrance-Kinds und Namen erscheinen.
`inspect:entrances` bewegt oder klickt nicht. Der Operator kann den Charakter manuell im `Forgotten Tower`-Vorraum oder an einem Durchgang positionieren; der Bot loggt Spielerposition, Hover-State und alle sichtbaren Entrance-IDs mit Unit-ID, Kind, Position, Delta und Distanz.
Jeder CLI-Lauf schreibt denselben Logstream zusätzlich in eine timestamped Datei unter `logs/`, damit verbose Testläufe vollständig nachträglich auswertbar bleiben.

Falls die Preset-Route in Town nicht zum lokalen Setup passt, kann eine Override-Route aufgezeichnet werden:

```powershell
go run ./cmd/d2rbot --pathing-test record-town-route:act1-waypoint --probe --verbose
```

## Grenzen

- Town-Walk ist nur für Rogue Encampment / Act 1 vorgesehen.
- Keine OCR-/Bild-Erkennung des Waypoint-Menüs.
- Kein Pickit und kein Loot-Aufheben.
- `kill-countess` nutzt keine Curses, Summons, Bone Prison, Potion-Logik oder Good-Chest-Interaktion.
- Kein robuster Tower-Solver: zufällige Tower-Layouts bleiben die größte Unsicherheit in Phase 4.5. Das ist bewusst als MVP-Grenze akzeptiert; spätere Run-Recording-/Playback-Routen sollen diesen Teil zuverlässig machen.

---
*Zuletzt aktualisiert: 2026-07-03*
