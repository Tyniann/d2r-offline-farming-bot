# Loot- und Recovery-Loop

## Überblick

Phase 5 erweitert den Countess-Run von einem Kill-MVP zu einem echten Farming-Loop. Der Bot soll nach dem Countess-Kill Drops aus dem Speicher erkennen, über Pickit-Regeln bewerten, ausgewählte Items sicher aufnehmen, geschützte Inventarbereiche respektieren und bei vollem Inventar oder Pickup-Fehlern kontrolliert abbrechen oder in eine Town-/Stash-Routine wechseln.

Der MVP bleibt bewusst eng: Runen und Countess-relevante Keys sind das erste Ziel. Komplexe Identify-, Sell-, Repair-, Merc- und Multi-Run-Logik wird dokumentiert, aber nicht in den ersten Implementierungsschritt gezogen.

## Ort im Code

- **Paket:** `internal/loot/`
- **Einstieg:** gemeinsame Run-Pipeline in `internal/tasks/run_pipeline.go`
- **Wichtige Dateien:** `internal/memory/items.go`, `internal/world/item.go`, `internal/loot/pickit.go`, `internal/loot/pickup.go`
- **Config:** `configs/pickit/profiles/*.yaml`, `configs/pickit-assignments.local.yaml` und globale Safety-Werte unter `loot:`

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
2. Den Kandidaten vor Input anhand seiner `UnitID` im aktuellen Snapshot erneut auflösen.
3. Liegt er außerhalb von `loot.pickup.max_distance_tiles` und höchstens 20 Tiles entfernt, höchstens drei durch frische Snapshots und 500 ms getrennte Teleports zu seiner aktuellen Memory-Position senden. Nur tatsächlich gesendete Inputs zählen als Versuch. Weiter entfernte Kandidaten werden ohne Chase-Teleport als `too_far` übersprungen.
4. Maus per spielerrelativer Projektion und Spiral-Suche bewegen.
5. Nur klicken, wenn `Hover.UnitType` und `Hover.UnitID` das Ziel-Item bestätigen.
6. Erfolg nur akzeptieren, wenn das Item vom Boden verschwindet oder nach `inventory` wechselt.
7. Nach Retry-/Timeout-Limit mit klarer Fehlerklasse abbrechen oder den unerreichbaren Kandidaten für die aktuelle Loot-Phase überspringen.
8. Bei `hover_not_found` oder `pickup_failed` einmalig pro `UnitID` distanzignorierend auf die Item-Position teleportieren und den Pickup genau einmal erneut starten; danach gilt weiter das Skip-Verhalten.

Es gibt keine Blind-Klicks und keine unbeschränkten Teleport-Schleifen. Die historische Bossposition dient nur als erste Annäherung; für den Pickup ist die aktuelle Position des ausgewählten Items autoritativ. Wenn Hover nicht bestätigt wird oder das Item nach den Annäherungsversuchen weiterhin zu weit entfernt ist, bleibt der Bot passiv und überspringt nur dessen `UnitID` — außer der einmaligen Recovery in Schritt 8.

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

Verfügbare Testoberflächen:

```powershell
go run ./cmd/d2rbot --probe --verbose
go run ./cmd/d2rbot --run countess --phase loot-and-return --probe --verbose
```

`loot-and-return` startet nur in `Tower Cellar Level 5` und enthält keinen Travel-Prefix. Der Operator kann Countess manuell oder über `boss` töten und anschließend die Loot-Phase isoliert validieren.

Wichtige Logs:

| Event | Felder |
|-------|--------|
| `countess loot scan` | `ground_item_count`, `candidate_count`, `blocked_candidate_count`, `has_target` |
| `loot pickup started` | `unit_id`, `name`, `distance` |
| `loot pickup complete` | `unit_id`, `name`, `finding`, `retry` |
| `loot pickup failed` | `unit_id`, `name`, `reason` |

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

Umgesetzt als [Inventory Model und Lock Grid](inventory-lock-grid.md): persönliche Inventar-Items werden read-only modelliert, der Charakter-Loadout schützt ein 4×10-Grid in OperatorSettings Schema 3, und Pickup-Kapazität fällt bei unbekannten Größen, Out-of-bounds oder Überschneidung fail-closed auf `0`.

### 5.3 Pickit-MVP

Umgesetzt als [Pickit Engine](pickit-engine.md): kleiner Ausdrucks-Subset mit revisionierten YAML-Profilen, geordneter Charakter-/Run-Zuordnung und read-only Match-Ergebnissen. Pickit bewertet ausschließlich `world.Item`-Felder aus den generierten Katalogen (`Code`, `Type`, `Name`, `Quality`, exakte Identität, Flags, Stats); die lokale D2R-Extraktion bleibt nur Regenerationsquelle, siehe [Item Enumeration Read-Only](item-enumeration.md).

### 5.4 Loot-Entscheidungspipeline

