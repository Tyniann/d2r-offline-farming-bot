# Session-Lifecycle

## Überblick

Der Session-Lifecycle führt eine endliche, duplikatfreie Folge registrierter Farming-Runs über bestätigte Offline-Spiele aus. Innerhalb eines Spiels teilen mehrere Runs den Game-Kontext, erhalten aber jeweils frischen Run-Zustand. Der zentrale Lifecycle entscheidet über Spielverifikation, Queue-Wrap, Retry-Spielgrenzen, Save & Exit und Session-Ende. Navigation, Kampf, Loot und Stash bleiben in ihren bestehenden Paketen.

Die Phase-7-Einzelzyklusarchitektur ist historischer Kontext. Seit Phase 11 verwenden Dashboard und autonome CLI denselben `SessionSupervisor` und `RuntimeQueueRunner`; der alte Cycle-/Multi-/Recovery-Stack wurde entfernt.

## Ort im Code

- **Orchestrierung:** `internal/app/supervisor.go`, `internal/app/queue_runtime.go`
- **Bestehender Run-Vertrag:** `internal/tasks/result.go`, `internal/tasks/runner.go`, `internal/tasks/registry.go`
- **Konfiguration und Inspect:** `internal/config/session.go`, `internal/app/session_plan.go`
- **Telemetrie:** `internal/telemetry/`
- **Input- und Prozessgrenzen:** `internal/input/`, `internal/process/`
- **Konzeptplan:** [`phase-7-implementation-plan.html`](../../phase-7-implementation-plan.html)

## Begriffe und Identitäten

| Begriff | Bedeutung | Lebensdauer |
|---------|-----------|-------------|
| Session | Eine vom Operator gestartete, endliche Multi-Run-Ausführung. | Vom Preflight bis `stopped`, `completed` oder `failed`. |
| Game | Genau eine bestätigte Offline-In-Game-Instanz zwischen Game-Start und Save & Exit. | Innerhalb einer Session. |
| Run | Genau eine frische Ausführung eines Namens aus `tasks.KnownRuns()`. | Innerhalb eines Game; mehrere eindeutige Queue-Einträge dürfen denselben Game-Kontext teilen. |
| Attempt | Ein begrenzter technischer Versuch innerhalb einer Lifecycle-Aktion. | Innerhalb eines Zustands, niemals über dessen Budget hinaus. |

Jede Session erhält eine stabile `session_id`. Jedes bestätigte Spiel erhält eine innerhalb der Session eindeutige `game_id`; jeder gestartete Run eine eindeutige `run_id`. Alle Lifecycle- und Run-Telemetrieereignisse tragen die verfügbaren IDs. IDs werden nie für Recovery-Entscheidungen wiederverwendet.

## Zustandsmodell

```text
preflight
  -> await_game
  -> verify_game
  -> run_active
  -> evaluate
     -> completed | stopped | failed
     -> exit_game -> cooldown -> start_game -> verify_game
```

| Zustand | Zweck | Zulässiger Input | Erfolg | Fehler |
|---------|-------|------------------|---------|--------|
| `preflight` | Konfiguration, Run, Route, Bindings, Telemetriepfad und statische Fensteranforderungen prüfen. | Keiner. | `await_game`. | `failed`. |
| `await_game` | Einen unterstützten Startzustand stabil beobachten. | Keiner. | `verify_game`. | Timeout → `failed`. |
| `verify_game` | Character Identity, Version, Startgebiet und Route/Layout prüfen. | Keiner. | `run_active`. | Mismatch oder fehlender Nachweis → `failed`. |
| `run_active` | Genau einen frischen `tasks.Runner` ausführen. | Nur durch den Task Runner und seine bestehenden Guards. | Terminales Run-Ergebnis → `evaluate`. | Lifecycle-Abbruch → `evaluate` oder `failed`. |
| `evaluate` | Ergebnis klassifizieren und Budgets atomar aktualisieren. | Keiner. | `exit_game`, `completed`, `stopped` oder `failed`. | Interner Widerspruch → `failed`. |
| `exit_game` | Das aktuelle Offline-Spiel kontrolliert verlassen. | Nur die einzeln gegateten Phase-7.2-Aktionen. | Bestätigter Offline-Menüzustand → `cooldown`. | Unbekannter Zustand/Timeout → `failed`. |
| `cooldown` | Eine endliche Pause zwischen Games erzwingen. | Keiner. | `start_game`. | Stop → `stopped`. |
| `start_game` | Charakter und Difficulty kontrolliert auswählen. | Nur die einzeln gegateten Phase-7.3-Aktionen. | Loading und danach bestätigtes In-Game → `verify_game`. | Unbekannter Zustand/Timeout → `failed`. |
| `completed` | Alle konfigurierten Runs oder die maximale Dauer planmäßig erreicht. | Keiner. | Terminal. | – |
| `stopped` | Operator-Stop wurde verarbeitet. | Keiner. | Terminal. | – |
| `failed` | Ein terminaler Sicherheits-, Kontext-, Budget- oder Infrastrukturfehler liegt vor. | Keiner. | Terminal. | – |

