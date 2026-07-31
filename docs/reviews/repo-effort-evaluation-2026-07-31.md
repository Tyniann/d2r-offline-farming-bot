# Repo Effort Evaluation

Holistische Bewertung des Repositorys zum Stand **2026-07-31** (HEAD nahe Release **v0.16.0** / Phase 18). Kontext: **100 % des Codes ist AI-generiert**.

**Canvas (visuelle Fassung):** [repo-effort-review.canvas.tsx](repo-effort-review.canvas.tsx) — archivierter Snapshot im Repo. Live-Ansicht in Cursor (neben dem Chat): Projektdatei unter den Cursor-Canvases, Dateiname `repo-effort-review.canvas.tsx`.

---

## Überblick

Das Repo ist kein Scaffold und kein „Prompt-Dump“. Es wirkt wie ein bewusst phasenweise gebautes Offline-Farming-Produkt mit klaren Paketgrenzen, fail-closed-Verträgen, Operator-Tooling und einem für AI-Code ungewöhnlich hohen Docs-/Test-Anteil.

| Signal | Wert (Stand Review) |
|--------|---------------------|
| Kalender | ~5 Wochen (2026-06-25 → 2026-07-31) |
| Commits | 62 |
| Go Prod-LOC | ~54 000 (265 Dateien) |
| Go Test-Dateien | 194 (~35 000 Zeilen) |
| Feature-Docs | 72 |
| Web/Electron TS | ~7 400 LOC |
| Vitest + e2e | 15 Vitest-Dateien + Playwright Electron |
| Live-Runs | Countess, Mephisto, Summoner, Nihlathak |
| Gewichtete Qualität | **~8,1 / 10** |

**Urteil**

- **Absoluter Aufwand:** hoch — grob vergleichbar mit **3–6 Engineer-Monaten** sorgfältiger menschlicher Arbeit (Memory-Bot + UI + Ops), plus Domain-Recherche und Live-Validierung.
- **Aufwand / Kalender:** extrem — nur plausibel mit starker menschlicher Direktion und fester Architektur-Runway.
- **AI-Code-Qualitätsleiste:** **8 / 10** — näher an einem frühen internen Produktwerkzeug als an einem Research-Prototyp; Hygiene-Schulden bleiben.

---

## Bewertungskriterien

Eigene Kriterien (0–10). Scores sind Reviewer-Urteil aus statischer Evidenz (Code, Docs, Git-Historie), kein `go test -cover` und kein Live-Playtest in diesem Pass.

| Kriterium | Score | Urteil |
|-----------|------:|--------|
| Structure / architecture | **9** | Strikter Fluss `process → memory → world → tasks/profile → input`; pathing/loot/town am World Model; API/UI als Operator-Schale. Für AI-Repos selten so sauber. |
| Feature completeness | **8,5** | Voller Offline-Stack: 4 Runs, Town, Pickit, Telemetrie, Electron-UI, Mercenary. Fehlend: Baal/Cows; Combat-Tiefe vor allem Necro. |
| Documentation | **9** | 72 Feature-Docs, Keep a Changelog, `handoff.html`, 13 Phase-Pläne. Ausnahmen: Package-Godoc-Lücken, README-/Versions-Drift. |
| Code quality | **8** | Fail-closed-Verträge, interface-mockbare Windows-APIs, gemeinsame Run-Pipeline, Safety-Hotkeys, hover-bestätigte Aktionen. |
| Code readability | **7,5** | Klare Domain-Typen und Reason-Codes; `internal/app` (~19k LOC) ist Dichte-Hotspot; etwas AI-Verbosität. |
| Test discipline | **8,5** | ~39 % der Go-Zeilen sind Tests; Phase-Contract-/Characterization-Tests; Vitest + Playwright. |
| Release hygiene | **5** | Changelog `0.16.0` vs. `version.go` `0.14.1` vs. README `v0.6.0` vs. Tags bei `v0.14.4`. Produkt ist den Metadaten voraus. |
| Product polish (UI/ops) | **8** | Settings/Queue/Pickit/History, OpenAPI-Client, NSIS-Packaging, deutsche Operator-Strings. |

---

## Methodik / Evidenzbasis

Ausgewertet u. a.:

- Verzeichnisbaum (`cmd/`, `internal/`, `web/`, `configs/`, `docs/`, `tools/`)
- LOC nach Sprache und Paket (ohne `node_modules` / `.tmp`)
- `docs/features/`, `docs/CHANGELOG.md`, `handoff.html`, Phase-Plan-HTMLs
- Paketinventar unter `internal/` inkl. Maturity-Einschätzung
- Stichproben: `internal/world/state.go`, `internal/app/supervisor.go`, Tasks-/Pipeline-Struktur
- Git: `git rev-list --count`, Log 2026-06-25 → 2026-07-31, Tags/`git describe`

---

## Maßstabs-Signale

### Größe

