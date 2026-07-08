# Loot- und Recovery-Loop

## Überblick

Phase 5 erweitert den Countess-Run von einem Kill-MVP zu einem echten Farming-Loop. Der Bot soll nach dem Countess-Kill Drops aus dem Speicher erkennen, über Pickit-Regeln bewerten, ausgewählte Items sicher aufnehmen, geschützte Inventarbereiche respektieren und bei vollem Inventar oder Pickup-Fehlern kontrolliert abbrechen oder in eine Town-/Stash-Routine wechseln.

Der MVP bleibt bewusst eng: Runen und Countess-relevante Keys sind das erste Ziel. Komplexe Identify-, Sell-, Repair-, Merc- und Multi-Run-Logik wird dokumentiert, aber nicht in den ersten Implementierungsschritt gezogen.

## Ort im Code

- **Paket:** `internal/loot/`
- **Einstieg:** Countess-State-Machine in `internal/tasks/countess.go`
- **Wichtige Dateien:** geplant `internal/memory/items.go`, `internal/world/item.go`, `internal/loot/filter.go`, `internal/loot/pickit.go`, `internal/pathing/item_clicker.go`
- **Config:** geplant `configs/pickit/*.nip`, `configs/config.example.yaml` unter `loot:`

## Funktionalität

### Referenzmodell

d2go und Koolo dienen als Lernreferenzen, bleiben aber keine Runtime-Abhängigkeit. Die relevanten Konzepte:

- d2go modelliert Items mit `UnitID`, Name, Quality, Position, Location, Stats, `Identified`, `Ethereal` und `IsHovered`.
- d2go unterscheidet Item-Locations wie `ground`, `inventory`, `equipped`, `belt`, `cursor`, `stash`, Shared-Stash-Tabs und `socket`.
- Koolo nutzt NIP-Dateien als Pickit-Regeln und bewertet Items über `itemfilter.Evaluate`.
- Koolo schützt Inventarplätze über `InventoryLock`; ungesperrte oder reservierte Slots werden nicht gestasht.
- Koolo bestätigt Pickup-Erfolg darüber, dass ein Ground-Item mit derselben `UnitID` verschwindet.

Für dieses Projekt werden diese Konzepte in die bestehende Architektur übersetzt:

```text
process -> memory (Item-Snapshot) -> world (Item-Modell) -> loot (Entscheidung) -> tasks -> input
```

### Pickit vs. Loot-Filter

Ein D2R-Loot-Filter ist nur eine visuelle Anzeigehilfe. Er kann Namen hervorheben, kürzen oder ausblenden, aber er ersetzt keine Bot-Entscheidung über Stats, Inventarplatz, Pickup-Priorität, Stash-Ziel oder geschützte Slots.

Die Bot-Logik verwendet deshalb Pickit-Regeln als Entscheidungsschicht. Loot-Filter können später optional als Operator-Komfort dokumentiert werden, sind aber kein Kernbestandteil von Phase 5.

### Item-Entscheidungen

Die Loot-Schicht trennt vier Begriffe strikt:

| Entscheidung | Bedeutung |
|--------------|-----------|
| `ignore` | Item wird nicht verfolgt oder angeklickt |
| `pick` | Item soll vom Boden aufgenommen werden |
| `keep` | Item soll nach Bewertung behalten werden |
| `stash` | Item soll aus dem Inventar in den Stash bewegt werden |

Diese Trennung ist wichtig, weil ein nicht identifiziertes Unique zwar `pick` sein kann, aber erst nach Identify sicher `keep` oder `stash` wird. Runen, Keys und Gems sind dagegen früh für den MVP geeignet, weil sie ohne Identify bewertet werden können.

### Pickup-Sicherheit

Item-Pickup folgt dem bestehenden Hover-Feedback-Prinzip:

1. Kandidat per `UnitID` aus `world.State.GroundItems` auswählen.
2. Zum Item navigieren, wenn es außerhalb der Pickup-Distanz liegt.
3. Maus per spielerrelativer Projektion und Spiral-Suche bewegen.
4. Nur klicken, wenn `Hover.UnitType` und `Hover.UnitID` das Ziel-Item bestätigen.
5. Erfolg nur akzeptieren, wenn das Item vom Boden verschwindet oder nach `inventory` wechselt.
6. Nach Retry-/Timeout-Limit mit klarer Fehlerklasse abbrechen.