Es gibt keinen direkten Übergang von `run_active` zu `start_game`. Ein neues Spiel ist erst nach Run-Auswertung, bestätigtem Game-Austritt und Cooldown zulässig.

## Unterstützte Startzustände

Phase 7 unterstützt im MVP ausschließlich:

1. einen validen `in_game`-Zustand im Rogue Encampment mit stabil bestätigter Character Identity; oder
2. den in Phase 7.1 eindeutig nachgewiesenen Offline-Charakterbildschirm.

`GamePhaseMenu` allein ist kein unterstützter Startzustand, weil er Charakterbildschirm, Difficulty-Dialog und andere Menüs nicht unterscheidet. Loading, unbekannte Dialoge, ein Dungeon-Start, ein toter Charakter, Prozessverlust oder widersprüchliche UI-Signale führen nicht zu einem Klick. Phase 7.1 darf den unterstützten Startumfang anhand live validierter read-only Signale enger fassen.

## Ergebnisvertrag

Der bestehende `tasks.RunOutcome` bleibt zunächst unverändert. Der Session-Lifecycle mappt ein terminales Task-Ergebnis in einen eigenen Vertrag:

| Run-Ergebnis | Bedeutung |
|--------------|-----------|
| `success` | Der Full Run hat seinen terminalen Erfolgsschritt erreicht. |
| `aborted` | Der Lifecycle hat einen gestarteten Run kontrolliert vorzeitig beendet, zum Beispiel wegen `hard_stuck` oder Operator-Stop. |
| `failed` | Der Run endete mit einem fachlichen oder technischen Fehler. |

`aborted` darf nicht als `success` gezählt werden. Stop ist kein Fehlerbudget-Verbrauch. Ein Hard Stuck ist `aborted` mit `reason=hard_stuck`; die anschließende Recovery-Entscheidung gehört separat zum Session-Ergebnis.

| Session-Ergebnis | Bedeutung |
|------------------|-----------|
| `completed` | `max_runs` oder `max_duration` wurde planmäßig erreicht, ohne einen weiteren Game-Start anzustoßen. |
| `stopped` | Der Operator hat Stop ausgelöst. |
| `failed` | Die Session kann innerhalb des Vertrags nicht sicher fortfahren. |

Pause friert Fristen und Fortschritt nach der später festzulegenden Monotonic-Time-Policy ein, erzeugt aber kein terminales Ergebnis. Bis diese Policy implementiert und getestet ist, darf Pause keine neue Lifecycle-Aktion beginnen.

## Fehler- und Recovery-Klassen

Der Lifecycle klassifiziert stabile Reason-Codes, nicht Logtexte. Unbekannte Codes sind immer `terminal`; Präfix- oder Substring-Matching ist verboten.

