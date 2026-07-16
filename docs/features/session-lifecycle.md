# Session-Lifecycle

## Überblick

Phase 7 führt einen generischen Session-Lifecycle oberhalb des bestehenden Task Runners ein. Eine Session führt endliche Folgen registrierter Farming-Runs über mehrere Offline-Spiele aus. Der Lifecycle entscheidet ausschließlich über Spielverifikation, Run-Start, Run-Auswertung, Save & Exit, Offline-Game-Start, Cooldown und Session-Ende. Navigation, Kampf, Loot und Stash bleiben in ihren bestehenden Paketen.

Phase 7.0 definiert den Vertrag und das Sicherheitsmodell. Sie implementiert noch keine Menüerkennung, UI-Klicks oder Multi-Run-Schleife. Die folgenden Slices dürfen den Vertrag nur mit dokumentierter Verhaltensänderung erweitern.

Phase 7.4 implementiert den generischen Einzelzyklus-Orchestrator hinter mockbaren Executor-Grenzen. CLI, Session-Konfiguration, Budgetverwaltung und die produktive Multi-Run-Schleife folgen weiterhin in Phase 7.5 und späteren Slices.

## Ort im Code

- **Orchestrierung:** `internal/app/session_cycle.go`
- **Bestehender Run-Vertrag:** `internal/tasks/result.go`, `internal/tasks/runner.go`, `internal/tasks/registry.go`
- **Konfiguration und Inspect:** `internal/config/session.go`, `internal/app/session_plan.go`
- **Zukünftige Telemetrie:** `internal/telemetry/`
- **Input- und Prozessgrenzen:** `internal/input/`, `internal/process/`
- **Konzeptplan:** [`phase-7-implementation-plan.html`](../../phase-7-implementation-plan.html)

## Begriffe und Identitäten

| Begriff | Bedeutung | Lebensdauer |
|---------|-----------|-------------|
| Session | Eine vom Operator gestartete, endliche Multi-Run-Ausführung. | Vom Preflight bis `stopped`, `completed` oder `failed`. |
| Game | Genau eine bestätigte Offline-In-Game-Instanz zwischen Game-Start und Save & Exit. | Innerhalb einer Session. |
| Run | Genau eine frische Ausführung eines Namens aus `tasks.KnownRuns()`. | Innerhalb eines Game; Phase-7-MVP führt einen Full Run pro Game aus. |
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
7. Save & Exit benötigt ein stabiles Quit-Menü-Gate; es gibt keinen optimistischen Blindklick.
8. Difficulty-Auswahl benötigt einen bestätigten Difficulty-Dialog; die konfigurierte Difficulty ist kein Layoutnachweis.
9. Character Identity, Game-Version, Rogue Encampment und Layout-Fingerprint werden nach jedem neuen Spiel erneut geprüft.
10. Stop sperrt unmittelbar neue Inputs und cancelt aktive Goals; Cleanup darf selbst keinen Gameplay-Input erzeugen.
11. Jeder Input wird mit Session-, Game-, Run-, Zustand-, Aktion- und Begründungskontext strukturiert geloggt.
12. Kein Lifecycle-Code schreibt D2R-Installations- oder Savegame-Dateien.

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

## Implementierung Phase 7.4

`sessionCycleOrchestrator` führt genau einen generischen Zyklus aus und kennt weder Countess-Schritte noch konkrete UI-Koordinaten. Ein `sessionCycleDriver` stellt folgende Grenzen bereit:

- `AwaitReady` sperrt jede neue Lifecycle-Aktion während Pause oder Stop.
- `StartGame` und `ExitGame` werden später durch die validierten Phase-7.3-/7.2-Executors erfüllt.
- `VerifyGame` kapselt Loading sowie die stabile Character-, Area-, Version- und Layout-Prüfung.
- `NewRun` liefert für jeden Zyklus zwingend eine frische Run-Instanz.
- `EmitLifecycle` muss ein Event synchron veröffentlichen; ein Fehler blockiert die folgende Aktion.

