# FarmQueue-Scheduler

## Überblick

Der `SessionSupervisor` führt eine eindeutige, geordnete Run-Folge innerhalb eines bestätigten Offline-Spiels aus. Countess und Mephisto teilen denselben Game-Kontext; erst nach dem letzten Eintrag, bei geordnetem Stop, Budgetende oder sicherer Recovery wird Save & Exit ausgeführt. Jeder Eintrag erhält frischen Task-/Profil-/Route-/Loot-/Town-Zustand und eine eigene Run-ID.

## Ort im Code

- **Vertrag und Preflight:** `internal/app/queue_scheduler.go`
- **Scheduler und Controls:** `internal/app/supervisor.go`
- **Game-Lifecycle und Run-Executor:** `internal/app/queue_runtime.go`
- **API-Projektion:** `internal/api/live_backend.go`
- **Config:** `session.queue` und `session.max_*`

## Funktionalität

### Eindeutiger Preflight

`ValidateFarmQueue` prüft die vollständige Queue read-only gegen einen Memory-bestätigten Charakter-/Difficulty-Kontext und eine Katalogrevision. Dieselbe Prüfung verwendet den gespeicherten Kampfprofil-Kontext des Charakters, sonst den Default seiner Klasse – nie das Necromancer-Kampfprofil als klassenlosen Paladin-Fallback. Die Liste muss nicht leer sein und darf jede registrierte Run-ID höchstens einmal enthalten. Ein Duplikat liefert `queue_duplicate_run` mit `run_id`, `first_index` und `duplicate_index`, bevor Prozessbindung, Worker oder Input entstehen. YAML-Defaults werden mit demselben Grund fail-closed abgewiesen.

Unbekannte, `stale`, `unavailable` oder kontextfremde Einträge behalten ihre präzisen Reason-Codes. `runtime_validation_required` bleibt zulässig, weil Playback vor dem ersten Routeninput live gegatet wird.

Im Desktoppfad lädt der Core vor Queue-Validierung und Queue-Start den Savekatalog frisch. Der ausgewählte Charakter muss weiterhin dieselbe Headerklasse und dasselbe in OperatorSettings gespeicherte, freigegebene Kampfprofil besitzen. Jeder angeforderte Run muss exakt dieses Profil verlangen und eine gültige Pickit-Zuordnung besitzen. Dasselbe enge Gate läuft nochmals vor jedem vom Supervisor gestarteten Queue-Eintrag. Klassen- oder Profilabweichungen stoppen vor Worker und Input; eine fehlende Pickit-Zuordnung sperrt ausschließlich den betroffenen Run.

Das beim Queue-Start eingefrorene Charakter-Loadout bleibt sowohl für die interne Session-Plan-Prüfung beim Runtime-Aufbau als auch für die erneute Run-Kontexterzeugung direkt vor der Task-Ausführung autoritativ. Profil-ID und Klasse werden an keiner dieser Grenzen erneut über den klassenlosen Necromancer-Notnagel aufgelöst. Dadurch prüft eine Hammerdin-Queue ihre Paladin-Routen durchgehend gegen `paladin_hammerdin`.

### Game-Lifecycle und Run-Executor

Der produktive Queue-Runner besitzt zwei getrennte Grenzen:

- `StartGame` startet beziehungsweise übernimmt genau ein Spiel und bestätigt Charakter, Difficulty und Rogue Encampment.
- `Run` verifiziert den Game-Kontext und führt genau einen frischen Run durch Combat, Loot und Town-Handoff aus. Diese Methode sendet niemals Play-, Difficulty- oder Save-&-Exit-Input.
- `ExitGame` ist der einzige Queue-Pfad für Save & Exit und ist nach bestätigtem Exit idempotent.
- `RevalidateGame` bestätigt beim Resume das unveränderte offene Spiel vor jedem Folgeinput.

