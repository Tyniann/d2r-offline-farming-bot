# Task Runner

## Überblick

Der Task Runner führt registrierte Run-Definitionen über eine gemeinsame Pipeline im Poll-Loop aus. `--run <id>` ohne Phase nutzt diese Pipeline vollständig; die generischen CLI-Phasen `travel-entry`, `play-route`, `boss`, `loot-and-return`, `stash-personal` und `town-ready` wählen isolierte Ausschnitte derselben Pipeline. Ein isoliertes `play-route` aus Act 1 beginnt am bestätigten Stash-/Town-Start der layoutgebundenen Kante zum Waypoint; es setzt den Charakter nicht bereits direkt am Waypoint voraus.

## Ort im Code

- **Paket:** `internal/tasks/`
- **Einstieg:** `cmd/d2rbot` mit `--run <id>` oder `runs.active` in der Config
- **App-Integration:** `internal/app/run_tick.go`, `internal/app/run_mode.go`
- **Wichtige Dateien:** `runner.go`, `registry.go`, `run_pipeline.go`, `pipeline_flow.go`, `pipeline_state.go`, `pipeline_deps.go`, `pipeline_travel.go`, `pipeline_boss.go`, `pipeline_loot.go`, `pipeline_return.go`, `run_contract.go`, `step.go`, `deps.go`, `result.go`
- **Config:** `configs/config.example.yaml` → Sektion `runs`

## Funktionalität

### Run-Auswahl

- CLI `--run` überschreibt YAML `runs.active`
- Leerer Name = passiver Modus (nur Monitor, wie bisher)
- Bekannte Definitionen: `countess`, `mephisto`, `summoner`, `nihlathak` und `cows` aus der typisierten `RunRegistry`. Die vier klassischen Runs verwenden die gemeinsame Standardpipeline. `cows` besitzt wegen seiner zwei festen Routenrollen und des linearen Setup-/Rezept-/Sweep-Vertrags eine eigene enge Pipeline; die Route-Hold-, Loot-, Portal-, Town- und Profil-Primitiven bleiben gemeinsam.

### Lazy Run-Start

Beim Startup wird nur `task run configured` geloggt (wenn ein Run gesetzt ist). `task run started` erscheint beim **ersten Poll-Tick**, an dem alle Guards in `shouldTickTasks` erfüllt sind:

- konfigurierter Run gesetzt
- `!Terminal()` und `!WasReset()`
- `input.enabled` + `Input.Status().Enabled`
- `world.Valid` und `Input.Bound()`
- nicht pausiert/gestoppt

Nach dem ersten Attach kann der Start um **1–2 Poll-Ticks** verzögert sein (Attach-Tick führt kein `World.Update` aus).

### Step-Modell

Die Standardpipeline ist nach fachlicher Verantwortung getrennt und verwendet weiterhin denselben internen `*runPipeline`-Receiver als Orchestrator. `run_pipeline.go` dispatcht nur Full-Run und isolierte Phasen. `pipeline_flow.go` besitzt Step-/Phasentopologie, `pipeline_state.go` den gruppierten Generation-State und die zentrale Reset-Barriere; Travel, Boss, Loot und Return enthalten ihre bestehenden Handler.

Der persistente Zustand besitzt genau eine fachliche Gruppe:

| Domäne | Persistenter Zustand |
|---|---|
| Core | Run-Definition, Phase, Route-/Combat-Konfiguration |
| Travel/Route | Navigator-/Route-Start, Resume, Route-Progress-Verfügbarkeit, Threat-Approach, Route-Loot und terminale Safe-Snapshots |
| Boss | Suchfallback, Boss-Pin, Encounter-Index, Approach, Nihlathak-Aim und Post-Boss-Cleanup |
| Loot | Drop-/Scan-Stabilität, Post-Kill-Reposition, Pickup-Ziel und begrenzte Pickup-Recovery |
| Return | Town-Egress sowie Portal-UnitID und begrenzte Portal-Recovery |

