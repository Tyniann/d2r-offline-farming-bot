# Notfall-Recovery für Run und Spielstart

## Überblick

Die Notfall-Recovery hält retrybare Run-Fehler und unerwartete Startakte innerhalb eines einzigen, begrenzten Lifecycle-Vertrags. Ein blockierter Rückweg erhält genau einen lokalen Räumversuch und einen erneuten Portalversuch. Scheitert der sichere Rückweg weiterhin, verlässt der zentrale Game-Lifecycle das aktuelle Offline-Spiel direkt aus dem bestätigten Gebiet und wiederholt denselben Queue-Eintrag in einem neuen Spiel. Startet dieses Spiel in Akt 2–5, führt die passende globale Spawn-Egress-Route vor jedem Run-Input nach Akt 1.

## Ort im Code

- **Paket:** `internal/tasks/`, `internal/app/`, `internal/telemetry/`, `internal/api/`
- **Einstieg:** `internal/tasks/portal_destination_recovery.go`, `internal/app/queue_runtime.go`
- **Wichtige Dateien:** `internal/tasks/retry_return.go`, `internal/tasks/runner.go`, `internal/app/fresh_game_normalization.go`, `internal/telemetry/recorder.go`, `internal/api/live_backend.go`
- **UI:** `web/src/features/dashboard/ActiveRunPanel.tsx`
- **Systemrouten:** `configs/routes/town/act2/egress/spawn-waypoint.yaml` bis `configs/routes/town/act5/egress/spawn-waypoint.yaml`
- **Config:** vorhandene Session-, Combat-, Portal-, Waypoint- und Egress-Konfiguration; kein zweiter Recovery-Konfigurationsbaum

## Funktionalität

### Begrenzte Portal-Recovery

`retry-return` bestätigt das aktuelle Routengebiet über frische, stabile Snapshots, erzeugt ein Stadtportal und wartet auf die Memory-bestätigte Zielstadt. Bleibt der Charakter nach dem ersten bestätigten Portalklick im Routengebiet, pinnt die Pipeline dieselbe Portal-UnitID. Sie räumt lebende Gegner in einem Radius von zwölf Tiles mit dem aktiven Kampfprofil. Der Versuch endet nach höchstens zwölf Kampfaktionen, sechs Sekunden Gesamtlaufzeit oder drei Sekunden ohne neue Aktion. Unvollständige Monsterabdeckung kann keinen freien Bereich bestätigen.

Nach `cleared` oder einem ausgeschöpften, aber technisch sauberen Clear teleportiert die Pipeline zum gepinnten Portal, wartet auf einen frischen Snapshot und führt genau einen weiteren hover-bestätigten Portalklick aus. Loading, ungültige Snapshots, ein Gebietswechsel, fehlende Abhängigkeiten oder ein Inputfehler stoppen den Teilablauf ohne zusätzliche Gameplay-Aktion.

### Direkter Spielaustritt und Retry

Scheitert auch der einzige Portal-Retry, bleibt der produktive Fehler in `original_reason` erhalten; die Rückkehrursache steht getrennt in `recovery_reason`. Der Queue-Runner autorisiert den direkten Exit nur aus einem weiterhin gültigen, gebundenen Offline-In-Game-Kontext. Derselbe zentrale Save-&-Exit-Automat öffnet das bestätigte Quit-Menü und klickt einmal auf „Speichern & Beenden“. Ein bestätigter Exit erzeugt einen abgebrochenen Run und erlaubt innerhalb der Session-Budgets einen Neustart am selben Queue-Index. Ein nicht bestätigter Exit beendet die Session als Fehler und startet kein neues Spiel.

### Normalisierung eines neuen Spiels

Ein vom Bot gestartetes Spiel darf im Rogue Encampment oder in der gespeicherten Stadt von Akt 2–5 erscheinen. Bereits der Offline-Startautomat akzeptiert diese fünf konkreten Stadt-Areas; andere Gebiete bleiben fail-closed. Für einen Fremdakt lädt der Lifecycle ausschließlich die passende globale `spawn-waypoint.yaml`, prüft Akt, Area, Version, Spawn-Toleranz und Character Identity und läuft damit zum lokalen Wegpunkt. Während des Wegpunkttransfers darf Memory kurz `in_game` mit unbekannter Area melden; dieser Zustand löst keinen Input aus und wird nur innerhalb des vorhandenen Normalisierungs-Timeouts abgewartet. Die Ankunft in Akt 1 muss durch Memory bestätigt werden, bevor `/players`, Town-Vorbereitung oder Run-Input möglich sind. Fehlende Route, falscher Spawn, Layoutabweichung, fehlende Layoutanker, eine konkrete falsche Ziel-Area und Timeout stoppen fail-closed.

## Datenmodell

Die Recovery erweitert das bestehende Telemetrie-Schema 4 additiv. Die Ereignisse `local_recovery_clear_started`, `local_recovery_clear_finished`, `return_portal_retry`, `direct_exit_started`, `direct_exit_completed`, `direct_exit_failed`, `start_town_normalization_started`, `start_town_normalization_completed` und `start_town_normalization_failed` tragen die verfügbaren Session-, Game-, Run-, Queue- und Retry-Korrelationen. Clear-Ereignisse enthalten Portal-/Blocker-UnitID, Radius, Budgets, Ergebnis, Aktionen, Dauer, Restbedrohungen und Coverage. Exit-Ereignisse bewahren Original- und Recovery-Grund. Die Historie unterscheidet den für einen Retry abgebrochenen Run von einem harten Session-Fehler.

Der Core-Status projiziert den aktuellen `recovery_step`. Das Dashboard übersetzt diesen Wert in kurze Bedienhinweise; es trifft keine eigene Recovery-Entscheidung und besitzt keine zweite Zustandsmaschine.

## Operator / CLI

Die Recovery läuft ausschließlich innerhalb einer gestarteten FarmQueue. Der Operator sieht unter anderem:

- „Rückkehr blockiert. Die unmittelbare Umgebung wird geräumt.“
- „Stadtportal wird erneut versucht.“
- „Sicherer Rückweg fehlgeschlagen. Das Spiel wird direkt beendet.“
- „Spiel startete in Akt X. Rückkehr nach Akt 1 läuft.“

F11 beziehungsweise der konfigurierte Notstopp sperrt weitere Inputs auch während Clear, direktem Exit und Startnormalisierung. Diagnose und Historie verwenden stabile Reason-Codes statt lokalisierter Texte.

## Abhängigkeiten

Die Recovery verwendet ausschließlich das konsistente World Model, das aktive Kampfprofil, den vorhandenen Portal- und Save-&-Exit-Vertrag, globale System-Egress-Routen und die bestehende Session-Telemetrie. Sie schreibt keine D2R-Installations- oder Savegame-Dateien und ist ausschließlich für bestätigte Offline-Spiele vorgesehen.

## Verwandte Features

- [Session-Lifecycle](session-lifecycle.md)
- [FarmQueue-Scheduler](farm-queue-scheduler.md)
- [Globaler System-Egress](system-egress.md)
- [Session-Recovery und Lifecycle-Telemetrie](session-recovery-telemetry.md)

---
*Zuletzt aktualisiert: 27. August 2026*
