# Phase-17-Core-Vertrag

## Überblick

Abschnitt 17.0 friert die unveränderte v0.13.0-Baseline und den compile-nahen Vertrag für bedrohungsbewusstes Route-Playback ein. Produktiv bleibt das Verhalten in diesem Abschnitt unverändert: Der Summoner-Run besitzt noch keinen Threat-Check vor Route-Movement. Die Abschnitte 17.1 bis 17.6 dürfen erst nach bestandenem Gate 17.0 nacheinander darauf aufbauen.

Phase 17 schützt ausschließlich den produktiven Summoner-Full-Run und `--run summoner --phase play-route`. Rohes Route-Playback, Candidate-Playback, Recording, Guided Validation sowie Countess, Mephisto und Nihlathak bleiben reine Navigation beziehungsweise behalten ihre bisherige Pipeline.

## Ort im Code

- **Task-Vertrag:** `internal/tasks/phase17_contract.go`
- **Config-Vertrag:** `internal/config/phase17_contract.go`
- **Memory-Coverage:** `internal/memory/phase17_contract.go`
- **World-Coverage:** `internal/world/phase17_contract.go`
- **Ressourcenkontext:** `internal/profile/phase17_contract.go`
- **Vertragstests:** `internal/tasks/phase17_contract_test.go`
- **Produktiver Einstieg ab 17.3:** `internal/tasks/run_pipeline.go` im Step `play_bound_route`
- **Detailplan:** `docs/plans/phase-17-implementation-plan.html`

## Status und Sequenzgrenze

17.0 definiert ausschließlich Typen, Defaults, Zustände, Reasons, Ownership und Nicht-Ziele. Der Abschnitt:

- bindet `RouteCombatConfig` noch nicht in `RunConfig` oder YAML ein;
- ändert weder Input noch RoutePlayer, Memory-Enumeration oder World-Mapping;
- registriert noch keine `route_clear`-Capability;
- akzeptiert noch keinen neuen Operator-Key;
- erzeugt noch keine Phase-17-Telemetrie.

Gate 17.0 ist eine rein automatische Abschnittsprüfung der Baseline und des Vertrags. Ein Benutzer-Livetest ist dafür weder erforderlich noch vorgesehen.

## Belegte Baseline

### Automatische Matrix

Am 29. Juli 2026 wurde der unveränderte Stand vor dem ersten Phase-17-Produktivcode vollständig geprüft:

| Bereich | Ergebnis |
|---|---|
| `go test ./...` | grün |
| `go build ./cmd/d2rbot` | grün |
| Webtests | 23 Dateien, 142 Tests grün |
| Web-Typecheck und Build | Renderer, Electron und Vite grün |
| Native Electron-Tests | 10 Tests grün |
| Release-Lint | 0 Issues |
| NSIS-Release-Smoke | Install, Provisionierung, Upgrade und Uninstall grün |

Der unveränderte Releasepfad erzeugte `D2R-Offline-Farming-Bot-0.13.0-Setup.exe` mit SHA-256 `913cf50fb39c404fb26c2e92731d52a5532b523da7868bb824c71771c152876a`.

Ein erster Web-Gesamtlauf und ein erster Releaseversuch liefen jeweils mit unterschiedlichen Onboarding-Tests exakt in das bestehende pauschale Fünf-Sekunden-Testlimit. Die unveränderte gezielte Wiederholung benötigte 188 ms; die anschließend unverändert wiederholte Gesamt- und Releasematrix war vollständig grün. Das ist als bestehende Timing-Flake-Beobachtung dokumentiert, aber keine Phase-17-Verhaltensänderung.

### Produktive Summoner-Hell-Route

Die gebundene Route `summoner-mrbones-e59fe08f23` wurde read-only charakterisiert und nicht editiert:

| Merkmal | Wert |
|---|---:|
| Segmente | 1, terminal und innerhalb Area 74 |
| Punkte | 32 |
| Punktabstand Minimum | 6,32 Tiles |
| Punktabstand Mittel | 18,50 Tiles |
| Punktabstand Maximum | 23,02 Tiles |
| `waypoint_tolerance_tiles` | 3 |
| `max_drift_tiles` | 8 |
| `max_local_corrections` | 2 |
| `segment_timeout_ms` | 30000 |
| `transition_timeout_ms` | 10000 |

