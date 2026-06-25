# D2R Offline Farming Bot

Go-basierter Bot fuer Diablo II: Resurrected (Offline/Singleplayer). Aktuell nur Projekt-Scaffold ohne Spielsteuerung.

## Voraussetzungen

- Windows (Zielplattform)
- Go 1.26+ ([go.dev/dl](https://go.dev/dl/))
- Optional: `make`, `golangci-lint`, `goimports`

## Schnellstart

```powershell
# Lokale Konfiguration anlegen
Copy-Item configs\config.example.yaml configs\config.yaml

# Abhaengigkeiten laden
go mod tidy

# Scaffold starten
go run ./cmd/d2rbot

# Oder bauen
go build -o bin\d2rbot.exe ./cmd/d2rbot
.\bin\d2rbot.exe
```

## Projektstruktur

```
cmd/d2rbot/          # Einstiegspunkt (main)
internal/
  app/               # Anwendungs-Orchestrierung
  config/            # Konfiguration & Logging
  process/           # Prozesssuche, Handles
  memory/            # Memory Reader, Snapshots
  world/             # Spielzustand, Entities
  pathing/           # Navigation / Teleport
  input/             # Tastatur & Maus
  tasks/             # Run-State-Machines
  loot/              # Pickit, Inventar
configs/             # YAML-Konfiguration
docs/                # Recherche & Architektur-Notizen
```

## Entwicklung

```powershell
# Dev-Tools installieren (einmalig)
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

go test ./...
golangci-lint run ./...
```

Weitere Architektur- und MVP-Details: `handoff.html` und `docs/`.
