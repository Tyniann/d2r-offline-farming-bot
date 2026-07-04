# World Model

## Überblick

Phase 2 legt die semantische Spielzustands-Schicht im Paket `internal/world` an: Domain-Typen (2.1), Snapshot-Mapping (2.2) und kontinuierliches App-Loop-Update (2.3). `memory.Snapshot` wird über `FromSnapshot` in einen immutable `State` pro Tick übersetzt; `Model` hält den letzten State über `Update`, `Current` und `Reset`. Ab Phase 5.1 enthält der State außerdem read-only Items.

## Ort im Code

- **Paket:** `internal/world/`
- **Einstieg:** `cmd/d2rbot` → `world.NewModel()` (hält aktuellen `State` nach `Update`)
- **Wichtige Dateien:**
  - `area.go` — `AreaID`, `Area`, `Act`, `AreaKind`, `LookupArea`, Klassifikationsmethoden
  - `areas_data.go` — eingebetteter Area-Katalog (Namen 1..136, Konstanten bis 141), manuell aus d2go kopiert
  - `position.go` — `Position` (X/Y als `uint32`)
  - `player.go` — `Player` mit Vitalwerten und Prozent-Hilfen
  - `state.go` — `GamePhase`, immutable `State` inkl. Entity- und Item-Slices
  - `item.go` — `Item`, `ItemQuality`, `ItemLocation`, Code-/Name-Lookup und Item-Queries
  - `object_ids.go`, `entrance_ids.go`, `npc_ids.go` — Countess-Kataloge
  - `entity.go` — Query-Helfer (`NearestObject`, `FindSuperUnique`, …)
  - `mapper.go` — `FromSnapshot`, `mapPhase`
  - `world.go` — `Model` mit `slices.Clone` in `Update`/`Current`
- **Config:** keine (reine Domain- und Mapping-Schicht)

## Funktionalität

### Area-Katalog

| Aspekt | Verhalten |
|--------|-----------|
| Quelle | d2go `pkg/data/area` @ Commit `16d248a53591`, manuell eingebettet |
| Namen | IDs `1..136`; ID `0` und `137..141` ohne Katalog-Name → Unknown-Fallback |
| `Act` | Immer via `AreaID.Act()` berechnet, nie in der Tabelle gespeichert |
| ID `0` | `ActUnknown` (Verbesserung gegenüber d2go, das Act 1 liefert) |
| Towns | `IsTown()` über ID-Switch (5 Städte); `Kind` bleibt `AreaKindUnknown` |
| Countess-Klassifikation | Outdoor: Blood Moor … Black Marsh; Special: Forgotten Tower; Dungeon: Tower Cellar 1–5 |

### Lookup-API

```go
area := world.LookupArea(world.BlackMarsh)
area.Name      // "Black Marsh"
area.Act       // Act1
area.Kind      // AreaKindOutdoor
area.IsTown()  // false
area.IsDungeon() // false
```

Unbekannte IDs: `Name = "Unknown Area <id>"`, `Kind = AreaKindUnknown`, `Act` aus `AreaID.Act()` (außer ID 0).

### Snapshot → State Mapper (`FromSnapshot`)

| Bedingung | Ergebnis |
|-----------|----------|
| `snap.Valid == true` | `Valid=true`, `Area=LookupArea(...)`, Player-Position und Vitals aus Snapshot |
| `snap.Valid == false` | `Valid=false`, `Reason=snap.Reason`, `Area`/`Player` Zero-Values |
| `snap.Phase` | Immer via `mapPhase(snap.Phase)` — auch bei `!snap.Valid` (z. B. `menu`, `loading`) |
| Entity-Slices | Aus Snapshot gemappt; leere non-nil Slices wenn nicht enumeriert |

**Phase:** `memory.GamePhase` wird ausschließlich über `mapPhase()` konvertiert. `Valid` und `Phase` sind orthogonal (Loading kann bei lesbarem Player auftreten).

### Entities (Phase 4.2)

Countess-minimale Enumeration in `memory.Snapshot`; Mapping nach `world.Object`, `world.Entrance`, `world.Monster` mit Kind und Name aus `*_ids.go`.

| Kategorie | Query-Helfer |
|-----------|--------------|
| Objects | `State.NearestObject(kind)` |
| Entrances | `State.NearestEntrance(kind)` |
| Monsters | `State.FindSuperUnique(npcID)` — `MonsterTypeFlag == 10`; `npcID == 0` = jede Super-Unique (Countess) |
| Items | `State.GroundItems()`, `State.ItemsByLocation(...)`, `State.FindItemByUnitID(...)` |

Allowlists in `internal/memory/countess_filter.go` (sync mit `world/*_ids.go` via `TestCountessFilterMatchesWorldIDs`).

### Model (`Update` / `Current` / `Reset`)

```go
state := model.Update(snap) // speichert geklonten State und gibt unabhängige Kopie zurück
copy := model.Current()     // slices.Clone auf Objects/Entrances/Monsters
reset := model.Reset(at, "process_lost")
```