Es gibt keine Blind-Klicks. Wenn Hover nicht bestätigt wird, bleibt der Bot passiv oder markiert das Item als `loot_unreachable`.

### Inventory-Lock

Phase 5 muss von Anfang an geschützte Inventarbereiche kennen. Der Bot darf vorhandene Build-Items nicht versehentlich verschieben, stashen oder später droppen.

Typische geschützte Items:

- Skiller und andere Charms, die der Build aktiv nutzt
- Hellfire Torch
- Annihilus
- Horadric Cube
- Town-Portal- und Identify-Tomes
- CTA-/Swap-bezogene Items
- reservierte Potion- oder Utility-Slots

Geplantes Config-Modell:

```yaml
loot:
  inventory_lock:
    # 4 Zeilen x 10 Spalten; 1 = geschützt/reserviert, 0 = für Loot verfügbar
    - [1, 1, 1, 1, 0, 0, 0, 0, 0, 0]
    - [1, 1, 1, 1, 0, 0, 0, 0, 0, 0]
    - [1, 1, 1, 1, 0, 0, 0, 0, 0, 0]
    - [1, 1, 1, 1, 0, 0, 0, 0, 0, 0]
```

MVP-Regel: Gelockte Slots zählen nicht als freier Platz und werden nie als Quelle für Stash-, Sell- oder Drop-Aktionen verwendet.

### Stash-Strategie

Stash-Automation ist riskanter als Pickup und wird nach dem Ground-Loot-MVP umgesetzt. Die spätere Stash-Routine muss:

- nur Items aus nicht gelockten Inventarplätzen verarbeiten
- nur Items stashen, die Pickit als `keep` bewertet
- Tomes, Keys, Cube, Potions und geschützte Charms ignorieren
- zunächst nur den Personal Stash verwenden
- Shared Stash erst nach eigener Tab-/Platzstrategie verwenden
- bei vollem Stash mit `stash_full` abbrechen

Droppen von Inventar-Items bleibt bis zu einer expliziten, getesteten Protection-Schicht verboten.

### Recovery

Phase 5 führt neue Fehlerklassen ein:

| Grund | Bedeutung |
|-------|-----------|
| `loot_unreachable` | Item wurde erkannt, konnte aber nicht hover-bestätigt oder erreicht werden |
| `pickup_failed` | Klickversuche waren bestätigt, Item blieb aber am Boden |
| `inventory_full` | kein freier, nicht gelockter Platz verfügbar |
| `stash_full` | Stash-Routine konnte kein Ziel finden |
| `pickit_config_invalid` | Pickit-Datei kann nicht geladen oder geparst werden |
| `loot_timeout` | Loot-Phase überschreitet ihr Zeitlimit |

Ein Loot-Fehler ist nicht automatisch ein Bot-Crash. Der Run soll mit sauberem Grund enden oder, wenn konfiguriert, in eine Town-/Stash-Routine wechseln.

## Datenmodell

Geplante World-Typen:

```go
type ItemLocation string

type Item struct {
    UnitID     uint32
    Name       string
    Quality    ItemQuality
    Position   Position
    Location   ItemLocation
    Ethereal   bool
    Identified bool
    IsHovered  bool
    Stats      map[StatID]StatValue
}
```

Geplante Query-Helfer:

```go
State.GroundItems()
State.InventoryItems()
State.ItemsByLocation(...)
State.FindItemByUnitID(...)
```

Die konkrete Struktur darf beim Implementieren an bestehende `world`-Konventionen angepasst werden.

## Operator / CLI

Geplante Testoberflächen:

```powershell
go run ./cmd/d2rbot --probe --verbose
go run ./cmd/d2rbot --run countess --phase loot-countess --probe --verbose
```

`loot-countess` startet nur in `Tower Cellar Level 5` und enthält keinen Travel-Prefix. Der Operator kann Countess manuell oder über `kill-countess` töten und anschließend die Loot-Phase isoliert validieren.

Geplante Logs:

| Event | Felder |
|-------|--------|
| `loot item seen` | `unit_id`, `name`, `quality`, `location`, `pos_x`, `pos_y` |
| `pickit item matched` | `unit_id`, `name`, `rule`, `decision` |
| `loot pickup started` | `unit_id`, `name`, `distance` |
| `loot pickup complete` | `unit_id`, `name`, `elapsed_ms` |
| `loot pickup failed` | `unit_id`, `name`, `reason` |
| `inventory full` | `free_slots`, `locked_slots` |
| `stash item complete` | `unit_id`, `name`, `tab` |

