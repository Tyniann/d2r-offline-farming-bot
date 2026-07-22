# Run-Telemetrie

## Überblick

Phase 5.10 schreibt pro aktivem Farming-Run eine eigene JSONL-Datei. Loot- und Stash-Fehler lassen sich dadurch anhand stabiler Events und Unit-IDs rekonstruieren, ohne Textlogs parsen zu müssen.

Die Telemetrie ist fail-closed: Kann die Datei beim Start nicht erstellt oder während des Runs nicht weitergeschrieben und geflusht werden, endet der Run mit `telemetry_failed`. Nach dem ersten I/O-Fehler sind alle weiteren Loot-/Stash-Inputs blockiert.

## Ort im Code

- **Paket:** `internal/telemetry/`
- **Recorder:** `internal/telemetry/recorder.go`
- **Event-Hooks:** `internal/app/loot_actions.go`
- **Task-Abbruch:** `internal/tasks/run_pipeline.go`
- **Config:** `telemetry.directory` in `configs/config.example.yaml`

## Datei und Run-ID

Seit Abschnitt 14.1 schreiben neue Run-Dateien Schema 3 und tragen `stream=run`. Produktive Queue-Runs erhalten ihre global eindeutige Run-ID vor dem ersten Lifecycle-Ereignis vom Supervisor; exakt diese ID wird an Session-Recorder, Run-Recorder, Dateinamen, Status und Logs weitergereicht:

```text
logs/telemetry/countess-<UTC-Zeit>-<Zufallssuffix>.jsonl
```

Der Dateiname ohne Erweiterung ist die `run_id`, die in jeder Zeile wiederholt wird. Jede Zeile ist ein vollständiges JSON-Objekt mit `schema_version=3`. Produktive Events tragen zusätzlich `mode=productive_farming`, `session_id`, `game_id`, Charakter, Difficulty, D2R-Version, Definition, Route/Fingerprint, Queue-Index/-Zyklus, Startzeit und den unveränderlichen Pickit-Snapshot. Eine abweichende Run-ID, Route oder ein Moduswechsel wird vor dem Write abgewiesen.

Isolierte CLI-, Route- und Town-Telemetrie wird explizit mit `mode=diagnostic` geschrieben und kann deshalb niemals anhand von Dateiname oder Run-ID in die Produktpopulation geraten. Bereits vorhandene Schema-1-Dateien bleiben unverändert auf Platte und werden von der Phase-14-Historie ignoriert.

Passive Probe-, Input-Test- und Pathing-Test-Läufe ohne aktiven Run erzeugen keine Run-Telemetrie.

Der isolierte `--run countess --phase loot-and-return` erzeugt dagegen bewusst eine Run-Datei. Dadurch kann Phase-13-Gate B denselben Pickit-Gewinner über Vorschau, Core-Log, `pickit_match`, `stash_attempt` und `stash_success` korrelieren.

Bei produktiven Queue-Runs schreibt der Runtime-Owner vor dem Schließen des Run-Recorders genau das zum Supervisor-Ergebnis passende Terminal: `run_completed` für Advance, `run_aborted` für einen kontrollierten Retry und `run_failed` für Stop. Erst danach schreibt der Queue-Owner dasselbe Terminal in den Session-Stream. So kann der strikt korrelierende HistoryReader einen vollständig abgeschlossenen Run aufnehmen; ein Emit- oder Close-Fehler bleibt `telemetry_failed` und wird nicht als Erfolg kaschiert.

## Events

| Event | Zeitpunkt | Deduplizierung |
|---|---|---|
| `drop_seen` | Ground-Item erscheint in einem Loot-Scan | einmal je Unit-ID und Run |
| `pickit_match` | Ground-Item erhält `classify_match` | einmal je Unit-ID und Run |
| `pickup_attempt` | der hover-bestätigte Linksklick wurde tatsächlich ausgeführt | jeder echte Klickversuch |
| `pickup_success` | Pickup ist über Inventory-Transition oder Ground-Abwesenheit bestätigt | terminal |
| `pickup_failed` | Pickup endet terminal; genauer Status steht in `reason` | terminal |
| `inventory_full` | Scan findet wertvollen No-Fit-Kandidaten | einmal beim Recovery-Übergang |
| `stash_attempt` | atomarer Ctrl+LMB-Transfer wurde tatsächlich ausgeführt | jeder echte Versuch |
| `stash_success` | Item ist aus dem Inventory verschwunden oder hat die Location gewechselt | je bestätigtem Item |
| `stash_full` | für spätere endliche Stash-Flächen reserviert | terminal |
| `profile_hook_action` | Profil-Hook hat einen Skill-Input erfolgreich angefordert | jede echte Hook-Aktion |
| `resource_potion_requested` | passender Belt-Trank wurde erfolgreich angefordert | jeder echte Potion-Input |
| `resource_consumption_confirmed` | ursprüngliche Potion-UnitID ist aus dem Belt verschwunden | jede Memory-Bestätigung |
| `profile_action_failed` | Skill-, Potion- oder Verify-Aktion endet mit stabilem Reason-Code | terminaler Profilfehler |
| `run_context` | Frische Session-Run-Generation bindet Definition und Assets | genau einmal vor dem ersten Task-Tick |
| `run_step_started` / `run_step_completed` / `run_step_failed` | gemeinsame Pipeline betritt oder beendet einen Step | jede Transition |
| `run_encounter_action_started` / `run_encounter_action_completed` | eine geordnete Pre-Combat-Aktion beginnt oder endet | je Definition und Aktionsindex |
| `boss_kill_confirmed` | die bestehende Kill-Bedingung bestätigt das Verschwinden der gepinnten Boss-Unit | genau einmal je Run |
| `sell_success` | die gepinnte Unit hat das persönliche Inventory nach dem Sell-Input verlassen | terminal je verkauftem Item |