- Entity-Slices werden in `Update`/`Current` per `slices.Clone` kopiert — Mutationen an Rückgabewerten ändern nicht den gespeicherten State.
- World-Log nutzt **Entity-Fingerprint** (Counts + sortierte `(kind, unitID)`-Paare), nicht Positionsvergleich.

### Player & State

- `Player.HPPercent()` / `ManaPercent()`: Integer-Math, `0` bei `Max == 0`, Clamp auf `100`.
- `State`: Value-Type; bei `Valid == false` ist `Reason` gesetzt (oder leer), `Area`/`Player` sind Zero-Values und nicht zu lesen.

## Datenmodell

| Typ | Rolle |
|-----|-------|
| `AreaID` | `uint32`, passend zu `memory.Snapshot.AreaID` |
| `Area` | Aufgelöste Area mit Name, Act, Kind |
| `Act` | `ActUnknown`, `Act1` … `Act5` |
| `AreaKind` | `Unknown`, `Outdoor`, `Dungeon`, `Special` |
| `Position` | Rohe Tile-Koordinaten |
| `Player` | Position + HP/Mana |
| `GamePhase` | `Unknown`, `Menu`, `Loading`, `InGame` — aus `memory.Snapshot.Phase` |
| `State` | Tick-Snapshot mit `At`, `Phase`, `Valid`, `Reason`, `Area`, `Player`, Entity- und Item-Slices |
| `Object`/`Entrance`/`Monster` | Countess-relevante Entities mit Kind, ID, UnitID, Position, Name |
| `Item` | Read-only Item mit UnitID, Code/Name, Qualität, Location, Position, Flags und Raw-Stats |

## Operator / CLI

Ab **2.3** hält `Model` den laufenden World-State; semantische Logs erscheinen mit `--probe` (siehe [State Probe](state-probe.md)). Ohne `--probe` werden Snapshots gelesen und gemappt, aber nicht geloggt.

## Validierung (Phase 2.4)

Phase 2.4 ergänzt keine neuen Runtime-Features. Sie auditet die bestehenden Unit-Tests, führt die automatisierte Validierung aus und dokumentiert die manuelle Countess-Route für den Operator.

### Automatisierter Test-Audit

Stand **2026-06-25**: alle Plan-Kriterien sind durch bestehende Tests abgedeckt; es wurden keine zusätzlichen Tests benötigt.

| Kriterium | Status | Abdeckung |
|-----------|--------|-----------|
| Town-Erkennung (`Rogue Encampment`) | PASS | `area_test.go`: `TestLookupAreaRogueEncampment`, `TestAreaIDIsTown` |
| Countess-Areas (`Black Marsh`, `Forgotten Tower`, Tower Cellars) | PASS | `TestLookupAreaBlackMarsh`, `TestLookupAreaForgottenTower`, `TestLookupAreaTowerCellarLevels`, `TestAreaCatalogComplete` |
| Unknown-Fallbacks (ID 0, 9999, 137–141) | PASS | `TestLookupAreaZero`, `TestLookupAreaUnknownID`, `TestLookupAreaConstantsWithoutNames` |
| Invalid Snapshot / Mapper | PASS | `mapper_test.go`: `TestFromSnapshotInvalid`, `TestFromSnapshotInvalidNotInGameStaysUnknownPhase`, `TestFromSnapshotValidAreaIDZero`, `TestFromSnapshotEffectiveMaxValues`, `TestModelReset` |
| App-Loop ohne `--probe` (Snapshot + World-Update) | PASS | `app_test.go`: `TestRunTickWithoutProbeUpdatesWorld` |
| World-Log-Policy (`lastLogged`, At ignorieren, verbose position-only) | PASS | `world_log_test.go`, `TestRunTickPositionOnlyKeepsLastLoggedWithoutVerbose` |
| `process_lost`-Reset auch ohne `--probe` | PASS | `TestRunTickLostResetsWorldState`, `TestRunTickLostResetsWorldWithoutProbe` |

Automatisierte Ausführung:

```powershell
go test ./internal/world
go test ./internal/app
go test ./...
go build ./cmd/d2rbot
```

Ergebnis: alle Pakete `ok`, Build erfolgreich.

### Manuelle Validierung (Countess-Route)

Voraussetzungen:

- D2R Offline/Singleplayer, Prozess `D2R.exe`.
- Bot mit passenden Rechten (bei D2R als Admin auch Terminal/Bot als Admin).
- Startup-Log `offset configuration` plausibel; Phase-1-Baseline: D2R `3.2.92777`.
- Charakter mit Black-Marsh-Waypoint oder kurzem Laufweg.
- Schaden/Mana-Verbrauch provozierbar für HP/Mana-Änderungen.

**Session A — World-Logs (`--probe`):**

```powershell
go run ./cmd/d2rbot --probe
```