| Klasse | Beispiele | Entscheidung |
|--------|-----------|--------------|
| `transient` | kurzzeitig ungültiger Snapshot oder Loading innerhalb des aktuellen Zustandsbudgets | Warten, ohne Input und ohne Game-Restart. |
| `run_restartable` | live validierter `hard_stuck`, `route_drift_exceeded`, `route_segment_timeout` oder `route_transition_failed` | Run abbrechen, Fehlerbudget abbuchen; Game-Restart nur über einen validierten sicheren Exit-Flow. |
| `terminal_context` | falscher Charakter, Game-Version oder Layout-Fingerprint; `unexpected_area`; unbekannter Menüscreen | Session stoppen, keine weiteren Inputs. |
| `terminal_config` | unbekannter Run, fehlende Route/Bindings, ungültige Budgets oder nicht verdrahtete Actions | Vor Input stoppen. |
| `terminal_infrastructure` | Prozessverlust, Telemetriefehler, nicht retrybarer Input-/Fensterfehler | Session stoppen und Telemetrie soweit möglich flushen. |
| `operator_stop` | Stop-Hotkey oder Context-Cancel durch Operator | Aktiven Executor canceln, keine neuen Inputs, Ergebnis `stopped`. |

Phase 7.0 erlaubt noch keinen automatischen Restart aus einem Dungeon. `run_restartable` wird erst wirksam, nachdem Phase 7.2 den Save-&-Exit-Flow aus dem betroffenen, gebundenen In-Game-Zustand isoliert live validiert hat. Vorher endet derselbe Code auf Session-Ebene terminal.

## Hard-Stuck-Vertrag

Ein Hard Stuck liegt nur vor, wenn ein zuständiger Executor seinen expliziten Fortschrittsvertrag verletzt und sein endliches lokales Recovery-Budget verbraucht hat. Reiner Zeitablauf ohne Executor-Kontext genügt nicht.

Für Route Playback umfasst der Nachweis mindestens Route-ID, Segment-ID, Punktindex, aktuelle Position, letzten bestätigten Routenpunkt, Stuck-Grenzwert, Drift, lokale Recovery-Versuche sowie erwartete und aktuelle Area.

Beim Hard Stuck gilt atomar in dieser Reihenfolge:

1. aktives Route-/Navigator-Goal canceln und Executor-Zustände resetten;
2. neue Gameplay-Inputs sperren;
3. `stuck_detected` synchron schreiben und flushen;
4. den Run mit `run_aborted`, `reason=hard_stuck` abschließen;
5. Fehlerbudget abbuchen und genau eine Recovery-Entscheidung protokollieren;
6. nur bei validiertem Exit-Precheck zu `exit_game` wechseln, sonst `failed`.

Ein fehlgeschlagener Telemetrie-Write verhindert jede folgende Recovery-Aktion. Ein bereits ausgeführter Input kann nicht rückgängig gemacht werden.

## Konfigurationsvertrag

Phase 7.5 implementiert das in Phase 7.0 reservierte Schema und ergänzt den expliziten Character-Namen sowie eine exakte Liste erlaubter Retry-Klassen:

```yaml
session:
  enabled: false
  run: countess
  difficulty: nightmare
  max_runs: 3
  max_duration_ms: 7200000
  cooldown_ms: 3000
  max_consecutive_failures: 2
  max_total_restarts: 3
  state_timeout_ms: 30000
  exit_timeout_ms: 15000
  start_timeout_ms: 30000
```

| Key | Vertrag |
|-----|---------|
| `enabled` | Explizites Opt-in. Ohne `true` bleibt der heutige Single-Run-Modus erhalten. |
| `run` | Muss exakt einem registrierten Full Run entsprechen; isolierte `--phase`-Diagnosemodi sind ausgeschlossen. |
| `difficulty` | `normal`, `nightmare` oder `hell`; steuert die Menüauswahl, autorisiert aber kein Playback. |
| `max_runs` | Positive, endliche Obergrenze gestarteter Runs. |
| `max_duration_ms` | Positive, endliche Session-Frist. |
| `cooldown_ms` | Nichtnegative Wartezeit nach bestätigtem Exit und vor neuem Game-Start. |
| `max_consecutive_failures` | Nichtnegative Grenze aufeinanderfolgender nicht erfolgreicher Runs; Erfolg setzt den Zähler zurück. |
| `max_total_restarts` | Nichtnegative Obergrenze automatisch angeforderter Game-Restarts nach Fehlern. Normale Übergänge nach erfolgreichen Runs zählen nicht als Fehler-Restart. |
| `state_timeout_ms` | Positive Frist für rein beobachtende Lifecycle-Zustände. |
| `exit_timeout_ms` | Positive Gesamtfrist für den validierten Exit-Flow. |
| `start_timeout_ms` | Positive Gesamtfrist für Auswahl, Loading und In-Game-Bestätigung. |

