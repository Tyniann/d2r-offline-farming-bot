# Historienanalyse und Boss-/Routenvergleich

## Überblick

Die Historienanalyse ist die einzige Rechenautorität für Farming-Kennzahlen. Sie verarbeitet defensive `HistorySnapshot`-Runs als reine Core-Funktion und liefert Summary, lokale Tages-Buckets, Boss-/Routenvergleich, Itemausbeute und Run-Drill-down. React, Export und API übernehmen exakt diese Ergebnisse und berechnen keine eigenen Statistiken.

## Ort im Code

- **Paket:** `internal/telemetry/`
- **Analyzer und Modelle:** `internal/telemetry/history_analyzer.go`
- **Reader/Index:** `internal/telemetry/history_reader.go`, `internal/telemetry/history_index.go`
- **Vertragsfixtures:** `internal/telemetry/history_analyzer_test.go`

## Funktionalität

### Kanonische Population und Filter

Nur `productive_farming` wird analysiert. Zeitraumgrenzen sind ein halboffenes UTC-Intervall. Run, Charakter, Difficulty, Ergebnis, Reason-Code, Pickit-Profil und Session-ID werden innerhalb ihrer Kategorie als Auswahlmenge und zwischen Kategorien gemeinsam angewendet. Aktive und unvollständige Runs bleiben in der Runliste sichtbar, fließen aber nicht in terminale Quoten, Dauern oder Ertragsraten ein.

### Lokale Tages-Buckets

Der Request benennt eine validierte IANA-Zeitzone; leer bedeutet `UTC`, unbekannte Werte liefern `history_timezone_invalid`. Die installierte Windows-Binärdatei enthält Go-`tzdata`, sodass die Auswertung nicht vom Hostbestand abhängt. Ein Bucket entspricht einem lokalen Kalendertag und veröffentlicht seine exakten UTC-Grenzen. Daher besitzen DST-Wechseltage in `Europe/Vienna` korrekt 23 beziehungsweise 25 Stunden. Tage ohne Runs bleiben als Nulltage erhalten. Terminale Runs, Erfolge, Erfolgsquote, aktive Millisekunden/Stunden, gesicherter Keep und Keep pro aktiver Stunde werden vollständig im Core berechnet.

### Zeit und Fehler

Die aktive Run-Zeit reicht vom korrelierten `run_started` bis zum eindeutigen Session-Terminal. Durchschnitt, Median, Minimum und Maximum verwenden ausschließlich terminale Runs. Step-Start/-Ende bilden disjunkte `travel`-, `combat`-, `loot`- und `return_town`-Intervalle; die nicht zugeteilte Differenz bleibt als `other` sichtbar. Überlappung, offene Steps in terminalen Runs, negative Dauer oder Events außerhalb der Run-Grenzen werden pro Run isoliert: der betroffene Run erscheint in den Meta-Diagnostics und fließt nicht in Summary, Vergleiche oder Itemaggregate ein; die übrigen Runs bleiben auswertbar. Unvollständige oder laufende Runs ohne `EndedAt` dürfen weiterhin einen offenen Step tragen. Ungültige Filter oder Zeitzonen bleiben fail-closed für die gesamte Anfrage.

Fehler werden nach letztem Step plus terminalem Reason-Code gezählt. Die zugehörige verlorene aktive Zeit summiert die vollständige Dauer dieser fehlgeschlagenen oder abgebrochenen Versuche.

### Loot-Funnel und Itemidentität

Jede Itemzahl dedupliziert nach `(run_id, unit_id)`. `KeepReturn` verlangt in derselben Unit-Kette `pickit_match(action=keep)`, `pickup_success` und `stash_success`. Ein bestätigter Verkauf verlangt entsprechend `action=sell`, Match, Pickup und `sell_success`; er bleibt vom Keep-Return getrennt. Match ohne Pickup und Keep-Pickup ohne Stash werden als getrennte Verlustpfade ausgewiesen.

Itemtabellen gruppieren ausschließlich nach dem vom Writer/Reader validierten stabilen Itemkey. Anzeigenamen sind kein Schlüssel. Gesicherter Ertrag pro Run, Bosskill und Stunde verwendet nur vollständige Keep-Ketten; Sell erhält keinen Geldwert.

### Boss-/Routenvergleich

Vergleiche trennen mindestens `(character, difficulty, definition_id, route_id)`. Route-IDs werden niemals zusammengelegt, auch wenn ein Asset später ersetzt oder archiviert ist. Standardsortierung ist absteigend Keep pro aktiver Stunde mit stabiler Vergleichs-ID als Tiebreaker. Fehlgeschlagene und abgebrochene terminale Zeit bleibt im Stundennenner. Bosskill-basierte Raten verwenden ausschließlich `boss_kill_confirmed`; unter zehn Bosskills ist `low_sample=true`.

## Getestete Grenzen

- Die handgerechnete Matrix ergibt fünf terminale Runs, 360 Sekunden, 60 % Erfolg, 72 Sekunden Mittelwert, 60 Sekunden Median, vier Bosskills, drei Keep-Returns, einen Sell und je einen Verlust vor/nach Pickup.
- Derselbe Funnel ergibt 0,6 Keep/Run, 0,75 Keep/Kill und 30 Keep/Stunde.
- Eine Route mit gleicher Keep-Anzahl, aber zusätzlicher Fehlzeit kann die stabilere Route nicht überholen.
- Abbruchzeit bleibt im Nenner; null aktive Zeit liefert keine erfundene Stundenrate.
- Neun Bosskills markieren geringe Datenbasis, zehn nicht.
- Filterpartitionen, gleiche Zeitstempel, stabile Tiebreaker und Division durch null sind deterministisch.
- UTC, `Europe/Vienna`, 23-/25-Stunden-DST, halboffene Grenzen und Nulltage besitzen handgerechnete Tests.

## Grenzen

- Keine p95-, Signifikanz-, Prognose-, Marktwert- oder Magic-Find-Normalisierung.
- Keine aktuelle Pickit-Regel wird rückwirkend auf alte Items angewendet.
- Pagination, HTTP-DTOs und Export werden in Abschnitt 14.5 ergänzt.

## Verwandte Features

- [History-Reader und In-Memory-Index](history-reader-index.md)
- [Phase-14-Core-Vertrag](phase-14-core-contract.md)
- [Run-Telemetrie](run-telemetry.md)
- [Session-Zusammenfassung](session-summary.md)

---
*Zuletzt aktualisiert: 2026-08-28*
