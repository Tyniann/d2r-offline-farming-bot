# FarmQueue-Scheduler

## Überblick

Abschnitt 11.7 erweitert den long-lived `SessionSupervisor` um eine typisierte, zyklische Runtime-Queue. Jeder Eintrag startet genau einen frischen vollständigen Phase-10-Worker. Nur Erfolg schaltet weiter; ein klassifizierter Retry bleibt auf demselben Index. YAML-Budgets beenden die Schleife vor jedem weiteren Worker und gewinnen damit gegen „run until stopped“.

## Ort im Code

- **Paket:** `internal/app/`
- **Preflight und Vertrag:** `internal/app/queue_scheduler.go`
- **Scheduler-State-Machine:** `internal/app/supervisor.go`
- **Availability:** `internal/app/run_availability.go`
- **API-Projektion:** `internal/api/live_backend.go`
- **Config:** `session.queue` und die bestehenden `session.max_*`-Budgets

## Funktionalität

### Vollständiger Preflight

`ValidateFarmQueue` prüft die gesamte nicht leere Liste read-only gegen genau einen Memory-bestätigten Charakter-/Difficulty-Kontext und eine unveränderte Katalogrevision. Unbekannte oder `unavailable` Runs sperren die gesamte Queue mit Eintragsindex, Run-ID und den unveränderten Phase-10-Reasons. `runtime_validation_required` ist zulässig, weil Route Playback weiterhin vor dem ersten Routeninput den Live-Fingerprint verlangt. Duplikate bleiben in ihrer ursprünglichen Reihenfolge erhalten.

Die API stellt denselben Vertrag über `POST /api/v1/queue/validate` ohne Control-Token bereit. Vorschau und Validierung starten keinen Worker, attachen keinen Prozess und senden keinen Input.

### Scheduler

Der Supervisor besitzt die immutable Queue, Index, Zyklus, Retry-Zähler, gestartete Runs, aufeinanderfolgende Fehler, Restarts und eine Kopie der harten Budgets. Ein Worker erhält ausschließlich seinen Run, Index, Zyklus und Retry. Bei `advance` wird der Index modulo Queue-Länge erhöht; Wrap-around erhöht den Zyklus. Bei `retry_current` bleiben Index und Zyklus unverändert. Ein terminales Ergebnis beendet mit `stopped_error`.

Vor jedem frischen Worker kann der Core über `FarmQueueGuard` Katalog, Lifecycle, Auswahl und Availability erneut prüfen. Eine zwischen Runs stale oder unavailable gewordene Route stoppt deshalb vor dem nächsten Worker und vor neuem Input.

### Budgets

- `max_runs` zählt jeden gestarteten Worker genau einmal, einschließlich Retries.
- `max_duration_ms` wird monoton vor jedem Folgestart geprüft.
- `max_consecutive_failures` begrenzt aufeinanderfolgende Retry-Ergebnisse; Erfolg setzt nur diesen Zähler zurück.
- `max_total_restarts` bleibt über die gesamte Queue erhalten.
- Dauer wird vor Run-Anzahl geprüft; beide verhindern einen weiteren Worker.
- Ein Prozessneustart lädt `session.queue` neu und beginnt wieder bei Index 0. Runtime-Änderungen werden nicht persistiert.

`pause_after_run` hält erst nach einem erfolgreichen vollständigen Worker vor dem bereits weitergeschalteten Eintrag. `resume` startet diesen Eintrag frisch. `stop_after_run` verwirft die aktive Queue. Abschnitt 11.8 bindet alle Commands an denselben Supervisor und die lokale API; F11 und Emergency Stop liefern beide `emergency_stop_requested`. Während eines Dashboard-Workers wird der konfigurierte globale Pause-Hotkey als `pause_after_run` an den Supervisor geroutet und ausdrücklich nicht als Mid-Run-Inputpause ausgeführt. D2R kann deshalb im Vordergrund bleiben, während der Intent vorgemerkt wird.

## Datenmodell

- `app.FarmQueueValidationRequest` und `app.FarmQueueValidationContext`: vorgeschlagene und autoritative Auswahl.
- `app.FarmQueuePlan`: immutable Queue, Auswahl, Katalogrevision und Budgets.
- `app.FarmQueueBudgets`: Run-, Dauer-, Failure- und Restart-Limits.
- `app.SupervisorSnapshot`: Queue, Index, Zyklus, Retry, Zähler und Budgets als defensive Kopien.
- `api.QueueValidationDTO` und `api.QueueStatusDTO`: transportneutrale Projektionen ohne Dateipfade oder YAML-Rohdaten.

## Operator / CLI

Der YAML-Default ist eine geordnete Liste; Duplikate sind erlaubt:

```yaml
session:
  queue: [countess, mephisto]
```

Das Dashboard startet nach read-only Preflight über `POST /api/v1/session/start`. Der erste Worker verwendet nur bei `idle_in_game` das bereits Memory-bestätigte Spiel. Jeder spätere Worker erzeugt eine frische run-spezifische Runtime, startet ein neues Spiel und führt exakt einen bestehenden Phase-10-Lauf einschließlich Town und Save & Exit aus. Dadurch werden Countess-/Mephisto-Profil, Pickit, Route und Town-Adapter nicht dynamisch umgebaut oder parallel dupliziert.

## Abhängigkeiten und Grenzen

- Verwendet ausschließlich den bestehenden Phase-10-Availability-Resolver und `SupervisorRunner`.
- Keine zweite Run-, Recovery-, Save-&-Exit- oder Telemetrie-Pipeline.
- Keine Queue-Persistenz außerhalb YAML; kein Zeitplan, Gewicht oder Zufall.
- Keine Route-Datei wird durch Preflight oder Scheduling verändert.

## Verwandte Features

- [Session-Lifecycle](session-lifecycle.md)
- [Run-Verfügbarkeit und Inspect](run-availability.md)
- [Phase-11-Core-Vertrag](phase-11-core-contract.md)
- [Farming-RouteCatalog und Lifecycle](route-lifecycle.md)

---
*Zuletzt aktualisiert: 16. Juli 2026*
