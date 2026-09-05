# Loot Decision Pipeline

## Überblick

Phase 5.4 führt eine read-only Entscheidungspipeline für Loot ein. Sie trennt Beobachtung, Pickit-Klassifikation, Pickup-Kandidaten, spätere Pickup-/Verify-Schritte sowie Keep-/Stash-Entscheidungen ausdrücklich voneinander.

Die Pipeline selbst führt keine Eingaben aus. Produktive Pickup-, Stash- und Town-Adapter konsumieren ihre Action Policy und behalten ihre eigenen Hover-, Kapazitäts-, Lock-, UI- und Verifikationsgates.

## Ort im Code

- **Paket:** `internal/loot/`
- **Einstieg:** `(*loot.Filter).Decide`
- **Wichtige Dateien:** `internal/loot/decision.go`, `internal/loot/pickit.go`, `internal/loot/inventory.go`
- **Config:** nutzt die effektive Pickit-Assignment-Policy und den Charakter-Loadout-Inventarschutz (`OperatorSettings` Schema 3)

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

`Pickit.Evaluate` bleibt eine read-only Regelauswertung und liefert stabile Profil-/Regelherkunft, `keep`/`sell`, beide Revisionen und den bis zum ersten Treffer tatsächlich ausgewerteten Trace. Seit Abschnitt 13.7 ist die eine geordnete effektive Policy autoritativ: `keep` kann Keep/Stash autorisieren, `sell` kann nach erforderlicher Identifikation ausschließlich den Vendorpfad autorisieren. Ein Match allein umgeht weiterhin kein Safety-Gate.

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

Ein `keep`-gematchtes Inventory-Item erzeugt `keep/pickit_match`, sofern keine Identifikation aussteht. Ein `sell`-Match erzeugt nach Identifikation `sell/sell_candidate`. Unidentifizierte Magic/Rare/Set/Unique/Crafted-Items erzeugen für beide Aktionen zunächst `identify_required` und niemals eine Stash- oder Sell-Freigabe. Zusätzlich erzeugt ein final bewertbares Keep-Item `stash/stash_candidate`, wenn:

- es ein persönliches Inventory-Item ist (`Location=inventory`, `PlayerOwned`, `Page=0`),
- sein Footprint gültig ist,
- die Inventory-Kapazität nicht unsafe ist,
- keine Zelle des Item-Footprints durch den Charakter-Inventarschutz geschützt ist.

Ein teilweise gelocktes mehrzelliges Item wird nie als `stash` markiert. Bei unsafe Capacity wird ein gematchtes Inventory-Item weiter als `keep` markiert, aber nicht als `stash`.

Stash und Town-Services werten das gepinnte Item unmittelbar vor jedem Transfer-, Identify- oder Sell-Input erneut gegen denselben aktiven Policy-Snapshot aus. No-Match, geänderte Aktion, Regelwechsel oder Identitätsdrift widerrufen die Freigabe fail-closed. Der Cow-Town-Dump vor Preflight ist davon getrennt: Er verkauft nur bereits identifizierte, ungelockte Nicht-Keep-Items und verlangt vor dem Klick weiterhin „noch da, noch ungelockt, noch kein Keep“, aber kein Pickit-`sell`-Match.

## Datenmodell

Zentrale Typen:

- `DecisionStage` beschreibt die Pipeline-Stufe.
- `DecisionKind` beschreibt die konkrete Entscheidung in dieser Stufe.
- `DecisionReason` liefert stabile maschinenlesbare Gründe.
- `ItemDecision` enthält Item-Identität, Stage/Kind/Reason, Pickit-Metadaten und Fit-Kontext.
- `DecisionReport` enthält Ground-/Inventory-Counts, `InventoryCapacity` und die geordnete Decision-Liste.

## Operator / CLI

Die read-only Pipeline besitzt keine eigene CLI-Oberfläche. Ihre Pickup-Auswertung ist produktiv in Run und Stash verdrahtet; der getrennte Item-Service kann isoliert mit `--town-test item-services:mephisto` abgenommen werden.

## Abhängigkeiten

- [Pickit Engine](pickit-engine.md) - liefert Regelmatches gegen `world.Item`
- [Inventory Model und Lock Grid](inventory-lock-grid.md) - liefert Inventory-Kapazität und geschützte Slots
- [Item Enumeration Read-Only](item-enumeration.md) - liefert Ground- und Inventory-Items ins World Model
- [Loot- und Recovery-Loop](loot-recovery-loop.md) - ordnet die Pipeline in Phase 5 ein

## Verwandte Features

- [World Model](world-model.md)
- [Countess-Run](countess-run.md)

---
*Zuletzt aktualisiert: 2026-09-02*
