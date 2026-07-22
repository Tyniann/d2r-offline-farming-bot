# Phase-14-Core-Vertrag

## Überblick

Abschnitt 14.0 friert die bestehende Telemetrie-, Queue-, Task-, Loot-, Town-, Pickit-, API- und React-Baseline ein und legt den Metrikvertrag für die neue Historienepoche fest. Dieser Abschnitt ändert keinen produktiven Writer, implementiert keinen HistoryReader und berechnet keine Statistik in React.

Nur neue Schema-3-Daten aus sessiongebundenen produktiven Farming-Runs werden später auswertbar. Schema 1 und 2 bleiben unveränderte Diagnoseartefakte und werden weder importiert noch heuristisch korreliert.

## Ort im Code

- **Core-Vertrag:** `internal/telemetry/history_contract.go`
- **Baseline und Sollmatrix:** `internal/telemetry/phase14_baseline_test.go`
- **Deterministische Fixtures:** `internal/telemetry/testdata/phase14/`
- **Bestehende Writer:** `internal/telemetry/recorder.go`, `internal/telemetry/session_recorder.go`
- **Queue und Run-Grenzen:** `internal/app/supervisor.go`, `internal/app/queue_runtime.go`, `internal/app/session_runtime.go`
- **Run-Pipeline:** `internal/tasks/run_pipeline.go`
- **API-/React-Baseline:** `internal/api/`, `web/src/app/`

## Eingefrorene Baseline

| Bereich | Stand vor Schema 3 | Verbindliche Erkenntnis |
|---|---|---|
| Run-JSONL | Schema 1, eigene vom Recorder erzeugte Run-ID, synchroner Flush | Enthält Step-, Loot-, Stash-, Town- und Pickit-Kontext, ist aber nicht lückenlos mit der Supervisor-Run-ID korreliert. |
| Session-JSONL | Schema 2, eigene Session-ID, Supervisor-Game-/Run-IDs | Enthält Lifecycle-Terminals, aber keine vollständigen Itemketten. |
| Queue | Frischer Run-Zustand pro Eintrag, gemeinsame Game-ID bis Wrap | Die globale Phase-14-Run-ID muss vor beiden Recordern entstehen und darf nicht aus Dateinamen erraten werden. |
| Tasks | Gemeinsame Countess-/Mephisto-Pipeline mit stabilen Step-Namen | Die Stage wird Core-seitig an der Definition/Pipeline gebunden; API und React leiten sie nicht aus Texten ab. |
| Loot/Town | Match, Pickup und Stash sind Memory-verifiziert; Sell-Erfolg ist die Inventory-Transition nach `item_sell` | Ein Klick oder Match allein ist kein gesicherter Return und kein bestätigter Verkauf. |
| Pickit | Unveränderlicher Profil-/Assignment-Snapshot pro Run | Historie zeigt die damalige Profil-/Regelherkunft und bewertet nie gegen aktuelle Regeln neu. |
| API | Loopback-/Origin-geschützt, OpenAPI ist Transportautorität | History bleibt read-only und benötigt keinen Control-Token. |
| React | Darstellung und Commands, keine fachliche Statistik | Aggregate, Raten, Sortierung und Fehlertexte kommen ausschließlich aus dem Core. |

Die vier nach der Phase-13-Abnahme verbleibenden Produktprofile sind `countess-standard`, `gems`, `keys` und `mephisto-standard`. Das Beispiel-Assignment referenziert ausschließlich diese vorhandenen Profile.

## Population und Ergebnisse

- Statistikfähig ist ausschließlich `schema_version=3` mit `mode=productive_farming`, vollständigem Session-/Game-/Run-Kontext und genau einem terminalen Ergebnis.
- Diagnose-, Route-, Town-, Loot-, Input- und isolierte Abnahmemodi sind ausgeschlossen. Es gibt keinen UI-Schalter, der sie zumischt.
- `run_completed` ergibt `success`, `run_failed` ergibt `failed`, `run_aborted` ergibt `aborted`.
- Eine Datei ohne Terminal ist `incomplete`; nur die vom aktiven Core bestätigte Run-ID darf als `running` erscheinen.
- Unvollständige und laufende Runs zeigen ihre beobachtete Dauer, gehören aber nicht in Rate, Durchschnitt, Median, Minimum oder Maximum.

## Metriken und Denominatoren

| Kennzahl | Definition |
|---|---|
| Aktive Run-Zeit | Summe `run_started` bis Terminal aller gefilterten terminalen produktiven Versuche. Fehler und Abbrüche zählen, Zeit zwischen Runs nicht. |
| Erfolgsquote | Erfolgreiche Runs geteilt durch alle terminalen Runs. |
| Bosskills | Pro Run höchstens ein semantisch Memory-bestätigtes `boss_kill_confirmed`. |
| Keep-Return | Unterschiedliche `(run_id, unit_id)` mit `pickit_match(action=keep)` → `pickup_success` → `stash_success`. |
| Keep/Run | Keep-Return geteilt durch alle terminalen Runs. |
| Keep/Kill | Keep-Return geteilt durch bestätigte Bosskills; ohne Kill kein Wert. |
| Keep/Stunde | Keep-Return geteilt durch aktive Run-Zeit in Stunden; ohne terminale Zeit kein Wert. |
| Pickup-Verlust | Gematchte Unit ohne `pickup_success`, unabhängig von wiederholten Attempts. |
| Nach-Pickup-Verlust | Erfolgreich aufgehobene Keep-Unit ohne `stash_success`. |
| Sell | Gematchte und aufgehobene Sell-Unit mit bestätigter Inventory-Transition. Sell ist nie Keep-Return und erhält ohne Goldmessung keinen Wert. |
| Dauerstatistik | Durchschnitt, Median, Minimum und Maximum terminaler Run-Dauern; kein p95. |
| Geringe Datenbasis | `low_sample=true` bei weniger als zehn bestätigten Bosskills pro Vergleichszeile. |