Der Route-Hash lautet `8fa2905b9a250b5a8f2081ea6b6f7e91bafdb8cc409f7e5690593bcc95517254`.

### Reale Telemetriebelege

Die Belege liegen im installierten Datenroot unter `logs/telemetry/`. Sie werden nicht ins Repository kopiert.

| Run-Datei | SHA-256 | Beleg |
|---|---|---|
| `summoner-20260728t203757999999999z-c13a2983.jsonl` | `4f47c9732f04bcc7343dfa443ebb0d4b86bfa0bd48c961f47db46cf389a97cac` | `route_segment_timeout`; Route-Start bis Fehler 30.087,608 ms |
| `summoner-20260728t210239999999999z-f4551475.jsonl` | `a03e2a25020601d7b382c384b7cf01ae53e33e1f20210b0e29d30b978b701692` | `route_segment_timeout`; Route-Start bis Fehler 30.089,157 ms |
| `summoner-20260728t205833999999999z-e9936446.jsonl` | `fffcba981adcfce17c62ac2b85c61af33211e292d4d91415d15583982aad145d` | Rejuvenation Unit 193 in Slot 4 angefordert; 1.471,947 ms später `potion_verify_timeout` im Step `play_bound_route` |
| `summoner-20260728t210020999999999z-e529a68f.jsonl` | `61865ccb1a1c25465dac8994223e570f916e5a3f111a74aecc7b453600bd4886` | erfolgreiches Route-Playback in 22.341,374 ms |

Die beiden Segmenttimeouts belegen das konfigurierte 30-Sekunden-Budget mit erwarteter Poll-Quantisierung. Sie belegen keinen konkreten Stuck-, Punkt- oder Recovery-Ablauf. Eine weitergehende „Stuck“-Behauptung ist ohne Positions-, Point- und Recovery-Ereignisse ausdrücklich unzulässig.

Die Telemetrie zeigt außerdem, dass Ressourcenverifikation und Route im selben äußeren Step liegen. Der aktuelle `Runner` kehrt bei `profile.StatusPending` vor der Run-State-Machine zurück. Damit kann passive Verifikation den Route-Step blockieren; 17.4 darf diesen Vertrag nur für den opt-in Route-Step öffnen.

Die aktuelle Route-Telemetrie enthält keine einzelnen Routepunkte oder lokalen Recovery-Ziele. Die gefährliche Recovery-Ausgangslage ist deshalb nicht als Live-„Stuck“ behauptet, sondern durch den produktiven Routevertrag mit zwei Korrekturen und den bestehenden Test `TestRouteSegmentPlayerDriftRecoversToPreviousPoint` charakterisiert: Bei Drift wird der vorherige bestätigte Punkt ohne Threat-Kontext als Ziel verwendet. 17.1 muss dieses tatsächliche Ziel read-only offenlegen; 17.5 schützt es vor einem zweiten wirkungslosen Cast.

## Ownership

| Verantwortung | Einziger Owner | Harte Grenze |
|---|---|---|
| Living-Monster und Hover | `memory` → `world` | Keine Pixelprüfung, kein Task-eigener Process-Read, kein Living-Cache |
| Reservoir und Coverage | `memory` → `world` | 512 nicht priorisierte Kandidaten plus unverdrängte Priority-Einheiten |
| erlaubte Route-Hostiles | `tasks.RunDefinition` | Keine rohe NPC-ID-Liste in YAML oder UI |
| Threat-Geometrie und Zustand | `internal/tasks` | Genau ein Assessment pro `world.State.At` |
| Build-spezifischer Clear | `internal/profile` | Kein Klassen- oder Skill-Switch in Tasks, Route oder Pathing |
| Routefortschritt und Recovery-Ziel | `internal/pathing` | Read-only Projektion, keine Rekonstruktion in Tasks |
| Deadline-Hold | App-Route-Adapter | Kein zweiter Task-Timeout |
| Skill, Aim, Hover und Klick | bestehende `CombatActions` | Kein direkter Input im Controller oder Profil |
| Trankwahl und Verifikation | `internal/profile` | Bestehender `profile.Result.Status` bleibt Tick-Autorität |

