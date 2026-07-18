# Session-Recovery und Lifecycle-Telemetrie

## Überblick

Der Phase-11-Supervisor klassifiziert terminale Run-Ergebnisse über exakte Reason-Codes und entscheidet innerhalb harter Fehler-/Restart-Budgets. Ein synchron geflushter JSONL-Recorder korreliert Session, Game und Run. Unbekannte, ähnlich benannte oder nicht konfigurierte Gründe sind niemals automatisch retrybar.

## Ort im Code

- **Recovery-Entscheidung und Budgets:** `internal/app/supervisor.go`
- **Konfigurierbare Retry-Freigabe:** `internal/app/run_failure.go`
- **Produktiver Game-/Run-Adapter:** `internal/app/queue_runtime.go`
- **Route-Reason-Mapping:** `internal/tasks/run_pipeline.go`
- **Recorder und Schema:** `internal/telemetry/session_recorder.go`, `internal/telemetry/recorder.go`
- **Config:** `session.max_consecutive_failures`, `session.max_total_restarts`, `session.retry_classes`

## Stabile Fehlerklassifikation

Nur die validierten Codes `hard_stuck`, `route_drift_exceeded`, `route_segment_timeout` und `route_transition_failed` können einen Retry auslösen. Zusätzlich muss der konkrete Code in `session.retry_classes` erlaubt sein. Texte wie `hard_stuck_extra` oder ein Fehlerstring, der zufällig „hard_stuck“ enthält, bleiben terminal.

## Budget- und Lifecycle-Semantik

- Erfolg setzt `consecutive_failures` auf null und schaltet zum nächsten Queue-Index.
- Ein retrybarer Fehler bleibt am aktuellen Index und erhöht Fehler-/Restart-Zähler innerhalb der YAML-Budgets.
- Eine Recovery verlässt nur einen als sicher bestätigten Game-Kontext über den zentralen `ExitGame`-Owner; andernfalls stoppt die Queue fail-closed.
- Normaler Queue-Wrap und Recovery-Restart sind getrennte Spielgrenzen. Nur Recovery verbraucht ein Restart-Budget.
- Ein unbekannter Reason-Code, ein erschöpftes Budget oder ein Telemetriefehler beendet die Queue terminal.

## Session-Recorder

`telemetry.NewSessionRecorder` erzeugt vor Session-Input eine Datei `logs/telemetry/session-<UTC-Zeit>-<Zufallssuffix>.jsonl`. Lifecycle-Events verwenden `schema_version=2` und tragen dieselbe `session_id`; Game-/Run-Events ergänzen `game_id`, `run_id`, Queue-Index, Zyklus und Ergebnisfelder. Zwischen zwei erfolgreichen Queue-Einträgen desselben Spiels gibt es kein `game_exited`.

Der Phase-5-Run-Recorder schreibt weiterhin Schema 1 für Loot-/Route-Detailereignisse. Jede frische Run-Generation beginnt mit genau einem `run_context`, das Definition, Route/Fingerprint, Waypoint-Ziel, Pickup-/Sell-Policy und Town-Herkunft bindet.

## Abnahme

Supervisor-, Queue-Lifecycle- und Telemetrietests decken exakte Retry-Freigabe, erschöpfte Budgets, Erfolgs-Reset, Retry am selben Index, kontrollierten Game-Restart, korrelierte IDs und den fehlenden Exit zwischen Countess und Mephisto ab. Die Live-Abnahme vom 17. Juli 2026 bestätigte Same-game-Pause/Resume, natürlichen Wrap sowie Stop-after-run mit genau einem abschließenden Exit.

## Verwandte Features

- [Session-Lifecycle](session-lifecycle.md)
- [FarmQueue-Scheduler](farm-queue-scheduler.md)
- [Session-Konfiguration und Inspect](session-configuration.md)
- [Run-Telemetrie](run-telemetry.md)

---
*Zuletzt aktualisiert: 2026-07-17*
