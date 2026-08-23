# Repo Effort Evaluation

Holistische Bewertung zum Stand **2026-08-23** (HEAD = Tag **v0.24.0** / Phase 23). Folge-Review nach [repo-effort-evaluation-2026-07-31.md](repo-effort-evaluation-2026-07-31.md) (damals v0.16.0 / Phase 18, vier Runs). Dieselben acht Kriterien, dieselbe Art von Evidenz.

Kontext: **100 % des Codes ist AI-generiert**.

---

## Überblick

Das Repo ist weiterhin kein Scaffold. Seit Juli sind die offensichtlichen Hygiene-Löcher geschlossen, und das Produkt ist gewachsen statt nur aufgeräumt: zwei weitere Runs, ein zweites Kampfprofil, i18n, Pickit-Workspace, öffentliches Listing. Die Architektur hat das Wachstum getragen. Die Rechnung sitzt in `internal/app` (~19 k → ~24 k Prod-LOC).

| Signal | 2026-07-31 | 2026-08-23 |
|--------|------------|------------|
| Kalender | ~5 Wochen | 59 Tage (~8,5 Wochen, ab 2026-06-25) |
| Commits | 62 | 114 (+52 nach v0.16.0) |
| Go Prod-LOC | ~54 000 (265 Dateien) | ~66 500 (316 Dateien, `internal/`+`cmd/`) |
| Go Test-Dateien | 194 (~35 000 Zeilen) | 250 (~46 000 Zeilen) |
| Feature-Docs | 72 | 82 |
| Web/Electron TS | ~7 400 LOC | ~10 600 LOC |
| Live-Runs | Countess, Mephisto, Summoner, Nihlathak | plus Kuh-Level, Lower Kurast |
| Kampfprofile | Necro | Necro + Hammerdin |
| Gewichtete Qualität | **~8,1 / 10** | **~8,8 / 10** |

**Urteil**

- **Absoluter Aufwand:** hoch — grob **6–9 Engineer-Monate** sorgfältiger menschlicher Arbeit (Memory-Bot + Desktop + Ops), plus Live-Validierung. Die Juli-Spanne 3–6 Monate galt für v0.16.
- **Aufwand / Kalender:** weiterhin extrem, aber die zweite Hälfte liest sich wie gerichtetes Vertiefen, nicht wie ein zweites Greenfield.
- **AI-Code-Qualitätsleiste:** **8,5 / 10** — näher an einem gepflegten internen Produkt mit öffentlicher Listing-Seite als an einem frühen Werkzeug mit Drift.

---

## Bewertungskriterien

Eigene Kriterien (0–10). Scores sind Reviewer-Urteil aus statischer Evidenz (Code, Docs, Git-Historie), kein `go test -cover` und kein Live-Playtest in diesem Pass.

| Kriterium | Jul | Aug | Urteil |
|-----------|----:|----:|--------|
| Structure / architecture | 9 | **9** | Fluss `process → memory → world → tasks/profile → input` hält. Weiche Lecks: Skill-IDs aus `memory` in `profile`/`pathing`. |
| Feature completeness | 8,5 | **9** | Sechs Runs, zwei Klassen, Replay, `/players N`. Fehlend: Baal; Town Act 2–5 nur Egress. |
| Documentation | 9 | **9** | 82 Feature-Docs, öffentliches README, Screenshots. Package-Godoc fehlt weiter bei acht Core-Paketen. |
| Code quality | 8 | **8,5** | Fail-closed, Hover-Confirm, F11, Input standardmäßig aus. Replay und Pipeline-Dep-Narrowing sind echt. `app` wächst weiter. |
| Code readability | 7,5 | **7,5** | Domain-Typen und Reason-Codes tragen das Repo. `app` ist 106 Dateien / ~24 k LOC. Phasennummern in Kommentaren. |
| Test discipline | 8,5 | **9** | 250 Go-Testdateien, ~41 % der Go-Zeilen. Web-Tests von ~15 Vitest-Dateien auf ~43 plus Playwright. |
| Release hygiene | 5 | **9** | Juli-Drift ist weg. HEAD = Tag `v0.24.0` = `version.go` = Changelog = README = `package.json`. |
| Product polish (UI/ops) | 8 | **9** | DE/EN (1455 Keys), Dashboard-Redesign, Pickit-Workspace, Onboarding, Installer-Sprache, Screenshots. |

Mittelwert der acht Scores: **8,8**. Der Sprung kommt fast ganz von Release-Hygiene und Produktpolitur, plus halbe Punkte bei Features, Qualität und Tests. Lesbarkeit und Architektur-Score sind bewusst unverändert.

---

## Methodik / Evidenzbasis

Ausgewertet u. a.:

- Verzeichnisbaum (`cmd/`, `internal/`, `web/`, `configs/`, `docs/`, `tools/`)
- LOC nach Sprache und Paket (ohne `node_modules` / `.tmp` / `dist`)
- `docs/features/` (82 Markdown-Dateien ohne README), `docs/CHANGELOG.md`, `docs/plans/handoff.html`, Phase-Pläne bis 23
- Paketinventar unter `internal/` (15 Top-Level-Pakete)
- Stichproben: `internal/world/state.go`, `internal/app/supervisor.go`, `internal/tasks` Pipeline/Registry, `internal/profile` Necro/Hammerdin
- Git: 114 Commits, 2026-06-25 → 2026-08-23, `git describe` = `v0.24.0`

