# Routenoberfläche

## Überblick

Die Electron-App führt veröffentlichte Farming-Routen, geführte Aufnahmen und unveröffentlichte Entwürfe in einer aufgabenorientierten Oberfläche zusammen. Die drei Bereiche „Meine Routen“, „Route aufnehmen“ und „Entwürfe“ teilen sich einen Charakterkontext, zeigen aber jeweils nur die Informationen und Aktionen der aktuellen Aufgabe.

Technische Identitäten, Prüfsummen, rohe Statuscodes, Workflow-Generationen und globale System-Egress-Routen werden nicht als Produktinformation dargestellt. React sendet weiterhin ausschließlich Intents; Aufnahme, Prüfung, Playback und Veröffentlichung bleiben Core-Verantwortung.

## Ort im Code

- **Paket:** `web/src/features/routes/`
- **Einstieg:** `RouteFeature.tsx`
- **Komponenten:** `components/RoutePageHeader.tsx`, `components/RouteLibraryPanel.tsx`, `components/RouteRecordingPanel.tsx`, `components/RouteDraftsPanel.tsx`, `components/RouteWorkflowPanel.tsx`, `components/DeleteDraftDialog.tsx`
- **Darstellungskatalog:** `routePresentation.ts`
- **Feature-Styles:** `RouteFeature.css`
- **Core-Projektion:** `internal/api/live_backend_routes.go`
- **HTTP-Endpunkte:** `internal/api/server.go`
- **Vertragsautorität:** `internal/api/schema/openapi.json`

## Funktionalität

### Meine Routen

Die Bibliothek verwendet kanonische deutsche Run-Namen und verdichtet Lifecycle, Management und Assignment zu „Aktiv“, „Nicht verwendet“, „Unvollständig“ oder „Archiviert“. Gespeicherte Routennamen und Route-IDs erscheinen nicht. Das Kuhlevel ist eine gemeinsame Run-Gruppe mit den Teilzeilen „Wirt-Route“ und „Cow-Route“.

Aktive Routen bieten Archivieren in einem kontextuellen Menü. Der Archivfilter zeigt Wiederherstellen und endgültiges Löschen. Fehlende Routen verlinken direkt auf den passenden Aufnahmekontext.

### Route aufnehmen

Desktop zeigt eine kompakte Run-Liste links und genau eine Anleitung rechts. Unter 760 Pixeln wird die Auswahl als Select oberhalb des Detailbereichs dargestellt. Start und Ziel werden als Ortsnamen gezeigt; Area-IDs und interne Startschlüssel bleiben unsichtbar.

Voraussetzungen erscheinen als kurze übersetzte Checkliste. Nur eine fehlende Voraussetzung erhält eine konkrete Erklärung. `F9` zum Beenden der Aufnahme und `F11` für den Notabbruch stehen direkt an der Aufnahmeaktion und bleiben während einer aktiven Aufnahme sichtbar.

Das Kuhlevel verwendet einen Zweischritt-Umschalter für Wirt- und Cow-Route. Der ausführliche Vorbereitungshinweis erscheint ausschließlich im Kuhlevel-Kontext. Nach einer fertigen Wirt-Aufnahme kann direkt zur Cow-Route gewechselt werden.

### Entwürfe

Entwürfe werden neueste zuerst als Arbeitsliste gezeigt. Sichtbar sind Run, optionale Teilroute, Aufnahmezeit, Schwierigkeit, verständlicher Prüfstatus und ein entscheidungsrelevanter Distanzwert. Kandidaten-ID und SHA-256 bleiben ausschließlich intern an Commands gebunden.

Je nach Zustand lautet die primäre Aktion „Testen“, „Erneut testen“ oder „Veröffentlichen“. „Löschen“ verwendet einen Preview-/Confirm-Vertrag, dessen Token Kandidaten-ID, Zustand und unveränderlichen Route-Hash bindet. Der Dialog zeigt nur Run und Aufnahmezeit. Eine veröffentlichte Farming-Route wird durch das Löschen eines Entwurfs nie verändert.

### Workflowdarstellung

`idle` rendert keinen Status. Aktive Core-Zustände werden in die Phasen „Start wird vorbereitet“, „Aufnahme läuft“, „Aufnahme wird geprüft“, „Test läuft“, „Sichere Rückkehr“ und verständliche Endzustände übersetzt. Unbekannte Backendwerte oder Gründe werden nie roh durchgereicht.

## Datenmodell

`RouteCandidateDTO.created_at` liefert die UTC-Aufnahmezeit für die lokale deutsche Darstellung. Die Mutation `delete_candidate` nutzt dieselben einmaligen, revisionsgebundenen Preview-/Confirm-Fähigkeiten wie die bestehende Routenverwaltung. `CandidateStore.Delete` prüft Zustand und SHA erneut und beschränkt die Löschung auf das validierte Kandidatenverzeichnis.

Die Produktoberfläche lädt Routenbibliothek, Aufnahmeoptionen, Hotkeys, Kandidaten und Workflow parallel. System-Egress-Status wird auf dieser Seite weder geladen noch gerendert.

## Operator / UI

- Der Charakter bleibt beim Wechsel zwischen den drei Bereichen erhalten.
- Ein aktiver Aufnahme- oder Testworkflow öffnet automatisch den passenden Bereich.
- Dialoge schließen per Escape, erhalten einen sichtbaren Fokus und zeigen keine technischen Identitäten.
- Ab 1000 Pixeln ist die Bibliothek zweispaltig; unter 760 Pixeln werden Panels und Aktionen einspaltig.
- Interaktive Ziele sind mindestens 44 × 44 Pixel groß; Status besitzt immer einen Text.

## Abhängigkeiten

Die Oberfläche verwendet die bestehende lokale Core-API, React und Lucide-Icons. Es existiert keine parallele Aufnahme-, Input- oder Egress-Logik im Renderer.

## Verwandte Features

- [Geführte Farming-Routenaufnahme](guided-route-recording.md)
- [Kandidaten-Playback und Routenverwaltung](route-management.md)
- [Farming-RouteCatalog und Lifecycle](route-lifecycle.md)
- [Lokale Core-API](local-core-api.md)

---
*Zuletzt aktualisiert: 13. August 2026*
