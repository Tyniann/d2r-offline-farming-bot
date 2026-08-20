# D2R Offline Farming Bot

Go-basierter Bot für Diablo II: Resurrected (Offline/Singleplayer). **v0.22.0** unterstützt autonome Farming-Runs für Countess, Mephisto, Summoner, Nihlathak, das Cow Level und Lower-Kurast-Supertruhen einschließlich Charakter-Loadouts, Hammerdin, Runtime Replay, Combat, Loot, Sockel-Pickit, Town-Diensten, Mercenary-Support und Desktop-App.

## Voraussetzungen

- Windows (Zielplattform)
- Go 1.26+ ([go.dev/dl](https://go.dev/dl/)) — nur für Entwicklung/Build
- Optional: `make`, `golangci-lint`, `goimports`

## Release (Windows EXE)

```powershell
# Windows-Installer bauen (dist/release/D2R-Offline-Farming-Bot-0.22.0-Setup.exe)
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1 -Version 0.22.0

# Oder über Make
make release
```

Der Installer enthält die Desktop-App, den Go-Core und die produktiven Standardkonfigurationen.

Version prüfen:

```powershell
.\d2rbot.exe --version
```

## Schnellstart (Entwicklung)

```powershell
# Lokale Konfiguration anlegen
Copy-Item configs\config.example.yaml configs\config.yaml

# Abhängigkeiten laden
go mod tidy

# Prozess-Monitor (Default)
go run ./cmd/d2rbot

# Mit World-State-Logging
go run ./cmd/d2rbot --probe

# Positionen auf Debug
go run ./cmd/d2rbot --probe --verbose

# Manueller Input-Test (input.enabled: true in config.yaml erforderlich)
go run ./cmd/d2rbot --input-test "belt:1"
go run ./cmd/d2rbot --input-test "portal"
go run ./cmd/d2rbot --input-test "skill:1"
go run ./cmd/d2rbot --input-test "center-click"
go run ./cmd/d2rbot --input-test "click:640,360"
go run ./cmd/d2rbot --input-test "belt:1,portal,skill:1" --input-test-observe-ms 3000

# Oder bauen
go build -o bin\d2rbot.exe ./cmd/d2rbot
.\bin\d2rbot.exe --probe
```

Optional: Offset-Overrides in `configs/offsets.local.yaml` (von `offsets.example.yaml` kopieren) und in `config.yaml` unter `memory.offsets_file` eintragen.

## Manual Input Test (Phase 3.5)

Expliziter CLI-Testmodus zur Validierung der Input-Primitives im Offline-Spiel. Sendet **echte OS-Eingaben** — nur mit bewusstem `--input-test` und `input.enabled: true` in der lokalen Config verwenden.

```powershell
.\d2rbot.exe --config configs\config.yaml --input-test "belt:1"
.\d2rbot.exe --config configs\config.yaml --input-test "portal"
.\d2rbot.exe --config configs\config.yaml --input-test "skill:1"
.\d2rbot.exe --config configs\config.yaml --input-test "center-click"
```

Aktionen: `belt:N` / `potion:N` (1–4), `portal`, `skill:N` (1–8), `center-click`, `click:X,Y`. Komma trennt kurze Sequenzen. `--input-test-observe-ms` (Default 3000) steuert die World-State-Beobachtung nach den Aktionen.

Der Testmodus wartet auf Prozess, Fensterbindung und gültigen In-Game-World-State, loggt Vor-/Nachher-Zustand (ohne `--probe`), führt die Aktionen aus und beendet sich sauber. Pause-/Stop-Hotkeys aus der Config bleiben aktiv. D2R sollte im Fokus sein — es gibt noch kein Fokus-Management.

Details: [`docs/features/input-controller.md`](docs/features/input-controller.md).

## Release bauen

Windows-Installer lokal erzeugen und bei Bedarf manuell verteilen:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1 -Version 0.22.0
# Ergebnis: dist\release\D2R-Offline-Farming-Bot-0.22.0-Setup.exe
```

Optional Version taggen (nur für Git-Historie):

```powershell
git tag -a v0.22.0 -m "Release v0.22.0: Lower Kurast Supertruhen"
git push origin v0.22.0
```

## Projektstruktur

```
cmd/d2rbot/          # Einstiegspunkt (main, CLI-Flags, Wiring)
internal/
  app/               # Orchestrierung, Supervisor/Queue-Lifecycle, Adapter
  process/           # D2R-Prozesssuche, Handles, Versionsgate
  memory/            # Memory Reader, Snapshots, Offsets
  world/             # Spielzustand (Area, Entities, Items)
  pathing/           # Navigation, Teleport, Routenaufnahme/-wiedergabe
  input/             # Tastatur & Maus, Fensterbindung, Safety-Hotkeys
  tasks/             # Run-State-Machines (Countess, Mephisto, Summoner, Nihlathak, Cow Level)
  profile/           # Klassen-/Combat-Profile, Encounter-Hooks, Route-Clear
  town/              # Town-Graph, Vendor/Stash-Dienste, System-Egress
  loot/              # Pickit, Inventar, Stash
  telemetry/         # JSONL Run-/Session-Telemetrie und History
  replay/            # Runtime-Traces und headless Replay
  api/               # Loopback-HTTP/SSE Core-API für die Desktop-UI
  api/ui/            # Eingebetteter React-Produktionsbuild
  config/            # Konfiguration & Logging
  version/           # Release-Version (Build-Zeit injizierbar)
web/                 # Electron-Desktop-App und React-Quellen
configs/             # YAML-Konfiguration, Pickit, Routen, Offset-Beispiele
tools/               # CASC-Katalog-Generatoren und Default-Bundle
scripts/             # Release-Build
docs/                # Feature-Docs, Changelog, Agent-Docs
```

## Entwicklung

```powershell
# Dev-Tools installieren (einmalig)
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

go test ./...
golangci-lint run ./...
```

## Dokumentation

| Was | Wo |
|-----|-----|
| Produkt & Architektur | [`handoff.html`](handoff.html) |
| Feature-Docs (Index) | [`docs/features/README.md`](docs/features/README.md) |
| Changelog | [`docs/CHANGELOG.md`](docs/CHANGELOG.md) |