---

## Maßstabs-Signale

### Größe

- 15 `internal/`-Pakete; größtes weiter `app` (~24,0 k Prod-LOC / 106 Dateien), danach `tasks` (~8,6 k), `api`, `memory`, `pathing`, `world`
- `tasks` ist der größte relative Zuwachs (~4,7 k → ~8,6 k): Cow-Zweiteilung, Lower-Kurast-Chest-Sweep, Hammerdin-Boss
- Config: 15 Top-Level-YAML-Abschnitte, 5 Pickit-Profile, ~47 Route-Dateien
- Operator dual: CLI und Electron gegen Loopback-Core-API; UI-Nav: Dashboard, Routen, Pickit, Verlauf, Settings

### Prozess

- Phasenlieferung bis Phase 23; Changelog enthält Live-Fixes (Merc-Tod, LK-Blocker, `/players N`, Town-Portal-Retry)
- Nahezu keine actionable `TODO`/`FIXME` in Go; Stubs überwiegend `//go:build !windows`
- Agent-Regeln (Architektur, Godoc, Changelog, Safety) werden weiter eingehalten; Godoc auf Paketebene bleibt die Lücke, die die Regeln eigentlich schließen sollten

### Paket-Maturity (Kurz)

| Paket | Jul LOC | Aug LOC | Maturity | Rolle |
|-------|---------|---------|----------|-------|
| `app` | ~19,3k | ~24,0k | Solid / denser | Supervisor, Session, Adapter |
| `tasks` | ~4,7k | ~8,6k | Solid | 6 Run-SMs + gemeinsame Pipeline |
| `api` | ~5,4k | ~5,7k | Solid | Loopback-HTTP + OpenAPI |
| `memory` | ~3,4k | ~4,6k | Solid | Snapshots, Offsets, Probes |
| `pathing` | ~4,1k | ~4,1k | Solid | Teleport, Routen, Town-Walk |
| `world` | ~3,2k | ~3,4k | Solid | Immutabler Domain-State |
| `telemetry` | ~3,2k | ~3,1k | Solid | JSONL History |
| `replay` | — | ~2,0k | Solid / spät | Opt-in Traces, Headless Replay |
| `town` / `loot` / `input` | ~6,3k | ~6,9k | Solid | Prep, Pickit, OS-Input |
| `profile` | im Mix | ~1,2k | Solid für 2 Klassen | Necro + Hammerdin |

Package-Kommentare vorhanden: `api`, `profile`, `replay`, `tasks`, `telemetry`, `town`, `version`. Fehlend: `app`, `config`, `input`, `loot`, `memory`, `pathing`, `process`, `world`.

---

## Stärken vs. typische AI-Schwachstellen

**Ungewöhnlich stark für AI-Code**

- Schichtenarchitektur statt God-`main`
- Safety first: Input standardmäßig aus, Emergency Stop, Hover-Confirm, fail-closed
- Tests als Phasen-Contracts; Replay für „jeder Snapshot war plausibel, die Reihenfolge war falsch“
- Docs und Changelog reisen mit den Releases
- Geschlossener Produktloop: Farm → Town → Loot → Telemetrie → UI
- Release-Metadaten stimmen endlich mit dem Produkt überein

**Typische AI-Fingerabdrücke**

- `app` als Akkretionsscheibe (106 Dateien, ~24 k LOC)
- Package-Godoc auf dem ursprünglichen Core-Pfad weiter offen
- Phasennummern in Kommentaren und Testdateinamen
- Deutsch/Englisch gemischt in Godoc ohne Sprachregel
- Zwei implementierte Klassen; die Registry wirkt allgemeiner als der Code
- Verbosе Characterization-Tests

Der Juli-Fingerabdruck „Versions-/Tag-/README-Drift“ ist entfallen.

---

## Was der Extra-Monat gekauft hat — und was nicht

Gekauft: Kuh-Level, Lower-Kurast-Supertruhen, Hammerdin inklusive Prebuff, Replay, i18n, Pickit-Workspace, öffentliches README, LICENSE, Dashboard-Redesign, Onboarding.

Nicht gekauft: Baal, dritte Klasse, volle Town-Graphen für Act 2–5, Package-Godoc für den Core-Pfad, eine Verkleinerung von `app`.

---

## Offene Lücken zum Review-Zeitpunkt

Fakten aus diesem Pass — keine Fixes in diesem Dokument:

1. Package-Docs fehlen für `app`, `config`, `input`, `loot`, `memory`, `pathing`, `process`, `world`
2. Baal nicht in der Run-Registry (explizit per Test als unbekannt)
3. `internal/app` weiter der Dichte-Hotspot und seit Juli gewachsen
4. Town Act 2–5: je eine Egress-YAML, kein Hub-Graph wie Act 1

---

## Verwandte Artefakte

- [Juli-Review (v0.16)](repo-effort-evaluation-2026-07-31.md)
- [`docs/CHANGELOG.md`](../CHANGELOG.md) — Release-Historie bis 0.24.0
- [`docs/features/README.md`](../features/README.md) — Feature-Index
- [`docs/plans/handoff.html`](../plans/handoff.html) — Produkt- und Architektur-Handoff
- `docs/plans/phase-*-implementation-plan.html` — Phasenpläne

---

*Erstellt: 2026-08-23 · Statische Review-Evidenz, kein Live-Playtest*
