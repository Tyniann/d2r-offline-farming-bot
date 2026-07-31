# Item Enumeration Read-Only

## Überblick

Phase 5.1 brachte Items read-only aus dem D2R-Speicher in das World Model. Der Fokus lag bewusst auf positionierten Ground-Drops nach einem Countess-Kill: Sie sollten im Probe-Log plausibel sichtbar sein, ohne dass der Bot Items anklickt, Pickit-Regeln bewertet oder Inventar/Stash verändert. Ab Phase 5.2 liest das Item-Modell zusätzlich persönliche Inventar-Items read-only für Lock-Grid und Kapazitätsberechnung.

## Ort im Code

- **Pakete:** `internal/memory/`, `internal/world/`, `internal/app/`
- **Einstieg:** `internal/app/run_tick.go` ruft `Probe.Snapshot()` und `World.Update()` im bestehenden Poll-Loop auf
- **Wichtige Dateien:** `internal/memory/items.go`, `internal/world/item.go`, `internal/app/world_log.go`
- **Config:** keine neuen Keys

## Funktionalität

### Memory-Enumeration

Items werden nur bei `Valid && Phase=in_game` enumeriert. Die Probe läuft über das UnitTable-Segment `4` mit eigenen Limits (`maxItemUnitVisits=4096`, `maxItemsPerSnapshot=128`) und verbraucht nicht das bestehende Entity-Budget für Countess-Objekte, Entrances und Monster.

Für 5.1 wurden live nur plausible Ground-Items akzeptiert (`RawLocation` 3 oder 5) und sie mussten eine lesbare Path-Position haben. Ab 5.2 bleibt diese Ground-Regel bestehen; zusätzlich werden persönliche Inventar-Items mit lesbaren Grid-/Page-Feldern modelliert. Nicht-persönliche Container, unbekannte Pages und unbekannte Owner bleiben sichtbar, zählen aber nicht als Pickup-Kapazität.

Item-Fehler sind fail-open: Ein kaputter Item-Walk liefert eine leere Item-Liste und Debug-Logs, löscht aber keine bereits gelesenen Countess-Entities. Einzelne nicht lesbare Items werden übersprungen. Die ab Abschnitt 13.2 gelesene rohe Set-/Unique-Referenz ist dagegen optional: Scheitert nur dieser Read, bleibt das Item erhalten und ausschließlich seine Identität wird als `item_identity_unavailable` markiert.

### World Model

`memory.ItemUnit` bleibt roh und enthält keine semantischen Labels. `world.Item` löst daraus Qualität, Location, Name/Code, Item-Type, `BaseTier`, Hover-Status sowie ab 5.2 Grid-Position und Inventar-Dimensionen auf. Der Item-Katalog ist eine statische, repo-lokale Generierung aus den lokal extrahierten D2R-Tabellen `data/global/excel/weapons.txt`, `armor.txt` und `misc.txt` für D2R `3.2.92777`; er enthält Code, Name, Type, Tier-Codes und Inventargröße, damit Drops und Inventaritems im Log lesbar werden, ohne d2go als Runtime-Dependency einzubinden. `BaseTier` wird ausschließlich für Waffen und Rüstungen aus konsistenten `normcode`/`ubercode`/`ultracode`-Ketten als `normal`, `exceptional` oder `elite` erzeugt. Misc-Items und unvollständige Equipment-Ketten bleiben `unknown`.

Abschnitt 13.1 erweitert denselben Generator um `setitems.txt`, `uniqueitems.txt` und die englischen Werte aus `item-names.json`. Der eingebettete Identitätskatalog enthält für D2R `3.2.92777` exakt 140 numerische Set- und 433 numerische Unique-Zeilen; eine Set- und sechs Unique-Metadatenzeilen ohne numerische `*ID` werden separat übersprungen. Jeder Eintrag trägt Identitätsart, rohe ID, stabilen Schlüssel, englischen Anzeigenamen, Basiscode, Spawnability und bei Set-Items Set-Schlüssel/-Name. Die fünf Tal-Rasha-Items sind explizit abgedeckt.

Die D2R-Quelldaten enthalten zwei echte Namenskollisionen: eine alte und eine aktuelle `Azurewrath`-Zeile sowie acht `Rainbow Facet`-Varianten. Nur für solche Kollisionen erzeugt der Generator aus Basiscode und bereits vorhandenen Excel-Eigenschaften einen stabilen disambiguierten Schlüssel; die UI kann weiterhin den kanonischen englischen Namen zeigen. Eine danach noch doppelte relevante Identität bricht die Generierung ab. Globale doppelte Übersetzungsschlüssel außerhalb der tatsächlich referenzierten Item-/Setmenge bleiben ohne Einfluss.

Abschnitt 13.2 transportiert die bekannte numerische Set-/Unique-Referenz aus den Item-Daten als vorzeichenbehafteten Rohwert samt eigenem Available-Flag. Nur Items mit Qualität `set` oder `unique` werden aufgelöst. Identitätsart und Raw-ID müssen im getrennten Katalograum existieren, und der dort gebundene Basiscode muss dem aus `TxtFileNo` aufgelösten Item-Code entsprechen. Erst dann übernimmt `world.Item` stabilen Schlüssel und kanonischen englischen Namen. Ein negativer Sentinel, unbekannte ID, Qualitäts- oder Basiswiderspruch bleibt fail-closed ohne geratenen Namen.

### Item-Katalog regenerieren

