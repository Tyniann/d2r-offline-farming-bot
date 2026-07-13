# Inventory-Full-Recovery

## Überblick

Phase 5.7 erkennt wertvollen, wegen fehlendem freien Inventarplatz blockierten Ground-Loot explizit als `inventory_full`. Der Bot beendet weitere Pickup-Versuche, castet ein Town Portal, betritt ausschließlich ein aus Memory erkanntes und hover-bestätigtes Spielerportal und verifiziert die Ankunft im Rogue Encampment.

Vorhandene Inventar-Items werden niemals gedroppt. Phase 5.8 hängt nach der Town-Recovery die geschützte Personal-Stash-Routine an.

## Ort im Code

- **Pakete:** `internal/loot`, `internal/tasks`, `internal/pathing`, `internal/memory`, `internal/world`
- **Einstieg:** `internal/tasks/countess.go`
- **Wichtige Dateien:** `internal/app/loot_actions.go`, `internal/pathing/town_portal.go`, `internal/memory/object_ids_data.go`, `internal/world/object_ids_data.go`
- **Config:** `pathing.town_portal` in `configs/config.example.yaml`
- **Generator:** `tools/generate-object-catalog`

## Funktionalität

### Inventory-Full-Entscheidung

`loot.Filter.Decide` bleibt die Quelle der Platzentscheidung. Ein Pickit-Match mit `DecisionReasonInventoryFull` wird im App-Adapter als blockierter wertvoller Kandidat gezählt. `LootScanResult.InventoryFull` ist nur dann `true`, wenn mindestens eine solche explizite No-Fit-Entscheidung vorliegt; `capacity_unsafe` wird nicht fälschlich als `inventory_full` bezeichnet.

Sobald ein Scan `InventoryFull=true` meldet:

1. startet kein weiterer Pickup-Executor,
2. wird das strukturierte Log-Ereignis `inventory_full` geschrieben,
3. wechselt der Countess-Run zur Town-Recovery.

Alle aktuellen Pickit-Matches gelten im MVP als wertvoll. Ein endgültiges `pickup_failed` betrifft dagegen nur das einzelne Item: Die Unit-ID wird für den Loot-Durchlauf übersprungen und weitere Kandidaten werden geprüft.

Ein einzelner gültiger Scan ohne Pickup-Kandidat beendet die Loot-Phase nicht. Sowohl der initiale `scan_loot`-Schritt als auch Rescans nach einem Pickup verlangen drei aufeinanderfolgende No-Target-Snapshots. Ein wieder sichtbarer Kandidat setzt den Zähler zurück. Dadurch führt kurzzeitiger Unit-Table-Churn nicht dazu, dass weiterer wertvoller Loot liegen bleibt und verfrüht das Portal gecastet wird.

### Town-Recovery

Der Abschluss des Full Runs und von `--phase loot-countess` lautet:

```text
scan_loot -> pick_loot? -> cast_town_portal -> enter_town_portal
-> wait_act1_town -> open_personal_stash -> stash_items
-> close_personal_stash -> complete
```

`enter_town_portal` verwendet `pathing.TownPortalActions`. Der Baustein wartet begrenzt auf ein `ObjectKindTownPortal`, verlangt vor dem Hover-Loop 500 ms lang dieselbe `UnitID` und Position und friert den bestätigten Kandidaten anschließend im eigenen Entity-Clicker ein. Ein Klick erfolgt nur, wenn der Memory-Hover `UnitType=object` und dieselbe `UnitID` bestätigt. Feste Portal-Bildschirmkoordinaten und blinde Klicks sind verboten.

`wait_act1_town` darf während Loading und inkonsistenten Snapshots ohne Input weiterlaufen. Erfolg ist ausschließlich ein gültiger `in_game`-Snapshot im `Rogue Encampment`. Tower Cellar Level 5 bleibt während der Übergangswartezeit zulässig; jedes andere gültige Gebiet endet mit `unexpected_area`.

### Versionsgebundene Objekt-IDs

Portal-, Waypoint- und Good-Chest-IDs stammen nicht aus d2go. Der eigene Generator liest das explizite Feld `*ID` aus der lokal extrahierten Datei `data/global/excel/objects.txt` und selektiert stabile Exportmerkmale:

