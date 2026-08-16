# Runtime-Trace und Headless Replay

## Überblick

Der Runtime-Trace zeichnet die fachlichen Entscheidungen eines explizit gestarteten Offline-Runs als begrenztes, komprimiertes Diagnosebundle auf. Er dient dazu, reale Fehler später ohne D2R-Prozess, Fenster oder OS-Input reproduzierbar zu untersuchen.

Die Aufzeichnung ist strikt opt-in. Sie beobachtet den bestehenden Task-Lauf, besitzt selbst keine Input-Schnittstelle und darf weder Aktionen auslösen noch Laufentscheidungen verändern.

## Motivation

Pakettests decken einzelne Zustände gut ab. Die zuletzt schwierigen Fehler waren überwiegend zeitlich: jeder Snapshot konnte für sich plausibel sein, während Reihenfolge, ein dazwischenliegender Input oder ein verlorenes Gate den Gesamtfluss beschädigte. Run-Telemetrie erklärt Ergebnisse, kann die Entscheidungspipeline aber nicht erneut ausführen.

Der Engineering-Hebel ist deshalb ein versionierter Trace echter Offline-Läufe plus ein headless Replay derselben produktiven `tasks.Runner`-Pipeline gegen aufgezeichnete Dependency-Antworten. Line-Coverage ersetzt das nicht. Der Replay beweist den verlassenen Fehlerpfad; eine neue Spielreaktion braucht anschließend einen gezielten Live-Lauf.

## Ort im Code

- **Paket:** `internal/replay/`
- **Einstieg:** `replay.NewRecorder`, `replay.InstrumentDeps`
- **Runtime-Integration:** `internal/app/runtime_trace.go`, `internal/app/run_tick.go`
- **CLI:** `cmd/d2rbot/main.go`
- **Artefakte:** `diagnostics/runtime-traces/*.trace.gz`

## Funktionalität

### Opt-in-Aufzeichnung

`--runtime-trace-capture <label>` ist nur zusammen mit einem expliziten vollständigen `--run <id>` zulässig. Phasen-, Desktop-, Session-, Inspect- und sonstige Testmodi sind ausgeschlossen. Das Label ist auf kleine Dateinamen aus Kleinbuchstaben, Ziffern und Bindestrichen begrenzt.

Vor jedem produktiven Task-Tick friert der Recorder den interpretierten `world.State`, die monotone Laufzeit und Snapshot-Generation, den Task-Zustand vor und nach dem Tick, die Input-Safety-Gates sowie geordnete Dependency-Ergebnisse und semantische Input-Intents ein.

Die Dependency-Wrapper leiten jeden Aufruf genau einmal und unverändert weiter. Sie führen keine Retries aus und erzeugen keine eigenen Aktionen.

### Terminales Bundle

Fehler, Prozessverlust und Operator-Stopp schließen den Trace ab. Erfolgreiche Runs erzeugen standardmäßig kein Bundle. Ein Fehlerbundle wird per Staging-Datei, `Sync` und atomarem Rename veröffentlicht. Ringpuffer, maximale Bundlegröße, Dateianzahl und Gesamtgröße begrenzen Speicher- und Plattenverbrauch; die Retention verwaltet ausschließlich reguläre `*.trace.gz`-Dateien.

### Datenschutz und Safety

Der Trace enthält nur semantische World-Felder. Insbesondere fehlen Memory-Pointer, Modulbasen, Prozess-Handles, `MapSeed`, rohe Item-Locations, Flags, rohe Item-Stats, Savegame-Daten und lokale Benutzerpfade. Vertrags- und Dependency-Maps werden vor der Aufnahme tief kopiert und redigiert. Vor dem Schreiben prüft eine zweite JSON-Sicherheitsvalidierung das vollständige Bundle fail-closed.

## Datenmodell