Die unveränderliche Core-Gruppe wird beim Aufbau einer Runner-Generation gesetzt. Die zentrale Reset-Barriere leert alle vier veränderlichen Gruppen, bevor eine andere Generation starten darf. Die Domain-Handler erhalten schmale Dependency-Sichten: Travel sieht nur Waypoint/Town-Walk/Route/Combat/Loot/Profile/Telemetry, Boss nur Pathing/Combat/Route-Clear/Profile/Telemetry, Loot nur Combat/Loot und Return nur seine Portal-/Stash-/Town-Abhängigkeiten. Damit kann kein Handler versehentlich auf einen Input-Eigentümer einer fremden Domäne zugreifen.

Der Runtime-Recorder lehnt einen Tick mit mehr als einem produktiven semantischen Input-Intent fail-closed ab. Damit bleibt die bereits fachlich geltende Ein-Input-Gelegenheit pro Tick auch während mechanischer Refactors als ausführbare Invariante erhalten.

Zwei Abschluss-Mechanismen (nicht vermischen):

| Mechanismus | Verwendung |
|-------------|------------|
| **Zeit-Timeout** (`step_timeout_ms`, Default 45000) | Warte-Steps auf Zustandsänderung |
| **Tick-Zähler** (`ticksInStep`) | Deterministische kurze Steps, wenn ein Run sie explizit markiert |
| **Sofort-Fail** | Bedingung klar verletzt -> sofort `task step failed` |

**Gemeinsame Full-Run-Pipeline:**

`precheck -> acquire_town_waypoint -> open_waypoint -> select_run_waypoint -> wait_entry_area -> play_bound_route -> acquire_boss -> engage_boss -> [clear_nearby_hostiles] -> reposition_for_loot -> wait_for_drops -> scan_loot -> pick_loot -> cast_town_portal -> enter_town_portal -> wait_origin_town -> open_personal_stash -> stash_items -> close_personal_stash -> prepare_town_handoff -> complete`

**Isolierte Default-Pfade derselben Pipeline:**

| Phase | Default-Pfad |
|---|---|
| `town-ready` | `precheck -> town_ready_profile -> complete` |
| `stash-personal` | `precheck -> open_personal_stash -> stash_items -> close_personal_stash -> complete` |
| `travel-entry` | `precheck -> acquire_town_waypoint -> open_waypoint -> select_run_waypoint -> wait_entry_area` |
| `play-route` | `precheck -> acquire_town_waypoint -> open_waypoint -> select_run_waypoint -> wait_entry_area -> play_bound_route` |
| `boss` | `precheck -> acquire_boss -> engage_boss` |
| `loot-and-return` | `precheck -> wait_for_drops -> scan_loot -> cast_town_portal -> enter_town_portal -> wait_origin_town -> open_personal_stash -> stash_items -> close_personal_stash -> complete` |

Loot-Ziel, Fremdakt-Rückweg und Travel-Resume bleiben dynamische Abzweigungen und stehen nicht in dieser Default-Tabelle. `loot-and-return` endet nach dem Stash bei `complete` und überspringt `prepare_town_handoff`.

**Vollständige Cow-Pipeline ab Gate 20.6:**

`cow_preflight -> cow_town_ready_profile -> cow_acquire_town_waypoint -> cow_open_waypoint -> cow_select_stony_field -> cow_wait_stony_field -> cow_play_leg_acquisition -> cow_open_wirt -> cow_pickup_leg -> cow_cast_return_portal -> cow_enter_return_portal -> cow_wait_rogue_encampment -> cow_buy_recipe_tome -> cow_setup_gate_complete -> cow_portal_recipe -> cow_recipe_gate_complete -> cow_play_cow_sweep -> cow_sweep_gate_complete -> cast_town_portal -> enter_town_portal -> wait_origin_town -> open_personal_stash -> stash_items -> close_personal_stash -> prepare_town_handoff -> complete`

