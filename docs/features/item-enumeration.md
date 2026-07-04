# Item Enumeration Read-Only

## Überblick

Phase 5.1 bringt Items read-only aus dem D2R-Speicher in das World Model. Der Fokus liegt bewusst auf positionierten Ground-Drops nach einem Countess-Kill: Sie sollen im Probe-Log plausibel sichtbar sein, ohne dass der Bot Items anklickt, Pickit-Regeln bewertet oder Inventar/Stash verändert.

## Ort im Code

- **Pakete:** `internal/memory/`, `internal/world/`, `internal/app/`
- **Einstieg:** `internal/app/run_tick.go` ruft `Probe.Snapshot()` und `World.Update()` im bestehenden Poll-Loop auf
- **Wichtige Dateien:** `internal/memory/items.go`, `internal/world/item.go`, `internal/app/world_log.go`
- **Config:** keine neuen Keys

## Funktionalität

### Memory-Enumeration

Items werden nur bei `Valid && Phase=in_game` enumeriert. Die Probe läuft über das UnitTable-Segment `4` mit eigenen Limits (`maxItemUnitVisits=4096`, `maxItemsPerSnapshot=128`) und verbraucht nicht das bestehende Entity-Budget für Countess-Objekte, Entrances und Monster.

Für 5.1 werden live nur plausible Ground-Items akzeptiert (`RawLocation` 3 oder 5) und sie müssen eine lesbare Path-Position haben. Nil- oder unlesbare Pfade werden übersprungen, weil das Pass-Kriterium positionierte Drops im Probe-Log verlangt. Nicht-Ground-Locations sind im World Model vorbereitet, aber nicht live-validiert.

Item-Fehler sind fail-open: Ein kaputter Item-Walk liefert eine leere Item-Liste und Debug-Logs, löscht aber keine bereits gelesenen Countess-Entities. Einzelne nicht lesbare Items werden übersprungen.

### World Model

`memory.ItemUnit` bleibt roh und enthält keine semantischen Labels. `world.Item` löst daraus Qualität, Location, Name/Code, Item-Type und Hover-Status auf. Der Item-Katalog ist eine statische, repo-lokale Generierung aus den lokal extrahierten D2R-Tabellen `data/global/excel/weapons.txt`, `armor.txt` und `misc.txt` für D2R `3.2.92777`; er enthält Code, Name, Type und Tier-Codes, damit Countess-Drops im Log lesbar werden, ohne d2go als Runtime-Dependency einzubinden.

### Item-Katalog regenerieren

Wenn `TxtFileNo`-IDs aus dem Speicher nicht zu sichtbaren Item-Namen passen, wird nicht an Memory-Offsets geraten. Zuerst wird geprüft, ob Unit, Hover und Position zusammenpassen. Beispiel aus Phase 5.1: Der sichtbare `Key of Terror` wurde gehovert, `hover_type=item hover_unit_id=309` passte zu einem Ground-Item mit `unit=309`, aber der alte Katalog kannte `id=662` nicht. Das bewies: Reader und Hover-Match waren plausibel, der semantische Katalog war veraltet.

Der Katalog wird aus der lokalen D2R-Installation neu erzeugt:

1. Mit CascView oder einem gleichwertigen CASC-Reader die lokale D2R-Storage read-only öffnen, z. B. `D:\Games\Diablo II Resurrected Infernal Edition`.
2. Nur diese Dateien extrahieren:
   - `data/global/excel/weapons.txt`
   - `data/global/excel/armor.txt`
   - `data/global/excel/misc.txt`
3. Die Dateien temporär unter `.tmp/d2r-excel/` ablegen.
4. `internal/world/item_catalog_data.go` aus genau dieser Reihenfolge generieren: `weapons.txt` -> `armor.txt` -> `misc.txt`.
5. Wie d2go Zeilen ohne `code` überspringen; jede akzeptierte Zeile erhöht die `TxtFileNo`.
6. Vor dem Einsatz konkrete Live-Beweise testen, z. B. für D2R `3.2.92777`: `538 -> Gold`, `615 -> Flawless Skull`, `634 -> Thul Rune`, `635 -> Amn Rune`, `662 -> Key of Terror`.

Diese Vorgehensweise ist absichtlich datengetrieben: Wenn zukünftige D2R-Versionen Item-IDs verschieben, wird der Katalog aus den Spieldaten aktualisiert. Memory-Offsets werden erst angefasst, wenn Hover/Unit/Position nicht mehr zusammenpassen oder der Reader selbst widersprüchliche Rohdaten liefert.

### Probe-Logging

Normale `world state`-Logs enthalten `item_count` und `ground_item_count`. Der Fingerprint berücksichtigt nur stabile Ground-Item-Identität (`UnitID`, `TxtFileNo`, `Location`), damit Inventory-/Unknown-Churn keine Operator-Logs spammt. Mit `--probe --verbose` erscheint zusätzlich ein gekappter `ground_items_hint` mit UnitID, Item-ID, Code, Type, Name, Qualität und Position. Offensichtliche Dummy-Types wie `body` werden aus diesem Operator-Hint ausgeblendet.

## Datenmodell

| Typ | Rolle |
|-----|-------|
| `memory.ItemUnit` | Rohe Item-Daten aus dem Item-Segment: `TxtFileNo`, `UnitID`, `Quality`, `RawLocation`, Position, Flags, Identified/Ethereal, rohe Stats |
| `memory.RawStat` | Bounded Raw-Stat-Eintrag ohne Life/Mana-Skalierung |
| `world.Item` | Semantisches Item im World State |
| `world.ItemQuality` | Qualität: normal, magic, rare, unique, set usw. |
| `world.ItemLocation` | Vorbereitete Locations: `ground`, `inventory`, `equipped`, `belt`, `cursor`, `stash`, `shared_stash_1..3`, `socket`, `unknown` |

Query-Helfer:

```go
state.GroundItems()
state.ItemsByLocation(world.ItemLocationGround)
state.FindItemByUnitID(unitID)
```

## Operator / CLI

Validierung erfolgt read-only:

```powershell
go run ./cmd/d2rbot --probe --verbose
```

Nach einem Countess-Kill sollen Ground-Drops im `world state` erscheinen, z. B. mit `ground_item_count` und `ground_items_hint`. Es werden keine Input-Aktionen ausgeführt.

## Abhängigkeiten

- [State Probe](state-probe.md) - liest Snapshots und Hover-Daten
- [World Model](world-model.md) - hält Items im semantischen State
- [Loot- und Recovery-Loop](loot-recovery-loop.md) - nutzt Items ab späteren Phase-5-Slices
- Recherche: d2go `pkg/memory/item.go` und `cmd/txttocode`; Katalogquelle: lokale D2R-Tabellen `3.2.92777`

## Verwandte Features

- [Countess-Run](countess-run.md)
- [Loot- und Recovery-Loop](loot-recovery-loop.md)

---
*Zuletzt aktualisiert: 2026-07-04*
