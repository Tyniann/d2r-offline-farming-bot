# D2R Offline Farming Bot

Go-basierter Bot für Diablo II: Resurrected (Offline/Singleplayer). **v0.2.0** umfasst Phase 1 (read-only Prozessbindung, Memory-Reader, State-Probe) und Phase 2 (World Model mit Area/Player-State). Keine Spielsteuerung — Phase 3 (Input) folgt.

## Voraussetzungen

- Windows (Zielplattform)
- Go 1.26+ ([go.dev/dl](https://go.dev/dl/)) — nur für Entwicklung/Build
- Optional: `make`, `golangci-lint`, `goimports`

## Release (Windows EXE)

```powershell
# Release-ZIP bauen (dist/d2rbot-v0.2.0-windows-amd64.zip)
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1

# Oder über Make
make release
```

Das ZIP enthält `d2rbot.exe`, Config-Beispiele und `INSTALL.txt`. Entpacken, `configs\config.example.yaml` nach `configs\config.yaml` kopieren, `d2rbot.exe` starten.

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

# Oder bauen
go build -o bin\d2rbot.exe ./cmd/d2rbot
.\bin\d2rbot.exe --probe
```

Optional: Offset-Overrides in `configs/offsets.local.yaml` (von `offsets.example.yaml` kopieren) und in `config.yaml` unter `memory.offsets_file` eintragen.

## Release bauen

Release-ZIP lokal erzeugen und bei Bedarf manuell verteilen:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1
# Ergebnis: dist\d2rbot-v0.2.0-windows-amd64.zip
```

Optional Version taggen (nur für Git-Historie):

```powershell
git tag v0.2.0
git push origin v0.2.0
```

## Projektstruktur

```
cmd/d2rbot/          # Einstiegspunkt (main)
internal/
  app/               # Anwendungs-Orchestrierung, World-Log-Loop
  config/            # Konfiguration & Logging
  process/           # Prozesssuche, Handles
  memory/            # Memory Reader, Offsets, State Probe
  world/             # Spielzustand (Area, Player, State)
  version/           # Release-Version (Build-Zeit injizierbar)
  pathing/           # Navigation / Teleport
  input/             # Tastatur & Maus
  tasks/             # Run-State-Machines
  loot/              # Pickit, Inventar
configs/             # YAML-Konfiguration & Offset-Beispiele
scripts/             # Release-Build
docs/                # Feature-Docs & Changelog
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
