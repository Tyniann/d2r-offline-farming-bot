# Personal-Stash MVP

## Überblick

Phase 5.8 leert nach der verifizierten Rückkehr ins Rogue Encampment ausgewählte Inventar-Items in den persönlichen Stash. Unterstützt werden die aktuellen Pickit-Matches Runen, Countess-Key, Rejuvenation Potions sowie Flawless/Perfect Gems und Skulls. D2R sortiert sie per `Ctrl+LMB` in die charaktergebundenen Sammelbereiche; Rejuvenation-Nachschub wird gesammelt, aber in Phase 9 noch nicht automatisch aus dem 99er-Stash-Slot in den Belt zurückgeführt.

Shared-Stash-Tabs, Tab-Wechsel, endliche Raster-Platzstrategien und das Droppen vorhandener Items sind ausdrücklich nicht Bestandteil dieses MVP. Phase 10.6 schließt run-spezifische Sell-Kandidaten vor dem Stash-Input aus; Identify/Sell selbst bleiben ein separater Town-Service.

## Ort im Code

- **Memory/UI:** `internal/memory/ui.go`, `internal/world/ui.go`
- **Objektquelle:** `tools/generate-object-catalog`, generierte `object_ids_data.go`
- **Town-Navigation:** `internal/pathing/personal_stash.go`
- **Transfer:** `internal/loot/stash.go`
- **Task-Integration:** `internal/tasks/run_pipeline.go`
- **Input:** `internal/input/mouse.go` (`ClickWithModifier`)
- **Config:** `loot.stash` in `configs/config.example.yaml`

## Funktionalität

### Stash-Wahrheit aus Memory

Die Stash-Objekt-ID wird nicht aus d2go übernommen. `tools/generate-object-catalog` liest den expliziten `*ID`-Wert der Klasse `Bank` aus dem lokalen D2R-`objects.txt`-Export. Für D2R `3.2.92777` ergab der lokale Export `PersonalStashID=267`.

Die UI-Flags wurden am 10.07.2026 read-only gegen drei Zustände kalibriert:

| Zustand | `InventoryOpen` (`UI-0x13`) | `StashOpen` (`UI+0x04`) |
|---|---:|---:|
| geschlossen | 0 | 0 |
| nur Inventar | 1 | 0 |
| Personal Stash | 1 | 1 |

Ein normales Inventarfenster gibt Stash-Aktionen deshalb nicht frei. Fixed-coordinate Input ist zusätzlich hart auf exakt `1280×720` gegatet; andernfalls endet der Ablauf mit `unsupported_resolution`.

### Town-Navigation

`PersonalStashActions` sucht `ObjectKindPersonalStash` im World Model. Aus dem Town-Portal- oder Waypoint-Bereich läuft die Figur per Force Move zum Stash; Teleport wird in Town nicht verwendet. Zwei relativ zum Memory-Stash definierte Detour-Anker `(+10,+18)` und `(+4,+14)` umgehen die live beobachtete Town-Geometrie. Relative Punkte vermeiden eine Abhängigkeit vom absoluten Koordinatenursprung der Town-Instanz.

Beim ersten Tick des Anlaufs merkt sich der Bot den Graph-Anker, von dem die Figur gekommen ist: ein nahes Town Portal, sonst ein naher Waypoint, sonst die Startposition. Bleibt der Force-Move `pathing.town_walk.stuck_timeout_ms` ohne Tile-Fortschritt stehen, läuft die Figur einmal zu diesem Anker zurück und wiederholt die Detours. Ein zweiter Stuck oder ein Stuck auf dem Rückweg endet mit `stash_approach_failed`; die Session macht dann Save & Exit und startet denselben Queue-Eintrag neu, ohne in der Stadt ein Recovery-Portal zu casten.

Der Stash wird nur innerhalb der Klickdistanz angeklickt. Nach dem letzten Force-Move muss die Memory-Position zunächst für `pathing.town_walk.settle_timeout_ms` stabil bleiben; jede Bewegung um mindestens ein Tile setzt diese Wartephase zurück und verwirft einen bereits begonnenen Hover-Versuch. Erst danach darf die passende Object-`UnitID` im Hover-Buffer den Klick freigeben. Erfolg ist ausschließlich `StashOpen=true` aus Memory.

### Auswahl und Schutz

Ein Transferkandidat muss gleichzeitig:

- im persönlichen Inventory liegen (`PlayerOwned`, `Page=0`),
- die geladene Pickit-Datei matchen,
- bekannte, gültige Dimensionen besitzen,
- keinen gelockten/reservierten Inventory-Slot überdecken,
- in einem insgesamt konsistenten Inventory-Grid liegen.