`Phase17ContractOwners` hält diese Grenzen compile-nah fest.

## Datenmodell

### RouteProgress

`tasks.RouteProgress` projiziert Route-ID, Segment und Punkt, den letzten bestätigten Punkt, das tatsächliche Movement-/Recovery-Ziel, Modus, Drift und verbrauchte Korrekturen. Seit 17.1 liefert `pathing.RouteSegmentPlayer` diese Projektion read-only; `RoutePlayer` delegiert sie und der App-Adapter mappt sie ohne Task-seitige Rekonstruktion. Seit 17.5 enthält ein aktuelles Recovery-Ziel zusätzlich den tatsächlich gesendeten Movement-Cast mit Cast-Zeit, Ursprungsposition, frühestem Folgeinput und dem vom Navigator autorisierten Mindestfortschritt.

Die Modi sind:

- `movement` — nächster regulärer Routenpunkt;
- `recovery` — tatsächlicher vorheriger Korrekturpunkt;
- `transition` — erwarteter Area-Übergang ohne Positionsziel.

`Progress` simuliert ausschließlich die im nächsten Route-Tick bestätigbaren, bereits erreichten Punkte. Es verändert weder Point-/Segmentzustand noch Navigator oder TransitionHandler. Vor Start, nach Reset, bei ungültiger Identität/Area und nach Abschluss liefert die Task-Oberfläche `false`.

### Deadline-sicheres Hold

`routePlaybackAdapter.Hold` prüft einen gültigen In-Game-Snapshot, die beim Start gebundene Identität, die aktive beziehungsweise erwartete Transition-Area und monotone `world.State.At`. Es tickt weder den Navigator noch den TransitionHandler.

Nur ein frischer Snapshot schreibt die reale Zeit seit dem letzten Adapter-Tick oder gutgeschriebenen Hold auf die aktive Segment-/Transition-Deadline gut. Wiederholungen desselben Snapshots verändern Deadline und Zeitanker nicht. `Reset` verwirft Player, Deadline, Snapshot-/Zeitanker und Identität gemeinsam.

Der synthetische 17.1-Vertrag hält eine Route zehn Sekunden, erzeugt dabei keinen Navigationsaufruf und setzt anschließend mit unverändertem Segment, Punkt, Modus und Ziel fort. Candidate-Playback verwendet weiterhin direkt `RoutePlayer.Tick`; Recording bleibt eine reine `RouteRecorder.Observe`-Pipeline. Beide erhalten dadurch weder `Hold` noch Phase-17-Combat.

### ThreatAssessment

`tasks.ThreatAssessment` bindet genau einen `world.State.At` an:

- priorisiertes Route-Ziel und Zone;
- optionales Density-Relief-Ziel;
- relevante Threat-Anzahl;
- erforderlichen lokalen Coverage-Radius;
- lokalen Vollständigkeitsstatus.

Der Typ besitzt keine Input-, Process- oder Cache-Oberfläche.

Seit 17.2 berechnet `assessThreats` Immediate-, Landing- und Corridor-Zone, deterministische Route-/Density-Ziele und lokale Coverage in genau einem O(n)-Durchlauf. Innerhalb einer Zone gelten kleinste Spielerdistanz und danach kleinste `UnitID`; die Zonenpriorität ist `immediate` vor `landing` vor `corridor`. Der Referenzbenchmark mit 512 Monstern auf dem i7-8700K liegt in drei Läufen bei 6.245–6.314 ns/op, 0 B/op und 0 allocs/op.

### Monster-Coverage

`memory.MonsterCoverage` und `world.MonsterCoverage` enthalten:

- `EligibleMonsterCount`;
- `MonstersTruncated`;
- `MonsterCoverageRadiusTiles`.

`Phase17MaxRuntimeMonsters` ist compile-nah `512`. Seit 17.2 hält die produktive Enumeration die 512 nächsten Nicht-Priority-Kandidaten; Priority-Einheiten liegen außerhalb dieses Reservoirs. `EligibleMonsterCount` wird vor der Reservoirentscheidung erhöht, Truncation beginnt beim 513. konkurrierenden Nicht-Priority-Kandidaten und der Radius entspricht dann dem weitesten behaltenen Nicht-Priority-Monster. `memory.Snapshot`, `world.State`, Mapping und Clone-Pfade führen die Metadaten ohne zweiten Process-Read.

