# Phase-11-Core-Vertrag

## Überblick

Dieser Vertrag friert vor dem Phase-11-Umbau die fachlichen Grenzen zwischen bestehendem Phase-10-Core, künftigem `SessionSupervisor`, Route-Lifecycle und lokaler API ein. Er beschreibt Zustände, Befehle, Queue-Semantik, DTO-Grundformen, Reason-Codes und verbotene Übergänge. Abschnitt 11.0 ändert keine produktive Input-Sequenz und startet weder API noch Web-Anwendung.

## Ort im Code

- **Paket:** `internal/app/`
- **Vertrag:** `internal/app/supervisor_contract.go`
- **Bestehender Einzelzyklus:** `internal/app/session_cycle.go`
- **Bestehende endliche Session:** `internal/app/session_multi.go`
- **Availability:** `internal/app/run_availability.go`
- **Frontend-Gates:** `internal/app/offline_game.go`, `internal/app/screen_anchor.go`
- **F11-/Stop-Grenze:** `internal/app/hotkey.go`, `internal/input/safety.go`
- **Telemetrie:** `internal/telemetry/recorder.go`, `internal/telemetry/session_recorder.go`

## Autoritäten und Invarianten

1. Der Go-Core ist alleinige Autorität für Zustand, Availability, Route-Lifecycle und erlaubte Mutationen. Transport und React projizieren nur diesen Vertrag.
2. Genau eine Supervisor-Generation und höchstens eine kontrollierte Worker-Goroutine dürfen aktiv sein.
3. Ein Queue-Eintrag ist genau eine vollständige Phase-10-Session: Game-Verifikation, Run, Town, Save & Exit.
4. Nur `success` schaltet zum nächsten Queue-Index. Ein erlaubter Retry bleibt am selben Index; terminale oder nicht retrybare Fehler stoppen die Queue.
5. `pause_after_run` und `stop_after_run` sind idempotente Intents im Zustand `running_run`, keine Mid-Run-Unterbrechungen. Sofort-Stopp gewinnt immer.
6. F11 und `emergency_stop` verwenden denselben Cancellation-Grund `emergency_stop_requested`, sperren sofort neue Inputs und garantieren kein Save & Exit.
7. Charakter, Difficulty und Queue sind in `starting_run`, `running_run`, `paused_between_runs` und `cancelling` unveränderlich.
8. YAML bleibt Startkonfiguration. Queue-Änderungen der Phase 11 leben nur im Prozessspeicher.
9. Dropdown, Preview, Abbruch und fehlgeschlagener Game-Start schreiben keinen Route-Lifecycle. Erst ein Memory-bestätigter Game-Start darf eine bestätigte Difficulty und gegebenenfalls Invalidation committen.
10. Farming-Routen werden niemals automatisch gelöscht, verschoben oder umgeschrieben. Town-Routen gehören nicht zum invalidierbaren Lifecycle.
11. JSONL bleibt persistente Diagnoseautorität. Künftige Live-Events dürfen Writer und Bot niemals blockieren.

## Supervisor-Zustände und Befehle

| Zustand | Bedeutung | Erlaubte mutierende Befehle |
|---|---|---|
| `idle` | Kein verifiziertes Spiel und keine aktive Queue. | `apply_selection`, `start_queue` |
| `activating_selection` | Screenshot- und Memory-verifizierte Auswahl läuft. | `emergency_stop` |
| `idle_in_game` | Auswahl ist im Spiel bestätigt; kein Run läuft. | `apply_selection`, `start_queue` |
| `starting_run` | Queue-Preflight und vollständige Reset-Barriere laufen. | `emergency_stop` |
| `running_run` | Eine vollständige Phase-10-Session läuft. | `pause_after_run`, `stop_after_run`, `emergency_stop` |
| `paused_between_runs` | Vor dem nächsten Queue-Eintrag angehalten. | `resume`, `emergency_stop` |
| `cancelling` | Cancellation wird propagiert; kein neuer Input. | keiner |
| `stopped_error` | Terminaler Queue-Fehler ist sichtbar. | `apply_selection`, `start_queue` nach vollständigem neuem Preflight |

Interne Worker-Ergebnisse sind keine Commands. Sie führen ausschließlich folgende Zustandswechsel aus:

- Selection-Erfolg: `activating_selection → idle_in_game`.
- Selection-Abbruch/-Fehler: `activating_selection → idle` beziehungsweise zum vorherigen inaktiven Zustand.
- Run-Start-Freigabe: `starting_run → running_run`.
- Run-Erfolg ohne Intent: nächster zyklischer Index und `starting_run`.
- Run-Erfolg mit Pause-Intent: `paused_between_runs`.
- Run-Erfolg mit Stop-Intent: Queue verwerfen und `idle`.
- Retry: gleicher Index und `starting_run`, ausschließlich innerhalb aller YAML-Budgets.
- Terminaler Fehler: `stopped_error`.
- Abschluss der Sofort-Cancellation: `idle` oder `stopped_error`, abhängig vom bestätigten Worker-Ergebnis.