- `Class=TownPortal`
- `Class=PlaceUniqueChest`
- `Name=Waypoint`
- `Class=Bank`

Er erzeugt synchron die Memory-Allowlist und den World-Katalog:

```powershell
go run ./tools/generate-object-catalog `
  -src .tmp/d2r-excel `
  -version 3.2.92777
```

Der Export für `3.2.92777` zeigte außerdem, dass der frühere Good-Chest-Wert veraltet war: `PlaceUniqueChest` wird nun ausschließlich aus `objects.txt` generiert.

## Datenmodell

| Feld / Typ | Rolle |
|------------|-------|
| `LootScanResult.InventoryFullCandidateCount` | Anzahl expliziter Pickit-No-Fit-Kandidaten |
| `LootScanResult.InventoryFull` | Stoppt Pickup und startet Town-Recovery |
| `world.ObjectKindTownPortal` | Semantische Klassifikation des generierten Portal-Objekts |
| `pathing.TownPortalActionResult` | Tick-Ergebnis für Discovery und hover-bestätigten Eintritt |

## Operator / CLI

```yaml
pathing:
  town_portal:
    appear_timeout_ms: 2000
    max_click_distance: 15
```

Stabile terminale Gründe:

| Reason | Bedeutung |
|--------|-----------|
| `town_portal_not_found` | Nach dem Cast erschien innerhalb des Limits kein generiertes Portal-Objekt |
| `town_portal_enter_failed` | Hover, Distanz, Projektion oder Input verhinderten den sicheren Klick |
| `unexpected_area` | Nach Portal-Eintritt erschien ein anderes gültiges Gebiet als Cellar 5 oder Rogue Encampment |

Manueller E2E-Test:

```powershell
go run ./cmd/d2rbot --run countess --phase loot-countess --probe --verbose
```

Der Operator startet in Tower Cellar Level 5. Der Test verarbeitet vorhandenen Loot, castet und betritt das Portal und endet erst nach verifizierter Ankunft im Rogue Encampment.

### Live-Validierung

Am 10.07.2026 wurde `loot-countess` mit D2R `3.2.92777` vollständig live validiert:

- Tir-Rune als Pickit-Kandidat erkannt, nach einem hover-bestätigten Klick aufgenommen und über `Location=inventory` bestätigt.
- Town Portal nach dem Cast als generiertes `ObjectKindTownPortal` mit einer konkreten Runtime-`UnitID` enumeriert.
- Portal-Hover nach acht Spiralpositionen bestätigt; vorher erfolgte kein Klick.
- Linksklick führte nach Loading ins Rogue Encampment; `wait_act1_town` bestätigte das Gebiet nach 103 ms.
- Task endete mit `outcome=success`.

Der erste Live-Anlauf zeigte außerdem einen transienten Rescan-Fehler: Von zwei erkannten Runen wurde zunächst nur eine aufgenommen. Die Loot-Abschlussbedingung wurde daraufhin von einem einzelnen auf drei stabile No-Target-Scans verschärft und mit einem Reappear-Integrationstest abgesichert.

Der eigentliche `inventory_full`-Branch ist zusätzlich durch Task-Integrationstests abgedeckt: Kein Pickup wird gestartet, Town-Recovery wird ausgeführt und der Run endet erst nach dem Town-Snapshot. Vorhandene Inventar-Items werden in keinem Pfad gedroppt.

## Abhängigkeiten

- [Loot Decision Pipeline](loot-decision-pipeline.md) – liefert die explizite No-Fit-Entscheidung
- [Hover-Confirmed Item Pickup](hover-confirmed-item-pickup.md) – überspringt endgültig fehlgeschlagene Einzelitems
- [Pathing](pathing.md) – stellt Projektion und hover-bestätigten Entity-Klick bereit
- Lokaler D2R-Export `data/global/excel/objects.txt` – alleinige Quelle semantischer Objekt-IDs

## Verwandte Features

- [Countess-Run](countess-run.md)
- [Inventory Model und Lock Grid](inventory-lock-grid.md)
- [Loot- und Recovery-Loop](loot-recovery-loop.md)

---
*Zuletzt aktualisiert: 2026-07-10*