- `replay.Bundle`: versionierter Container mit Metadaten, Vertrags-Snapshot, Checkpoints, Frames und terminalem Ergebnis.
- `replay.Frame`: ein produktiver Tick mit World-Projektion, Gates, Dependency-Aufrufen, Intents und Task-Zuständen.
- `replay.ContractSnapshot`: unveränderliche Run-, Profil-, Routen-, Loadout- und Tuningwerte der Ausführung.
- `replay.WorldFrame`: normalisierte, pointerfreie Entscheidungsfläche aus `world.State`.

Schemaänderungen erfordern eine neue `replay.SchemaVersion`. Unbekannte Felder und nicht unterstützte Versionen werden beim Lesen abgelehnt.

### Headless Replay

Der Replayer baut ausschließlich den produktiven `tasks.Runner` auf. Eine Fake Clock verwendet die aufgezeichneten monotonen Tickabstände; Transcript-Adapter liefern die protokollierten Dependency-Antworten. Ein reiner Intent-Recorder beobachtet die dadurch erneut erzeugten semantischen Aktionen. Prozesssuche, Memory Reader, D2R-Fensterbindung, Hotkeys und OS-Input-Controller sind in diesem Pfad nicht erreichbar.

Jeder Dependency-Aufruf muss nach Art, Reihenfolge, Argumenten, Rückgabe und Fehler mit dem Bundle übereinstimmen. Ebenso werden Task-Step vor und nach jedem Tick sowie die Intent-Folge geprüft. Die erste Abweichung beendet den Lauf mit Tick, Step, Area, erwartetem und aktuellem Wert. Exakter Erfolg verlangt denselben terminalen Step, Outcome und Reason-Code.

Ein Trace mit abgeschnittenem detailliertem Ringpuffer wird fail-closed nicht replayt, weil der interne Pipeline-Zustand dann nicht sicher vom Runbeginn rekonstruiert werden kann.

## Operator / CLI

```powershell
.\d2rbot.exe --runtime-trace-capture focus-loss --run mephisto
.\d2rbot.exe --replay-runtime-trace diagnostics\runtime-traces\<bundle>.trace.gz
```

Das Bundle liegt bei einem installierten Datenroot unter `diagnostics/runtime-traces/`, sonst relativ zum Arbeitsverzeichnis. Der normale Stop-/Pause-Hotkey und alle bestehenden Fail-closed-Input-Gates bleiben autoritativ.

Der Replay-Modus ist mutual-exklusiv und zweigt vor dem Laden der Runtime-Konfiguration ab. Ein erfolgreicher Replay gibt einen kleinen JSON-Bericht aus; eine Divergence liefert einen Fehler und damit einen Exit-Code ungleich null.

## Simulationsgrenze

Runtime Replay reproduziert Botentscheidungen gegen die ursprünglich beobachteten D2R-Antworten. Wenn eine Korrektur erstmals einen anderen Intent erzeugt, sind spätere World-Frames des alten Laufs keine Simulation der neuen Spielreaktion. Der erste Divergence Point beweist den verlassenen Fehlerpfad; ein gezielter Live-Lauf muss die neue Reaktion anschließend bestätigen.

## Regression-Promotion

Ein relevanter realer Fehlertrace wird nicht automatisch zur Suite. Der verbindliche Prozess:

1. Operator aktiviert den Diagnosemodus für einen gezielten Offline-Lauf.
2. Der reale Fehler persistiert ein lokales, versioniertes Bundle.
3. Der Headless-Replayer reproduziert denselben Step, Intent, Outcome und Reason-Code.
4. Die Korrektur wird gegen genau diesen Trace entwickelt; der erste Divergence Point zeigt den verlassenen Fehlerpfad.
5. Ein gezielter D2R-Live-Lauf bestätigt die neue Spielreaktion.
6. Der relevante Trace wird minimiert, von lokaler Identität bereinigt und als kleines Fixture eingecheckt.

