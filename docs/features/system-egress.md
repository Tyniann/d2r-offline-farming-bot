# Globaler System-Egress

## Überblick

System-Egress-Routen führen in den Akten 2–5 ausschließlich vom Memory-bestätigten `portal_arrival` zum lokalen Wegpunkt. Sie sind globale Systemassets und ausdrücklich keine charaktergebundenen Farming-Routen.

## Ort im Code

- **Vertrag und Persistenz:** `internal/town/system_egress_contract.go`
- **Runtime-Adapter:** `internal/app/town_egress.go`
- **Setup-Kommandos:** `internal/app/town_egress_mode.go`
- **Dateien:** `configs/routes/town/<act>/egress/portal-waypoint.yaml`
- **Config:** `town.egress.act2` bis `town.egress.act5`

## Vertrag und Validierung

Jede Datei bindet Schema-Version, Akt, Town-Area, Game-Version, Layout-Fingerprint, die Semantik `portal_arrival → waypoint`, `walk`, Ankunftstoleranz, Sampling-Distanz und mindestens zwei Weltpunkte. Character, Klasse, Difficulty, Map Seed und Farming-Route-ID existieren in diesem Format nicht; unbekannte Felder werden beim Laden abgewiesen.

Die Aufnahme startet ausschließlich in der erwarteten Town-Area und innerhalb von `pathing.town_portal.max_click_distance` eines aus Memory sichtbaren Town Portals. Diese bereits für die sichere Portalinteraktion konfigurierte Distanz wird als Ankunftstoleranz im globalen Vertrag gespeichert. Finish veröffentlicht nur innerhalb von `pathing.waypoint.max_click_distance` zum Memory-bestätigten Waypoint, bei genau einem same-town Segment und Walk-only-Bewegung; nach 30 Minuten endet die Aufnahme fail-closed. Ein valider, als bereit gemeldeter Egress kann über das Dashboard nicht überschrieben werden. Ein vorhandener malformed Draft wird über einen synchronisierten temporären Write atomisch ersetzt.

Vor dem ersten Playback-Input prüft der Adapter Akt, Town-Area, Game-Version, Layout-Bindung und Memory-bestätigte Portalnähe (`portal_arrival`). Die Figur wird nicht gegen den ersten Walk-Punkt gemessen — D2R setzt sie nur in der Nähe des Portals ab; der Walker schließt die Restlücke. Die Layout-Bindung vergleicht bei vorhandenen `anchors` dieselben Objekt-/Entrance-Identitäten mit einer Koordinatentoleranz von vier Tiles (beobachteter Personal-Stash-Jitter in Lut Gholein); der SHA-256-Hash bleibt diagnostisch und für Legacy-Dateien ohne `anchors` weiterhin exakt. Die Wiedergabe erfolgt ausschließlich über den area-gebundenen Force-Move-Walker. Fehlende oder falsche Dateien blockieren fail-closed. Die Readiness-Projektion prüft zusätzlich, dass Datei-Akt, Town-Area und Game-Version zum angefragten Setup-Akt passen.

## Operator / CLI

```powershell
go run ./cmd/d2rbot --route inspect-egress:act2
go run ./cmd/d2rbot --route record-egress:act2 --route-name "Lut Gholein Portal bis Wegpunkt"
go run ./cmd/d2rbot --route validate-egress:act2
go run ./cmd/d2rbot --route play-egress:act2
```

`act2` darf durch `act3`, `act4` oder `act5` ersetzt werden. Eine Difficulty-Angabe ist bei globalen Egress-Aufnahmen ungültig. Play führt nach dem Walk den registrierten Wegpunkttransfer zum Rogue Encampment aus und bestätigt die Ziel-Area aus Memory.

Während `preflight` meldet der Core im Dashboard am betroffenen Akt ausdrücklich, dass die Aufnahme noch nicht begonnen hat. Alle zwei Sekunden protokolliert er zusätzlich World-Gültigkeit, aktuelle und erwartete Area, Portal-Sichtbarkeit, gemessene Distanz und konfigurierte Maximaldistanz. Erst der sichtbare Status `recording` ist die Freigabe, zum Wegpunkt loszulaufen. Ein vorzeitiges F9 wird fail-closed mit `town_egress_start_unconfirmed` abgewiesen.

## Abgrenzung

System-Egress-Dateien liegen unter `routes/town`, während Lifecycle, Assignment und Farming-Katalog ausschließlich `routes.farming_root` scannen. Dadurch können Egress-Aufnahmen Availability niemals als Farming-Route beeinflussen.

## Live-Abnahme Phase 12

Die globalen Egresses für Akt 2, 4 und 5 wurden über das Dashboard vom jeweiligen Memory-bestätigten Portal-Ankunftspunkt bis zum lokalen Wegpunkt aufgenommen. Der bereits migrierte Akt-3-Egress blieb über denselben generischen Vertrag funktionsfähig. Die abschließende read-only CLI-Validierung bestätigt für alle vier Akte `walk`, den richtigen Akt und einen vollständigen Layout-Fingerprint; Character, Difficulty und Map Seed sind in keiner Datei enthalten.

## Verwandte Features

- [Town Services](town-services.md)
- [Route Recording und Playback](route-recording-playback.md)
- [Farming-RouteCatalog und Lifecycle](route-lifecycle.md)

---
*Zuletzt aktualisiert: 28. Juli 2026*