Für spätere Auswertung soll die Loot-Schicht JSONL-Telemetrie vorbereiten, ohne den menschenlesbaren `slog`-Stream zu ersetzen.

## Abhängigkeiten

- [World Model](world-model.md) - hält Ground-, Inventory- und Stash-Items als semantischen State
- [State Probe](state-probe.md) - liest Items aus dem D2R-Prozess
- [Pathing](pathing.md) - nähert Items an und liefert Projektion/Hover-Feedback
- [Input Controller](input-controller.md) - führt Mausbewegung, Klicks, Town Portal und spätere UI-Aktionen aus
- [Countess-Run](countess-run.md) - bindet die Loot-Phase in den Run ein
- Referenzen: d2go Item-Modell/NIP-Parser, Koolo Pickup/Stash/InventoryLock, Botty BNIP, Kolbot-NIP-Guides

## Phase-5-Slices

### 5.0 Konzeptdokumentation

Dieses Dokument hält die Architektur, Grenzen und späteren Pflicht-Themen fest.

### 5.1 Ground-Item-Enumeration

Items read-only aus dem Speicher lesen und im Probe-/World-Log sichtbar machen. Kein Pickup. Umgesetzt als [Item Enumeration Read-Only](item-enumeration.md): Live-Pass-Kriterium sind positionierte Ground-Drops nach Countess-Kill; andere Locations sind Modellvorbereitung für spätere Slices.

### 5.2 Inventory-Modell und Lock-Grid

Umgesetzt als [Inventory Model und Lock Grid](inventory-lock-grid.md): persönliche Inventar-Items werden read-only modelliert, `loot.inventory_lock` schützt ein 4x10 Grid, und Pickup-Kapazität fällt bei unbekannten Größen, Out-of-bounds oder Überschneidung fail-closed auf `0`.

### 5.3 Pickit-MVP

Umgesetzt als [Pickit Engine](pickit-engine.md): kleiner, line-numbered NIP-Subset mit `loot.pickit_file`, Default `configs/pickit/countess.nip` und read-only Match-Ergebnissen. Pickit bewertet ausschließlich `world.Item`-Felder aus dem generierten Item-Katalog (`Code`, `Type`, `Name`, `Quality`, Flags, Stats); die lokale D2R-Extraktion bleibt nur Regenerationsquelle, siehe [Item Enumeration Read-Only](item-enumeration.md).

### 5.4 Loot-Entscheidungspipeline

Umgesetzt als [Loot Decision Pipeline](loot-decision-pipeline.md): `Observe -> Classify -> PickCandidate -> PickupAttempt -> Verify -> Keep/Stash/Fail` wird als read-only Stage-Liste modelliert. Pickit bleibt ein Regelmatch; `pick`, `keep` und `stash` bleiben getrennte Entscheidungen ohne Input-, Identify- oder Stash-Automation.

### 5.5 Hover-bestätigter Item-Pickup

Items nur nach Hover-Match anklicken und Pickup über Ground-Verschwinden oder Location-Wechsel bestätigen.

### 5.6 Countess-Loot-Phase

Full Run um `wait_for_drops`, `scan_loot` und `pick_loot` erweitern. Isolierte Phase `loot-countess` ergänzen.

### 5.7 Inventory-Full-Recovery

Keinen freien Slot als `inventory_full` erkennen. Bei wertvollem Loot kontrolliert abbrechen oder Town-/Stash-Routine vorbereiten.

### 5.8 Personal-Stash-MVP

Nur Personal Stash, nur nicht gelockte Inventar-Items, keine Shared-Stash-Automatik.

### 5.9 Identify-Strategie

Nicht identifizierte Items aufnehmen, aber Stats erst nach einer späteren Identify-Routine final bewerten.

### 5.10 Loot-Telemetrie

Drop-, Pickit-, Pickup-, Inventory- und Stash-Ereignisse strukturiert protokollieren.

## Grenzen

- Kein Savegame- oder D2R-Dateizugriff.
- Keine Runtime-Dependency auf Koolo, d2go, Botty oder Kolbot.
- Kein Droppen vorhandener Inventar-Items im Phase-5-MVP.
- Keine Shared-Stash-Automatik im ersten Stash-Schnitt.
- Kein Identify-/Sell-/Repair-/Merc-Loop im ersten Ground-Loot-MVP.

---
*Zuletzt aktualisiert: 2026-07-08*
