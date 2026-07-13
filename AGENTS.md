# .cursor/rules/00-d2rbot-project.mdc

---
description: Core project context and conventions for the D2R Offline Farming Bot
alwaysApply: true
---

## Project spec
**D2R Offline Farming Bot** — externe Windows-Software für **Offline/Singleplayer** D2R. Ziel: wiederholbare Farming-Runs (MVP: **Countess**), später optional Mephisto, Cows, Baal. Koolo, d2go und Botty dienen nur als Recherche und Lern Referenzen - KEINE dependency!

- **Ansatz:** Memory Bot (Prozess lesen → World Model → Tasks → Input). Kein Pixelbot, kein Savegame-Hack, keine Spielstands-Manipulation.
- **Scope:** Nur privater Offline-Einsatz. Kein Battle.net / Online-Modus.
- **Status:** Greenfield / Phase 0–1. Backward compatibility und Legacy-Verhalten sind **keine** Ziele, außer explizit gewünscht.
- **Referenz:** Produkt- und Architekturdetails in `handoff.html`; Feature-Docs unter `docs/features/`; Changelog in `docs/CHANGELOG.md`.

## Tech stack
- **Language / runtime:** Go 1.26+, Zielplattform **Windows** (`GOOS=windows`).
- **Module:** `github.com/Tyniann/d2r-offline-farming-bot`
- **Repo:** `Tyniann/d2r-offline-farming-bot` (privat). Git-Remote per SSH: `git@github.com-tyniann:Tyniann/...` — **nicht** auf DHMG-Account oder Standard-`github.com`-Host wechseln.
- **Config:** YAML unter `configs/` (`config.example.yaml` versionieren; `config.yaml` lokal, gitignored).
- **Logging:** `log/slog` (strukturiert). Später optional JSONL für Run-Telemetrie.
- **Lint / format:** `golangci-lint`, `goimports`, `gofmt` (siehe `.golangci.yml`, `Makefile`).
- **Später optional:** TypeScript/React nur für Dashboard/Pickit-Editor — **nicht** als Memory-/Input-Core.

## Layout & architecture
Feature-first unter `internal/`. Paket-Grenzen strikt einhalten:

| Paket | Verantwortung |
|-------|----------------|
| `cmd/d2rbot` | `main`, CLI-Flags, Wiring — keine Business-Logik |
| `internal/app` | Orchestrierung, Lifecycle, Komponenten zusammenführen |
| `internal/process` | D2R-Prozess finden, Handles, Polling-Takt |
| `internal/memory` | Rohdaten lesen, Snapshots, binäre Strukturen |
| `internal/world` | Interpretierter Spielzustand (Area, Entities, Items) |
| `internal/pathing` | Navigation / Teleport |
| `internal/input` | Tastatur & Maus |
| `internal/tasks` | Run-State-Machines (Countess, …) |
| `internal/loot` | Pickit, Inventar, Stash |
| `internal/config` | Config-Laden, Logger-Setup |

**Datenfluss:** `process` → `memory` (Snapshot) → `world` (Model) → `tasks` (Entscheidung) → `input` (Aktion). `pathing` und `loot` hängen am World Model, nicht direkt an Raw Memory.

**MVP-Phasen** (nicht vorauslaufen): Phase 1 = read-only probe → Phase 2 = world model → Phase 3 = input → Phase 4 = Countess ohne Pickit → Phase 5 = loot/recovery loop.

## In-code-Dokumentation

1. **Godoc-Pflicht:** Neue oder geänderte exportierte Go-Symbole sofort dokumentieren. Der Kommentar beginnt mit dem Symbolnamen und endet mit einem Punkt.
2. **Design-Kommentare:** Nicht offensichtliche Invarianten, autoritative Datenquellen, Safety-Gates, Reihenfolgen sowie Retry-, Reset- und Fail-closed-Entscheidungen direkt am relevanten Code erklären.
3. **State-Machines:** Zustände, erlaubte Übergänge, über mehrere Ticks persistierenden Zustand sowie Erfolgs-, Abbruch- und Timeout-Bedingungen benennen.
4. **Wiederverwendung:** Verträge, Seiteneffekte und bewusste Einsatzgrenzen dokumentieren, wenn sie nicht bereits eindeutig aus Typen und API hervorgehen.
5. **Kein Kommentarrauschen:** Warum und wann erklären; nicht wiederholen, was der Code eindeutig ausdrückt. Kommentare am kleinsten hilfreichen Scope platzieren, nicht zeilenweise kommentieren.
6. **Syntax und Pflege:** `[Name]` für verlinkte Go-Symbole und `` `backticks` `` für Werte, Config-Keys und Windows-APIs verwenden. Kommentare bei jeder relevanten Verhaltensänderung aktualisieren oder entfernen.

