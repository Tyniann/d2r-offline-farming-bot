# Project rules

## Project spec
**D2R Offline Farming Bot** — externe Windows-Software für **Offline/Singleplayer** D2R. Ziel: wiederholbare Farming-Runs.

- **Ansatz:** Memory Bot (Prozess lesen → World Model → Tasks → Input). Kein Pixelbot, kein Savegame-Hack, keine Spielstands-Manipulation.
- **Scope:** Nur privater Offline-Einsatz. Kein Battle.net / Online-Modus.
- **Status:** Backward compatibility und Legacy-Verhalten sind **keine** Ziele, außer explizit gewünscht.
- **Referenz:** Produkt- und Architekturdetails in `docs/plans/handoff.html`; Feature-Docs unter `docs/features/`; Changelog in `docs/CHANGELOG.md`.

## Tech stack
- **Language / runtime:** Go 1.26+, Zielplattform **Windows** (`GOOS=windows`).
- **Module:** `github.com/Tyniann/d2r-offline-farming-bot`
- **Repo:** `Tyniann/d2r-offline-farming-bot`. Remote bleibt dieses GitHub-Repository.
- **Config:** YAML unter `configs/` (`config.example.yaml` versionieren; `config.yaml` lokal, gitignored).
- **D2R-IDs:** Alle IDs und statischen Spieldaten stammen ausschließlich aus den lokalen CASC-Extrakten unter `.tmp/d2r-excel`. Eingecheckte Werte müssen über Generator oder Test-Fixture auf Datei und stabilen Zeilenschlüssel zurückführbar sein. Fehlt ein benötigter Extrakt, keine ID raten oder aus Fremdprojekten übernehmen: dem Entwickler die konkret benötigte CASC-Datei sowie nach Möglichkeit Zeilen/Spalten nennen und um Nachreichung bitten.
- **Logging:** `log/slog` (strukturiert). JSONL für Run-Telemetrie.
- **Lint / format:** `golangci-lint`, `goimports`, `gofmt` (siehe `.golangci.yml`, `Makefile`).
- **Race Detector / MSYS2:** Nur bei tatsächlich neuer Nebenläufigkeit: Unter Windows die native **UCRT64**-Toolchain (`/ucrt64/bin/gcc`, Target `x86_64-w64-mingw32`) verwenden, nicht die MSYS-Toolchain unter `/usr/bin`. Aus PowerShell Race-Tests über eine initialisierte UCRT64-Shell starten: `$env:MSYSTEM='UCRT64'; $env:CHERE_INVOKING='1'; C:\msys64\usr\bin\bash.exe -lc 'export PATH=/ucrt64/bin:/c/Program\ Files/Go/bin:/usr/bin; cd /d/CSharpProjekte/D2R-Offline-Farming-Bot; go test -race -p 1 ./...'`. Nur `C:\msys64\ucrt64\bin` an `PATH` anzuhängen reicht auf diesem Host nicht zuverlässig.

## Layout & architecture
Feature-first unter `internal/` und `web/src/features/`. Paket-Grenzen strikt einhalten:

| Paket | Verantwortung |
|-------|----------------|
| `cmd/d2rbot` | `main`, CLI-Flags, Wiring — keine Business-Logik |
| `internal/app` | Orchestrierung, Supervisor/Queue-Lifecycle, Adapter zwischen Fachpaketen |
| `internal/process` | D2R-Prozess finden, Handles, Polling-Takt, Versionsgate |
| `internal/memory` | Rohdaten lesen, Snapshots, binäre Strukturen |
| `internal/world` | Interpretierter Spielzustand (Area, Entities, Items) |
| `internal/pathing` | Navigation, Teleport, Routenaufnahme/-wiedergabe |
| `internal/input` | Tastatur & Maus, Fensterbindung, Safety-Hotkeys |
| `internal/tasks` | Run-State-Machines (Countess, Mephisto, Summoner, Nihlathak, Cow Level) |
| `internal/profile` | Klassen-/Combat-Profile, Encounter-Hooks, Route-Clear |
| `internal/town` | Town-Graph, Vendor/Stash-Dienste, System-Egress |
| `internal/loot` | Pickit, Inventar, Stash |
| `internal/telemetry` | JSONL Run-/Session-Telemetrie und History |
| `internal/replay` | Opt-in Runtime-Traces und headless Replay ohne Memory oder OS-Input |
| `internal/api` | Loopback-HTTP/SSE Core-API für die Desktop-UI |
| `internal/api/ui` | Eingebetteter React-Produktionsbuild |
| `internal/config` | Config-Laden, Logger-Setup |
| `internal/version` | Eingebettete Build-Version und Commit |
| `web/` | Electron-Desktop-App und React-Quellen (`web/src/features/…`) |
| `tools/` | CASC-Katalog-Generatoren und Default-Bundle |
| `configs/` | YAML für Runs, Pickit, Routen, Offsets (`config.example.yaml` versionieren) |
| `docs/` | Feature-Docs, Changelog, Agent-Docs |

