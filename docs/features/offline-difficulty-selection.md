# Verifizierter Offline-Game-Start

## Überblick

Phase 7.3 erweitert die kontrollierte Difficulty-Auswahl aus Phase 6.1c zu einem vollständigen, fail-closed Offline-Game-Start. Der Bot beweist per Memory und engen Screen-Ankern zunächst den ausgewählten Charakter und den aktiven Play-Button, bestätigt nach dem ersten Klick den Difficulty-Dialog und verwendet anschließend die bereits live validierte Difficulty-Primitive.

## Ort im Code

- **Paket:** `internal/app`, Screenshot-Backend in `internal/input`
- **Einstieg:** `(*Runtime).RunOfflineDifficultyTest`
- **Wichtige Dateien:** `offline_game.go`, `screen_anchor.go`, `window_capture_windows.go`
- **Anker:** `configs/ui/character-play.png`, `configs/ui/difficulty-dialog.png`, `configs/ui/characters/<name>-selected.png`
- **CLI:** `--offline-difficulty-test normal|nightmare|hell --offline-character <name>`

## Funktionalität

Der isolierte Ablauf besitzt die Zustände `await_character`, `await_difficulty`, `await_game` und `complete`:

1. Drei stabile Memory-Ticks müssen `menu` ohne validen Spieler zeigen.
2. Bei exakt 1280×720 werden der konfigurierte Charakter und der aktive Play-Button durch kleine, versionierte RGB-Anker bestätigt.
3. Der Bot fokussiert D2R und klickt Play genau einmal.
4. Nach drei weiteren stabilen Menü-Ticks muss der Difficulty-Dialog visuell passen.
5. Die kanonische Phase-6.1c-Primitive klickt genau eine Position: Normal `(640,311)`, Nightmare `(640,355)` oder Hell `(640,403)`.
6. Erfolg verlangt drei stabile `in_game`-Ticks mit der erwarteten Character Identity und Rogue Encampment.

Die mittlere normalisierte RGB-Abweichung jedes Ankers darf höchstens `0.16` betragen. Vollbild-Matching wird bewusst vermieden, damit Hintergrundanimationen und Beleuchtung nicht relevant sind.

## Operator / CLI

```powershell
go run ./cmd/d2rbot --offline-difficulty-test nightmare --offline-character MrBones
```

Der Modus ist gegenseitig exklusiv zu Runs, Route-, Input-, Pathing-, Exit- und Probe-Modi. `input.enabled=true` ist erforderlich. Pause/Stop, Fokusprüfung, Client-Geometrie und endliche Stage-/Gesamt-Timeouts bleiben aktiv.

Nach Save & Exit kann Memory bereits `menu` melden, während D2R den Charakterbildschirm noch zeichnet. Der Offline-Start wartet deshalb zusätzlich auf ein begrenztes Render-Settle. Ein noch nicht passender Charakter- oder Difficulty-Anker führt innerhalb des Stage-Timeouts zu einer erneuten stabilen Prüfung, niemals zu einem optimistischen Klick.

## Sicherheitsgrenzen

- Kein Klick wird allein durch `menu` oder einen Timer freigegeben.
- Ein fehlender oder abweichender Character-, Play- oder Dialog-Anker bricht vor dem nächsten Input ab.
- Falsche Character Identity, anderes Startgebiet, Fokusverlust oder falsche Auflösung brechen fail-closed ab.
- Character-Templates sind explizit pro Name. Ein unbekannter Charakter besitzt keine implizite Fallback-Auswahl.
- Difficulty bleibt kontrollierter Auswahlkontext; spätere Route-Freigabe benötigt weiterhin den Layout-Fingerprint.

## Abhängigkeiten

- `github.com/kbinani/screenshot` kapselt den read-only Capture der gebundenen Windows-Clientfläche.
- Windows User32 bleibt für Fensterbindung, Fokus und Input zuständig.
- Koolo dient ausschließlich als Lernreferenz; es ist keine Dependency.

## Live-Abnahme

Am 11.07.2026 wurden drei vollständige Nightmare-Starts bei 1280×720 mit `MrBones` erfolgreich wiederholt. Jeder Durchlauf bestätigte beide Frontend-Stufen, führte genau einen Play- und einen Difficulty-Klick aus und endete nach drei stabilen Memory-Snapshots in Rogue Encampment. Die anschließenden Save-&-Exit-Rücksetzungen waren ebenfalls erfolgreich.

## Verwandte Features

- [Read-only Game Identity](game-identity.md)
- [Layout-Fingerprint](layout-fingerprint.md)
- [Verifiziertes Offline Save & Exit](offline-game-exit.md)
- [Read-only UI-State-Probe](ui-state-probe.md)

---
*Zuletzt aktualisiert: 2026-07-11*
