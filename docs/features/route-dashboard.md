# Routenbibliothek und Setup-Assistent

## Überblick

Das lokale Dashboard stellt Farming-Routen, unveröffentlichte Kandidaten und globale System-Egress-Bereitschaft über versionierte, pfadfreie Core-DTOs dar. Aufnahme und Egress-Setup rufen dieselben Runtime-Adapter und Recorder auf wie die CLI; die Web-Anwendung besitzt keine parallele Routen- oder Inputlogik.

## Ort im Code

- **Core-Projektion:** `internal/api/live_backend_routes.go`
- **HTTP-Endpunkte:** `internal/api/server.go`
- **Vertragsautorität:** `internal/api/schema/openapi.json`
- **React-Feature:** `web/src/features/routes/RouteFeature.tsx`
- **Recorder:** `internal/app/recording_coordinator.go`, `internal/app/route_record.go`

## API und Sicherheit

`GET /api/v1/routes?character=…&include_archived=…` liefert nur Farming-Katalogeinträge. Recording-Optionen liegen unter `GET /api/v1/route-recording/options`, Aufnahme und idempotentes Finish unter `POST /api/v1/route-recordings` beziehungsweise `POST /api/v1/route-recordings/{id}/finish`, Kandidatentests unter `POST /api/v1/route-candidates/{id}/test` und Systembereitschaft unter `GET /api/v1/system-routes/status`. Publish, Archive, Restore und Delete verwenden die im Phase-12-Vertrag benannten Preview-/Confirm-Pfade. DTOs enthalten IDs, Zustände, Revisionen und Hashes, aber keine lokalen Dateipfade.

Preview-Aufrufe sind seiteneffektfrei. Confirm, Workflow-Start und API-Finish benötigen den pro Prozess zufälligen Control-Token. Management-Bestätigungen binden Katalog-, Lifecycle- und Assignment-Revision sowie einen einmal verwendbaren Token. Workflow-Starts müssen die aktuelle Workflow-Generation nennen und kollidieren fail-closed mit aktiven Sessions. Während eines Workflows sind Selection, Queue, Sessionbefehle und Routenmutationen gesperrt; Kandidatentest und Publish revalidieren den live bestätigten Character-/Difficulty-Kontext.

SSE verwendet die bestehende monotone Eventfolge. `route_workflow_changed` enthält Workflow-ID, Zustand, Run beziehungsweise Akt, Area, Segment, Fortschritt und Reason; `route_library_changed` meldet persistente Mutationen. Beide lösen im Browser einen autoritativen Refresh aus; Browser-Reconnect verwendet weiterhin `Last-Event-ID` und den Server-Replay-Puffer.

## Bedienung

Die Farming-Bibliothek zeigt niemals Town- oder Egress-Routen. Fehlende globale Egresses werden ausschließlich im Setup-Bereich pro Akt angezeigt. Der betroffene Akt zeigt dort dauerhaft den Core-Zustand: `preflight` bedeutet, dass der Memory-bestätigte Portal-Ankunftspunkt noch fehlt und die Aufnahme noch nicht läuft; erst `recording` fordert zum Loslaufen auf. Farming-Aufnahmen verwenden davon getrennte Anweisungen: am Startwegpunkt warten, erst bei `recording` der Teleport-Route folgen, den Boss nicht angreifen und an der gewählten Kampfposition F9 drücken. Die Run-Bereitschaft ersetzt den früheren abstrakten Validierungstext durch einen konkreten Hinweis: Fernkämpfer sollen die Aufnahme mit etwas Abstand zum Boss beenden, weil diese Position später als Kampfanker dient. Freeze, Prüfung, beide TP-Rückwege und Kandidaten-Playback weisen ausdrücklich darauf hin, dass keine Benutzereingabe erfolgen darf. Bereits vorhandene Egresses können isoliert abgespielt werden.

Kandidaten werden auf den ausgewählten Charakter gefiltert. Da das Routenfeature bereits vor der asynchronen Core-Bestätigung gemountet wird, übernimmt es einen später bestätigten Charakter in seinen anfangs leeren Filterzustand; eine bewusst manuell gewählte andere Bibliotheksansicht wird nicht überschrieben. Kandidaten können erst aus `validated` getestet und erst nach `test_passed` zur revisionsgebundenen Veröffentlichung vorgeschlagen werden. Während eines aktiven Workflows sind weitere Aufnahme-, Test- und Managementaktionen gesperrt. Archive, Restore und endgültiges Delete verwenden zugängliche Preview-/Confirm-Dialoge; Escape schließt, der passende Bestätigungsbereich erhält Fokus. Bei Delete muss der Benutzer die angezeigte Route-ID exakt selbst eingeben. Beim Replace nennt der eine Freigabedialog den unverändert zu archivierenden Vorgänger.

Die dauerhaft auffindbare Hotkey-Hilfe liest die effektiven Werte aus dem Core: F9 friert eine Aufnahme ein, F10 stoppt nach dem aktuellen Run, F11 ist der sofortige Emergency Stop und Pause merkt Pause-after-run vor.

Phase 20.2 projiziert für `cows` zwei Recording-Optionen mit `route_role`, Startart, rollenbezogenen Anweisungen und Operatorhinweisen. Das Dashboard zeigt getrennt „Wirt-Route aufnehmen“ und „Cow-Route aufnehmen“, übernimmt die Rolle in Workflow, Kandidatenreview und Routenbibliothek und zeigt den Kandidatenhash. Die Wirt-Karte weist auf das bereits offene Tristram-Portal und den verbotenen Wirt-Klick hin; die Cow-Karte verlangt das manuell geöffnete sowie vor der Aufnahme vollständig geleerte Cow Level. React berechnet keine gemeinsame Kompatibilität, sondern rendert ausschließlich Core-DTOs.

## Grenzen

React sendet nur Intents. Sampling, Validierung, Kandidatenspeicherung, Test, Publish und System-Egress-Playback bleiben Core-Verantwortung. Der System-Setup-Flow schreibt ausschließlich den globalen `portal-waypoint.yaml`-Vertrag außerhalb des Farming-Katalogs.

## Live-Abnahme Phase 12

Der vollständige Countess-Zyklus wurde im Dashboard erfolgreich abgenommen: geführte Aufnahme, F9-Freeze, erster TP-Rückweg, isolierter Test ab `portal_arrival`, Town-/Waypoint-Normalisierung, kandidatengenaues Playback, erneute Boss-/Distanzprüfung, zweiter TP-Rückweg und sichtbarer Status „Test bestanden“. Der einzige Freigabedialog zeigte den neuen Eintrag sowie `black-marsh-cellar5-nightmare-mrbones` als unverändert zu archivierenden Vorgänger. Nach einer Bestätigung ist die neue Route Countess/`MrBones` zugewiesen und der Vorgänger in der Archivansicht vorhanden.

## Verwandte Features

- [Geführte Farming-Routenaufnahme](guided-route-recording.md)
- [Kandidaten-Playback und Routenverwaltung](route-management.md)
- [Globaler System-Egress](system-egress.md)
- [Lokale Core-API](local-core-api.md)

---
*Zuletzt aktualisiert: 1. August 2026*
