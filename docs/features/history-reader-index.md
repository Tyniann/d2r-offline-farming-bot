# History-Reader und In-Memory-Index

## Überblick

Der Phase-14-History-Reader liest neue Schema-3-Telemetrie strikt und read-only. Der In-Memory-Index korreliert getrennte Session- und Run-Dateien zu stabilen Farming-Runs. JSONL bleibt die einzige persistente Autorität; der Index kann jederzeit vollständig daraus rekonstruiert werden.

## Ort im Code

- **Paket:** `internal/telemetry/`
- **Reader:** `internal/telemetry/history_reader.go`
- **Index und Modelle:** `internal/telemetry/history_index.go`
- **Writer-Vertrag:** `internal/telemetry/recorder.go`, `internal/telemetry/session_recorder.go`
- **Config:** `telemetry.directory` in `configs/config.example.yaml`

## Funktionalität

### Begrenzter Schema-3-Reader

Der Reader betrachtet nur reguläre `*.jsonl`-Dateien direkt im konfigurierten Telemetrieverzeichnis. Eine Datei ist auf 32 MiB, eine Zeile auf 1 MiB und eine Datei auf 100.000 Ereignisse begrenzt. Jede Schema-3-Zeile muss genau ein bekanntes JSON-Objekt mit bekanntem Event, passendem Stream, UTC-Zeit und stabilem Kontext enthalten. Unbekannte Felder, Mischschemas, rückwärts laufende Zeit, doppelte Terminals oder Bosskills, unstabile Run-/Itemkontexte und ungültige Stage-Zuordnungen schließen die ganze Datei aus.

Schema 1 und 2 gehören zur Vor-Epoche. Sie bleiben unverändert auf Platte, werden gezählt und ohne Migration oder Heuristik ignoriert. Eine gerade wachsende Datei ohne abschließenden Zeilenumbruch wird nicht als beschädigt gemeldet: Ein vorhandener letzter stabiler Indexstand bleibt erhalten und der nächste Refresh liest erneut.

### Cross-Stream-Korrelation

Ein produktiver History-Run benötigt dieselbe globale Run-ID in Session-Lifecycle, Run-Dateiname und jedem Run-Event. Session-ID, Game-ID, Modus, Charakter, Difficulty, D2R-Version, Run, Queue-Index und Queue-Zyklus müssen exakt übereinstimmen. Definition, Route, Layout-Fingerprint, Startzeit und Pickit-Snapshot bleiben innerhalb der Run-Datei unveränderlich.

Das Ergebnis stammt aus genau einem Session-Terminal `run_completed`, `run_failed` oder `run_aborted`. Fehlt es, ist der Run `incomplete`. Nur wenn der aufrufende Core genau diese Run-ID aktuell als aktiv bestätigt, projiziert ein Snapshot vorübergehend `running`. Diagnose-Runs ohne produktiven Sessionvertrag werden weder als Farming-Run noch als Fehler in die Hauptpopulation aufgenommen.

### Rebuildbarer Index

`Refresh` bildet einen neuen Dateisatz außerhalb des Locks, bindet Größe und Änderungszeit und erkennt Inhaltsänderungen über SHA-256. Stimmt der Inhaltsnachweis mit dem bereits vollständig validierten Stand überein, wird dessen immutable Projektion ohne erneutes JSON-Decoding und ohne erneute semantische Validierung wiederverwendet. Geänderte Inhalte werden vollständig neu geparst; Runs, Diagnosen und Generation werden anschließend atomar ersetzt. Unveränderte Inhalte erhöhen die Generation nicht. `Rebuild` verwirft nur den flüchtigen Cache und liest dieselben JSONL-Quellen erneut; es existiert weder SQLite noch ein persistenter Cache, Watcher oder Hintergrunddienst.

Snapshots sind defensiv kopiert und stabil nach Startzeit plus Run-ID sortiert. Gleichzeitige Abfragen, Refreshes und synchron flushende Writer teilen keine mutierbaren Event-Slices.

## Fehlerverhalten

- Eine beschädigte oder überlange Datei erzeugt eine Diagnose mit Dateiname, stabilem Reason-Code und deutscher Erklärung.
- Die betreffende Datei trägt keine teilweise gelesenen Events oder Aggregate bei.
- Eine Änderung ersetzt den alten Dateiinhalt vollständig; wird eine zuvor gültige Datei beschädigt, verschwindet ihr Run aus dem Index.
- Eine fehlende produktive Gegenhälfte liefert `history_stream_missing`; Kontextabweichungen liefern `history_run_id_mismatch`.
- Reader und Index öffnen keine Pfade außerhalb des konfigurierten Verzeichnisses und verändern, löschen oder reparieren keine Datei.

## Grenzen

- Die Berechnung von Kennzahlen, Funnel-Verlusten und Stage-Dauern beginnt erst in Abschnitt 14.4.
- API, Pagination und Exporte werden erst in Abschnitt 14.5 auf den Index gesetzt.
- Es gibt keine automatische Rotation, Kompression oder Retention.

## Verwandte Features

- [Phase-14-Core-Vertrag](phase-14-core-contract.md)
- [Run-Telemetrie](run-telemetry.md)
- [Session-/Recovery-Telemetrie](session-recovery-telemetry.md)

---
*Zuletzt aktualisiert: 2026-07-22*
