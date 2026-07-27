# Task Runner

## Überblick

Der Task Runner führt registrierte Run-Definitionen über eine gemeinsame Pipeline im Poll-Loop aus. `--run <id>` ohne Phase nutzt diese Pipeline vollständig; die generischen CLI-Phasen `travel-entry`, `play-route`, `boss`, `loot-and-return`, `stash-personal` und `town-ready` wählen isolierte Ausschnitte derselben Pipeline. Ein isoliertes `play-route` aus Act 1 beginnt am bestätigten Stash-/Town-Start der layoutgebundenen Kante zum Waypoint; es setzt den Charakter nicht bereits direkt am Waypoint voraus.

## Ort im Code

- **Paket:** `internal/tasks/`
- **Einstieg:** `cmd/d2rbot` mit `--run <id>` oder `runs.active` in der Config
- **App-Integration:** `internal/app/run_tick.go`, `internal/app/run_mode.go`
- **Wichtige Dateien:** `runner.go`, `registry.go`, `run_pipeline.go`, `run_contract.go`, `step.go`, `deps.go`, `result.go`
- **Config:** `configs/config.example.yaml` → Sektion `runs`

## Funktionalität

### Run-Auswahl

- CLI `--run` überschreibt YAML `runs.active`
- Leerer Name = passiver Modus (nur Monitor, wie bisher)
- Bekannte Definitionen: `countess` und `mephisto` aus der typisierten `RunRegistry`. Die Pipeline übergibt das Waypoint-Ziel der Definition an den gemeinsamen registrierten Executor; vorhandene Routen bleiben bis zur Live-Abnahme mit `route_runtime_validation_required` gesperrt.

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
| **Zeit-Timeout** (`step_timeout_ms`, Default 45000) | Warte-Steps auf Zustandsänderung |
| **Tick-Zähler** (`ticksInStep`) | Deterministische kurze Steps, wenn ein Run sie explizit markiert |
| **Sofort-Fail** | Bedingung klar verletzt -> sofort `task step failed` |

**Gemeinsame Full-Run-Pipeline (10.2, produktiv für Countess und Mephisto):**

`precheck -> acquire_town_waypoint -> open_waypoint -> select_run_waypoint -> wait_entry_area -> play_bound_route -> acquire_boss -> engage_boss -> wait_for_drops -> scan_loot -> pick_loot -> cast_town_portal -> enter_town_portal -> wait_origin_town -> open_personal_stash -> stash_items -> close_personal_stash -> prepare_town_handoff -> complete`

Entry-Area, Route-Terminal, Waypoint-Ziel, Boss, Suchanker, geordnete Encounter-Aktionen und Rückkehrakt stammen aus `RunDefinition`; Route, Combat und Loot-Policies aus dem ausgewählten `RunConfig`. `wait_entry_area`, Route-, Boss-, Loot- und Portal-Areagates vergleichen ausschließlich gegen diese Definition.

Der Encounter-Aktionsindex beginnt pro Boss-Pin bei `0`. Jede Aktion erhält getrennte Start-/Abschluss-Telemetrie; erst danach darf regulärer Combat laufen. Pro Poll-Tick wird höchstens eine Action-Input-Gelegenheit konsumiert.

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
- **`Terminal()`** (success/failed): keine weiteren `Tick`-Aufrufe; ein expliziter `--run`-Prozess kehrt danach mit Erfolg beziehungsweise Fehler selbstständig zur Shell zurück
- **Passiver Modus ohne Run:** World-Monitor und Probe laufen weiterhin bis Stop-Hotkey oder Prozesssignal
- **`WasReset()`** (z. B. `process_lost`): blockiert Lazy-Re-Start nach Re-Attach
- **Kein zweiter Run ohne App-Neustart** — weder nach terminal noch nach reset

Bei `process_lost` feste Reihenfolge: `Input.Unbind()` → `Tasks.Reset("process_lost")` → `World.Reset()`. Die zentrale, idempotente Run-Barriere leert dabei Waypoint, Portal, Town-Walk, Stash, Navigator, Route-Player, Combat, Loot, Town, Profil, Boss-Pin und Aktionsindex genau einmal.

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

JSONL-Transitionen `run_step_started`, `run_step_completed` und `run_step_failed` enthalten `definition_id`, `step`, `stage` und `outcome`. Die vollständige Core-Zuordnung ordnet jeden gemeinsamen Countess-/Mephisto-Step genau einer der stabilen Kategorien `travel`, `combat`, `loot` oder `return_town` zu; unbekannte Steps werden nicht ohne Stage persistiert. Encounter-Events ergänzen `action_index`. Nach der bestehenden Memory-bestätigten Kill-Bedingung schreibt die Pipeline genau ein `boss_kill_confirmed` für die gepinnte Unit. Schlägt ein persistierendes Telemetrie-Emit fehl, endet die Pipeline vor dem folgenden Input beziehungsweise Kill-Abschluss mit `telemetry_failed`.

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
*Zuletzt aktualisiert: 2026-07-22*
