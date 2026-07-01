# Task Runner

## Überblick

Phase 4.1 führt ein testbares Task-Framework ein: konfigurierbare Runs werden als State-Machine im Poll-Loop ausgeführt. Der erste Run ist ein **Countess-Stub** (Precheck → Armed → Complete) ohne echtes Pathing oder Input — zur Validierung von Steps, Timeouts und strukturiertem Logging.

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
| **Zeit-Timeout** (`step_timeout_ms`, Default 30000) | Warte-Steps auf Zustandsänderung (später) |
| **Tick-Zähler** (`ticksInStep`) | Deterministische kurze Steps — z. B. `armed` (2 Ticks) |
| **Sofort-Fail** | Bedingung klar verletzt → sofort `task step failed` |

**Countess-Stub (4.1):**

| Step | Verhalten | Abschluss |
|------|-----------|-----------|
| `precheck` | Town-Check | sofort OK (Town) oder sofort Fail (`not_in_town`) |
| `armed` | kein Input | nach 2 Ticks |
| `complete` | Run beenden | `outcome=success` |

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
*Zuletzt aktualisiert: 2026-06-26*
