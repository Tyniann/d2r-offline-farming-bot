# D2R Offline Farming Bot

Windows-Desktop-App, die Diablo II: Resurrected **offline** farmt. Go liest den Prozess, baut ein World Model und steuert Tastatur und Maus. Electron zeigt Queue, Routen, Pickit und Verlauf. Battle.net ist nicht im Scope und nicht implementiert.

Das Repo ist ein persönliches Fallbeispiel: Architektur und Abnahme kommen von mir, der Code ist mit AI geschrieben. Es ist kein Produkt und keine Aufforderung, die Blizzard-EULA zu umgehen.

English: a Windows desktop app (Go core, Electron UI) for repeatable **offline** D2R farming. Personal AI-assisted engineering case study, not an online cheat and not affiliated with Blizzard.

![Dashboard](docs/screenshots/dashboard.png)

![Routenaufzeichnung](docs/screenshots/route-recording.png)

![Pickit](docs/screenshots/pickit.png)

## Was es kann

- Farming-Ziele: Countess, Mephisto, Summoner, Nihlathak, Cow Level, Lower-Kurast-Supertruhen
- Kampfprofile: Necromancer und Hammerdin, inklusive Mercenary
- Selbst aufgezeichnete Routen mit Playback gegen das Memory-World-Model
- Pickit-Profile, Town (Identifizieren, Verkaufen, Stash), Session-Queue
- Desktop-UI auf Deutsch und Englisch, plus Windows-Installer

Auflösung 1280×720. D2R startet der Operator selbst.

## Architektur

```
D2R.exe  →  process  →  memory snapshot  →  world model
                                              ↓
Electron UI  ←  loopback API  ←  app  ←  tasks / profile / town / loot
                                              ↓
                                           input (SendInput)
```

Pathing, Loot und Town hängen am World Model, nicht an Rohbytes. Die UI redet nur mit `internal/api` auf localhost. Input geht erst nach explizitem Opt-in und bleibt über Hotkeys abbrechbar.

## Wie es gebaut wurde

Phasenpläne, fail-closed Gates, Feature-Docs und ein Changelog vor jedem Release. Live-Abnahme im Spiel, nicht nur grüne Tests. Die Agent-Regeln stehen in [`AGENTS.md`](AGENTS.md), die Phasen in [`docs/plans/`](docs/plans/).

Eine Momentaufnahme vom 31. Juli 2026 (damals v0.16, vier Runs, ohne Cows, Lower Kurast, Hammerdin und i18n) liegt in der [Repo-Effort-Evaluation](docs/reviews/repo-effort-evaluation-2026-07-31.md). Die Zahlen dort sind der Stand von da, kein aktuelles Scoreboard.

## Lizenz und Grenzen

Siehe [`LICENSE`](LICENSE). Quelltext ansehen und daraus lernen: ja. Battle.net, Verkauf als Produkt, Blizzard-Affiliation: nein.

Diablo II: Resurrected ist eine Marke von Blizzard Entertainment, Inc. Dieses Projekt ist inoffiziell.

## Installer

Windows 10/11 x64, unsignierter NSIS-Installer. SmartScreen kann warnen. Daten liegen unter `%LOCALAPPDATA%\D2ROfflineFarmingBot\`.

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1 -Version 0.23.0
```

Ergebnis: `dist/release/D2R-Offline-Farming-Bot-0.23.0-Setup.exe` plus SHA-256. Details: [`docs/INSTALLATION.md`](docs/INSTALLATION.md).

## Entwicklung

Windows, Go 1.26+, Node/pnpm für die UI.

```powershell
Copy-Item configs\config.example.yaml configs\config.yaml
go test ./...
go run ./cmd/d2rbot --version
```

UI: `web/` (Vite, Electron, Vitest). Feature-Docs: [`docs/features/README.md`](docs/features/README.md). Changelog: [`docs/CHANGELOG.md`](docs/CHANGELOG.md). Interne Produktskizze: [`docs/plans/handoff.html`](docs/plans/handoff.html).

CLI-Flags wie `--probe` und `--input-test` sind Diagnose, nicht der Produktstart. Dafür ist die installierte App da. `--input-test` sendet echte OS-Eingaben und braucht `input.enabled: true`.

## Projektstruktur

```
cmd/d2rbot/     Einstieg, Flags, Wiring
internal/       process, memory, world, pathing, input, tasks, profile, town, loot, api
web/            Electron-Desktop und React
configs/        YAML-Beispiele, Pickit, Routen
docs/           Features, Pläne, Changelog
scripts/        Release-Build
```