Start und Resume aktivieren nach erfolgreicher Memory-Verifikation das gebundene D2R-Fenster über denselben gegateten `Input.Focus()`-Pfad wie die Charakterauswahl. Eine nicht bestätigte Vordergrundaktivierung stoppt vor Run-Input. Übergibt ein erfolgreicher Run den Charakter am Waypoint, erkennt der frische Folgerun den live sichtbaren Waypoint in Klickdistanz als autoritativen Startanker und versucht nicht erneut die Stash-zu-Waypoint-Kante.

Die Go-Runtime eines Eintrags darf nach dem Town-Handoff geschlossen werden; das beendet nicht das D2R-Spiel. Der `RuntimeQueueRunner` bleibt der einzige Ressourcen- und Game-Lifecycle-Owner und erzeugt für den nächsten Eintrag frischen Run-Zustand.

Der `town_ready`-Profilhook bleibt bewusst run-spezifisch: Bone Armor wird zu Beginn jedes Runs neu gewirkt. Nur die für einen echten neuen Spielstart konfigurierte Anfangsverzögerung entfällt bei einem Folgerun, weil der vorherige Run bereits einen sicheren Town-Handoff im selben bestätigten Spiel geliefert hat. Settle-Verifikation und eigentlicher Skill-Input bleiben unverändert.

### Scheduler, Wrap und Recovery

- Erfolg schaltet zum nächsten Index im selben Spiel.
- Der letzte erfolgreiche Eintrag führt genau einen Exit aus. Bei freien Budgets beginnt Index 0 mit neuer Game-ID und erhöhtem Spielzyklus.
- Retry bleibt am Index. Nur ein als sicher bestätigtes Ergebnis darf über den verifizierten Exit-Vertrag ein Recovery-Spiel beginnen; andernfalls stoppt die Queue fail-closed.
- `mercenary_died_during_run`, `combat_resource_exhausted` und `route_mana_recovery_failed` erzwingen unabhängig von der konfigurierbaren Retry-Liste genau diesen kontrollierten Rückweg. Jeder anschließend tatsächlich neu gestartete und verifizierte Spiel-Lifecycle aktiviert die Run-Readiness erneut, auch wenn derselbe Run seine Runtime-Einheit wiederverwendet. Nach Merc-Tod stellt sie den angeheuerten Merc per bestehendem Kashya-Plan wieder her; der Versuch wird nicht an einem alten Routenpunkt fortgesetzt. Same-Game-Handoffs und Pause/Resume aktivieren sie nicht erneut.
- Vor dem Portal-Cast bestätigt `retry-return` das aktuelle Routengebiet über drei frische Snapshots und drei Sekunden stabile Gebietsevidenz. Loading, Area 0 oder ein Gebietswechsel bleiben inputfrei. Dadurch startet kein Recovery-Portal mehr auf dem ersten möglicherweise gemischten Snapshot nach Waypoint- oder Gebietswechsel.
- Scheitert auch dieser Rückweg, bleibt `retry_return_failed` der terminale Code. `original_reason` bewahrt zusätzlich den produktiven Run-Fehler und `recovery_reason` die konkrete Rückkehrursache, etwa `town_portal_not_found`; beide Felder werden strukturiert im Supervisor- und API-Status projiziert.
- Terminale Ergebnisse starten keinen anderen Eintrag.
- Start aus `idle_in_game` übernimmt das Spiel nur, wenn der passive Monitor gleichzeitig Prozess, Fenster, gültiges `in_game` und Rogue Encampment bestätigt. Nach Apply allein, auf dem Charakterbildschirm oder ohne Town-Nachweis startet die Queue denselben Offline-Selector wie ein frischer `idle`-Start. Der Queue-Runner bestätigt Charakter und Startgebiet anschließend erneut über Memory, bevor Run-Input möglich ist.
- `max_runs` zählt gestartete Run-Einträge. `max_duration_ms` wird vor jedem Folgestart ausgewertet. Budgetende führt an der sicheren Run-Grenze zu einem Exit.
- Normaler Queue-Wrap erhöht den Spielzyklus, nicht den Recovery-Restart-Zähler.

### Controls

