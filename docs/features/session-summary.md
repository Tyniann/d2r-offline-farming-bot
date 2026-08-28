# Session-Zusammenfassung

## Überblick

Nach dem Ende einer Farming-Session zeigt die Desktop-App einen Dialog mit der Sitzungsdauer sowie den aggregierten aufgehobenen und verkauften Items. Die Listen bleiben zugeklappt, bis der Operator den jeweiligen Header öffnet. Kennzahlen kommen unverändert aus der Historienanalyse.

## Ort im Code

- **Paket:** `internal/app/`, `internal/telemetry/`, `internal/api/`, `web/src/features/dashboard/`
- **Einstieg:** `web/src/features/dashboard/SessionSummaryDialog.tsx`
- **Wichtige Dateien:** `internal/app/supervisor.go`, `internal/app/queue_runtime.go`, `internal/telemetry/history_analyzer.go`, `internal/api/history_server.go`, `web/src/app/App.tsx`
- **Config:** keine neuen Keys

## Funktionalität

### Auslösung

Der Dialog öffnet sich nur beim Übergang aus einem aktiven Sessionzustand in `idle`, `idle_in_game` oder `stopped_error`. Ein Reload im Leerlauf öffnet ihn nicht erneut. Ein neuer Sessionstart schließt einen noch offenen Dialog.

Die native Windows-Benachrichtigung bei unfokussiertem Fenster bleibt unverändert.

### Inhalt

Der Dialog zeigt die Wanduhr-Dauer vom Sessionstart bis zum terminalen Supervisor-Ende als `HH:MM:SS`. Darunter stehen zwei Header:

- aufgehobene Items als `keep_return` der Historienanalyse
- verkaufte Items als bestätigte Sell-Kette

Ein Klick auf den Header klappt die nach stabilem Itemkey aggregierte Liste auf. Die sichtbare Menge pro Zeile ist `stashed` beziehungsweise `sold`. Anzeigenamen kommen aus dem CASC-Katalog. Der Dialog hat eine begrenzte Höhe; längere Listen scrollen.

### Datenquelle

`last_result.session_id` und `last_result.duration_ms` kommen aus dem Status nach Sessionende. Summary lädt über `GET /api/v1/history/summary?session=…` ohne `limit`; Itemlisten über `GET /api/v1/history/items` mit demselben Sessionfilter und einer Seite bis 200 Zeilen. `limit` ist nur auf den paginierten Listenendpunkten erlaubt. React berechnet keine eigenen Funnel-Zahlen.

## Datenmodell

- `SupervisorSnapshot.LastSessionID` und `LastSessionDurationMs` überleben das Löschen des aktiven Run-Requests
- `SessionResultDTO.session_id`, `duration_ms`
- `HistoryFilter.SessionIDs` als Query `session`

## Operator / CLI

- Nach Sessionende den Dialog lesen, Header bei Bedarf aufklappen und schließen
- Bei Ladefehler erneut versuchen; Farming bleibt unabhängig möglich
- CLI ohne Desktop-UI zeigt den Dialog nicht

## Abhängigkeiten

Historienindex, Loopback-API, React-Dialog und i18n DE/EN.

## Verwandte Features

- [Session-Lifecycle](session-lifecycle.md)
- [Historienanalyse und Boss-/Routenvergleich](history-analysis.md)
- [Historien-API und Export](history-api-export.md)
- [Live-Dashboard](live-dashboard.md)

---
*Zuletzt aktualisiert: 2026-08-28*