Der erste später organisch auftretende Produktfehler folgt demselben Prozess und blockiert ein Release nicht. Traces werden nie automatisch hochgeladen oder an die normale History angehängt.

## Initiale Failure-Matrix

Die Matrix wird vollständig headless durch die gezielten Pakettests ausgeführt. Die Einträge benennen den jeweils kleinsten autoritativen Nachweis; der echte Mephisto-Trace ergänzt die konstruierten Kanten um einen produktiven Integrationspfad.

| Fehlerklasse | Headless-Nachweis |
|---|---|
| Staler Snapshot während einer laufenden Auswahl | `TestCombatAdapterSkillSelectionDoesNotAuthorizeTargetHover`, `TestBossLootRepositionRetriesOnlyAfterFreshSnapshotsThenContinuesToItemScan` |
| Focus-Verlust vor einem Cast | `TestReplayReachesIdenticalTerminalFailure`, `TestProfileCorpseCastRequiresFocusAndPlayableProjection` |
| Geöffnete UI vor Folgeinput | `TestOfflineExitMachineWaitsForTownUIToCloseBeforeSettle`, `TestTownItemServiceInputGatesCainAkaraAndClosesUI` |
| Boss verschwindet oder wechselt die UnitID | `TestMephistoRejectsReplacementUnitAndConfirmsTrueAbsence`, `TestMephistoFailsWhenBossPinIsLostBeforeActionsComplete` |
| Item verschwindet oder bleibt außerhalb der Reichweite | `TestLootPickupRecoversOnceAfterHoverNotFound`, `TestLootPickupRecoveryIsBoundedToOneTeleportPerUnit` |
| Mercenary `alive → dead` | `TestObserveMercenaryDeathEmitsOnceOnAliveToDead` |
| Process Detachment | `TestRunTickLostUnbindsBeforeWorldReset`, `TestRunPipelineCentralResetBarrierClearsGenerationOnce` |
| Route-No-progress und ausgeschöpftes Recovery | `TestReplayRealOfflineMephistoHardStuckFixture`, `TestRouteThreatControllerNoProgressAndProgressBeyondTwelveSeconds`, `TestRouteRecoveryGuardStopsSecondIneffectiveInput` |
| Emergency Stop in einer offenen Multi-Tick-Interaktion | `TestAbortOpenStepEmitsFailedAndIsIdempotent`, `TestCombatAdapterResetClearsPendingSelection` |

`internal/tasks/testdata/phase22-standard-pipeline.golden` friert zusätzlich die vollständigen Step-Folgen von Countess, Mephisto, Summoner und Nihlathak sowie Loot-, Post-Kill-, Retry-Return- und Timeout-Kanten bytegenau ein.

### Permanentes Live-Fixture

`internal/replay/testdata/mephisto-live-hard-stuck.trace.gz` stammt aus einem echten Offline-D2R-Lauf. Das Bundle-Terminal bleibt `play_bound_route / failed / hard_stuck` nach 328 Ticks. Seit der produktiven `wait_entry_area`-Settle-Änderung vom 16.08.2026 divergiert der Replay am ersten geänderten Tick 82; spätere Frames werden nicht erfunden. Lokale Identitäten und für diesen Fehler nachweislich irrelevante World-Daten fehlen. Das Fixture bleibt kleiner als 32 KiB.

## Abhängigkeiten

- `internal/world` für die semantische Snapshot-Projektion,
- `internal/tasks` für Task- und Dependency-Verträge,
- Go-Standardbibliothek für JSON, Gzip und atomische Dateiveröffentlichung.

Der Recorder liest weder Prozessspeicher noch D2R-Installations- oder Savegame-Dateien.

## Verwandte Features

- [Task Runner](task-runner.md)
- [World Model](world-model.md)
- [Input Controller](input-controller.md)
- [Run-Telemetrie](run-telemetry.md)

---
*Zuletzt aktualisiert: 16. August 2026*
