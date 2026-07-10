# Task Runner

## Überblick

Der Task Runner führt konfigurierbare Runs als State-Machine im Poll-Loop aus. Ab Phase 5.6 ist `--run countess` ohne Phase der vollständige Countess-Run mit Loot-Pickup; die isolierten Phasen `travel-marsh`, `travel-cellar5`, `kill-countess` und `loot-countess` bleiben als Testoberflächen verfügbar.

## Ort im Code

- **Paket:** `internal/tasks/`
- **Einstieg:** `cmd/d2rbot` mit `--run countess` oder `runs.active` in der Config
- **App-Integration:** `internal/app/run_tick.go`, `internal/app/run_mode.go`
- **Wichtige Dateien:** `runner.go`, `registry.go`, `countess.go`, `step.go`, `deps.go`, `result.go`
- **Config:** `configs/config.example.yaml` → Sektion `runs`

## Funktionalität

### Run-Auswahl

- CLI `--run` überschreibt YAML `runs.active`
- Leerer Name = passiver Modus (nur Monitor, wie bisher)
- Bekannte Runs: `tasks.KnownRuns()` / `tasks.IsKnownRun()` — einzige Quelle für Run-Name-Validierung in `app.validateRunMode()`

### Lazy Run-Start

Beim Startup wird nur `task run configured` geloggt (wenn ein Run gesetzt ist). `task run started` erscheint beim **ersten Poll-Tick**, an dem alle Guards in `shouldTickTasks` erfüllt sind:

- konfigurierter Run gesetzt
- `!Terminal()` und `!WasReset()`
- `input.enabled` + `Input.Status().Enabled`
- `world.Valid` und `Input.Bound()`
- nicht pausiert/gestoppt

Nach dem ersten Attach kann der Start um **1–2 Poll-Ticks** verzögert sein (Attach-Tick führt kein `World.Update` aus).

### Step-Modell

Zwei Abschluss-Mechanismen (nicht vermischen):

| Mechanismus | Verwendung |
|-------------|------------|
| **Zeit-Timeout** (`step_timeout_ms`, Default 30000) | Warte-Steps auf Zustandsänderung |
| **Tick-Zähler** (`ticksInStep`) | Deterministische kurze Steps, wenn ein Run sie explizit markiert |
| **Sofort-Fail** | Bedingung klar verletzt -> sofort `task step failed` |

**Full Countess (5.6):**

`precheck -> acquire_town_waypoint -> open_waypoint -> select_black_marsh -> wait_black_marsh -> find_tower -> enter_cellar_1 -> enter_cellar_2 -> enter_cellar_3 -> enter_cellar_4 -> enter_cellar_5 -> locate_countess -> engage_countess -> wait_for_drops -> scan_loot -> pick_loot -> cast_town_portal -> complete`

`wait_black_marsh` darf als Non-Input-Step während Loading/invalid Snapshots weitergetickt werden; alle anderen Input-Schritte laufen nur mit gültigem `in_game`-World-State. `wait_for_drops`, `scan_loot` und `pick_loot` verlangen gültige Cellar-5-Snapshots; ein gültiger Snapshot in einem anderen Gebiet bricht mit `unexpected_area` ab.

### Safety-Potion-Guard

Vor dem normalen `run.onTick` prüft der Runner globale Safety:

- nur bei `Valid`, `Phase=in_game` und `Player.MaxHP > 0`
- `HPPercent() <= 35` castet Belt Slot 4
- `HPPercent() <= 65` castet Belt Slot 1
- Throttle: 1500 ms
- ein erfolgreicher Potion-Cast verbraucht den Poll-Tick; der normale Step läuft erst im nächsten Tick weiter
- fehlende Run-Actions werden ignoriert; fehlschlagende vorhandene Belt-Actions beenden den Run mit `safety_potion_failed`

### Nach Run-Ende und process_lost

- **`ConfiguredRun()`** bleibt gesetzt (CLI/config-Name) und dient weiterhin Logs/Diagnose, auch nach Terminal oder Reset
- **`Terminal()`** (success/failed): keine weiteren `Tick`-Aufrufe; passiver World-Monitor läuft weiter
- **`WasReset()`** (z. B. `process_lost`): blockiert Lazy-Re-Start nach Re-Attach
- **Kein zweiter Run ohne App-Neustart** — weder nach terminal noch nach reset

Bei `process_lost` feste Reihenfolge: `Input.Unbind()` → `Tasks.Reset("process_lost")` → `World.Reset()`.

Pause blockiert Ticks still; Step-Timer und Tick-Zähler frieren ein (kein Tick = kein Fortschritt).

## Datenmodell

- `tasks.RunConfig` — `StepTimeout` aus `runs.step_timeout_ms`
- `tasks.TickResult` — `Active`, `Outcome`, `Step`, `Reason`
- `tasks.RunOutcome` — `idle`, `running`, `success`, `failed`

## Operator / CLI

```powershell
go run ./cmd/d2rbot --run countess --probe   # input.enabled: true erforderlich
```

| Event | Felder |
|-------|--------|
| `task run configured` | `run`, `source` (Startup, nur wenn Run gesetzt) |
| `task run started` | `run` (erster Guard-OK-Tick) |
| `task step started` | `run`, `step` |
| `task step complete` | `run`, `step`, `elapsed_ms` oder `ticks` |
| `task step failed` | `run`, `step`, `reason`, `elapsed_ms` |
| `task run finished` | `run`, `outcome`, `reason` |
| `task run reset` | `run`, `reason` |

`--run` und `--input-test` schließen sich gegenseitig aus. Run erfordert `input.enabled: true`.

## Abhängigkeiten

- `internal/world` — Area/Town-Check für Precheck
- `internal/input`, `internal/pathing` — Deps (Compile-Time-Checks in `deps.go`)
- App-Run-Loop in `internal/app`

## Verwandte Features

- [State Probe](state-probe.md) — World-Update vor Task-Tick
- [Input Controller](input-controller.md) — Safety-Guards für Task-Ticks
- [World Model](world-model.md) — `world.State` / Area-Katalog

---
*Zuletzt aktualisiert: 2026-07-04*
