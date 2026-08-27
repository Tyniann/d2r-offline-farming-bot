# Globaler System-Egress

## Überblick

System-Egress-Routen führen in den Akten 2–5 von einem Memory-bestätigten Startanker zum lokalen Wegpunkt. Der Vertrag kennt getrennte Routen für die Ankunft aus einem Stadtportal (`portal_arrival`) und für den Start eines neuen Spiels (`spawn`). Beide sind globale Systemassets und ausdrücklich keine charaktergebundenen Farming-Routen.

## Ort im Code

- **Vertrag und Persistenz:** `internal/town/system_egress_contract.go`
- **Runtime-Adapter:** `internal/app/town_egress.go`
- **Setup-Kommandos:** `internal/app/town_egress_mode.go`
- **Dateien:** `configs/routes/town/<act>/egress/portal-waypoint.yaml`, `configs/routes/town/<act>/egress/spawn-waypoint.yaml`
- **Config:** `town.egress.act2` bis `town.egress.act5`

## Vertrag und Validierung

Jede Datei bindet Schema-Version, Akt, Town-Area, Game-Version, Layout-Fingerprint, die Semantik `<startanker> → waypoint`, `walk`, Ankunftstoleranz, Sampling-Distanz und mindestens zwei Weltpunkte. `spawn`-Routen speichern zusätzlich den Routenpunkt, bis zu dem der Layoutnachweis aus Memory vorliegen muss. Character, Klasse, Difficulty, Map Seed und Farming-Route-ID existieren in diesem Format nicht; unbekannte Felder werden beim Laden abgewiesen.

Eine `portal_arrival`-Aufnahme startet ausschließlich in der erwarteten Town-Area und innerhalb von `pathing.town_portal.max_click_distance` eines aus Memory sichtbaren Stadtportals. Eine `spawn`-Aufnahme verlangt stattdessen eine stabil bestätigte Character Identity und einen Start innerhalb der gespeicherten Spawn-Toleranz. Finish veröffentlicht nur innerhalb von `pathing.waypoint.max_click_distance` zum Memory-bestätigten Waypoint, bei genau einem same-town Segment und Walk-only-Bewegung; nach 30 Minuten endet die Aufnahme fail-closed. Ein valider, als bereit gemeldeter Egress kann über das Dashboard nicht überschrieben werden. Ein vorhandener fehlerhafter Draft wird über einen synchronisierten temporären Write atomisch ersetzt.

Vor dem ersten Playback-Input prüft der Adapter Startanker, Akt, Town-Area, Game-Version und Startposition. Bei `portal_arrival` muss das gepinnte Portal aus Memory in Reichweite liegen. Bei `spawn` darf die Layout-Bindung verzögert eintreffen: Der Walker beginnt erst nach stabil bestätigter Character Identity und muss den gespeicherten Layoutnachweis spätestens am gepufferten Prüfpunkt erbringen. Fehlende Anker, ein falscher Spawn oder ein Layoutwechsel stoppen vor weiterem Input. Die Figur wird nicht exakt gegen den ersten Walk-Punkt gemessen; der Walker schließt die begrenzte Restlücke. Die Layout-Bindung vergleicht bei vorhandenen `anchors` dieselben Objekt-/Entrance-Identitäten mit einer Koordinatentoleranz von vier Tiles. Die Wiedergabe erfolgt ausschließlich über den area-gebundenen Force-Move-Walker.

## Operator / CLI

```powershell
go run ./cmd/d2rbot --route inspect-egress:act2
go run ./cmd/d2rbot --route record-egress:act2 --route-name "Lut Gholein Portal bis Wegpunkt"
go run ./cmd/d2rbot --route validate-egress:act2
go run ./cmd/d2rbot --route play-egress:act2
go run ./cmd/d2rbot --route record-egress:act2/spawn --route-name "Lut Gholein Spawn bis Wegpunkt"
go run ./cmd/d2rbot --route validate-egress:act2/spawn
go run ./cmd/d2rbot --route play-egress:act2/spawn
```

`act2` darf durch `act3`, `act4` oder `act5` ersetzt werden. Eine Difficulty-Angabe ist bei globalen Egress-Aufnahmen ungültig. Play führt nach dem Walk den registrierten Wegpunkttransfer zum Rogue Encampment aus und bestätigt die Ziel-Area aus Memory. Ein kurzzeitiger `in_game`-Snapshot ohne Area während dieses Transfers wird ohne weiteren Input bis zur Zielbestätigung oder zum bestehenden Timeout abgewartet; eine konkrete falsche Ziel-Area bleibt terminal. Der Queue-Lifecycle verwendet die eingecheckte `spawn`-Variante automatisch, wenn ein von ihm gestartetes Spiel in Akt 2–5 erscheint.

Während `preflight` meldet der Core im Dashboard am betroffenen Akt ausdrücklich, dass die Aufnahme noch nicht begonnen hat. Alle zwei Sekunden protokolliert er zusätzlich World-Gültigkeit, aktuelle und erwartete Area, Portal-Sichtbarkeit, gemessene Distanz und konfigurierte Maximaldistanz. Erst der sichtbare Status `recording` ist die Freigabe, zum Wegpunkt loszulaufen. Ein vorzeitiges F9 wird fail-closed mit `town_egress_start_unconfirmed` abgewiesen.

## Abgrenzung

System-Egress-Dateien liegen unter `routes/town`, während Lifecycle, Assignment und Farming-Katalog ausschließlich `routes.farming_root` scannen. Dadurch können Egress-Aufnahmen Availability niemals als Farming-Route beeinflussen.

## Live-Abnahme Phase 12

Die globalen Egresses für Akt 2, 4 und 5 wurden über das Dashboard vom jeweiligen Memory-bestätigten Portal-Ankunftspunkt bis zum lokalen Wegpunkt aufgenommen. Der bereits migrierte Akt-3-Egress blieb über denselben generischen Vertrag funktionsfähig. Die abschließende read-only CLI-Validierung bestätigt für alle vier Akte `walk`, den richtigen Akt und einen vollständigen Layout-Fingerprint; Character, Difficulty und Map Seed sind in keiner Datei enthalten.

## Live-Abnahme Phase 24

Die getrennten Spawn-Egresses für Akt 2–5 wurden mit D2R `3.2.92777` aufgenommen, abgespielt und anschließend über `validate-egress:<act>/spawn` geprüft. Jede Wiedergabe erreichte Memory-bestätigt das Rogue Encampment. Der verzögerte Layoutnachweis verhindert dabei weder einen legitimen Start ohne sofort sichtbare Anker noch einen fail-closed Abbruch, wenn der Nachweis bis zum gebundenen Routenpunkt ausbleibt.

## Verwandte Features

- [Town Services](town-services.md)
- [Route Recording und Playback](route-recording-playback.md)
- [Farming-RouteCatalog und Lifecycle](route-lifecycle.md)
- [Notfall-Recovery für Run und Spielstart](emergency-run-recovery.md)

---
*Zuletzt aktualisiert: 27. August 2026*