`max_runs` und `max_duration_ms` sind beide erforderlich; die zuerst erreichte Grenze beendet die Session planmäßig. Der MVP kennt keinen Wert für „unbegrenzt“. Alle Werte werden vor Prozess-Attach und Input validiert. CLI-`--run` und `runs.active` bleiben Single-Run-Auswahl; der spätere Session-Modus erhält ein explizites, gegenseitig exklusives Opt-in. `--session-max-runs N` überschreibt bei positivem `N` nur für den aktuellen Prozess die endliche Session-Anzahl; das Phase-9-Gate verwendet `--session-max-runs 1`, ohne die lokale YAML zu verändern.

## Budget-Semantik

- Jeder Zustand besitzt genau eine monotone Deadline; Poll-Ticks verlängern sie nicht.
- Lokale Retries und Session-Restarts sind getrennte Budgets.
- Ein normal erfolgreich beendeter Run verbraucht keinen Fehler-Restart.
- Ein vor dem ersten Run-Input erkannter Kontextfehler verbraucht kein Run-Budget, endet aber terminal.
- Ein gestarteter Run erhöht den Run-Zähler genau einmal, unabhängig von `success`, `aborted` oder `failed`.
- `max_consecutive_failures` zählt `aborted` und `failed`, nicht `stopped`.
- `max_duration_ms` verhindert spätestens in `evaluate` oder `cooldown` einen weiteren Game-/Run-Start. Läuft die Frist während `run_active` ab, wird keine Input-Aktion unterbrochen; der aktuelle atomare Tick endet, der Run wird mit `reason=session_duration_exceeded` kontrolliert abgebrochen und die Session ohne neues Spiel `completed`.
- Bei gleichzeitig erreichten Grenzen gilt: Stop → terminaler Fehler → Zeitlimit → Run-Limit → Restart.
- Kein Budget darf durch Reattach, Pause/Resume oder einen neuen `tasks.Runner` zurückgesetzt werden; nur ein neuer Session-Start erzeugt neue Session-Budgets.

## Input-Invarianten

1. Kein Lifecycle-Input ohne gebundenes D2R-Fenster, passenden Prozess, aktivierten Input und unterstützte Client-Geometrie.
2. Kein Input bei Loading, inkonsistentem Snapshot, unbekanntem UI-Zustand, Pause oder Stop.
3. `GamePhaseMenu` allein autorisiert keinen Klick.
4. Jede UI-Aktion folgt `stabiler Zustandsnachweis → genau eine geloggte Aktion → bestätigtes Ergebnis oder Timeout`.
5. Zwischen Aktion und Ergebnisbestätigung wird keine zweite Lifecycle-Aktion ausgelöst.
6. Feste Koordinaten sind client-relativ, versioniert und im MVP ausschließlich für exakt 1280×720 zugelassen.
7. Save & Exit startet nach dem ersten validen Rogue-Encampment-Snapshot mit stabiler Character Identity genau ein nicht zurückgesetztes Drei-Sekunden-Settle-Fenster. Spielerbewegung oder ein nach dem Wegpunktwechsel stale gemeldetes Town-UI verlängern es nicht. Danach öffnet Escape das Quit-Menü; erst dessen stabile Memory-Bestätigung autorisiert den Save-&-Exit-Klick. Normaler Queue-Abschluss und kontrollierter Retry-Rückweg verwenden exakt diese gemeinsame Routine.
8. Difficulty-Auswahl benötigt einen bestätigten Difficulty-Dialog; die konfigurierte Difficulty ist kein Layoutnachweis.
9. Character Identity, Game-Version, Rogue Encampment und Layout-Fingerprint werden nach jedem neuen Spiel erneut geprüft.
10. Stop sperrt unmittelbar neue Inputs und cancelt aktive Goals; Cleanup darf selbst keinen Gameplay-Input erzeugen.
11. Jeder Input wird mit Session-, Game-, Run-, Zustand-, Aktion- und Begründungskontext strukturiert geloggt.
12. Kein Lifecycle-Code schreibt D2R-Installations- oder Savegame-Dateien.