`stash_full` wird im aktuellen Personal-Stash-MVP mit unbegrenzten Sammel-Tabs nicht heuristisch erzeugt.

## Schema

Beispiel:

```json
{"schema_version":1,"timestamp":"2026-07-10T15:04:57.036Z","event":"stash_attempt","run_id":"countess-...","run":"countess","phase":"stash-personal","area_id":1,"unit_id":240,"code":"glr","name":"Flawless Ruby","attempt":1}
```

Gemeinsame Felder:

- `schema_version`, `timestamp`, `event`, `run_id`, `run`, optional `phase`
- Pipeline-Kontext: `definition_id`, `step`, `stage`, `outcome`, optional `action_index`; Encounter-Grenzen tragen zusätzlich die gepinnte Boss-`unit_id`. Jeder Step gehört Core-autoritativ genau zu `travel`, `combat`, `loot` oder `return_town`.
- Run-Kontext: `route_id`, `route_layout_fingerprint`, `waypoint_target`, `loot_pickup_policy`, optionale `loot_sell_policy` und `town_origin`.
- Item-Kontext: `area_id`, `unit_id`, `txt_file_no`, `code`, `name`, `item_key`, `item_name`, `base_code`, `quality`, `item_identity_kind` und bei belastbarer Set-/Unique-Auflösung `item_identity_key`. Exakte Set-/Unique-Items verwenden ihren stabilen Katalogschlüssel, andere Items Basiscode plus Qualität; unvollständige Identität wird nicht geraten.
- Pickit-Kontext: `pickit_profile_id`, `pickit_rule_id`, `pickit_action`, `pickit_profile_revision`, `pickit_assignment_revision`
- Ergebnis-Kontext: `reason`, `attempt`, `hover_attempt`, `candidate_count`
- Stash-Kontext kann zusätzlich Inventory-Grid-Koordinaten tragen.
- Profil-Kontext: `profile`, `hook`, `skill`, `skill_id`, `target`, Boss-`unit_id`.
- Ressourcen-Kontext: `resource`, `threshold_percent`, `belt_slot`, Potion-`unit_id`, `confirmed`.

## Fehlerverhalten

- Fehler beim Erstellen des Verzeichnisses oder der Datei verhindern den Runtime-Start.
- Write-/Flush-Fehler werden im Loot-Adapter gespeichert und bleiben für den restlichen Run aktiv.
- Der aktuelle Task endet mit `telemetry_failed`.
- Nach dem Fehler startet kein weiterer Pickup, Ctrl-Klick oder Stash-Close-Input.
- Profil-Telemetriefehler beenden den Task mit `profile_telemetry_failed`; Reset entfernt pending Hooks, Potion-Verifikation und Cooldowns.
- Pipeline-Telemetrie wird vor der folgenden Input-Gelegenheit geflusht. Ein Fehler beendet den Task mit `telemetry_failed` und durchläuft die zentrale Run-Reset-Barriere.
- Ein Fehler, der beim Protokollieren einer gerade ausgeführten Aktion entsteht, kann diese bereits ausgeführte Aktion naturgemäß nicht rückgängig machen; er verhindert aber jede folgende Aktion.
- Ein Boss-Kill wird nicht aus Combat-Start oder Step-Abschluss abgeleitet. Schlägt sein synchroner Emit fehl, darf der Kill-Step nicht erfolgreich abschließen.
- Ein `item_sell`-Shopinput ist nur `town_action`. Erst ein späterer kohärenter World-Snapshot, in dem die gepinnte Unit das persönliche Inventory verlassen hat, erzeugt `sell_success`.

## Live-Validierung

Der isolierte `stash-personal`-Lauf wurde bei 1280×720 mit einer Dol-Rune (`r14`) im ungeschützten Inventory-Bereich geprüft. Der Ctrl+LMB-Transfer wurde über Memory bestätigt und die zugehörige Run-Datei enthielt genau die erwartete Reihenfolge `stash_attempt` → `stash_success` mit derselben Run- und Unit-ID. Die Stash-Oberfläche wurde anschließend per Escape geschlossen und beide UI-Flags wurden als geschlossen bestätigt.

Phase-13-Gate B bestätigte dieselbe Korrelation für UnitID `225` (`Arrows`/`aqv`): `pickit_match`, `stash_attempt` und `stash_success` tragen übereinstimmend `phase13-live-acceptance`, `arrows-live-gate`, `keep`, Profilrevision `2` und Assignment-Revision `2`; dazwischen liegen genau ein `pickup_attempt` und ein `pickup_success`. Der isolierte Run endete mit `outcome=success`.

## Grenzen

- Noch keine Rotation, Kompression oder automatische Bereinigung.
- Abschnitt 11.3 projiziert ausgewählte Zustandsänderungen zusätzlich über einen flüchtigen, begrenzten Live-Publisher ins lokale Dashboard. Dieser Pfad blockiert niemals den Bot und ersetzt keine JSONL-Ereignisse; JSONL bleibt die autoritative persistente Diagnosequelle.
- Keine Upload-Integration.
- Abschnitt 14.1 hebt beide weiterhin getrennten Streams für neue Daten auf Schema 3. Alte Schema-1-/Schema-2-Dateien bleiben reine Diagnoseartefakte und werden nicht migriert.

## Verwandte Features

- [Loot- und Recovery-Loop](loot-recovery-loop.md)
- [Hover-Confirmed Item Pickup](hover-confirmed-item-pickup.md)
- [Personal-Stash MVP](personal-stash-mvp.md)

---
*Zuletzt aktualisiert: 2026-07-22*
