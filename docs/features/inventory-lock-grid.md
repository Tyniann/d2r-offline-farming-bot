# Inventory Model und Lock Grid

## Überblick

Phase 5.2 erweitert das read-only Item-Modell um persönliche Inventarplätze und ein konfigurierbares 4x10 Lock-Grid. Ziel ist Schutz vor versehentlichem Verschieben: gelockte Slots zählen nie als Pickup-Kapazität und werden in späteren Stash-/Drop-Flows nicht als Quelle verwendet.

Die Implementierung bleibt konservativ. Unklare Items bleiben im World Model sichtbar, erzeugen aber keine optimistische freie Kapazität.

## Ort im Code

- **Pakete:** `internal/config/`, `internal/memory/`, `internal/world/`, `internal/loot/`, `internal/app/`
- **Einstieg:** `internal/app/run_tick.go` aktualisiert den World State; `Runtime.logWorldState` ergänzt verbose Inventory-Diagnostik
- **Wichtige Dateien:** `internal/memory/items.go`, `internal/world/item.go`, `internal/loot/inventory.go`
- **Config:** `configs/config.example.yaml` unter `loot.inventory_lock`

## Funktionalität

### Inventory-Lock

`loot.inventory_lock` ist optional. Fehlt der Key, materialisiert `config.Config` ein vollständig gelocktes Grid. Wenn der Key vorhanden ist, muss er exakt 4 Zeilen x 10 Spalten enthalten; erlaubt sind nur `0` und `1`.

```yaml
loot:
  inventory_lock:
    - [1, 1, 1, 1, 0, 0, 0, 0, 0, 0]
    - [1, 1, 1, 1, 0, 0, 0, 0, 0, 0]
    - [1, 1, 1, 1, 0, 0, 0, 0, 0, 0]
    - [1, 1, 1, 1, 0, 0, 0, 0, 0, 0]
```

Semantik:

- `1` = geschützt, nicht benutzen
- `0` = grundsätzlich für Loot verfügbar
- `reserved` ist noch nicht implementiert

### Memory- und World-Modell

Items werden nicht mehr nur als Ground-Drops gelesen. Der Reader behält die bestehende Ground-Regel bei: Ground-/Dropping-Items brauchen eine lesbare Weltposition. Inventar-Items brauchen lesbare Grid-/Page-Felder.

Die persönliche Inventar-Kapazität folgt dieser zentralen Mapping-Tabelle:

| Rohdaten | World Location | Kapazität |
|----------|----------------|-----------|
| `RawLocation=0`, player-owned, `Page=0` | `inventory` | ja |
| `RawLocation=0`, player-owned, `Page=3` | `cube` | nein |
| `RawLocation=0`, player-owned, andere Page | `stash` | nein |
| `RawLocation=1`, player-owned | `equipped` | nein |
| `RawLocation=2`, player-owned | `belt` | nein |
| `RawLocation=3` oder `5` | `ground` | nein |
| `RawLocation=4` | `cursor` | nein |
| `RawLocation=6` | `socket` | nein |
| unbekannter Owner/Page/Location | `unknown` | nein |

`GridX` ist die Spalte `0..9`, `GridY` die Zeile `0..3`. `Width` erweitert über Spalten, `Height` über Zeilen.

### Kapazität

`internal/loot` berechnet Kapazität aus `State.InventoryItems()` und dem Lock-Grid:

```go
type InventoryCapacity struct {
    FreeSlots   int
    LockedSlots int
    Unsafe      bool
    Reason      string
}
```

Stabile `Reason`-Werte:

- `unknown_size`
- `out_of_bounds`
- `overlap`

Wenn Kapazität unsafe ist, ist `FreeSlots=0` und `CanFit` liefert immer `false`. Items mit unbekannten Dimensionen bleiben im World Model, machen aber persönliche Inventar-Kapazität unsafe.

### Item-Katalog

`internal/world/item_catalog_data.go` enthält zusätzlich Inventar-Dimensionen. Die Daten können über das manuelle Tool neu erzeugt werden:

```powershell
go run ./tools/generate-item-catalog -src .tmp/d2r-excel -out internal/world/item_catalog_data.go
```

Das Tool erwartet lokal extrahierte D2R-Dateien:

- `weapons.txt`
- `armor.txt`
- `misc.txt`

Normale Tests hängen nicht von `.tmp/d2r-excel` oder einer lokalen D2R-Installation ab.

## Operator / CLI

Validierung bleibt read-only:

```powershell
go run ./cmd/d2rbot --probe --verbose
```

Normale `world state`-Logs ignorieren Inventar-Churn. Mit `--probe --verbose` erscheinen zusätzlich Inventar-Hints und Kapazitätsfelder:

- `inventory_item_count`
- `inventory_free_slots`
- `inventory_locked_slots`
- `inventory_capacity_unsafe`
- `inventory_capacity_reason`
- `inventory_items_hint`

## Abhängigkeiten

- [Item Enumeration Read-Only](item-enumeration.md) - liefert Items in das World Model
- [Loot- und Recovery-Loop](loot-recovery-loop.md) - nutzt das Lock-Grid in späteren Pickup-/Stash-Slices
- [World Model](world-model.md) - hält semantische Items und Query-Helfer

## Verwandte Features

- [Countess-Run](countess-run.md)
- [Input Controller](input-controller.md)

---
*Zuletzt aktualisiert: 2026-07-04*