Die Cow-Pipeline sperrt die automatische Runner-Profil- und Safety-Eingabe, weil jeder Setup-Schritt genau einen eigenen Input-Eigentümer besitzt. `cow_play_leg_acquisition` delegiert die reine Wiedergabe an den vorhandenen RoutePlayer, lässt Route-Combat wegen des fehlenden autoritativen LOS-Modells für Stony-/Tristram-Hausgeometrie aber deaktiviert. Route-, Drift-, Stuck-, Timeout- und Portal-Gates bleiben aktiv; Setup-Route-Loot ist ebenfalls ausgeschaltet. Wirt-/Leg-Fehler kehren vor dem terminalen Ergebnis nach Town zurück; ein endgültiger Portalrückkehrfehler fordert genau einen supervisor-eigenen Save-&-Exit-Fallback an.

`cow_portal_recipe` besitzt eine interne lineare Unter-State-Machine, bleibt für den Runner aber genau ein Step und ein Input-Eigentümer. Die im Preflight gebundene Cube-UnitID sowie die später gebundenen Leg-/Tome-UnitIDs werden unverändert übergeben. Erfolg ist erst nach exakt einem Memory-gegateten Transmute, bestätigtem Verbrauch beider Zutaten, drei stabilen Snapshots der neuen Permanent-Portal-UnitID, hover-bestätigtem Eintritt und Area 39 möglich. Ein terminaler Zustand nach Transmute kann keinen zweiten Klick auslösen.

`cow_play_cow_sweep` delegiert wieder an denselben RoutePlayer und Route-Hold-Controller, diesmal mit aktivem Route-Combat und Cow-spezifischer stationärer Aktionswahl. Der Wrapper bindet den vorhandenen `RouteClearExecutor` plus die bereits UnitID-autorisierte CE-Oberfläche; er besitzt keine Movement-Aktion. AD wird einmal pro Hold gesendet, danach wechseln Bone Spear und höchstens zwei CE-Versuche pro aktuelle Leiche anhand frischer World-Snapshots. Ein Input allein setzt den Watchdog nicht zurück. `cow_combat_no_progress` ist retryfähig, volles Inventar beendet den Sweep nicht, und der terminale Routenpunkt benötigt drei verschiedene lokal vollständige Safe-Snapshots.

Der Cow-Runner startet beide Rollen innerhalb desselben Runs. Telemetrie behält `cow_sweep` als immutable Primär-`route_id`, bindet `leg_acquisition` additiv als `setup_route_id` und markiert die tatsächlich aktive Rolle auf Playback-, Threat- und Hold/Clear-Events. Ein Setup-Playback darf deshalb weder einen zweiten Recorder erzeugen noch die Primär-ID eines bestehenden Recorders ersetzen.

Nach den drei terminalen Safe-Snapshots beendet `cow_sweep_gate_complete` nicht den Runner. Die Cow-Pipeline delegiert den abschließenden Act-1-Rückweg, persönlichen Stash und Town-Handoff an dieselben bestehenden Steps wie Standard-Runs. Dadurch entstehen weder eine zweite Runner-Generation noch ein zusätzlicher Queue-/Budget-/History-Verbrauch. Die Foreign-Town-Egress-Verzweigung bleibt für Cow unerreichbar, weil `ReturnOrigin=act1` registriert ist.

Entry-Area, Route-Terminal, Waypoint-Ziel, Boss, Suchanker, geordnete Encounter-Aktionen und Rückkehrakt stammen aus `RunDefinition`; Route, Combat und Loot-Policies aus dem ausgewählten `RunConfig`. `wait_entry_area`, Route-, Boss-, Loot- und Portal-Areagates vergleichen ausschließlich gegen diese Definition.