Damit bleiben Cube, Tomes, Potions und Charms in geschützten Slots unangetastet. Es werden auch keine nicht von Pickit bewerteten Items gestasht.

Wenn die ausgewählte Run-Config eine Sell-Policy besitzt, werden deren Matches ebenfalls niemals gestasht. Dadurch bleiben Mephisto-Sell-Kandidaten für den UnitID-gepinnten Cain-/Akara-Service im Inventory, während Gems aus derselben Pickup-Policy weiterhin in den Personal Stash gelangen.

Der lokale Item-Katalog enthält alle 35 Gem-/Skull-Zeilen aus `misc.txt` mit `1×1`. Der Generator bricht künftig ab, falls ein lokaler `gem*`-Eintrag andere Dimensionen liefert.

### Transfer und Verifikation

Das Inventory-Raster für `1280×720` verwendet `left=847`, `top=369` und Zellen von `33×33` Pixeln. Pro Kandidat geschieht atomar:

1. Maus auf das aus `GridX/GridY` berechnete Zellzentrum bewegen.
2. `Ctrl down → LMB down/up → Ctrl up`; Cleanup lässt `Ctrl` auch bei Fehlern nicht gedrückt.
3. Auf einen neuen konsistenten Memory-Snapshot warten.
4. Erfolg, wenn die Unit aus dem Inventory verschwindet oder ihre Location nicht mehr `inventory` ist.

Ohne Bestätigung wird bis `loot.stash.max_retries` wiederholt. Danach endet der Run mit `stash_failed`; das Item wird weder gedroppt noch anderweitig bewegt. `stash_full` ist als Status reserviert, wird für die derzeit unterstützten unbegrenzten Sammel-Tabs aber nicht heuristisch erzeugt.

Nach dem letzten Kandidaten sendet der Bot einmal `Esc`. Erfolg ist nur bestätigt, wenn `StashOpen=false` und `InventoryOpen=false` werden; andernfalls folgt `stash_close_failed`.

## Task-Flows

Isolierter E2E-Test:

```powershell
go run ./cmd/d2rbot --run countess --phase stash-personal --verbose
```

State-Machine:

```text
precheck -> open_personal_stash -> stash_items -> close_personal_stash -> complete
```

`loot-and-return` und der vollständige Countess-Run hängen denselben Workflow nach `wait_origin_town` an. Es gibt keine Rückkehr in den Dungeon.

## Fehler und Abbruch

| Reason | Bedeutung |
|---|---|
| `stash_not_found` | Kein lokal klassifiziertes Personal-Stash-Objekt im World Model |
| `stash_approach_failed` | Zweiter Town-Walking-Stuck oder Projektionsfehler nach einem lokalen Rückweg zum Portal-/Waypoint-Anker; Session-Retry |
| `stash_open_failed` | Hover/Klick/UI-Bestätigung fehlgeschlagen oder falsches UI offen |
| `stash_failed` | Kandidat/Inventory unsicher, Input fehlgeschlagen oder Transfer nicht bestätigt |
| `stash_close_failed` | `Esc` oder geschlossene UI nicht bestätigt |
| `unsupported_resolution` | Clientgröße ist nicht exakt `1280×720` |
| `stash_full` | Für spätere endliche Stash-Flächen reserviert |

## Live-Validierung

Am 10.07.2026 wurden zwei E2E-Schnitte validiert:

- Portalbereich → relative Detour → Hover-Bestätigung nach zwei Probes → Stash-UI bestätigt; `outcome=success` nach rund 2,8 Sekunden.
- Sechs angekündigte Kandidaten (2× Tir, 2× Amn, Flawless Ruby, Flawless Emerald) jeweils beim ersten Ctrl-Klick Memory-bestätigt; Stash anschließend per `Esc` in 201 ms geschlossen; `outcome=success`. Cube, Tomes und Charms blieben unverändert.

## Grenzen

- Nur Personal Stash und die aktuellen Pickit-MVP-Typen.
- Keine Shared-Stash-Automatik oder Tab-Klicks.
- Keine allgemeine Identify-/Keep-Strategie; produktiv unterstützt ist ausschließlich der enge Mephisto-Sell-Pfad aus Phase 10.6.
- Kein Droppen vorhandener Items.
- Town-Detour ist für Rogue Encampment / Act 1 validiert.

## Verwandte Features

- [Inventory-Full-Recovery](inventory-full-recovery.md)
- [Inventory Model und Lock Grid](inventory-lock-grid.md)
- [Pickit Engine](pickit-engine.md)
- [Countess-Run](countess-run.md)

---
*Zuletzt aktualisiert: 2026-08-28*
