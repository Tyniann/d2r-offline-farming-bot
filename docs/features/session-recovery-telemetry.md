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

Nur die validierten Codes `hard_stuck`, `route_drift_exceeded`, `route_segment_timeout`, `route_transition_failed`, `route_clear_no_progress`, `route_threat_out_of_range`, `route_mana_recovery_failed`, `route_recovery_unsafe`, `boss_combat_unprojectable` und `cow_combat_no_progress` können einen Retry auslösen. Zusätzlich muss der konkrete Code in `session.retry_classes` erlaubt sein. Texte wie `hard_stuck_extra` oder ein Fehlerstring, der zufällig „hard_stuck“ enthält, bleiben terminal.

## Budget- und Lifecycle-Semantik

- Erfolg setzt `consecutive_failures` auf null und schaltet zum nächsten Queue-Index.
- Ein retrybarer Fehler bleibt am aktuellen Index und erhöht Fehler-/Restart-Zähler innerhalb der YAML-Budgets.
- Eine Recovery versucht zunächst den kontrollierten Portalrückweg. Nach einem begrenzten lokalen Clear und genau einem Portal-Retry darf nur der zentrale `ExitGame`-Owner einen weiterhin sicher bestätigten Offline-Game-Kontext direkt verlassen; andernfalls stoppt die Queue fail-closed.
- Normaler Queue-Wrap und Recovery-Restart sind getrennte Spielgrenzen. Nur Recovery verbraucht ein Restart-Budget.
- Ein unbekannter Reason-Code, ein erschöpftes Budget oder ein Telemetriefehler beendet die Queue terminal.

## Session-Recorder

`telemetry.NewSessionRecorderWithContext` erzeugt vor Session-Input eine Datei `logs/telemetry/session-<UTC-Zeit>-<Zufallssuffix>.jsonl`. Neue Lifecycle-Events verwenden `schema_version=4`, `stream=session`, `mode=productive_farming` und tragen dieselbe `session_id` sowie unveränderlich Charakter, Difficulty und D2R-Version. `game_started` und `game_exited` ergänzen ausschließlich die `game_id`; sie gehören zur Session-Grenze und übernehmen keinen zufällig aktuellen Run-Kontext. Nur `run_started` und das zugehörige Run-Terminal tragen die global eindeutige `run_id`, Run-Definition, Queue-Index und Zyklus. Zwischen zwei erfolgreichen Queue-Einträgen desselben Spiels gibt es kein `game_exited`. Doppelte Run- oder Session-Terminals sowie Kontextdrift werden vor dem Write abgewiesen.

Der getrennte Run-Recorder schreibt ebenfalls Schema 4 und verwendet exakt dieselbe Supervisor-Run-ID im Dateinamen und im einmaligen gemeinsamen Kontext. Jede frische Run-Generation beginnt mit genau einem `run_context`, das Definition, Route/Fingerprint, Queue-Kontext und Pickit-Snapshot bindet; spätere Zeilen enthalten nur Ereignisdaten. Ältere Schemata werden nicht verändert oder importiert.

### Recovery-Ereignisse und Projektion

Die Task-Pipeline schreibt `local_recovery_clear_started`, `local_recovery_clear_finished` und `return_portal_retry`. Start und Abschluss enthalten den gepinnten Portalanker, optionalen Blocker, Radius, Aktions-/Zeitbudgets, Ergebnis, gesendete Aktionen, Dauer, Restbedrohungen und Monster-Coverage. `return_portal_retry` trägt stets `attempt=1` und ein explizites Ergebnis.

Der Game-Lifecycle schreibt `direct_exit_started`, `direct_exit_completed` oder `direct_exit_failed` sowie `start_town_normalization_started`, `start_town_normalization_completed` oder `start_town_normalization_failed`. Diese Ereignisse übernehmen dieselben Session-, Game-, Run-, Queue-, Zyklus- und Retry-IDs wie der zugehörige Versuch. Direkte Exits bewahren `original_reason` und `recovery_reason`; Startnormalisierung bindet Akt, Area, Routendatei und Startposition. Die History klassifiziert einen bestätigten Recovery-Restart als `run_aborted`, einen harten Lifecycle-Abbruch als `run_failed`.

Die Core-API projiziert Originalgrund, Recovery-Grund und den aktuellen `recovery_step`. SSE meldet Änderungen als `recovery_step_changed`. Das Dashboard übersetzt nur die Core-Projektion und entscheidet weder Retry noch Exit.

## Abnahme

Supervisor-, Queue-Lifecycle- und Telemetrietests decken exakte Retry-Freigabe, erschöpfte Budgets, Erfolgs-Reset, Retry am selben Index, kontrollierten Game-Restart, korrelierte IDs und den fehlenden Exit zwischen Countess und Mephisto ab. Die Live-Abnahme vom 17. Juli 2026 bestätigte Same-game-Pause/Resume, natürlichen Wrap sowie Stop-after-run mit genau einem abschließenden Exit.

## Verwandte Features

- [Session-Lifecycle](session-lifecycle.md)
- [FarmQueue-Scheduler](farm-queue-scheduler.md)
- [Session-Konfiguration und Inspect](session-configuration.md)
- [Run-Telemetrie](run-telemetry.md)
- [Notfall-Recovery für Run und Spielstart](emergency-run-recovery.md)

---
*Zuletzt aktualisiert: 2026-08-27*
