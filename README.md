# D2R Offline Farming Bot

Windows-Desktop-App, die Diablo II: Resurrected **offline** farmt. Go liest den Prozess, baut ein World Model und steuert Tastatur und Maus. Electron zeigt Queue, Routen, Pickit und Verlauf. Battle.net ist nicht im Scope und nicht implementiert.

Persönliches Fallbeispiel: Architektur und Abnahme kommen von mir, der Code ist mit AI geschrieben. Kein Produkt, keine Aufforderung, die Blizzard-EULA zu umgehen.

English: a Windows desktop app (Go core, Electron UI) for repeatable **offline** D2R farming. Personal AI-assisted engineering case study, not an online cheat and not affiliated with Blizzard.

## Was es kann

- Farming-Ziele: Countess, Mephisto, Summoner, Nihlathak, Cow Level, Lower-Kurast-Supertruhen
- Kampfprofile: Necromancer und Hammerdin, inklusive Mercenary
- Selbst aufgezeichnete Routen mit Playback gegen das Memory-World-Model
- Pickit-Profile, Town (Identifizieren, Verkaufen, Stash), Session-Queue
- Desktop-UI auf Deutsch und Englisch, plus Windows-Installer

Auflösung 1280×720. D2R startet der Operator selbst.

## Oberfläche

**Dashboard.** Laufende Session auf Hell: Queue Unter-Kurast → Countess → Mephisto, Countess aktiv im Turmkeller, Fortschritt der Aufnahme und der Session, Kennzahlen und letzte Ausführungen.

![Dashboard mit aktiver Countess-Route, Queue und Sessionstatistik](docs/screenshots/dashboard.png)

**Routen aufzeichnen.** Aufnahme für Unter-Kurast: Ziel (hohe Runen, Edelsteine), erfüllte Voraussetzungen, Start- und Zielgebiet, kurze Anleitung plus Referenzbilder, Hotkeys zum Beenden.

![Routenaufnahme für Unter-Kurast mit Voraussetzungen, Pfad und Referenzbildern](docs/screenshots/route-recording.png)

**Pickit.** Profilworkspace: Bibliothek links, Regelbau rechts. Hier „Countess Standard“ mit Schnellfiltern, Sockelregel und geordneter Behalten-Liste.

![Pickit-Editor mit Profilbibliothek, Regelbau und Countess-Standardregeln](docs/screenshots/pickit.png)

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

Phasenpläne, fail-closed Gates, Feature-Docs und ein Changelog vor jedem Release. Live-Abnahme im Spiel, nicht nur grüne Tests. Arbeitsregeln für den Agenten: [`AGENTS.md`](AGENTS.md). Phasenpläne: [`docs/plans/`](docs/plans/).

Eine Momentaufnahme vom 31. Juli 2026 (damals v0.16, vier Runs, ohne Cows, Lower Kurast, Hammerdin und i18n) liegt in der [Repo-Effort-Evaluation](docs/reviews/repo-effort-evaluation-2026-07-31.md). Die Zahlen dort sind der Stand von da, kein aktuelles Scoreboard.

## Lizenz und Grenzen

[`LICENSE`](LICENSE): Quelltext ansehen und daraus lernen ja. Battle.net, Verkauf als Produkt, Blizzard-Affiliation nein.

Diablo II: Resurrected ist eine Marke von Blizzard Entertainment, Inc. Dieses Projekt ist inoffiziell.

## Installer und Entwicklung

Windows 10/11 x64, unsignierter NSIS-Installer. SmartScreen kann warnen. Daten: `%LOCALAPPDATA%\D2ROfflineFarmingBot\`.

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1 -Version 0.24.0
```

Ergebnis: `dist/release/D2R-Offline-Farming-Bot-0.24.0-Setup.exe` plus SHA-256. Installer-Hinweise: [`docs/INSTALLATION.md`](docs/INSTALLATION.md).

Lokal: Windows, Go 1.26+, Node/pnpm. `Copy-Item configs\config.example.yaml configs\config.yaml`, dann `go test ./...`. UI unter `web/`. Feature-Docs: [`docs/features/README.md`](docs/features/README.md). Changelog: [`docs/CHANGELOG.md`](docs/CHANGELOG.md).

```
cmd/d2rbot/     Einstieg
internal/       process, memory, world, pathing, input, tasks, profile, town, loot, api
web/            Electron und React
configs/        YAML, Pickit, Routen
docs/           Features, Pläne, Changelog
scripts/        Release-Build
```
