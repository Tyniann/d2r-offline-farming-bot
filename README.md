# D2R Offline Farming Bot

Go-basierter Bot für Diablo II: Resurrected (Offline/Singleplayer). **Phase 1** ist umgesetzt: read-only Prozessbindung, Memory-Reader und optionaler State-Probe (HP/Mana/Area/Position). Keine Spielsteuerung.

## Voraussetzungen

- Windows (Zielplattform)
- Go 1.26+ ([go.dev/dl](https://go.dev/dl/))
- Optional: `make`, `golangci-lint`, `goimports`

## Schnellstart

```powershell
# Lokale Konfiguration anlegen
Copy-Item configs\config.example.yaml configs\config.yaml

# Abhängigkeiten laden
go mod tidy

# Phase 1: Prozess-Monitor (Default)
go run ./cmd/d2rbot

# Mit State-Probe (Memory-Snapshots im Spiel)
go run ./cmd/d2rbot --probe

# Positionen auf Debug
go run ./cmd/d2rbot --probe --verbose

# Oder bauen
go build -o bin\d2rbot.exe ./cmd/d2rbot
.\bin\d2rbot.exe --probe
```

Optional: Offset-Overrides in `configs/offsets.local.yaml` (von `offsets.example.yaml` kopieren) und in `config.yaml` unter `memory.offsets_file` eintragen.

## Projektstruktur

```
cmd/d2rbot/          # Einstiegspunkt (main)
internal/
  app/               # Anwendungs-Orchestrierung, Probe-Loop
  config/            # Konfiguration & Logging
  process/           # Prozesssuche, Handles
  memory/            # Memory Reader, Offsets, State Probe
  world/             # Spielzustand (Phase 2)
  pathing/           # Navigation / Teleport
  input/             # Tastatur & Maus
  tasks/             # Run-State-Machines
  loot/              # Pickit, Inventar
configs/             # YAML-Konfiguration & Offset-Beispiele
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
