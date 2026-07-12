# Run-Telemetrie

## Überblick

Phase 5.10 schreibt pro aktivem Farming-Run eine eigene JSONL-Datei. Loot- und Stash-Fehler lassen sich dadurch anhand stabiler Events und Unit-IDs rekonstruieren, ohne Textlogs parsen zu müssen.

Die Telemetrie ist fail-closed: Kann die Datei beim Start nicht erstellt oder während des Runs nicht weitergeschrieben und geflusht werden, endet der Run mit `telemetry_failed`. Nach dem ersten I/O-Fehler sind alle weiteren Loot-/Stash-Inputs blockiert.

## Ort im Code

- **Paket:** `internal/telemetry/`
- **Recorder:** `internal/telemetry/recorder.go`
- **Event-Hooks:** `internal/app/loot_actions.go`
- **Task-Abbruch:** `internal/tasks/countess.go`
- **Config:** `telemetry.directory` in `configs/config.example.yaml`

## Datei und Run-ID

Für jeden konfigurierten Run erzeugt `app.New` vor dem ersten Spielinput:

```text
logs/telemetry/countess-<UTC-Zeit>-<Zufallssuffix>.jsonl
```

Der Dateiname ohne Erweiterung ist die `run_id`, die in jeder Zeile wiederholt wird. Jede Zeile ist ein vollständiges JSON-Objekt mit `schema_version=1`. Nach jedem Event wird synchron geflusht.

Passive Probe-, Input-Test- und Pathing-Test-Läufe ohne aktiven Run erzeugen keine Run-Telemetrie.

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

`stash_full` wird im aktuellen Personal-Stash-MVP mit unbegrenzten Sammel-Tabs nicht heuristisch erzeugt.

## Schema

Beispiel:

```json
{"schema_version":1,"timestamp":"2026-07-10T15:04:57.036Z","event":"stash_attempt","run_id":"countess-...","run":"countess","phase":"stash-personal","area_id":1,"unit_id":240,"code":"glr","name":"Flawless Ruby","attempt":1}
```

Gemeinsame Felder:

- `schema_version`, `timestamp`, `event`, `run_id`, `run`, optional `phase`
- Item-Kontext: `area_id`, `unit_id`, `txt_file_no`, `code`, `name`
- Ergebnis-Kontext: `reason`, `attempt`, `hover_attempt`, `candidate_count`
- Stash-Kontext kann zusätzlich Inventory-Grid-Koordinaten tragen.

## Fehlerverhalten

- Fehler beim Erstellen des Verzeichnisses oder der Datei verhindern den Runtime-Start.
- Write-/Flush-Fehler werden im Loot-Adapter gespeichert und bleiben für den restlichen Run aktiv.
- Der aktuelle Task endet mit `telemetry_failed`.
- Nach dem Fehler startet kein weiterer Pickup, Ctrl-Klick oder Stash-Close-Input.
- Ein Fehler, der beim Protokollieren einer gerade ausgeführten Aktion entsteht, kann diese bereits ausgeführte Aktion naturgemäß nicht rückgängig machen; er verhindert aber jede folgende Aktion.

## Live-Validierung

Der isolierte `stash-personal`-Lauf wurde bei 1280×720 mit einer Dol-Rune (`r14`) im ungeschützten Inventory-Bereich geprüft. Der Ctrl+LMB-Transfer wurde über Memory bestätigt und die zugehörige Run-Datei enthielt genau die erwartete Reihenfolge `stash_attempt` → `stash_success` mit derselben Run- und Unit-ID. Die Stash-Oberfläche wurde anschließend per Escape geschlossen und beide UI-Flags wurden als geschlossen bestätigt.

## Grenzen

- Noch keine Rotation, Kompression oder automatische Bereinigung.
- Keine Dashboard-/Upload-Integration.
- Phase 7.7 ergänzt einen separaten Schema-v2-Session-Recorder für Lifecycle-, Recovery- und Summary-Ereignisse; der Phase-5-Recorder bleibt für Run-/Loot-Details kompatibel.

## Verwandte Features

- [Loot- und Recovery-Loop](loot-recovery-loop.md)
- [Hover-Confirmed Item Pickup](hover-confirmed-item-pickup.md)
- [Personal-Stash MVP](personal-stash-mvp.md)

---
*Zuletzt aktualisiert: 2026-07-10*