Verboten sind insbesondere parallele Starts, Resume außerhalb `paused_between_runs`, Selection-/Queue-Mutation während Aktivität, Pause/Stop-after-run außerhalb `running_run`, ein zweiter Emergency-Command während `cancelling` sowie jeder direkte Sprung über Preflight oder Reset-Barriere.

## Queue-Vertrag

- Die Queue ist eine geordnete, nicht leere Liste stabiler Run-IDs; Duplikate sind erlaubt.
- Der Default kommt aus YAML, Runtime-Overrides werden nicht persistiert.
- Der vollständige Preflight löst alle Einträge gegen exakt denselben bestätigten Character-/Difficulty-Kontext und dieselbe Katalogrevision auf.
- `advance` erhöht den Index modulo Queue-Länge.
- `retry_current` verändert den Index nicht und verbraucht die bestehenden Fehler-/Restart-Budgets.
- `stop` beginnt keinen weiteren Eintrag.
- Run-, Dauer-, Failure- und Restart-Budgets aus YAML gewinnen gegen den zyklischen Modus.
- Ein Prozessneustart lädt den YAML-Default und beginnt bei Index `0`; Session-, Run-, Command- und Kataloggenerationen werden nicht wiederverwendet.

## Route-Lifecycle-Vertrag

| Status | Bedeutung | Queue-Start |
|---|---|---|
| `valid` | Statische Bindung, Lifecycle und bestätigter Live-Fingerprint passen. | erlaubt |
| `runtime_validation_required` | Statische Bindung und Lifecycle passen; der Live-Fingerprint wird erst am Zielanker bestätigt. | erlaubt, Playback bleibt vor erstem Routeninput gegatet |
| `stale` | Route wurde bestätigt invalidiert oder vor der letzten Invalidation aufgenommen. | gesperrt |
| `unavailable` | Datei, Schema, Bindung, Eindeutigkeit oder Kontext ist ungültig. | gesperrt |

`runtime_validation_required` ist der kanonische Status. Der bestehende Phase-10-Reason `route_runtime_validation_required` bleibt als präziser Availability-Grund erhalten. Es entsteht kein Synonym `route_validation_pending`.

## DTO-Grundvertrag

Die Transporttypen werden erst in 11.2 unter `internal/api` erzeugt. Folgende fachliche Formen und Feldnamen sind bereits verbindlich; kein HTTP-Typ darf in fachliche App-APIs zurückfließen.

### `StatusSnapshot`

- `schema_version`, `generation`, `state`, `pending_intent`;
- Core-Version und D2R-Prozess-/Fenster-/Input-Status;
- bestätigte Auswahl und separater Entwurf;
- Runtime-Queue, `queue_index`, `cycle`, Retry und harte Budgets;
- `session_id`, `game_id`, `run_id`, aktueller Run, Step, Area und Laufzeit;
- strukturierter letzter Fehler mit `code`, deutscher `message` und optionalen `details`.

### `CatalogSnapshot`

- immutable `revision`;
- Character-Einträge mit Konfigurations-/Anker-Availability;
- Difficulties und Profile;
- Run-Metadaten, Run-Availability, Route-ID, Route-Lifecycle-Status und sortierte Reason-Codes;
- keine Dateipfade, YAML-Rohwerte, Memory-Adressen oder Savegame-Inhalte.

### Commands

Jede Mutation trägt `command_id` und `expected_generation`. Wiederholung derselben ID mit identischem Inhalt ist idempotent; abweichender Inhalt ist `request_invalid`; eine veraltete Generation ist `state_changed`.

`SelectionPreview` ist seiteneffektfrei und bindet Character, alte/neue Difficulty, Katalogrevision, betroffene Route-IDs, Invalidation-Grund und Bestätigungspflicht. `SelectionApply` verlangt das unveränderte Preview-Token. `QueueValidation` prüft die vollständige Queue read-only. `SessionStart` übernimmt ausschließlich eine erfolgreich geprüfte Queue desselben Snapshots.

### Live-Events

Jedes Event trägt `schema_version`, monotone `sequence`, `timestamp`, `event` und die vorhandenen Korrelations-IDs. Ein neuer Client erhält zuerst einen vollständigen Snapshot und danach nur Ereignisse mit höherer Sequenz. Terminale Command-, Run-, Session- und Error-Events dürfen nicht dedupliziert werden.

## Stabile Reason-Codes

Vorhandene semantisch identische Phase-10-Codes werden wiederverwendet. Die folgende Liste ist für Phase 11 reserviert und darf nicht durch Schreibvarianten ergänzt werden.

| Bereich | Codes |
|---|---|
| Supervisor | `command_conflict`, `state_changed`, `session_not_running`, `session_not_paused`, `emergency_stop_requested` |
| API | `request_unauthorized`, `origin_rejected`, `request_invalid`, `payload_too_large`, `api_version_unsupported` |
| Charakter | `character_save_missing`, `character_unconfigured`, `character_anchor_missing`, `character_screen_unconfirmed`, `character_selection_unconfirmed`, `character_identity_mismatch` |
| Difficulty | `selection_confirmation_required`, `selection_preview_stale`, `difficulty_dialog_unconfirmed`, `difficulty_change_failed` |
| Route-Lifecycle | `route_stale`, `route_lifecycle_unavailable`, `route_manifest_corrupt`, `route_manifest_write_failed`, `route_layout_mismatch`, `route_runtime_validation_required` |
| Queue | `queue_empty`, `queue_entry_unavailable`, `queue_context_mismatch`, `queue_locked`, `run_budget_exhausted`, `duration_budget_exhausted` |