Der Encounter-Aktionsindex beginnt pro Boss-Pin bei `0`. Jede Aktion erhält getrennte Start-/Abschluss-Telemetrie; erst danach darf regulärer Combat laufen. Pro Poll-Tick wird höchstens eine Action-Input-Gelegenheit konsumiert.

`clear_nearby_hostiles` ist ein Registry-Opt-in für Summoner und Nihlathak. Er läuft unmittelbar nach der Kill-Bestätigung und ausdrücklich vor `reposition_for_loot`, damit der Charakter nicht zuerst zur Bossleiche in ein verbliebenes Pack teleportiert. Summoner nutzt den Standardangriff aus `CombatConfig` gegen seine run-spezifisch erlaubten lebenden Gegner innerhalb von 18 Tiles und höchstens 20 tatsächlich gesendete Aktionen. Nihlathak wirkt nach seinem bestätigten Tod einmal Amplify Damage und räumt anschließend die vollständige gebietseigene Halls-of-Vaught-Hostile-Allowlist innerhalb von 30 Tiles mit Bone Spear; ein bereits gehovtes lebendes Monster wird dort sofort akzeptiert, weil Nihlathak keine Corpse Explosion mehr auslösen kann. Seine dichtere Begegnung besitzt ein eigenes Budget von 40 gesendeten Aktionen. Der Step endet erst nach drei gegnerfreien Snapshots oder dem jeweiligen Aktionsbudget. Countess und Mephisto überspringen den Step: im Tower Cellar stehen zu viele Gegner hinter Wänden.

Seit Phase 17.3 besitzt der Summoner-Route-Step ein Threat-Interleave; Phase 20.5 verwendet denselben Controller für `cow_sweep`. Vor jedem möglichen `Route.Tick` werden effektives Movement-/Recovery-Ziel und der aktuelle World-Snapshot einmal bewertet. Ein Immediate-, Corridor- oder Landing-Blocker beziehungsweise eine lokale Coverage-Lücke führt exklusiv zu `Route.Hold` und stationärem Profil-Clear; Cow hält zusätzlich für jede lebende lokale Kuh innerhalb der Angriffsdistanz. Im selben Tick ist Route-Movement unerreichbar. Drei frische lokal vollständige freie Snapshots bestätigen den Clear, der nächste frische Tick setzt exakt denselben Routefortschritt fort. Zwölf Sekunden ohne objektiven Ziel-/Threat-/Coverage-Fortschritt scheitern fail-closed; Aktionen allein verlängern den Watchdog nicht.

Ist derselbe Blocker drei frische Snapshots lang nicht projizierbar, nähert die Pipeline kontrolliert über Force Move zum bereits validierten nächsten Routenpunkt an. Nach 500 ms und einem frischen Snapshot muss die Distanz zur eingefrorenen Zielposition mindestens ein Tile kleiner sein. Fortschritt fordert einen neuen Projektionsbeleg; ohne Fortschritt sind höchstens drei Versuche erlaubt. `Route.Tick` bleibt dabei gesperrt und `route_threat_out_of_range` entsteht erst nach dem dritten wirkungslosen Versuch.

Im selben opt-in Route-Step folgt nach einer freigegebenen Threat-Bewertung und vor `Route.Tick` ein punktgebundener Pickit-Check. Er nutzt ausschließlich die immutable Benutzer-Pickit-Kette des laufenden Runs und wählt nur `keep`-Treffer im 30-Tiles-Umkreis. Alle Treffer werden vor dem nächsten Routeninput sequenziell über den bestehenden Pickup-Executor und dessen begrenzte Teleport-Annäherung abgearbeitet. Währenddessen hält `Route.Hold` den Fortschritt unverändert. Sobald ein frischer Snapshot einen Threat meldet, gewinnt Combat exklusiv; nach dessen Clear wird derselbe Punkt erneut gescannt. Ein volles Inventar oder ein übersprungenes Einzelitem beendet die Route nicht.