Der installierte Core schreibt seine strukturierten Laufzeitlogs nach `%LOCALAPPDATA%\D2ROfflineFarmingBot\logs`. Während Save & Exit wird einmal pro Sekunde der aktuelle Automatenschritt samt Phase, Area, Position, UI-Flags, Escape-Zähler und Laufzeiten geloggt; ein Timeout enthält denselben Diagnosezustand.

## Telemetrievertrag

Phase 7 erweitert das vorhandene JSONL-Schema additiv. Vorgesehene Lifecycle-Events:

| Event | Mindestfelder |
|-------|---------------|
| `session_started` | `session_id`, aufgelöste Run-Auswahl und Budgetgrenzen |
| `game_started` | `session_id`, `game_id`, Character Identity, Difficulty-Label, Version |
| `run_started` | alle IDs, Run-Name, ordinaler Run-Zähler |
| `run_context` | Definition, Route/Fingerprint, Waypoint-Ziel, Loot-Policies und Town-Herkunft der frischen Run-Generation |
| `stuck_detected` | alle IDs plus Route-/Segment-/Punkt- und Fortschrittskontext |
| `run_completed` | alle IDs, Ergebnis `success`, Dauer, letzter Step |
| `run_aborted` | alle IDs, Ergebnis `aborted`, stabiler Reason-Code, Dauer, letzter Step |
| `run_failed` | alle IDs, Ergebnis `failed`, stabiler Reason-Code, Dauer, letzter Step |
| `game_exit_requested` | alle IDs, Grund und bestätigter Ausgangszustand |
| `game_exited` | alle IDs und Dauer des Exit-Flows |
| `game_restart_requested` | alle IDs, Reason-Code und verbleibende Fehlerbudgets |
| `session_completed` / `session_stopped` / `session_failed` | `session_id`, terminaler Grund, Summen und Laufzeit |

Jedes Ereignis wird synchron geflusht. Ein terminales Session-Ereignis enthält mindestens: gestartete, erfolgreiche, abgebrochene und fehlgeschlagene Runs; normale Game-Wechsel; Fehler-Restarts; aufeinanderfolgende Fehler; Gesamtdauer.

## Abhängigkeiten und Grenzen

- Der Lifecycle verwendet den bestehenden `tasks.Runner` über eine frische Instanz pro Run; er kennt keine Countess-Step-Namen.
- Run-spezifische Reason-Codes werden über eine explizite Tabelle klassifiziert, nicht innerhalb von `internal/tasks` mit Session-Entscheidungen vermischt.
- Phase 7.1 erforscht Quit-Menü, Charakterbildschirm und Difficulty-Dialog read-only. Erst danach dürfen Phase 7.2/7.3 UI-Input implementieren.
- Enges Template Matching für einen Charakterbildschirm-Anker ist nur zulässig, wenn Memory keinen eindeutigen Nachweis liefert. Gameplay bleibt Memory-basiert.
- D2R-Prozessstart, Crash-Restart, Death-Recovery, Online-Modus und beliebige Auflösungen bleiben außerhalb von Phase 7.

## Historischer Phase-7-Zyklus

Phase 7 führte pro Run einen vollständigen Start-/Run-/Exit-Zyklus aus. Diese Komposition war für den damaligen Einzelrun korrekt, wurde aber in Phase 11 vollständig entfernt, weil eine Queue mehrere frische Runs innerhalb desselben Spiels ausführt. Die weiterhin gültigen Verify-, Run-, Town- und Exit-Bausteine werden nun ausschließlich durch `RuntimeQueueRunner` und `SessionSupervisor` komponiert.

## Game- und Run-Verifikation Phase 7.6

Vor jedem Game beginnt der Orchestrator mit `cycle_reset_requested`. Die reale Runtime-Barriere setzt Navigator, Waypoint-, Town-Portal-, Town-Walk-, Personal-Stash-, Combat-, Loot- und Route-Adapter zurück und invalidiert anschließend das World Model. Ein fehlender Resetter oder World-Reset ist terminal; erst nach erfolgreicher Barriere darf Start beziehungsweise Game-Verifikation beginnen.