Weiterhin gültige Phase-10-Reasons wie `run_unknown`, `run_config_missing`, `route_missing`, `route_binding_mismatch`, `profile_class_mismatch`, `town_egress_missing`, `hard_stuck` und `telemetry_failed` bleiben unverändert. API-Responses dürfen sie als Details durchreichen, aber nicht in neue Synonyme übersetzen.

## Characterization-Schutz

| Bestehendes Verhalten | Beweis vor Umbau |
|---|---|
| Phase-10-Einzelzyklus | `phase10_characterization_test.go` und `session_cycle_test.go`: frischer Run, Reset vor Exit, genau ein Save & Exit, kein direkter Neustart. |
| F11-/Stop-Cancellation | `hotkey_test.go` und `input/safety_test.go`: Input-Stop vor Context-Cancel, idempotenter Stop und keine Folgeinputs. |
| Run-Availability | `run_availability_test.go` und `session_plan_test.go`: deterministisches JSON, identischer Session-Preflight und kein Attach/Input. |
| Offline-Game-Start | `offline_game_test.go`: Character-/Play-/Dialog-Reihenfolge, 1280×720 und stabile Memory-Identität. |
| UI-Anker | `screen_anchor_test.go` und `ui_state_probe_test.go`: enge Bounds, Schwellwert und kein Memory-/Timer-Blindpfad. |
| Telemetrie-Sinks | `recorder_test.go`, `session_recorder_test.go` und Recovery-Tests: synchrone Flushes, Korrelation und fail-closed Folgeaktionen. |

## Migrationsmatrix

| Bestehender One-shot-Pfad | Künftige Supervisor-Verwendung | Später zu entfernendes/anzupassendes Wiring |
|---|---|---|
| `Runtime.runSession` / `sessionMultiRunner.run` | Worker für genau einen vollständigen Queue-Eintrag; bestehende Budgets bleiben übergeordnet autoritativ. | Der direkte CLI-Lifecycle-Start wurde in 11.1 durch einen dünnen `SessionSupervisor`-Adapter ersetzt. |
| `sessionCycleOrchestrator.execute` | Unveränderte fachliche Einheit für Verify → Run → Exit. | Keine zweite Cycle-Pipeline; nur Start-/Ergebnisadapter. |
| `Runtime.RunOfflineDifficultyTest` | Selection-Apply-Executor nach bestätigtem Preview. | CLI-Aufruf bleibt Diagnoseadapter; Character-Listen-Navigation kommt erst 11.4. |
| `Runtime.RunOfflineExitTest` / Session-Exit-Executor | Geordneter Abschluss eines Queue-Eintrags. | Keine API-eigene Save-&-Exit-Sequenz. |
| `Runtime.handleHotkeyEvent` | Emergency-Command auf denselben Cancellation-Pfad. | Direkter `cancel()`-Aufruf wird zentralisiert, sobald der Supervisor ihn besitzt. |
| `ResolveRunAvailabilities` / `ResolveSessionPlan` | Catalog- und Queue-Preflight. | Kein zweiter GUI-Resolver und keine React-Availability. |
| `tasks.Runner` / gemeinsame Pipeline | Frische Run-Instanz innerhalb des bestehenden Cycle-Workers. | Kein langlebiger Runner über Queue-Einträge hinweg. |
| `telemetry.SessionRecorder` / Run-Recorder | Persistente Diagnose plus Live-Projektion ab 11.3. | JSONL bleibt bestehen; Live-Publisher wird nur additiv angebunden. |

## Grenzen von Abschnitt 11.0

- Kein Supervisor-Worker, kein HTTP-Server, kein SSE-Publisher und kein React-/Vite-Scaffold.
- Keine Config-Migration und kein Lifecycle-Manifest.
- Keine neue Character-Screen-Automation und keine produktive Input-Änderung.
- Keine Route-Datei wird gelesen, geschrieben, verschoben oder gelöscht, außer durch unveränderte read-only Tests auf bestehenden Pfaden.

## Verwandte Features

- [Session-Lifecycle](session-lifecycle.md)
- [Session-Konfiguration und Inspect](session-configuration.md)
- [Run-Verfügbarkeit und Inspect](run-availability.md)
- [Verifizierter Offline-Game-Start](offline-difficulty-selection.md)
- [Read-only UI-State-Probe](ui-state-probe.md)
- [Run-Telemetrie](run-telemetry.md)
- [Session-Recovery und Lifecycle-Telemetrie](session-recovery-telemetry.md)

---
*Zuletzt aktualisiert: 16. Juli 2026*