Wenn `TxtFileNo`-IDs aus dem Speicher nicht zu sichtbaren Item-Namen passen, wird nicht an Memory-Offsets geraten. Zuerst wird geprüft, ob Unit, Hover und Position zusammenpassen. Beispiel aus Phase 5.1: Der sichtbare `Key of Terror` wurde gehovert, `hover_type=item hover_unit_id=309` passte zu einem Ground-Item mit `unit=309`, aber der alte Katalog kannte `id=662` nicht. Das bewies: Reader und Hover-Match waren plausibel, der semantische Katalog war veraltet.

Der Katalog wird aus der lokalen D2R-Installation neu erzeugt:

1. Mit CascView oder einem gleichwertigen CASC-Reader die lokale D2R-Storage read-only öffnen, z. B. `D:\Games\Diablo II Resurrected Infernal Edition`.
2. Diese Dateien für Basis- und Identitätskatalog extrahieren:
   - `data/global/excel/weapons.txt`
   - `data/global/excel/armor.txt`
   - `data/global/excel/misc.txt`
   - `data/global/excel/setitems.txt`
   - `data/global/excel/uniqueitems.txt`
   - englische `item-names.json`
3. Die Dateien temporär unter `.tmp/d2r-excel/` ablegen.
4. `internal/world/item_catalog_data.go` aus genau dieser Reihenfolge generieren: `weapons.txt` -> `armor.txt` -> `misc.txt`.
5. Wie d2go Basiszeilen ohne `code` überspringen; jede akzeptierte Basiszeile erhöht die `TxtFileNo`. Numerische Set-/Unique-IDs bleiben in ihren getrennten ID-Räumen. Pflichtspalten, Zahlen, doppelte relevante IDs/Schlüssel, Spawnability, englische Namen, unbekannte Basiscodes, Tier-Referenzen, Zyklen und die exakte Quellversion werden vor dem Schreiben validiert.
6. Vor dem Einsatz konkrete Live-Beweise testen, z. B. für D2R `3.2.92777`: `538 -> Gold`, `615 -> Flawless Skull`, `634 -> Thul Rune`, `635 -> Amn Rune`, `662 -> Key of Terror`.

Diese Vorgehensweise ist absichtlich datengetrieben: Wenn zukünftige D2R-Versionen Item-IDs verschieben, wird der Katalog aus den Spieldaten aktualisiert. Memory-Offsets werden erst angefasst, wenn Hover/Unit/Position nicht mehr zusammenpassen oder der Reader selbst widersprüchliche Rohdaten liefert.

### Probe-Logging

Normale `world state`-Logs enthalten `item_count` und `ground_item_count`. Der Fingerprint berücksichtigt nur stabile Ground-Item-Identität (`UnitID`, `TxtFileNo`, `Location`), damit Inventory-/Unknown-Churn keine Operator-Logs spammt. Mit `--probe --verbose` erscheinen gekappte Ground- und persönliche Inventory-Hints mit UnitID, Item-ID, Basiscode/-name, Qualität, Identified, Ethereal, rohen Flags hexadezimal, getrennter Active-/Base-Stat-194-Evidenz (`unreadable` / `absent` / `value:N`), produktiven Sockelfeldern (`sockets`, `sockets_available`, `socketed`), Identitätsart, Raw-ID, Available-Flag, kanonischem Namen, Konsistenzstatus und Validitätsgrund. Offensichtliche Dummy-Types wie `body` werden aus dem Ground-Hint ausgeblendet. Diese Diagnose ist read-only und löst keine Input-Aktion aus. Produktive Sockelfelder und Pickit-Syntax: [Sockel-Support für Pickit](socket-pickit.md).

## Datenmodell

| Typ | Rolle |
|-----|-------|
| `memory.ItemUnit` | Rohe Item-Daten aus dem Item-Segment: `TxtFileNo`, `UnitID`, `Quality`, optionale `UniqueSetID`, `RawLocation`, Position, Flags, Identified/Ethereal, `Sockets`/`SocketsAvailable`/`Socketed`, rohe Stats, Gate-19.0 Active-/Base-Stat-194-Evidenz |
| `memory.RawStat` | Bounded Raw-Stat-Eintrag ohne Life/Mana-Skalierung |
| `world.Item` | Semantisches Item mit streng validierter Set-/Unique-Identität, `Sockets`/`SocketsAvailable`/`Socketed` und explizitem Validitätsgrund im World State |
| `world.ItemQuality` | Qualität: normal, magic, rare, unique, set usw. |
| `world.ItemLocation` | Locations: `ground`, `inventory`, `equipped`, `belt`, `cursor`, `cube`, `stash`, `shared_stash_1..3`, `socket`, `unknown` |
| `world.ItemIdentityCatalogEntry` | Eingebettete Set-/Unique-Identität mit Art, Raw-ID, stabilem Schlüssel, englischem Namen, Basiscode und Spawnability |

Query-Helfer:

```go
state.GroundItems()
state.InventoryItems()
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
- [Inventory Model und Lock Grid](inventory-lock-grid.md) - nutzt Inventar-Items für geschützte Slots und Pickup-Kapazität
- [Loot- und Recovery-Loop](loot-recovery-loop.md) - nutzt Items ab späteren Phase-5-Slices
- Recherche: d2go `pkg/memory/item.go` und `cmd/txttocode`; Katalogquelle: lokale D2R-Tabellen `3.2.92777`

## Verwandte Features

- [Countess-Run](countess-run.md)
- [Inventory Model und Lock Grid](inventory-lock-grid.md)
- [Loot- und Recovery-Loop](loot-recovery-loop.md)
- [Sockel-Support für Pickit](socket-pickit.md)

---
*Zuletzt aktualisiert: 2026-07-31*