`sessionGameVerifier` besitzt eine monoton steigende Game-Generation. Nach jedem Reset verlangt er drei zeitlich strikt neue World-Snapshots mit:

- `in_game`, valider und bestätigter Character Identity;
- exakt dem konfigurierten Character und der erwarteten D2R-Version;
- Rogue Encampment als Startgebiet;
- geschlossenem Inventory, Stash und Quit-Menü.

Loading oder vorübergehend invalide Snapshots setzen nur den lokalen Stability-Zähler zurück. Falscher Character, Version, Area, offene UI oder wiederverwendete Snapshot-Zeitstempel sind fail-closed. Stability-Ticks aus Game N können Game N+1 nicht bestätigen.

Der Route-Layout-Nachweis bleibt bewusst ein zweites Gate: Der aufgezeichnete Fingerprint beginnt in Black Marsh und kann nicht seriös im Rogue Encampment berechnet werden. Beim Erreichen des ersten Routenpunkts baut der Route-Adapter den Fingerprint aus dem aktuellen Game neu auf und führt `ValidateRoutePrecheck` aus, bevor ein `RoutePlayer` beziehungsweise Navigator-Goal entstehen kann. Character, Klasse, Game-Version, Area, Startdistanz und Hash müssen passen.

Eine frische Run-Instanz wird weiterhin ausschließlich über `NewRun` erzeugt. Damit gehören Task-Ergebnis, Route Player, Navigator-Ziele, Loot-Skip-Zustände und der spätere Run-Telemetrie-Recorder genau einem Zyklus; Phase 7.7 ergänzt dafür die korrelierten Session-/Game-/Run-IDs.

## Recovery-Policy und Telemetrie

Route-Fehler werden weiterhin auf die stabilen Codes `hard_stuck`, `route_drift_exceeded`, `route_segment_timeout` oder `route_transition_failed` gemappt. Der Supervisor hält die harten Run-, Dauer-, Fehler- und Restart-Budgets; Retry bleibt auf dem aktuellen Queue-Index und beginnt über eine kontrollierte Spielgrenze neu. Unbekannte Gründe sind terminal.

Der Schema-v2-Recorder korreliert `session_id`, `game_id` und `run_id` und flusht jedes Ereignis synchron. Ein Telemetriefehler blockiert die folgende Lifecycle-Aktion fail-closed.

## Historischer Phase-7-Multi-Run

Der frühere `sessionMultiRunner` startete für jeden Run ein neues Spiel. Er wurde im Phase-11-Abschlussaudit zusammen mit dem alten Cycle- und Recovery-Stack entfernt. Der Live-Nachweis vom 12.07.2026 bleibt als historische Einzelrun-Charakterisierung erhalten, ist aber nicht mehr die Queue-Semantik.

## Long-lived SessionSupervisor Phase 11.1

`app.SessionSupervisor` ist die thread-sichere Command-Grenze oberhalb des unveränderten Session-Workers. Er besitzt genau eine cancellable Worker-Generation, monotone Generationen, immutable Snapshots und eine pro Prozess idempotente Command-ID-Tabelle. Dieselbe Command-ID mit identischem Inhalt liefert das ursprüngliche Ergebnis; Wiederverwendung mit anderem Inhalt wird abgewiesen. Mutationen gegen eine veraltete Generation liefern `state_changed`, parallele Starts `command_conflict`.

Der Supervisor hält `pause_after_run` und `stop_after_run` ausschließlich als Intent während `running_run`. Nach dem sicheren Town-Handoff bleibt Pause im aktuellen Spiel und `resume` revalidiert diesen Kontext; Stop führt über den zentralen Lifecycle zu Save & Exit. `emergency_stop` setzt zuerst `cancelling`, cancelt den Worker und veröffentlicht nach dessen Ende `emergency_stop_requested`; dieser Reason ist mit F11 identisch. Ein Worker-Panic wird an der Goroutine-Grenze abgefangen und als `worker_panic` in `stopped_error` projiziert.

Dashboard und autonome CLI starten beide über `SessionSupervisor.StartQueue`. Die CLI baut den Plan aus der YAML-Queue und beginnt am vorbereiteten Charakterbildschirm; das Dashboard kann zusätzlich einen bereits bestätigten `idle_in_game`-Kontext übernehmen. Beide verwenden denselben `RuntimeQueueRunner`, dieselbe Run-Pipeline und denselben Exit-Owner.