Die Standardsortierung der Boss-/Routentabelle ist Keep/Stunde absteigend. Jede Sortierung endet mit einem stabilen ID-Tiebreaker. Der Fixture-Vertrag umfasst fünf terminale produktive Runs mit `360 s` aktiver Zeit, drei Erfolge, zwei Fehler, vier Bosskills, drei Keep-Returns, einen bestätigten Verkauf, einen Pickup-Verlust und einen Nach-Pickup-Verlust. Daraus folgen exakt `60 %` Erfolg, `72 s` Durchschnitt, `60 s` Median, `30/120 s` Minimum/Maximum, `0,6` Keep/Run, `0,75` Keep/Kill und `30` Keep/Stunde. Ein weiterer produktiver Run ist unvollständig und ein Diagnose-Run wird vollständig ausgeschlossen.

## Funnel und Itemidentität

Der sichtbare Funnel bleibt `gesehen → gematcht → aufgehoben → eingelagert/verkauft`. Ereignisse werden innerhalb derselben Run-ID über dieselbe Unit-ID und denselben unveränderlichen Pickit-Snapshot korreliert.

Der Aggregationsschlüssel ist:

- exaktes Set: `set:<stabiler-katalogschlüssel>`;
- exaktes Unique: `unique:<stabiler-katalogschlüssel>`;
- sonst `base:<excel-code>:<quality>`.

Anzeigenamen sind keine Identität. Fehlender Basiscode, unbekannte Qualität oder widersprüchliche exakte Identität wird nicht geraten und liefert `history_item_identity_invalid`.

## Stage-Vertrag

Jeder produktive Pipeline-Step gehört genau einer disjunkten Kategorie:

| Stage | Steps |
|---|---|
| `travel` | `precheck`, `town_ready_profile`, `acquire_town_waypoint`, `open_waypoint`, `select_run_waypoint`, `wait_entry_area`, `play_bound_route` |
| `combat` | `acquire_boss`, `engage_boss` |
| `loot` | `reposition_for_loot`, `wait_for_drops`, `scan_loot`, `pick_loot` |
| `return_town` | `cast_town_portal`, `enter_town_portal`, `wait_origin_town`, `play_town_egress`, `open_origin_waypoint`, `select_hub_waypoint`, `wait_hub_area`, `open_personal_stash`, `stash_items`, `close_personal_stash`, `prepare_town_handoff`, `complete` |

Stage-Intervalle dürfen nicht überlappen oder die aktive Run-Zeit überschreiten. Nicht klassifizierte Core-Zeit bleibt sichtbar als sonstige Laufzeit und wird nicht verteilt.

## Filter, Pagination und Export

Das kanonische Filterobjekt gilt für Summary, Vergleiche, Items, Runs und Export: UTC-Intervall, Run, Charakter, Difficulty, Ergebnis, Reason-Code und optional Pickit-Profil. Browser-Presets werden vor dem Request aus lokaler Zeit in ein UTC-Halboffenintervall umgewandelt.

- Runlisten sortieren standardmäßig nach UTC-Startzeit, Vergleiche nach Keep/Stunde und Items nach Anzeigename. Vergleiche können außerdem Core-seitig nach Erfolgsquote oder kürzester Durchschnittsdauer sortiert werden; jede Sortierung verwendet einen stabilen ID-Tiebreaker.
- Default-Limit ist `50`, hartes Maximum `200`. Cursor binden Filter, Sortierung und Indexstand.
- Run-CSV besitzt 14 stabile Spalten von `run_id` bis `post_pickup_lost`.
- Item-CSV besitzt 12 stabile Spalten von `item_key` bis `yield_per_hour`.
- JSON, CSV und GUI verwenden dieselbe Core-Analyse. CSV-Zellen werden gegen Spreadsheet-Formeln neutralisiert.

## Reason-Codes

Reader: `history_file_invalid`, `history_file_too_large`, `history_line_too_large`, `history_schema_unsupported`, `history_event_invalid`.

Korrelation: `history_context_missing`, `history_run_id_mismatch`, `history_stream_missing`, `history_terminal_duplicate`, `history_time_invalid`.

Farming: `history_boss_duplicate`, `history_stage_invalid`, `history_item_identity_invalid`, `history_item_chain_invalid`.

API: `history_filter_invalid`, `history_run_not_found`, `history_cursor_invalid`, `history_export_invalid`, `history_unavailable`.

Die deutschen Benutzertexte stammen aus `telemetry.HistoryReasonMessage`; React erfindet keine Synonyme.

## Grenzen

- Kein produktiver Schema-3-Writer, Reader, Index oder Analyzer in 14.0.
- Keine Altlog-Migration, SQLite-Datenbank, persistenter Cache, Retention oder Reparatur.
- Keine Chart-Bibliothek, React-Aggregation, Marktpreise, Goldschätzung, Magic-Find-Normalisierung oder statistische Siegerbehauptung.
- Beschädigte Fixtures werden als ganze Datei verworfen; einzelne Zeilen werden nicht still übersprungen.

## Verwandte Features

- [Run-Telemetrie](run-telemetry.md)
- [Session-Recovery und Lifecycle-Telemetrie](session-recovery-telemetry.md)
- [Session-Lifecycle](session-lifecycle.md)
- [FarmQueue-Scheduler](farm-queue-scheduler.md)
- [Loot Decision Pipeline](loot-decision-pipeline.md)
- [Lokale Core-API](local-core-api.md)

---
*Zuletzt aktualisiert: 22. Juli 2026*
