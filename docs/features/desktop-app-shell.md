# Desktop-App-Shell und Designsystem

## Überblick

Abschnitt 15.5 fasst Dashboard, Routen, Pickit, Historie und Einstellungen in einer einzigen React-App-Shell zusammen. Die Oberfläche bleibt Projektions- und Intent-Owner: Fachzustand, Revisionen, Validierungen und Mutationen stammen weiterhin ausschließlich aus dem Core.

## Ort im Code

- **Paket:** `web/src/app/`
- **Einstieg:** `web/src/app/App.tsx`
- **Wichtige Dateien:** `navigation.ts`, `ui.tsx`, `app.css`, `web/public/portal-mark.svg`
- **Featurekomponenten:** `web/src/features/routes/`, `web/src/features/pickit/`, `web/src/features/history/`

## Funktionalität

### Eine Shell, fünf stabile Ziele

Die linke Navigation verwendet ohne Routerpaket die stabilen Hash-Ziele `#dashboard`, `#routes`, `#pickit`, `#history` und `#settings`. Ein unbekanntes oder leeres Ziel fällt auf das Dashboard zurück; der bisherige Link `#betrieb` bleibt als lesbarer Dashboard-Alias erhalten. Beim Zielwechsel erhält der Inhaltsbereich Fokus und der Dokumenttitel wird aktualisiert.

Jedes Ziel rendert die vorhandene Featurekomponente. Es gibt keine zweite Routen-, Pickit-, Historien- oder Dashboardlogik. Der technische Live-Event-Rohfeed ist auf maximal 40 Einträge begrenzt und liegt ausschließlich im Diagnosebereich der Einstellungen.

### Design- und Zustandsbasis

CSS-Custom-Properties definieren Ember, Gold, Crimson, Flächen, Text, Fokus, Abstände und semantische Zustände. Das abstrakte Portalzeichen ist ein originales, lokal ausgeliefertes SVG. Lucide-Icons ergänzen sichtbaren Text und sind dekorativ markiert; Status wird nie nur durch Farbe vermittelt.

Kleine gemeinsame Komponenten decken reale Mehrfachverwendung ab: Seitenkopf, Buttonvarianten, Statusbadges, Loading-/Empty-/Error-Zustände und Dialoge. Dialoge setzen den Anfangsfokus, halten Tabulatorfokus innerhalb des Dialogs, schließen per Escape und geben den Fokus an den Auslöser zurück.

### Responsive und reduzierte Bewegung

Unter 1000 Pixeln wird die Seitenleiste zur kompakten oberen Navigation; unter 620 Pixeln bleiben zugängliche Linknamen erhalten, während visuell nur die Icons gezeigt werden. Die Layouts vermeiden horizontalen Seitenüberlauf. `prefers-reduced-motion: reduce` reduziert Animationen und Übergänge auf ein Minimum.

## Datenmodell

Die Shell führt kein Fach-Datenmodell ein. `StatusDTO`, Katalog, Queue, Routen-, Pickit- und Historien-DTOs bleiben Core-autoritär. Auch der D2R-Kompatibilitätszustand wird nur projiziert und sperrt Auswahl, Queue, Resume und Live-Routenaktionen zusätzlich zu den serverseitigen Gates.

## Operator / CLI

- Dashboard ist das Startziel.
- App-/Core-Version und Verbindung stehen im Desktoplayout unten links; im kompakten Layout bleibt der Livezustand im Seitenkopf sichtbar.
- Settings zeigt in 15.5 nur den vorhandenen Diagnosefeed. Die Bindung der Operator- und Desktop-Einstellungen folgt in Abschnitt 15.6.
- Relevante Fehler, Loading- und Empty-Zustände bleiben persistent sichtbar; es gibt keine rein farbliche Rückmeldung.

## Abhängigkeiten

- React für die bestehende Renderer-Oberfläche
- Lucide React für dekorative Navigations- und Zustandsicons
- Keine Router-, Theme-, Tailwind- oder Komponentenframework-Abhängigkeit

## Verwandte Features

- [Live-Dashboard und Session-Steuerung](live-dashboard.md)
- [Routenbibliothek und Setup-Assistent](route-dashboard.md)
- [Pickit-Profilbibliothek und Editor](pickit-editor.md)
- [Run-Historie im Dashboard](run-history.md)
- [Tatsächliches D2R-Versionsgate](d2r-version-gate.md)

---
*Zuletzt aktualisiert: 22. Juli 2026*