## FarmQueue-Scheduler Phase 11.7

Der Phase-11-Supervisor besitzt zusätzlich eine immutable, duplikatfreie Runtime-Queue mit Index, Spielzyklus, Retry und den YAML-authoritativen Run-, Dauer-, Failure- und Restart-Budgets. Die Queue beschreibt mehrere Runs innerhalb eines Spiels: Jeder Eintrag erhält frischen Run-Zustand, während Game-ID und bestätigter Kontext bis zum Wrap stabil bleiben. Erst der letzte Eintrag, geordneter Stop, Budgetende oder sichere Recovery führt über den zentralen Game-Lifecycle zu Save & Exit. Retry behält den Index; terminale Ergebnisse stoppen. Vor jedem Folgestart führt `FarmQueueGuard` den vollständigen Preflight erneut aus, sodass geänderte Availability vor Worker und Input sperrt.

## Dashboard-Controls Phase 11.8

Der lokale API-Backend serialisiert Selection- und Session-Commands und bindet sie an monotone Core-Generationen. Wiederholte `command_id` mit identischem Namen, Generation und Payload liefern exakt die gespeicherte Antwort; eine Wiederverwendung mit anderem Inhalt wird vor dem Supervisor abgewiesen. Start validiert die vollständige Queue erneut gegen bestätigte Auswahl und aktuelle Katalogrevision und beendet den passiven Monitor, bevor ein produktiver Worker attachen darf.

`RuntimeQueueRunner` erzeugt für jeden Eintrag eine frische, auf die Run-ID konfigurierte Runtime. Ein aus `idle_in_game` gestarteter Queue-Lauf übernimmt das bereits bestätigte Spiel; folgende Einträge laufen nach ihrem sicheren Town-Handoff im selben Spiel. Erst Queue-Wrap, geordneter Stop, Budgetende oder sichere Recovery führen zu Save & Exit. Jeder Worker verwendet dieselbe Run-State-Machine, Route, Profil, Loot und Town-Rückkehr aus Phase 10. Ein retrybarer terminaler Run liefert `retry_current`; andere Fehler stoppen. F11 im Worker wird als `emergency_stop_requested` an denselben Supervisor-Abschluss wie der API-Emergency-Command übergeben.

Der Statussnapshot projiziert aktive und effektive persistente Charakter-Queue getrennt. Das Dashboard startet diese Core-autoritativ gespeicherte Reihenfolge read-only; Änderungen erfolgen ausschließlich über die revisionierten Operator-Einstellungen. Pending Intent, aktiver Run, Step, Index, Zyklus, Retry und Budgets stammen ausschließlich aus Core-Snapshots. Passive Monitor-Ticks dürfen diese Supervisor-Felder nicht überschreiben.

`session.queue` liefert die duplikatfreie Startreihenfolge. Die Queue bleibt prozesslokal; ein Neustart lädt YAML und beginnt bei Index 0. Der CLI-Aufruf verwendet dieselbe Queue-, Supervisor- und Lifecycle-Pipeline wie das Dashboard.

## Abnahme Phase 7.0

Phase 7.0 ist abgeschlossen, wenn Zustände und Übergänge, unterstützte Startzustände, Run-/Session-Ergebnisse, Hard-Stuck-Semantik, fail-closed Fehlerklassen, endliche Budgets, Input-Invarianten sowie Telemetrie- und Korrelationsverträge festgelegt sind. Phase 7.1 muss danach ohne offene Architekturentscheidung mit read-only UI-State-Forschung beginnen können.

## Verwandte Features

- [Task Runner](task-runner.md)
- [Run-Telemetrie](run-telemetry.md)
- [Read-only Game Identity](game-identity.md)
- [Offline-Difficulty-Auswahl](offline-difficulty-selection.md)
- [Route Recording und Playback](route-recording-playback.md)
- [Input Controller](input-controller.md)
- [Phase-11-Core-Vertrag](phase-11-core-contract.md)

---
*Zuletzt aktualisiert: 2026-07-17 (Phase 11.10 Abschlussaudit)*
