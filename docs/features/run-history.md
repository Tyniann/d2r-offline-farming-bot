# Run-Historie im Dashboard

## Überblick

Abschnitt 14.6 ergänzt das lokale React-Dashboard um eine eigenständige Historienseite. Sie beantwortet ohne UI-eigene Statistik, welche Runs und Routen gesicherten Keep-Return liefern, wo aktive Zeit verloren geht und an welcher Stelle Loot verloren wurde. Alle Zahlen, Gruppierungen, Sortierungen und deutschen Reason-Texte stammen aus dem Go-Core.

## Ort im Code

- **Feature:** `web/src/features/history/HistoryFeature.tsx`
- **Komponententests:** `web/src/features/history/HistoryFeature.test.tsx`
- **Navigation und SSE-Refresh:** `web/src/app/App.tsx`
- **Responsive Darstellung:** `web/src/app/app.css`
- **Generierter Client:** `web/src/api/generated.ts`

## Funktionalität

### Navigation und Filter

Der Navigationspunkt `Historie` verwendet den stabilen Direktlink `#history`; Browser-Refresh rekonstruiert dieselbe Seite aus Core-Daten. Die Filterleiste bietet gesamte Historie, heute, 7 Tage, 30 Tage oder einen freien lokalen Zeitraum sowie Charakter, Difficulty, Run, Ergebnis, Reason-Code und optionales Pickit-Profil. Zeitwerte werden vor dem Request in UTC konvertiert. Angewendete Filter bleiben als Text sichtbar und lassen sich vollständig zurücksetzen.

### Übersicht und Vergleich

Die Übersicht zeigt terminale Runs, Bosskills, Erfolgsquote, Durchschnitt, Median, gesicherten Keep, bestätigten Sell, Keep pro Run/Kill/Stunde und die getrennten Verlustpfade. Aktive und unvollständige Runs werden genannt, aber nicht in terminale Kennzahlen eingerechnet. Der häufigste Fehler wird zusammen mit verlorener aktiver Zeit priorisiert.

Die Boss-/Routentabelle startet Core-seitig mit `keep_per_hour`. Der Operator kann auf Erfolgsquote oder kürzeste Durchschnittsdauer umschalten; jede Umschaltung fragt die serverseitige deterministische Sortierung neu ab. Sichtbar sind Boss-Sample, Warnung unter zehn Kills, Keep pro Stunde/Run/Kill, Sell, Erfolg, Dauer, alle vier Stage-Anteile und die häufigste Fehlerursache.

### Items, Runs und Detail

Die Itemtabelle trennt gesehen, gematcht, aufgehoben, eingelagert, verkauft, Pickup-Verlust und Nach-Pickup-Verlust und zeigt Yield pro Run, Kill und Stunde. Die Runliste enthält Start, Route, Ergebnis, Dauer, Keep, Sell, beide Verlustpfade und den deutschen Fehlerkurztext. Items und Runs laden weitere serverseitige Cursor-Seiten additiv.

Der Drill-down zeigt Kontext, Route, Ergebnis, Memory-bestätigte Bosskills, den semantischen Funnel, fünf Zeitanteile und jede konkrete Itemkette. Bei Fehlern stehen der stabile Reason-Code, der letzte Step und die deutsche Erklärung nebeneinander. Gematchte Items zeigen den damals eingefrorenen Pickit-Snapshot mit Profil-ID, Regel-ID, Aktion, Profilrevision und Assignment-Revision; aktuelle Profile werden nicht nachträglich ausgewertet. Fehlende Itemereignisse werden ausdrücklich als fehlender Loot dargestellt und nicht ergänzt. Rohereignisse bleiben in einem eingeklappten Diagnoseabschnitt und werden erst beim Öffnen nachgeladen.

### Zustände und Aktualisierung

Loading, leere Historie, gefiltert ohne Treffer, API-Fehler, Exportfehler, aktive/unvollständige Runs, geringe Stichprobe und isolierte Dateidiagnosen besitzen getrennte zugängliche Texte. `history_changed` wird nur nach einer geänderten korrelierten terminalen Population gesendet; einzelne Flushes eines laufenden Runs erzeugen kein SSE-Reload. Das Signal und der manuelle Aktualisieren-Button lösen denselben serialisierten Reload aus. JSON-, Run-CSV- und Item-CSV-Downloads verwenden die aktiven Filter und den vom Core gelieferten sicheren Dateinamen.

Auf kleinen Viewports werden die drei großen Tabellen innerhalb des Features in beschriftete Zeilengruppen umgestellt. Dadurch entsteht kein horizontaler Seitenüberlauf. Tabellen-Captions, native Labels, Fokusrahmen, Buttons und der eingeklappte Diagnosebereich bleiben per Tastatur erreichbar. Phase 14 verwendet keine Chart-Bibliothek.

## Operator / CLI

```powershell
go run ./cmd/d2rbot --config configs/config.yaml --ui
```

Die Seite ist read-only. Filter, Sortierung, Detail und Export senden keinen Control-Command und können keinen Gameplay-Input auslösen.

## Live-Abnahme Phase 14.7

Der produktive Nightmare-Run `countess-20260722t022448999999999z-8ae8e37f` mit `MrBones` bestätigt die vollständige Provenienz über Session- und Run-Stream. Beide Ströme korrelieren dieselbe Session-, Game- und Run-ID; der Run bindet `countess-mrbones-fd1756c208`, den Layout-Fingerprint, Queueposition, D2R-Version und den unveränderlichen Pickit-Snapshot. Memory bestätigte genau einen Countess-Kill. Die Historie zeigt 68,523 Sekunden aktive Zeit mit 47,448 Sekunden Reise, 5,145 Sekunden Kampf, 2,495 Sekunden Loot, 13,194 Sekunden Rückkehr/Stadt und 241 Millisekunden sonstiger Zeit.

Von 14 gesehenen Items matchten zwei die aktive Policy; beide wurden Memory-bestätigt aufgenommen und in den persönlichen Stash transferiert. Deshalb zeigt der Core exakt zwei Keep, keinen Sell und keinen Verlust. Summary, Runliste, JSON-Report und Run-CSV projizieren dieselbe Population mit einem terminalen Erfolg, einem Bosskill und derselben Dauer. Vier sichtbare Dateidiagnosen gehören ausschließlich zu den vorangegangenen fehlgeschlagenen Gate-Artefakten; 105 ältere Dateien liegen vor Schema 3 und werden erwartungsgemäß ignoriert.

Das automatisierte Produktgate verwendet in Analyzer, API/JSON/CSV und React dieselbe kanonische Ergebnismatrix: fünf terminale produktive Runs für Countess und Mephisto auf drei Routen, drei Erfolge, zwei Fehler, vier Bosskills, drei Keep, einen bestätigten Sell sowie je einen Verlust vor und nach Pickup. Aktive und unvollständige Kontrollfälle bleiben sichtbar, aber aggregationsfrei.

## Abhängigkeiten

- React und Browser-Standard-APIs; kein Chart- oder Statistikpaket.
- Generierte OpenAPI-DTOs und History-Clientfunktionen.
- [Historien-API und Export](history-api-export.md) als Transportgrenze.

## Verwandte Features

- [Phase-14-Core-Vertrag](phase-14-core-contract.md)
- [Historienanalyse und Boss-/Routenvergleich](history-analysis.md)
- [Live-Dashboard und Session-Steuerung](live-dashboard.md)

---
*Zuletzt aktualisiert: 22. Juli 2026*