**Datenfluss:** `process` → `memory` (Snapshot) → `world` (Model) → `tasks`/`profile` (Entscheidung) → `input` (Aktion). `pathing`, `loot` und `town` hängen am World Model, nicht direkt an Raw Memory. Operator-UI: `web` ↔ `internal/api` ↔ `internal/app`. `telemetry` beobachtet Runs/Sessions, steuert sie nicht. `replay` zeichnet opt-in Runtime-Traces und spielt sie headless gegen die Task-Pipeline ab.

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
3. **Tests:** für Config-Parsing, World-Mapping und Task-Übergänge; Windows-API-Code über Interfaces mockbar halten. Minimale Testanzahl.
4. **Commits:** nur auf ausdrückliche Anfrage; nie `git config` ändern; kein Force-Push auf `master`.
5. **Validierung:** Nach einer abgeschlossenen Änderung nur die kleinsten betroffenen Tests und Builds einmal ausführen. Keine automatische Gesamtsuite. Vollständige Go-/UI-Tests, Lint, Produktbuild und Installer-Smokes nur bei ausdrücklichem Gesamtvalidierungs- oder Release-Auftrag über `scripts/build-release.ps1`.
6. **UI facing strings:** Nur für Bot Benutzer relevante und nützliche Informationen anzeigen. Simple, klare Formulierungen - KEIN Technobabble.
7. **Coding Prinzipien:** Strebe nach KISS und YAGNI Implementierungen.
8. **Replay Tool:** Verwenden um bugs zu finden für die gilt "jeder Snapshot war plausibel, die Reihenfolge der Entscheidungen war falsch". CLI plus `--runtime-trace-capture`

## Lokaler Release-Workflow

Der Auftrag `Release [X.Y.Z|patch|minor|major]` autorisiert den vollständigen Releaseflow; fehlt die Version, nach SemVer aus `Unreleased` ableiten und nur bei echter Mehrdeutigkeit nachfragen. Keine GitHub Actions.

1. Diff, Branch, `origin` und GitHub-Account prüfen; keine unerwarteten oder lokalen Dateien veröffentlichen.
2. Version und Release-Datum in Code, Changelog, README, `docs/plans/handoff.html` und betroffenen Paketmetadaten konsistent aktualisieren; aus dem Changelog eine verständliche GitHub-Beschreibung erstellen.
3. Alle vorgesehenen Release-Änderungen einschließlich Metadaten committen und `scripts/build-release.ps1 -Version X.Y.Z` genau einmal ohne Skip-Schalter auf diesem Commit ausführen.
4. Nur bei grüner Pipeline Installer und SHA-256-Datei gemeinsam als `D2R-Offline-Farming-Bot-X.Y.Z-Windows-x64.zip` packen, Commit pushen, annotierten Tag `vX.Y.Z` pushen, mit `gh release create` samt ZIP veröffentlichen und als latest markieren.
5. Tag, Releasebeschreibung, Assetname und Prüfsummen abschließend verifizieren. Bei Fehler vor Tag/Publish stoppen; nach teilweisem Publish Zustand prüfen und idempotent fortsetzen statt doppelt anzulegen.

# Documentation rules

## Feature-Dokumentation

Diese Regel definiert, wann und wie **Feature-Dokumentation** (Architektur & Verhalten) erstellt wird — ergänzend zu **Godoc** in `Project rules`.

| Ebene | Ort | Inhalt |
|-------|-----|--------|
| **Code** | Godoc an exportierten Go-Symbolen | API, Parameter, Warum/Wann |
| **Feature** | `docs/features/*.md` | Modul-Verhalten, Datenfluss, Config, Grenzen |

Produktkontext und Gesamtarchitektur: `docs/plans/handoff.html`.

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

Windows-APIs, Go-Pakete und lokale CASC-Quellen.

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

#Changelog rules

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

## Agent skills

### Issue tracker

GitHub Issues on `Tyniann/d2r-offline-farming-bot`, via `gh`. See `docs/agents/issue-tracker.md`.

### Triage labels

Default role names: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: root `CONTEXT.md` plus `docs/adr/`. See `docs/agents/domain.md`.
