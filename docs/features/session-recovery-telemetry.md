# Session-Recovery und Lifecycle-Telemetrie

## Überblick

Phase 7.7 klassifiziert terminale Run-Ergebnisse über exakte Reason-Codes und entscheidet innerhalb harter Fehler-/Restart-Budgets. Ein eigener synchron geflushter JSONL-Recorder korreliert Session, Game und Run. Unbekannte, ähnlich benannte oder nicht konfigurierte Gründe sind niemals automatisch retrybar.

## Ort im Code

- **Recovery-Policy:** `internal/app/session_recovery.go`
- **Route-Reason-Mapping:** `internal/tasks/countess.go`
- **Stuck-Kontext:** `internal/pathing/route_segment_player.go`, `internal/app/route_adapter.go`
- **Recorder und Schema:** `internal/telemetry/session_recorder.go`, `internal/telemetry/recorder.go`
- **Config:** `session.max_consecutive_failures`, `session.max_total_restarts`, `session.retry_classes`

## Stabile Fehlerklassifikation

Nur folgende exakten Codes können als `run_restartable` gelten:

- `hard_stuck`
- `route_drift_exceeded`
- `route_segment_timeout`
- `route_transition_failed`

Zusätzlich muss der Code in `session.retry_classes` erlaubt sein. Das Mapping verwendet ausschließlich `errors.Is` gegen Sentinel-Fehler. Texte wie `hard_stuck_extra` oder ein Fehlerstring, der zufällig „hard_stuck“ enthält, bleiben terminal.

Der Route Player erzeugt `ErrRouteHardStuck` erst, nachdem Navigator und route-lokale Korrekturen ausgeschöpft sind. Der Adapter hält Route-ID, Segment-ID, nächsten und letzten bestätigten Punkt, Zielkoordinaten, Drift und lokale Recovery-Versuche strukturiert fest.

## Budget-Semantik

- Ein erfolgreicher Run setzt `consecutive_failures` auf null.
- `failed` und `aborted` erhöhen den Zähler; Operator-Stop verbraucht kein Fehler- oder Restart-Budget.
- Ein Restart wird nur erlaubt, wenn Reason exakt klassifiziert und konfiguriert ist, die neue Fehleranzahl das Maximum nicht überschreitet und noch ein Total-Restart verfügbar ist.
- Budgets werden vor der geloggten Recovery-Entscheidung atomar aktualisiert.
- Ein unbekannter Reason-Code, ein erschöpftes Budget oder ein Telemetriefehler ergibt `fail_session`.

## Ereignisreihenfolge

Bei Hard Stuck gilt zwingend:

1. `stuck_detected` mit vollständigem Fortschrittskontext;
2. `run_aborted` mit stabilem Reason, letztem Step und Laufzeit;
3. genau eine Entscheidung `game_restart_requested` oder terminaler Session-Abschluss.

Jedes Event wird geschrieben und geflusht, bevor die nächste Stufe berechnet beziehungsweise freigegeben wird. Scheitert `stuck_detected` oder `run_aborted`, bleibt das Restart-Budget unverändert und es darf kein Exit-/Restart-Input folgen.

## Session-Recorder

`telemetry.NewSessionRecorder` erzeugt vor Session-Input eine Datei:

```text
logs/telemetry/session-<UTC-Zeit>-<Zufallssuffix>.jsonl
```

Lifecycle-Events verwenden `schema_version=2` und tragen dieselbe `session_id`. Game-/Run-Events ergänzen `game_id`, `run_id`, Run-Ordinal und Ergebnisfelder. Terminale Events `session_completed`, `session_stopped` oder `session_failed` enthalten gestartete, erfolgreiche, abgebrochene und fehlgeschlagene Runs, aufeinanderfolgende Fehler, Restarts und Gesamtdauer.

Der bestehende Phase-5-Run-Recorder bleibt kompatibel und schreibt weiterhin Schema 1 für Loot-/Route-Detailereignisse. Der Phase-7.8-Multi-Run-Kern erzeugt eindeutige korrelierte Run-IDs; die Live-Verdrahtung mit neu aufgenommenen Nightmare-Assets bleibt Teil der offenen E2E-Freigabe.

## Abnahme

Die Fehler-Injektion deckt die vollständige Hard-Stuck-Reihenfolge, exakte Reason-Prüfung ohne Text-Matching, erlaubte und erschöpfte Restart-Budgets, Erfolgs-Reset, Telemetriefehler vor Recovery, korrelierte JSONL-IDs und terminale Summary-Zähler ab. Reale Multi-Run-Steuerung und der kontrollierte Game-Restart folgen im E2E-Slice 7.8.

## Verwandte Features

- [Session-Lifecycle](session-lifecycle.md)
- [Session-Konfiguration und Inspect](session-configuration.md)
- [Run-Telemetrie](run-telemetry.md)
- [Route Recording und Playback](route-recording-playback.md)

---
*Zuletzt aktualisiert: 2026-07-11*