Nach jedem terminalen Run-Ergebnis wird der Run-Executor vor einem möglichen Game-Exit zurückgesetzt. Die äußere Recovery-/Multi-Run-Schicht bildet `success`, `failed` und `aborted` anschließend auf `run_completed`, `run_failed` und `run_aborted` ab. Bei Operator-Stop wird der aktive Executor zurückgesetzt, aber keine neue Exit- oder Start-Aktion begonnen. Ein Run-Fehler darf einen Exit anfordern; ob der aktuelle In-Game-Zustand sicher verlassen werden kann, entscheidet allein der Exit-Executor fail-closed.

Die fake-basierten Integrationstests decken drei erfolgreiche Zyklen mit drei verschiedenen Run-Instanzen, Run-Fehler/Hard-Stuck, Reset-vor-Exit, Pause/Resume, Stop ohne Folgeinput, Loading-/Verify-Timeout, aktiven In-Game-Start sowie Telemetriefehler vor Input ab.

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

## Recovery-Policy und Telemetrie Phase 7.7

Phase 7.7 implementiert die exakte Klassifikation und harte Budgets aus diesem Vertrag. Route-Fehler werden per Sentinel auf `hard_stuck`, `route_drift_exceeded`, `route_segment_timeout` oder `route_transition_failed` gemappt. Nur zusätzlich in `session.retry_classes` erlaubte Codes dürfen einen Restart anfordern; unbekannte oder ähnlich benannte Gründe sind terminal.

Der Schema-v2-Session-Recorder korreliert `session_id`, `game_id` und `run_id` und flusht jedes Ereignis synchron. Ein Hard Stuck erzeugt zwingend `stuck_detected` mit Route-/Segment-/Punkt-/Drift-Kontext, danach `run_aborted` und anschließend höchstens ein `game_restart_requested`. Jeder Telemetriefehler stoppt diese Sequenz vor der folgenden Recovery-Aktion. Terminale Session-Events enthalten Ergebniszähler, Fehlerfolge, Restarts und Gesamtdauer.

## Multi-Run-Kern Phase 7.8

`sessionMultiRunner` konsumiert den generischen Einzelzyklus und führt ausschließlich endliche Sessions aus. Er schreibt `session_started`, erzeugt pro Ordinal eindeutige Game-/Run-IDs, wertet jedes Run-Ergebnis über die Phase-7.7-Policy aus, erzwingt Cooldown und beendet bei `max_runs`, `max_duration`, Stop, terminalem Fehler oder Telemetriefehler mit genau einem Summary-Event.

Drei fake-basierte Zyklen beweisen frische IDs und die Sequenz Initial-Game → Exit → neuer Game-Start. Ein injizierter Hard Stuck fordert innerhalb des Budgets genau einen Restart an; ein unbekannter Fehler beendet die Session.

Die produktive Freigabe erfolgte am 12.07.2026 mit Session `session-20260712T020655.472818000Z-880ddc70`: drei aufeinanderfolgende autonome Nightmare-Countess-Zyklen einschließlich Spielstart, Navigation, Kill, Loot, Town Portal, Stash und Save & Exit endeten erfolgreich. Die Route enthält dafür einen terminalen Keller-5-Abschnitt bis zum Countess-Raum.

## Long-lived SessionSupervisor Phase 11.1

`app.SessionSupervisor` ist die thread-sichere Command-Grenze oberhalb des unveränderten Session-Workers. Er besitzt genau eine cancellable Worker-Generation, monotone Generationen, immutable Snapshots und eine pro Prozess idempotente Command-ID-Tabelle. Dieselbe Command-ID mit identischem Inhalt liefert das ursprüngliche Ergebnis; Wiederverwendung mit anderem Inhalt wird abgewiesen. Mutationen gegen eine veraltete Generation liefern `state_changed`, parallele Starts `command_conflict`.