- 14 `internal/`-Pakete; größtes `app` (~19,3k Prod-LOC), danach `api`, `tasks`, `pathing`, `memory`, `telemetry`, `world`
- Config: 15 Top-Level-YAML-Abschnitte, 4 Pickit-Profile, ~47 Route-Dateien, UI-Screen-Anchors
- Operator-Oberfläche dual: CLI (30+ Flags) und Electron-Desktop gegen Loopback-Core-API mit Control-Token

### Prozess

- Phasenlieferung bis Phase 18; Changelog enthält Acceptance-Notizen (z. B. 10/10 Summoner Hell)
- Nahezu keine actionable `TODO`/`FIXME` in Go; Stubs überwiegend `//go:build !windows`
- Agent-Regeln (Architektur, Godoc, Changelog, Safety) werden über die Historie hinweg eingehalten

### Paket-Maturity (Kurz)

| Paket | Prod-LOC | Maturity | Rolle |
|-------|----------|----------|-------|
| `app` | ~19,3k | Solid / dense | Supervisor, Session, Adapter |
| `api` (+ UI-Embed) | ~5,4k | Solid | Loopback-HTTP + OpenAPI |
| `tasks` | ~4,7k | Solid | 4 Run-SMs + gemeinsame Pipeline |
| `pathing` | ~4,1k | Solid | Teleport, Routen, Town-Walk |
| `memory` | ~3,4k | Solid | Snapshots, Offsets, Probes |
| `telemetry` | ~3,2k | Solid | JSONL Run-/Session-History |
| `world` | ~3,2k | Solid | Immutabler Domain-State |
| `town` / `loot` / `input` | ~6,3k | Solid | Prep, Pickit, OS-Input |
| `profile` / `process` / `config` | ~3,1k | Solid | Combat-Policy, Attach, YAML |

---

## Stärken vs. typische AI-Schwachstellen

**Ungewöhnlich stark für AI-Code**

- Echte Schichtenarchitektur statt God-`main`
- Safety first: Input standardmäßig aus, Emergency Stop, fail-closed
- Tests als Phasen-Contracts, nicht nur dekorative Asserts
- Docs weitgehend mit Shipping-Features mitgezogen
- Konsistente Domain-Sprache (Reasons, Stages, Budgets)
- Geschlossener Produktloop: Farm → Town → Loot → Telemetrie → UI

**Typische AI-Fingerabdrücke**

- Versions-/Tag-/README-Drift
- Orchestrierungs-Konzentration in `app`
- Unvollständige Package-Level-Godocs bei mehreren Core-Paketen
- Necro-zentrische Combat-Tiefe; Multi-Class eher strukturell
- Verbosе Characterization-/Phase-Contract-Tests
- Einzelne veraltete Kommentare (z. B. Session „inspect only“)

---

## Canvas

### Zweck

Die Canvas-Datei visualisiert dieselbe Bewertung: Scores als Balkendiagramm, Stats, Tabellen zu Kriterien/Paketen, Stärken/Schwächen und Aufwandsurteil.

### Ablage

| Ort | Rolle |
|-----|--------|
| [`repo-effort-review.canvas.tsx`](repo-effort-review.canvas.tsx) | **Versionierter Snapshot** im Repo (dieses Review-Artefakt) |
| Cursor-Projekt-Canvases (`…/canvases/repo-effort-review.canvas.tsx`) | **Live-Canvas** zum Öffnen neben dem Chat (SDK `cursor/canvas`) |

Die Live-Canvas wird von Cursor nur aus dem verwalteten Canvases-Ordner gerendert. Der Snapshot hier dient der Nachvollziehbarkeit im Git; bei inhaltlichen Updates beide Stellen angleichen.

### Inhalt der Canvas (Abschnitte)

1. Headline + Pills (Maturity, Kalender, LOC, Version)
2. Verdict-Callout
3. Kennzahlen-Grid (Go/Tests/Docs/Runs/Web/Qualität)
4. Kriterien-Scores (BarChart + Tabelle)
5. Scale- und Process-Signale
6. Package-Maturity-Tabelle
7. Stärken vs. AI-Fingerabdrücke
8. Aufwandsurteil (absolut / Kalender / AI-Qualitätsleiste)

---

## Bekannte Drift zum Review-Zeitpunkt

Fakten aus dem Review-Pass — keine Fixes in diesem Dokument:

1. Changelog / Commit: **0.16.0**; `internal/version/version.go` Default: **0.14.1**; letztes Tag oft **v0.14.4**; README noch **v0.6.0**
2. `web/package.json` Version: `0.0.0`
3. Package-Docs fehlen u. a. für `app`, `config`, `input`, `loot`, `memory`, `pathing`, `process`, `world`
4. Baal/Cows nicht im Run-Registry

---

## Verwandte Artefakte

- [`docs/CHANGELOG.md`](../CHANGELOG.md) — Release-Historie bis 0.16.0
- [`docs/features/README.md`](../features/README.md) — Feature-Index
- [`handoff.html`](../../handoff.html) — Produkt- und Architektur-Handoff
- Root: `phase-*-implementation-plan.html` — Phasenpläne

---

*Erstellt: 2026-07-31 · Statische Review-Evidenz, kein Live-Playtest*