Dokumentation und UI-facing Strings müssen ordentliches Deutsch und Umlaute verwenden.

## Safety & bot behavior
- **Stop/Pause-Hotkey** und Stuck/Timeout-Recovery früh mitdenken; Tasks müssen abbrechen können.
- Jede **Input-Aktion** loggen (was, warum, Ergebnis).
- Memory-Snapshots als **konsistent** behandeln: bei inkonsistenten Reads lieber erneut lesen als auf fehlerhaftem State handeln.
- **Niemals** D2R-Installations- oder Savegame-Dateien schreiben/verändern.

## Common guidelines
1. **Minimale Diffs** — nur angeforderte Änderungen; keine Drive-by-Refactors.
2. **Fehler:** mit Kontext wrappen (`fmt.Errorf("…: %w", err)`), auf oberster Ebene loggen; nicht still schlucken.
3. **Tests:** für Config-Parsing, World-Mapping und Task-Übergänge; Windows-API-Code über Interfaces mockbar halten.
4. **Commits:** nur auf ausdrückliche Anfrage; nie `git config` ändern; kein Force-Push auf `master`.
5. **Agent:** `go test ./...` und `go build ./cmd/d2rbot` nach relevanten Änderungen ausführen. Keine Spielsteuerung implementieren, solange die aktuelle Phase read-only ist — außer explizit beauftragt.

# .cursor/rules/01-dokumentation.mdc

---
description: Wann und wie Feature-Dokumentation in docs/features/ erstellt wird
alwaysApply: true
---

## Feature-Dokumentation

Diese Regel definiert, wann und wie **Feature-Dokumentation** (Architektur & Verhalten) erstellt wird — ergänzend zu **Godoc** in `00-d2rbot-project.mdc`.

| Ebene | Ort | Inhalt |
|-------|-----|--------|
| **Code** | Godoc an exportierten Go-Symbolen | API, Parameter, Warum/Wann |
| **Feature** | `docs/features/*.md` | Modul-Verhalten, Datenfluss, Config, Grenzen |

Produktkontext und Gesamtarchitektur: `handoff.html`.

### 1. Wann dokumentieren?

**Dokumentation IST erforderlich für:**
- Neue Pakete unter `internal/` (z. B. `memory`, `tasks`, `input`)
- Neue Run-State-Machines oder größere Task-Flows (z. B. Countess-Run)
- Read-only-Probes, Debug-UI oder CLI-Oberflächen mit neuem Verhalten
- Signifikante Änderungen an bestehenden Features
- Neue Integrationen (Windows-APIs, D2R-Prozess, externe Tools)

**Dokumentation ist NICHT erforderlich für:**
- Bugfixes ohne Verhaltensänderung
- Einzelne Memory-Offsets, Struct-Felder oder Config-Keys ohne neues Feature
- Refactoring ohne funktionale Änderung
- Performance-Optimierungen ohne sichtbares Verhalten
- Dependency-Updates

### 2. Ablage & Benennung

Alle Feature-Docs unter `docs/features/`.

**Dateinamen:** kebab-case, passend zum Feature:
- `docs/features/process-detection.md`
- `docs/features/memory-reader.md`
- `docs/features/countess-run.md`

### 3. Template