Der Supervisor hält `pause_after_run` und `stop_after_run` ausschließlich als Intent während `running_run`. Erst das terminale Ergebnis der vollständigen Session-Einheit wechselt nach `paused_between_runs` beziehungsweise `idle`. `resume` erzeugt eine frische Worker-Generation. `emergency_stop` setzt zuerst `cancelling`, cancelt den Worker und veröffentlicht nach dessen Ende `emergency_stop_requested`; dieser Reason ist mit F11 identisch. Ein Worker-Panic wird an der Goroutine-Grenze abgefangen und als `worker_panic` in `stopped_error` projiziert.

Der bestehende CLI-Sessionpfad startet seinen bisherigen Runtime-Worker jetzt über `SessionSupervisor.Start` und wartet über `Wait`. Offline-Start, Run-Polling, Save & Exit und Cooldown akzeptieren dabei den Supervisor-Context. Die fachliche Phase-7-/Phase-10-Pipeline wurde nicht dupliziert und keine produktive Input-Reihenfolge geändert.

## FarmQueue-Scheduler Phase 11.7

Der Supervisor besitzt nun zusätzlich eine immutable Runtime-Queue mit Index, Zyklus, Retry und den YAML-authoritativen Run-, Dauer-, Failure- und Restart-Budgets. `StartQueue` startet pro Eintrag genau einen frischen `SupervisorRunner`; Erfolg schaltet modulo Queue-Länge weiter, Retry behält Index und Zyklus, terminale Ergebnisse stoppen. Vor jedem Folgestart kann ein `FarmQueueGuard` den vollständigen Preflight erneut ausführen, sodass eine zwischen Runs geänderte Availability vor Worker und Input sperrt.

## Dashboard-Controls Phase 11.8

Der lokale API-Backend serialisiert Selection- und Session-Commands und bindet sie an monotone Core-Generationen. Wiederholte `command_id` mit identischem Namen, Generation und Payload liefern exakt die gespeicherte Antwort; eine Wiederverwendung mit anderem Inhalt wird vor dem Supervisor abgewiesen. Start validiert die vollständige Queue erneut gegen bestätigte Auswahl und aktuelle Katalogrevision und beendet den passiven Monitor, bevor ein produktiver Worker attachen darf.

`RuntimeQueueRunner` erzeugt für jeden Eintrag eine frische, auf die Run-ID konfigurierte Runtime. Nur der erste Worker eines aus `idle_in_game` gestarteten Queue-Laufs konsumiert das bereits bestätigte Spiel. Nach Save & Exit starten folgende Einträge jeweils ein neues Spiel. Jeder Worker verwendet unverändert Run-State-Machine, Route, Profil, Loot, Town und Offline-Exit aus Phase 10. Ein retrybarer terminaler Run liefert `retry_current`; andere Fehler stoppen. F11 im Worker wird als `emergency_stop_requested` an denselben Supervisor-Abschluss wie der API-Emergency-Command übergeben.

Der Statussnapshot projiziert aktive und YAML-Default-Queue getrennt, damit „Auf YAML-Default zurücksetzen“ auch nach einem verworfenen Runtime-Entwurf funktioniert. Pending Intent, aktiver Run, Step, Index, Zyklus, Retry und Budgets stammen ausschließlich aus Core-Snapshots. Passive Monitor-Ticks dürfen diese Supervisor-Felder nicht überschreiben.

`session.queue` liefert die Startreihenfolge und erlaubt Duplikate. Die Queue bleibt prozesslokal; ein Neustart lädt YAML und beginnt bei Index 0. Der bisherige CLI-Adapter bleibt ein expliziter Einzel-Worker-Aufruf derselben Supervisor-Grenze und erzeugt keine konkurrierende Queue-Pipeline.

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
*Zuletzt aktualisiert: 2026-07-16 (Phase 11.8 Dashboard-Controls)*