Seit Phase 17.4 besitzt nur dieser opt-in Route-Step auch die Ressourcenkoordination. Alle anderen Steps behalten die bisherige Runner-Semantik und kehren sowohl bei `StatusAction` als auch `StatusPending` vor ihrer State-Machine zurück. Im Summoner-Route-Step kehrt nur `StatusAction` früh zurück; passives `StatusPending` darf das aktuelle Assessment noch in Hold/Clear oder – bei ausreichendem Mana – Movement überführen. Unter 20 % beginnt ein Mana-Hold, der unter 35 % kein Route-Movement zulässt. Immediate-Threat bei höchstens 10 % setzt den Emergency-Kontext. Fehlende passende Belt-Ressource endet sofort, ausbleibende Erholung nach fünf Sekunden mit `route_mana_recovery_failed`. Der unabhängige Clear-Watchdog wird durch keinen Ressourcenstatus zurückgesetzt.

Phase 17.5 schützt auch `RouteProgress.Mode=recovery`. Das effektive Previous-Point-Ziel durchläuft dieselbe Immediate-/Corridor-/Landing-Prüfung; ein bekannter Blocker führt daher zu Hold/Clear statt Movement. Nach einem tatsächlich vom Navigator gemeldeten Recovery-Cast darf die Pipeline inputfrei bis zum nächsten möglichen Castzeitpunkt pollen. Liegt dann weniger Positionsfortschritt als das autoritative `stuck_progress_tiles` vor, endet der Step vor `Route.Tick` mit `route_recovery_unsafe`. Inputfreie Navigator-Ticks zählen nicht als Cast, bestätigter Fortschritt erlaubt den bestehenden Korrekturpfad, und Combat verändert weder Point noch Correction-Budget.

Retryfähige Routenfehler verwenden die isolierte Phase `retry-return`. Sie ist nur in den registrierten Gebieten der veröffentlichten Run-Route zulässig und führt ausschließlich `precheck -> cast_town_portal -> enter_town_portal -> wait_origin_town` plus den vorhandenen Foreign-Town-Egress nach Akt 1 aus. Loot, Stash und Town-Service werden übersprungen. Erst ein Memory-bestätigtes Rogue Encampment setzt `SafeToExit`; anschließend führt der Supervisor exakt denselben Save-&-Exit-Automaten wie am normalen Queue-Ende aus und startet denselben Queue-Index innerhalb der konfigurierten Fehler- und Restart-Budgets neu. Der gemeinsame Automat wartet vor Escape mindestens drei Sekunden auf eine unveränderte Spielerposition bei geschlossenem Town-UI. Bestätigt Memory nach dem ersten Escape weiterhin ein geschlossenes Quit-Menü, wird Escape nach 1,5 Sekunden genau einmal erneut gesendet; ein bereits offenes Menü kann dadurch nicht zugeklappt werden. Ein fehlgeschlagener Rückweg endet mit `retry_return_failed`, nicht mit einem unbestätigten Exit.

### Safety-Potion-Guard

Vor dem normalen `run.onTick` prüft der Runner globale Safety:

- nur bei `Valid`, `Phase=in_game` und `Player.MaxHP > 0`
- `HPPercent() <= 35` castet Belt Slot 4
- `HPPercent() <= 65` castet Belt Slot 1
- Throttle: 1500 ms
- ein erfolgreicher Potion-Cast verbraucht den Poll-Tick; der normale Step läuft erst im nächsten Tick weiter
- fehlende Run-Actions werden ignoriert; fehlschlagende vorhandene Belt-Actions beenden den Run mit `safety_potion_failed`

### Nach Run-Ende und process_lost