### ResourceContext

`profile.ResourceContext` besitzt genau:

- `MobilityCritical`;
- `Threatened`;
- `EmergencyMana`.

Er ergänzt Kontext, aber keine parallele Aktionswahrheit. `StatusAction`, `StatusPending`, `StatusComplete` und `StatusFailed` bleiben autoritativ.

## Zustandsautomat

`RouteThreatState` definiert:

- `route_moving`;
- `route_clearing`;
- `density_relief`;
- `route_mana_recovery`;
- `route_recovery_guard`.

Alle Zustände sind run-generation-scoped. Stop, `process_lost`, Run-Ende, Route-Neustart und Step-Wechsel müssen sie vollständig leeren. Es gibt keinen persistenten Resume-Checkpoint.

Seit 17.3 liest `play_bound_route` vor jedem möglichen Route-Tick genau einmal `RouteProgress` und bildet genau ein `ThreatAssessment`. Der generation-scoped `RouteThreatController` erlaubt danach exklusiv entweder `Route.Hold` plus stationären Profil-Clear oder genau einen `Route.Tick`. Immediate-, Landing- und Corridor-Threats erreichen daher keinen Movementpfad.

Nach einer Blockade zählen nur drei frische, lokal vollständige und threat-freie Snapshots als `stable_clear`; auch der dritte Snapshot bleibt ein Hold. Erst ein weiterer frischer Snapshot darf Movement fortsetzen. Bei lokaler Coverage-Lücke wird ausschließlich ein innerhalb der Angriffsdistanz liegendes Density-Ziel verwendet. Ein global abgeschnittener, lokal vollständiger Snapshot blockiert nicht.

Der objektive Fortschrittswatchdog wird nur durch ein verschwundenes/nicht mehr relevantes beauftragtes Ziel, sinkende Eligible-/Threat-Anzahl, wachsenden Coverage-Radius, neu vollständige Coverage oder Memory-bestätigte Annäherung gesetzt. 25 tatsächlich als `action` gemeldete Clear-Ticks über 24 Sekunden bestehen bei sinkender Eligible-Anzahl; unveränderte Snapshots scheitern nach exakt zwölf Sekunden trotz gemeldeter Casts. Ein unverändert nicht angreifbarer Blocker löst nach drei frischen Bestätigungen einen Force-Move-Versuch zum validierten nächsten Routenpunkt aus. Erst drei solche Versuche ohne mindestens ein Tile Distanzgewinn enden mit `route_threat_out_of_range`.

Das Profil registriert ausschließlich die codegestützte Strategie `single_target` für `necro_bone_spear`. Ihr Interface enthält nur `CastAttackAtMonster` und `StopAttack`; Teleport, Laufbewegung und Navigator sind strukturell nicht erreichbar. Aim, Skillwahl oder Hover-Warten verbrauchen ebenso den Controller-Tick wie ein bestätigter Rechtsklick, sodass kein Route-Tick im selben Snapshot folgen kann.

17.4 ergänzt den schmalen `profile.ResourceContext` mit `MobilityCritical`, `Threatened` und `EmergencyMana`. Nur der aktivierte Summoner-Route-Step wertet Ressourcen innerhalb des Threat-Interleaves aus. `StatusAction` beendet den Tick vor Clear und Movement; `StatusPending` ist inputfrei und darf deshalb Hold, stationären Clear oder bei ausreichendem Mana sicheres Movement fortsetzen. Andere Run-Steps behalten ihre bisherige frühe Rückkehr auch bei `StatusPending`.

Die Mana-Hysterese beginnt strikt unter 20 %. Ein noch nicht begonnener Hold lässt 20–34 % ohne Immediate-Threat zu; ein aktiver Hold endet erst ab 35 %. Immediate-Threat plus höchstens 10 % setzt den Emergency-Kontext. Die Profilreihenfolge ist HP-kritische Rejuvenation, Route-Emergency-Mana, Rejuvenation-Fallback, Mobility-Mana und anschließend normale Regeln. Fehlt jede passende Ressource, endet der Route-Step sofort; ohne bestätigte Erholung endet er nach exakt fünf Sekunden mit `route_mana_recovery_failed`.