1. D2R starten → `process attached`.
2. Ersten `world state` im **nächsten** Poll-Tick erwarten (nicht im Attach-Tick).
3. `Rogue Encampment`: `area_name=Rogue Encampment`, plausibles `hp_pct`, `mana_pct`, `pos_x`, `pos_y`.
4. `Black Marsh`: Area-Wechsel mit `area_name=Black Marsh`.
5. `Forgotten Tower`: `area_name=Forgotten Tower` (Special, nicht Dungeon).
6. Optional: `Tower Cellar Level 1` … `Level 5` — semantische Area-Namen bei Levelwechsel.
7. Kurzzeitiges `Unknown Area 0` bei Übergängen ist kein Fehler, solange stabile Ziel-Areas danach korrekt erscheinen.
8. D2R beenden → `world unavailable reason=process_lost`; Re-Attach → forced Log.

**Session B — Verbose Positionsdiagnose:**

```powershell
go run ./cmd/d2rbot --probe --verbose
```

- Positionsänderungen nur auf Debug; bei `poll_interval_ms=100` bis ca. 10 Debug-Zeilen/s während Bewegung (erwartet).

**Session C — Default ohne Operator-World-Logs:**

```powershell
go run ./cmd/d2rbot
```

- Nur Prozess-Logs, kein `world state`. World-State wird intern aktualisiert (durch Unit-Tests abgesichert). `process_lost` ohne Operator-Log.

**PASS-Kriterien:**

- Area-Namen entlang Town → Black Marsh → Forgotten Tower → Tower Cellar korrekt oder nachvollziehbarer Unknown-Fallback.
- HP/Mana-Prozente plausibel und ändern sich bei Verbrauch/Schaden.
- Kein position-only Info-Spam ohne `--verbose`.
- Keine Panics, kein Snapshot vor Attach.

**PASS mit Einschränkung:** kurzes `Unknown Area 0` bei Übergängen; Route nur bis Forgotten Tower statt Cellar Level 5.

### Manuelles Ergebnis

| Aspekt | Stand | Ergebnis |
|--------|-------|----------|
| Automatisierter Test-Audit | 2026-06-25 | PASS — alle Plan-Kriterien abgedeckt |
| `go test ./...` / `go build ./cmd/d2rbot` | 2026-06-25 | PASS |
| Live Countess-Route (Session A, `--probe`) | 2026-06-25, D2R `3.2.92777` | PASS mit Einschränkung — `Rogue Encampment` → `Black Marsh` → `Forgotten Tower` → `Tower Cellar Level 1`; Area-Namen, Kinds, HP/Mana-Prozente und `process_lost` plausibel |
| Verbose Positionsdiagnose (Session B, `--probe --verbose`) | 2026-06-25, D2R `3.2.92777` | PASS — reine Positionsänderungen wurden als Debug-`world state` mit wechselnden `pos_x`/`pos_y` geloggt |
| Default ohne Operator-World-Logs (Session C) | 2026-06-25, D2R `3.2.92777` | PASS — Prozess-Attach und Offset-Scan ohne `world state`-Operatorlogs |

Beobachtung Session A: Der erste `world state` kam nach `process attached` im nächsten Poll-Tick; `Black Marsh` wurde als Outdoor, `Forgotten Tower` als Special und `Tower Cellar Level 1` als Dungeon erkannt. Mana-Verbrauch und Regeneration wurden plausibel geloggt, HP blieb stabil bei `100%`. Beim Beenden von D2R wurde `world unavailable reason=process_lost` ausgegeben.

Beobachtung Session B/C: Im Verbose-Lauf kamen Positionsänderungen in `Rogue Encampment` auf Debug-Ebene ungefähr im Poll-Takt, ohne semantische State-Änderung als Info-Log zu duplizieren. Im Default-Lauf (`probe_enabled=false`) wurden Prozess- und Memory-Initialisierung geloggt, aber keine `world state`-Operatorlogs ausgegeben.

Phase 2.4 ist abgeschlossen. Die Live-Route ist bis `Tower Cellar Level 1` bestätigt; tiefere Tower-Cellars bleiben optional, weil die Mindeststrecke bis `Forgotten Tower` und ein Cellar-Übergang erfolgreich validiert wurden.

Low-Level Memory/Offset-Validierung: [State Probe](state-probe.md) (Phase 1, D2R `3.2.92777`).

## Abhängigkeiten

- Keine Runtime-Dependency auf d2go
- `mapper.go` und `world.go` importieren `internal/memory`; Domain-Dateien (`area.go`, `state.go`, …) bleiben memory-frei

## Grenzen

- Item-Live-Validierung in Phase 5.1 ist auf positionierte Ground-Drops begrenzt; Nicht-Ground-Locations sind vorbereitet, aber noch kein Pass-Kriterium
- `Model` ohne Concurrency-Schutz (single-threaded Run-Loop)
- Shutdown/Detach setzt den World-State in 2.3 nicht zurück (nur `process lost`)

## Verwandte Features

- [State Probe](state-probe.md) — liefert `memory.Snapshot` inkl. Phase und Entities
- [Memory Reader](memory-reader.md) — Low-Level-Reads unter der Probe
- [Task Runner](task-runner.md) — Task-Ticks blockiert bei `Phase != InGame`

---
*Zuletzt aktualisiert: 2026-06-26*