Umgesetzt als [Loot Decision Pipeline](loot-decision-pipeline.md): `Observe -> Classify -> PickCandidate -> PickupAttempt -> Verify -> Keep/Stash/Fail` wird als read-only Stage-Liste modelliert. Pickit bleibt ein Regelmatch; `pick`, `keep` und `stash` bleiben getrennte Entscheidungen ohne Input-, Identify- oder Stash-Automation.

### 5.5 Hover-bestätigter Item-Pickup

Umgesetzt als [Hover-Confirmed Item Pickup](hover-confirmed-item-pickup.md): Ein isolierter Pickup-Baustein friert einen Pickit-/Inventory-geprüften Ground-Item-Kandidaten ein, klickt nur nach `Hover.UnitType=item` und passender `UnitID`, bestätigt Erfolg über gültige In-Game-Verify-Ticks und bricht bei Retry-, Distanz- oder Monster-Sicherheitsgrenzen ab. Live-Validierung läuft über `--pathing-test pickup:item`; die Countess-Integration ist in 5.6 umgesetzt.

### 5.6 Countess-Loot-Phase

Umgesetzt im Countess-Run: Nach `engage_boss` wartet `wait_for_drops` auf drei gültige Cellar-5-Ticks, `scan_loot` bewertet Ground-Loot über Pickit und Inventory-Kapazität, und `pick_loot` hebt Kandidaten über den hover-bestätigten Pickup-Executor auf. Fehlgeschlagene Pickup-Kandidaten werden innerhalb des aktuellen `pick_loot`-Steps per `UnitID` übersprungen, damit derselbe Drop nicht endlos neu versucht wird. Nach `hover_not_found` oder `pickup_failed` folgt vorher einmalig ein distanzignorierender Item-Teleport und ein zweiter Pickup-Versuch.

Der Full Run endet jetzt:

```text
engage_boss -> wait_for_drops -> scan_loot -> pick_loot -> cast_town_portal
-> enter_town_portal -> wait_origin_town -> open_personal_stash
-> stash_items -> close_personal_stash -> complete
```

Wenn keine passenden Kandidaten vorhanden sind oder alle Kandidaten übersprungen wurden, castet der Bot trotzdem Town Portal und beendet den Run regulär. Die isolierte Testphase ist umgesetzt:

```powershell
go run ./cmd/d2rbot --run countess --phase loot-and-return --probe --verbose
```

`loot-and-return` startet in `Tower Cellar Level 5`, verlangt Teleport wegen globalem Runtime-Precheck, Town Portal für den Abschluss und Belt-Slots 1/4 für die aktive Safety-Potion.

### 5.7 Inventory-Full-Recovery

Umgesetzt als [Inventory-Full-Recovery](inventory-full-recovery.md): Ein Pickit-Match ohne passenden freien, nicht gelockten Platz erzeugt explizit `inventory_full`. Weitere Pickups stoppen; vorhandene Items werden niemals gedroppt. Full Run und `loot-and-return` casten anschließend Town Portal, erkennen das Portal über den aus lokalem `objects.txt` generierten Objektkatalog, klicken nur nach passendem Memory-Hover und enden erst nach verifizierter Ankunft im Rogue Encampment. Ein endgültiges `pickup_failed` überspringt dagegen nur das betroffene Item.

### 5.8 Personal-Stash-MVP

Umgesetzt als [Personal-Stash MVP](personal-stash-mvp.md): nur Pickit-Matches im persönlichen Inventory, niemals gelockte/reservierte Items, Ctrl+LMB-Autosortierung in die charaktergebundenen Gems/Materials/Runes-Tabs und Memory-Verifikation pro Item. Shared Stash und Droppen bleiben ausgeschlossen.

### 5.9 Identify-Strategie

Umgesetzt als [Identification-Strategie](identification-strategy.md): Quality-Regeln dürfen unidentifizierte Items zum Pickup auswählen, Statregeln sind bis `Identified=true` gesperrt. Magic/Rare/Set/Unique/Crafted erzeugen vor Keep/Stash explizit `identify_required`; eine echte Identify-Routine bleibt späteren Phasen vorbehalten.

### 5.10 Loot-Telemetrie

Umgesetzt als [Run-Telemetrie](run-telemetry.md): eine fail-closed JSONL-Datei pro Run mit `drop_seen`, `pickit_match`, Pickup-, `inventory_full`- und Stash-Events. Beobachtungsereignisse werden je Unit-ID dedupliziert; echte Versuche werden einzeln geschrieben und sofort geflusht.

## Grenzen

- Kein Savegame- oder D2R-Dateizugriff.
- Keine Runtime-Dependency auf Koolo, d2go, Botty oder Kolbot.
- Kein Droppen vorhandener Inventar-Items im Phase-5-MVP.
- Keine Shared-Stash-Automatik im ersten Stash-Schnitt.
- Kein Identify-/Sell-/Repair-/Merc-Loop im ersten Ground-Loot-MVP.

---
*Zuletzt aktualisiert: 2026-07-27*