Seit 17.5 verwendet Recovery dieselbe Threat-Geometrie mit dem tatsächlich effektiven Previous-Point-Ziel. Ein Threat hält und räumt, bevor die Route wieder tickt. Nach einem bestätigten Recovery-Cast bleibt dessen Beleg Segment-/Point-/Ziel-gebunden. Poll-Ticks vor dem projizierten nächsten Inputzeitpunkt dürfen weiterlaufen, zählen aber nicht als Cast. Ist beim frühesten möglichen zweiten Input weniger als `stuck_progress_tiles` Positionsfortschritt sichtbar, liefert der Controller `route_recovery_unsafe`, bevor `Route.Tick` und damit ein zweiter Cast erreichbar sind. Bei bestätigtem Fortschritt bleibt der bestehende lokale Recovery-Pfad aktiv. Es gibt kein alternatives Ziel, keinen Jitter und keinen Combat-Reset des Korrekturbudgets.

## Config und Defaults

`config.RouteCombatConfig` ist der additive Operatorvertrag. `Enabled` ist ein Pointer, damit fehlend und explizit `false` unterscheidbar bleiben. Seit 17.2 werden Defaults vor Validierung run-ID-bewusst angewendet: fehlend bedeutet nur für Summoner `true`, für alle anderen Runs `false`; explizites `false` bleibt der Rollback.

| Key | Default |
|---|---:|
| `immediate_radius_tiles` | 18 |
| `corridor_width_tiles` | 7 |
| `landing_radius_tiles` | 10 |
| `attack_distance_tiles` | 30 |
| `no_progress_timeout_ms` | 12000 |
| `teleport_mana_reserve_percent` | 20 |
| `resume_mana_percent` | 35 |
| `emergency_mana_percent` | 10 |
| `mana_recovery_timeout_ms` | 5000 |

Compile-nahe weitere Grenzen sind 512 nicht priorisierte Runtime-Monster und drei freie, lokal vollständige Snapshots.

Die Summoner-Definition besitzt als einzige `route_clear` und den unveränderlichen Katalog `Specter (40)`, `Hell Clan (56)`, `Ghoul Lord (131)`. Diese IDs sind weder YAML- noch UI-Tuning. Positive endliche Radien, Geometrieordnung, Prozenthysterese und beide Timeoutbereiche werden strikt validiert; ein aktivierter Block außerhalb einer route-clear-fähigen Definition scheitert.

## Watchdogs, Pause und Safety

- Clear besitzt kein Monster-, Aktions- oder Gesamtdauerlimit.
- Nur zwölf Sekunden ohne objektiven Ziel-, Threat- oder Coverage-Fortschritt führen zu `route_clear_no_progress`.
- Aim, Skillwahl, Cast, Zielwechsel und Trankinput setzen den Clear-Watchdog nicht zurück.
- Mana-Recovery besitzt unabhängig davon maximal fünf Sekunden.
- Benutzer-Pause friert Task-Ticks und damit beide Watchdogs.
- Invalid/Loading sendet keinen Input und bestätigt weder Kill noch Stable-Clear.
- Clear-Abschluss und Route-Movement liegen niemals im selben Tick.
- F11, Cancellation und `process_lost` bleiben zentrale Reset-Barrieren.

## Stabile Reasons

`Phase17RouteThreatReasons` definiert vollständig:

- `route_clear_no_progress`;
- `route_threat_out_of_range`;
- `route_mana_recovery_failed`;
- `route_recovery_unsafe`;
- `route_threat_state_invalid`.

Die ersten vier sind später retryable über den bestehenden Lifecycle. `route_threat_state_invalid` bleibt terminal und wird keiner Operatorliste automatisch hinzugefügt. 17.0 ändert die aktuelle Retry-Konfiguration noch nicht.

## Nicht-Ziele

`Phase17NonGoals` verbietet insbesondere:

