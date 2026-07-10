# Loot Decision Pipeline

## Überblick

Phase 5.4 führt eine read-only Entscheidungspipeline für Loot ein. Sie trennt Beobachtung, Pickit-Klassifikation, Pickup-Kandidaten, spätere Pickup-/Verify-Schritte sowie Keep-/Stash-Entscheidungen ausdrücklich voneinander.

Die Pipeline führt keine Eingaben aus. Sie klickt keine Items an, prüft keinen echten Pickup-Erfolg, identifiziert nichts und bewegt keine Items in den Stash. Ihr Zweck ist ein stabiles, testbares Entscheidungsmodell, damit spätere Phasen nicht Pickit-Match, Aufheben, Behalten und Stashen vermischen.

## Ort im Code

- **Paket:** `internal/loot/`
- **Einstieg:** `(*loot.Filter).Decide`
- **Wichtige Dateien:** `internal/loot/decision.go`, `internal/loot/pickit.go`, `internal/loot/inventory.go`
- **Config:** keine neuen Keys; nutzt `loot.pickit_file` und `loot.inventory_lock`

## Funktionalität

### Pipeline-Stufen

Die Pipeline modelliert folgende Stufen:

```text
Observe -> Classify -> PickCandidate -> PickupAttempt -> Verify -> Keep/Stash/Fail
```

`DecisionReport.Decisions` ist eine geordnete Event-Liste. Mehrere Einträge pro Item sind normal: ein Ground-Item mit Pickit-Match kann nacheinander `classify_match`, `pick_candidate`, `pickup_pending` und `verify_pending` erzeugen. Das ist bewusst kein einzelner finaler Status pro Item.

### Begriffe

| Begriff | Bedeutung |
|---------|-----------|
| `pick` | Ein Ground-Item soll später vom Boden aufgenommen werden. |
| `keep` | Ein Inventory-Item soll nach Pickit-Auswertung behalten werden. |
| `stash` | Ein behaltenes Inventory-Item wäre für eine spätere Stash-Routine geeignet. |
| `fail` | Die Pipeline kann für ein gematchtes Item nicht sicher fortfahren. |

`Pickit.Evaluate` bleibt nur ein Regelmatch. Ein Pickit-Match ist deshalb noch kein Pickup, kein Keep und kein Stash.

### Ground-Items

Ground-Items werden in Snapshot-Reihenfolge aus `world.State.GroundItems()` verarbeitet.

- Kein Pickit-Match erzeugt `ignore/pickit_no_match`.
- Ein Pickit-Match erzeugt zuerst `classify_match/pickit_match`.
- Wenn das aktuelle Inventarmodell unsicher ist, folgt `fail/capacity_unsafe`; der bestehende Kapazitätsgrund (`unknown_size`, `out_of_bounds`, `overlap`) wird als Kontext übernommen.
- Wenn der Ground-Kandidat selbst keine bekannte Größe hat, folgt `fail/unknown_size`.
- Wenn aktuell kein freier, ungelockter Platz passt, folgt `fail/inventory_full`.
- Wenn das Item jetzt passt, folgen `pick_candidate/pickit_match`, `pickup_pending/pickup_not_attempted` und `verify_pending/verify_not_attempted`.

Die Kapazitätsprüfung ist eine Momentaufnahme pro Item. Sie reserviert keinen Platz für vorherige Kandidaten und ist kein Multi-Item-Pickup-Plan.

### Inventory-Items

Inventory-Items werden nach den Ground-Items in Snapshot-Reihenfolge aus `world.State.InventoryItems()` verarbeitet. Nicht gematchte Inventory-Items erzeugen in Phase 5.4 keine Decision, weil Inventory-Auswertung aktuell nur für Keep/Stash relevant ist.

Ein gematchtes Inventory-Item erzeugt `keep/pickit_match`, sofern keine Identifikation aussteht. Unidentifizierte Magic/Rare/Set/Unique/Crafted-Items erzeugen stattdessen `keep/identify_required` und niemals eine Stash-Decision. Zusätzlich erzeugt ein final bewertbares Item `stash/stash_candidate`, wenn:

- es ein persönliches Inventory-Item ist (`Location=inventory`, `PlayerOwned`, `Page=0`),
- sein Footprint gültig ist,
- die Inventory-Kapazität nicht unsafe ist,
- keine Zelle des Item-Footprints durch `loot.inventory_lock` geschützt ist.

Ein teilweise gelocktes mehrzelliges Item wird nie als `stash` markiert. Bei unsafe Capacity wird ein gematchtes Inventory-Item weiter als `keep` markiert, aber nicht als `stash`.

## Datenmodell

Zentrale Typen:

- `DecisionStage` beschreibt die Pipeline-Stufe.
- `DecisionKind` beschreibt die konkrete Entscheidung in dieser Stufe.
- `DecisionReason` liefert stabile maschinenlesbare Gründe.
- `ItemDecision` enthält Item-Identität, Stage/Kind/Reason, Pickit-Metadaten und Fit-Kontext.
- `DecisionReport` enthält Ground-/Inventory-Counts, `InventoryCapacity` und die geordnete Decision-Liste.

## Operator / CLI

Phase 5.4 ergänzt keine CLI-Oberfläche und verändert den Countess-Run nicht. `Filter.Decide` ist für spätere Phasen vorbereitet, wird aber noch nicht in `cmd`, `app` oder `tasks` verdrahtet.

## Abhängigkeiten

- [Pickit Engine](pickit-engine.md) - liefert Regelmatches gegen `world.Item`
- [Inventory Model und Lock Grid](inventory-lock-grid.md) - liefert Inventory-Kapazität und geschützte Slots
- [Item Enumeration Read-Only](item-enumeration.md) - liefert Ground- und Inventory-Items ins World Model
- [Loot- und Recovery-Loop](loot-recovery-loop.md) - ordnet die Pipeline in Phase 5 ein

## Verwandte Features

- [World Model](world-model.md)
- [Countess-Run](countess-run.md)

---
*Zuletzt aktualisiert: 2026-07-08*
