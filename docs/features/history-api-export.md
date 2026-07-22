# Historien-API und Export

## Überblick

Abschnitt 14.5 stellt die vom Core berechnete Run-Historie read-only für Dashboard und Datei-Export bereit. Alle Endpunkte und Exporte verwenden exakt denselben `HistoryIndex`-Stand, denselben kanonischen Filter und dieselbe `telemetry.AnalyzeHistory`-Auswertung; Handler und React berechnen keine fachlichen Kennzahlen nach.

## Ort im Code

- **Paket:** `internal/api/`
- **Handler:** `internal/api/history_server.go`
- **DTOs und Projektion:** `internal/api/history_dto.go`, `internal/api/history_mapping.go`
- **Backend-Anbindung:** `internal/api/live_backend.go`
- **Maschinenvertrag:** `internal/api/schema/openapi.json`
- **Generierter Client:** `web/src/api/generated.ts`

## Funktionalität

### Read-only Endpunkte

| Endpunkt | Inhalt |
|---|---|
| `GET /api/v1/history/summary` | Population, Ergebnisse, Dauer, Stages, Funnel, Top-Fehler und Dateidiagnosen |
| `GET /api/v1/history/comparisons` | Charakter-/Difficulty-/Definition-/Routenvergleich; Core-Sortierung nach Keep/Stunde, Erfolgsquote oder Durchschnittsdauer |
| `GET /api/v1/history/items` | Stabil nach Itemname und Itemkey sortierte, cursor-paginierte Itemaggregate |
| `GET /api/v1/history/runs` | Absteigend nach UTC-Start und Run-ID sortierte, cursor-paginierte Runliste |
| `GET /api/v1/history/runs/{runID}` | Semantischer Drill-down; `include_raw=true` ergänzt eingeklappte Rohereignisse |
| `GET /api/v1/history/export` | Vollständiger JSON-Report oder CSV-Tabelle `runs` beziehungsweise `items` |

Die Endpunkte benötigen keinen Control-Token, bleiben aber unter dem bestehenden Loopback-, Host- und Origin-Sicherheitsumschlag. Mutierende History-Routen existieren nicht.

### Filter und Pagination

Das identische Filtermodell unterstützt `from`, `to`, `run`, `character`, `difficulty`, `outcome`, `reason` und `pickit_profile`. Listenwerte dürfen als wiederholte Parameter oder kommasepariert ankommen und werden kanonisch sortiert und dedupliziert. Zeitgrenzen müssen explizites UTC-RFC3339 bilden; das Intervall ist halboffen `[from,to)`. Vergleiche akzeptieren `keep_per_hour`, `success_rate` und `average_duration` als Core-seitig deterministisch gebrochene Sortierungen. Ungültige Filter liefern `history_filter_invalid` mit stabilem deutschem Text.

Run- und Itemseiten verwenden standardmäßig 50 und höchstens 200 Zeilen. Ein opaker Cursor bindet Offset, Dataset, Filter, Serversortierung und Indexgeneration. Eine geänderte Population oder Query wird mit `history_cursor_invalid` abgelehnt, statt Zeilen zu überspringen oder doppelt zu liefern.

### Export

`format=json` serialisiert Meta, Summary, Vergleiche, Items und Runs direkt aus derselben Analyzer-Antwort wie die GUI. `format=csv&dataset=runs|items` streamt UTF-8 mit stabiler Spaltenreihenfolge und UTC-Zeitstempeln. Zellen mit spreadsheet-aktiven Präfixen `=`, `+`, `-`, `@` oder Tab werden mit einem Apostroph neutralisiert. Downloadnamen enthalten nur Core-erzeugte Dataset- und UTC-Zeitwerte, niemals Filter oder lokale Pfade.

### Aktualisierung

Jede History-Abfrage aktualisiert den rebuildbaren In-Memory-Index serialisiert. Nach einem terminalen Run-Wechsel wird zusätzlich aktualisiert. Nur wenn sich die Indexgeneration ändert, publiziert der Core `history_changed` mit der neuen Generation. Vollständige Telemetriezeilen, Pfade oder Dateiinhalte werden nicht über SSE transportiert.

## Operator / CLI

Die Historie ist Teil des normalen lokalen UI-Modus:

```powershell
go run ./cmd/d2rbot --config configs/config.yaml --ui
```

Beschädigte Dateien erscheinen einzeln mit Basisdateiname, stabilem Code und deutscher Erklärung. Der übrige gültige Datenbestand bleibt auswertbar. Ein History-Fehler stoppt keinen laufenden Bot und verändert keine JSONL-Datei.

## Abhängigkeiten

- Go-Standardbibliothek für HTTP, JSON, CSV, SHA-256 und URL-Verarbeitung.
- `internal/telemetry` als einzige Autorität für Index, Filter, Kennzahlen, Sortierung und Reason-Texte.
- OpenAPI als Transport-Source-of-Truth; TypeScript-DTOs und Query-Client werden generiert.

## Verwandte Features

- [Phase-14-Core-Vertrag](phase-14-core-contract.md)
- [History-Reader und In-Memory-Index](history-reader-index.md)
- [Historienanalyse und Boss-/Routenvergleich](history-analysis.md)
- [Lokale Core-API](local-core-api.md)

---
*Zuletzt aktualisiert: 22. Juli 2026*