- Battle.net, Online-Modus sowie Savegame- oder Installationsmutation;
- universelle Combat-AI, Rotations-DSL und Klassen-Switches in Tasks;
- Amplify Damage, Corpse Explosion und ein Leichenmodell;
- Area-Clear, Spatial Index, Quadtree oder zweiten Memory-Read;
- Annäherungsteleport, Jitter und alternative Recovery-Ziele;
- impliziten Combat für rohes Playback, Recording oder Guided Validation;
- automatische Aktivierung für andere Runs;
- neue Settings-/History-Oberfläche oder rohe NPC-ID-Konfiguration;
- harte Monster-, Aktions- oder Gesamtdauerlimits.

## Gate 17.0

Das automatische Gate und die compile-nahen Verträge sind umgesetzt. Geprüft wurden:

1. Route und Telemetrie-Hashes gegen den installierten Datenroot.
2. Die präzise Trennung der belegten Ursachen:
   - Route bewegt ohne Threat-Interleave;
   - passive Ressourcenverifikation blockiert den äußeren Route-Step;
   - Recovery kann den vorherigen Punkt ohne Threat-Kontext anfordern.
3. Keine unbelegte „Stuck“-Behauptung aus den beiden 30-Sekunden-Timeouts.
4. Vollständigkeit von Ownership, States, Defaults, Reasons, Watchdogs, Safety und Nicht-Zielen.

Damit ist 17.0 vollständig abgeschlossen; 17.1 darf darauf aufbauen.

## Gate 17.1

Der synthetische Zehn-Sekunden-Hold, Progress-/Recovery-/Transition-Projektion, identische Snapshots, Deadline, Reset sowie isoliertes Candidate-Playback und Recording sind vollständig grün.

## Gate 17.2

Config-Presence und Validierung, Summoner-Capability/Allowlist, 32/33/512/513-Coverage, Priority-Nichtverdrängung, Memory→World-Projektion, strikte lokale Coverage-Grenze und die Geometriematrix sind grün. `BenchmarkThreatAssessment512` bestätigt deutlich unter einer Millisekunde und null Allokationen.

## Gate 17.3

Immediate-, Corridor- und Landing-Blocker verhindern Route-Ticks; der Profil-Clear bleibt stationär und UnitID-/Hover-gebunden. Drei freie Snapshots plus frischer Resume, Density-Relief nur bei lokaler Lücke, mehr als 20 Aktionen über mehr als zwölf Sekunden mit objektivem Fortschritt, echter Zwölf-Sekunden-Stillstand, dreifaches Out-of-Range-Gate und generation-scoped Reset sind automatisch grün.

## Gate 17.4

Die exakte 20/35-Hysterese, Emergency bei höchstens 10 % plus Immediate-Threat, Mana-vor-Rejuvenation samt Rejuvenation-Fallback, sofort fehlende Ressource, exakter Fünf-Sekunden-Timeout sowie die getrennte `action`-/`pending`-Semantik sind im Vertrag abgebildet. Die Abschlusskorrektur wurde auf ausdrückliche Operatoranweisung nicht automatisch getestet. Ein Ressourceninput verhindert weiterhin jeden zweiten Input desselben Ticks; Nicht-Route-Steps erhalten einen leeren Kontext und ihre bisherige frühe Rückkehr.

## Gate 17.5

Navigator, SegmentPlayer, App-Adapter und Task-Pipeline projizieren einen tatsächlich gesendeten Recovery-Cast ohne Inputannahme aus bloßen Poll-Ticks. Ein Pack am Previous Point erzwingt Hold/Clear bei unverändertem Correction-Budget. Ein unveränderter synthetischer Spielerzustand darf bis zum throttle-seitig nächsten Inputzeitpunkt pollen und endet dann vor einem zweiten `Route.Tick` mit `route_recovery_unsafe`; exakt `stuck_progress_tiles` bestätigter Fortschritt erlaubt die Fortsetzung.

## Verwandte Features

- [Summoner-Run](summoner-run.md)
- [Route Recording und Playback](route-recording-playback.md)
- [Task Runner](task-runner.md)
- [Character- und Encounter-Profile](character-encounter-profiles.md)
- [World Model](world-model.md)
- [Memory Reader](memory-reader.md)
- [Run Registry](run-registry.md)

---
*Zuletzt aktualisiert: 29. Juli 2026*