- **`ConfiguredRun()`** bleibt gesetzt (CLI/config-Name) und dient weiterhin Logs/Diagnose, auch nach Terminal oder Reset
- **`Terminal()`** (success/failed): keine weiteren `Tick`-Aufrufe; ein expliziter `--run`-Prozess kehrt danach mit Erfolg beziehungsweise Fehler selbstständig zur Shell zurück
- **Passiver Modus ohne Run:** World-Monitor und Probe laufen weiterhin bis Stop-Hotkey oder Prozesssignal
- **`WasReset()`** (z. B. `process_lost`): blockiert Lazy-Re-Start nach Re-Attach
- **Kein zweiter Run ohne App-Neustart** — weder nach terminal noch nach reset

Bei `process_lost` feste Reihenfolge: `Input.Unbind()` → `Tasks.Reset("process_lost")` → `World.Reset()`. Die zentrale, idempotente Run-Barriere leert dabei Waypoint, Portal, Town-Walk, Stash, Navigator, Route-Player, Route-Threat-Controller, Combat, Loot, Cow-Setup, Cow-Rezept samt gebundener Item-/Portal-UnitIDs, Town, Profil, Boss-Pin und Aktionsindex genau einmal.

Pause blockiert Ticks still; Step-Timer und Tick-Zähler frieren ein (kein Tick = kein Fortschritt).

## Datenmodell

- `tasks.RunConfig` — `StepTimeout` aus `runs.step_timeout_ms`
- `tasks.TickResult` — `Active`, `Outcome`, `Step`, `Reason`
- `tasks.RunOutcome` — `idle`, `running`, `success`, `failed`

## Operator / CLI

```powershell
go run ./cmd/d2rbot --run countess --probe   # input.enabled: true erforderlich
```

| Event | Felder |
|-------|--------|
| `task run configured` | `run`, `source` (Startup, nur wenn Run gesetzt) |
| `task run started` | `run` (erster Guard-OK-Tick) |
| `task step started` | `run`, `step` |
| `task step complete` | `run`, `step`, `elapsed_ms` oder `ticks` |
| `task step failed` | `run`, `step`, `reason`, `elapsed_ms` |
| `task run finished` | `run`, `outcome`, `reason` |
| `task run reset` | `run`, `reason` |

JSONL-Transitionen `run_step_started`, `run_step_completed` und `run_step_failed` enthalten `definition_id`, `step`, `stage` und `outcome`. Die vollständige Core-Zuordnung ordnet jeden gemeinsamen Countess-/Mephisto-Step genau einer der stabilen Kategorien `travel`, `combat`, `loot` oder `return_town` zu; unbekannte Steps werden nicht ohne Stage persistiert. Encounter-Events ergänzen `action_index`. Nach der bestehenden Memory-bestätigten Kill-Bedingung schreibt die Pipeline genau ein `boss_kill_confirmed` für die gepinnte Unit. Schlägt ein persistierendes Telemetrie-Emit fehl, endet die Pipeline vor dem folgenden Input beziehungsweise Kill-Abschluss mit `telemetry_failed`.

`--run` und `--input-test` schließen sich gegenseitig aus. Run erfordert `input.enabled: true`.

### Mercenary-Ressourcen (Phase 18)

`AllowMercenary` ist nur in `engage_boss`, aktivem Route-Clear und `clear_nearby_hostiles` gesetzt. Travel, Loot, Town und Stash erlauben keinen Merc-Trank. Die Run-State-Machines bleiben merc-frei; der Resource-Executor entscheidet zentral. Details: [Mercenary Support](mercenary-support.md).

## Abhängigkeiten

- `internal/world` — Area/Town-Check für Precheck
- `internal/input`, `internal/pathing` — Deps (Compile-Time-Checks in `deps.go`)
- App-Run-Loop in `internal/app`

## Verwandte Features

- [State Probe](state-probe.md) — World-Update vor Task-Tick
- [Input Controller](input-controller.md) — Safety-Guards für Task-Ticks
- [World Model](world-model.md) — `world.State` / Area-Katalog
- [Mercenary Support](mercenary-support.md) — Combat-/Town-Söldner

---
*Zuletzt aktualisiert: 2026-08-16*