```markdown
# [Feature-Name]

## Überblick

Kurz: Was macht das Feature und warum existiert es?

## Ort im Code

- **Paket:** `internal/[package]/`
- **Einstieg:** `cmd/d2rbot/main.go` oder Haupttyp (z. B. `*memory.Reader`)
- **Wichtige Dateien:** Liste der relevanten `.go`-Dateien
- **Config:** `configs/…` falls zutreffend

## Funktionalität

### [Teilbereich 1]
Beschreibung.

### [Teilbereich 2]
Beschreibung.

## Datenmodell

Falls zutreffend:
- Go-Structs und deren Rolle
- Memory-Layouts / Snapshot-Felder (ohne fragile Offset-Listen, wenn instabil)
- YAML-Config-Keys und Defaults

## Operator / CLI

- CLI-Flags, Log-Ausgabe, Hotkeys (Stop/Pause)
- Erwartetes Verhalten bei Fehlern und Timeouts

## Abhängigkeiten

Windows-APIs, Go-Pakete, externe Referenzen (z. B. Koolo als Lernquelle).

## Verwandte Features

Links zu anderen `docs/features/*.md`.

---
*Zuletzt aktualisiert: [Datum]*
```

### 4. Wann aktualisieren?

Bestehende Docs aktualisieren bei:
- Neuem Verhalten oder neuen Task-States
- Geändertem Datenfluss (`process` → `memory` → `world` → …)
- Geändertem Config-Schema oder Operator-Interface

### 5. Wann entfernen?

Wenn ein Feature vollständig entfällt oder ersetzt wird:
1. `docs/features/[feature-name].md` löschen
2. Eintrag in `docs/features/README.md` entfernen
3. Eintrag unter `## [Unreleased]` → **Removed** in `docs/CHANGELOG.md`

### 6. Index

Index pflegen: `docs/features/README.md` — alle dokumentierten Features mit Kurzbeschreibung und Link.

### 7. Workflow

Bei neuem Feature, das Dokumentation braucht:

1. Feature implementieren
2. Godoc für neue exportierte Symbole ergänzen
3. `docs/features/[feature-name].md` nach Template anlegen
4. `docs/features/README.md` aktualisieren
5. Eintrag in `docs/CHANGELOG.md` unter **Added**
6. Commits nur auf ausdrückliche Anfrage des Nutzers

# .cursor/rules/02-changelog.mdc

---
description: Pflege von docs/CHANGELOG.md für den D2R Offline Farming Bot
globs:
  - "internal/**"
  - "cmd/**"
  - "configs/**"
  - "docs/CHANGELOG.md"
---

## Changelog

Änderungshistorie für **D2R Offline Farming Bot**. Format: [Keep a Changelog](https://keepachangelog.com/).

**Datei:** `docs/CHANGELOG.md`

### Kategorien (Reihenfolge)

| Kategorie | Verwendung |
|-----------|------------|
| **Added** | Neue Pakete, Features, CLI-Flags, Config-Optionen, Task-Runs |
| **Changed** | Verhalten, Architektur, Defaults, Log-Format |
| **Fixed** | Bugfixes, falsche Memory-Reads, Task-Abbrüche |
| **Removed** | Entfernte Features oder Config-Keys |
| **Security** | Sicherheitsrelevante Änderungen |

### Was eintragen?

**Immer:**
- Neue Features (auch kleinere Bot-Fähigkeiten)
- Bugfixes mit Nutzer-/Operator-Bezug
- Entfernte Features
- Breaking Changes (Config, CLI, Task-Verhalten)

**Optional (wenn bemerkenswert):**
- Performance bei Polling/Snapshots
- Refactoring mit Verhaltensänderung
- Go- oder Tooling-Updates (Major)

**Nicht eintragen:**
- Reines Refactoring ohne sichtbare Änderung
- Code-Style, Godoc-Nachzüge
- Interne Dev-Only-Anpassungen

### Eintragsformat

- Imperativ auf **Englisch** (Keep a Changelog-Konvention): „Add …“, „Fix …“, „Remove …“
- Verständlich ohne Code-Kontext
- Issue/PR-Referenz wenn vorhanden: `(#12)`

**Gut:**
```markdown
### Added
- Add read-only D2R process detection in `internal/process` (#3)

### Fixed
- Fix snapshot retry when area ID reads as zero during load screen
```

**Schlecht:**
```markdown
### Added
- process stuff
- fixed memory
```

### Versionierung

[Semantic Versioning](https://semver.org/): `MAJOR.MINOR.PATCH`

- **MAJOR:** Breaking Changes (Config, CLI, Task-API)
- **MINOR:** Neue Features, neue Runs/Tasks
- **PATCH:** Bugfixes, kleine Verbesserungen

### Workflow

Bei jeder relevanten Änderung:
1. Eintrag unter `## [Unreleased]` in der passenden Kategorie
2. Bei Release: `[Unreleased]` → `[X.Y.Z] - YYYY-MM-DD`, neues leeres `[Unreleased]` oben

Commits und Releases nur auf ausdrückliche Anfrage des Nutzers.