`pause_after_run` beendet Run, Loot und Town-Handoff, lässt das Spiel geöffnet und setzt den Index bereits auf den nächsten Eintrag. `resume` verlangt denselben Prozess, dasselbe Spiel, Charakter, Difficulty, Rogue Encampment und sicheren UI-Kontext. Abweichungen liefern `paused_game_lost` vor Input.

`stop_after_run` gewinnt gegen Pause und führt nach Town genau einen Exit aus. Der globale, konfigurierbare `input.stop_after_run_hotkey` setzt denselben Supervisor-Intent wie der Dashboard-Button, ohne Rendererfokus, Mid-Run-Cancellation oder Änderung des Input-Pausezustands. Standard ist F10. Emergency Stop und der separate `input.stop_hotkey` verwenden `emergency_stop_requested`, canceln unmittelbar und garantieren keinen Exit. Die globale Pause-Taste setzt ausschließlich den Supervisor-Intent; sie pausiert keine Route mitten im Input und benötigt keinen Rendererfokus.

## Datenmodell und Telemetrie

- `SupervisorSnapshot` projiziert Queue, Index, Spielzyklus, Retry, Game-ID, Run-Instanz-ID und Budgets defensiv.
- Jede Spielgeneration besitzt eine Game-ID; jeder Queue-Eintrag eine neue Run-ID.
- SSE veröffentlicht `game_started`, `run_started`, `run_finished` und `game_exited` mit denselben Korrelationen.
- Der Queue-Runner schreibt zusätzlich eine synchron geflushte Session-JSONL-Datei mit `session_started`, `game_started`, Run-Terminalevents und `game_exited`.
- Zwischen Countess und Mephisto desselben Spielzyklus existiert kein Game-Exit-Ereignis.

## Operator / CLI

```yaml
session:
  queue: [countess, mephisto]
```

Eine Einzelqueue wie `[countess]` wiederholt Countess über sauber getrennte Spiele. `[countess, countess]` ist ungültig.

Die autonome CLI liest diesen YAML-Plan über `RunConfiguredQueue` und verwendet anschließend denselben `RuntimeQueueRunner`, `SessionSupervisor.StartQueue`, Run-Executor und zentralen Exit-Owner wie das Dashboard. Sie beginnt bewusst am vorbereiteten Offline-Charakterbildschirm und übernimmt kein laufendes Spiel implizit. Ist der Savekatalog auflösbar, bindet sie denselben Klassen-Default wie der Desktop; ohne Save bleibt `necro_bone_spear` der klassenlose Notnagel.

### Live-Abnahme

Die vollständige Phase-11-Abnahme wurde am 17. Juli 2026 abgeschlossen. Pause und Resume behielten Countess und Mephisto im selben bestätigten Spiel. Der natürliche Wrap erzeugte erst nach beiden Runs genau einen Exit und begann anschließend einen neuen Spielzyklus. Der abschließende Stop-after-run-Nachweis in Session `session-20260717T162034.579726900Z-2f02744c` nahm F10 während Mephisto an, ließ Boss, Repositionierung, Loot und Town-Handoff vollständig enden und erzeugte danach genau ein `game_exited` mit `reason=stop_after_run`; ein weiterer Run oder Spielzyklus startete nicht.

## Grenzen

- Der Desktop startet ausschließlich die persistente Charakter-Queue aus den Core-eigenen Operator-Einstellungen. Explizite CLI-Testoverrides bleiben auf den Repositorybetrieb begrenzt und sind keine zweite Produktkonfiguration.
- Keine Zeitpläne, Gewichte, Zufallsauswahl oder Mehrspiel-Separatoren.
- Keine zweite Run-, Recovery-, Exit- oder Telemetrie-Pipeline.
- Queue-Preflight und Scheduling verändern keine Route-Datei.

## Verwandte Features

- [Session-Lifecycle](session-lifecycle.md)
- [Lokale Core-API](local-core-api.md)
- [Live-Dashboard](live-dashboard.md)
- [Phase-11-Core-Vertrag](phase-11-core-contract.md)

---
*Zuletzt aktualisiert: 17. August 2026*
