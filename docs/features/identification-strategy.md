# Identification-Strategie

## Überblick

Phase 5.9 verankert die Sicherheitsgrenze: NIP-Statregeln dürfen nur identifizierte Items bewerten, während Quality-Regeln unidentifizierte Magic/Rare/Set/Unique/Crafted-Items zum Pickup auswählen können. Phase 10.6 ergänzt eine enge produktive Cain-Routine ausschließlich für explizite Mephisto-Sell-Kandidaten.

Das Countess-MVP bleibt auf Runen, Keys, Gems und Skulls fokussiert. Diese Typen benötigen keine Identifikation und können weiterhin automatisch in den Personal Stash transferiert werden.

## Ort im Code

- **Pickit:** `internal/loot/pickit.go`
- **Decision Pipeline:** `internal/loot/decision.go`
- **Stash-Gate:** `internal/loot/stash.go`
- **World-Felder:** `world.Item.Identified`, `world.Item.Quality`, `world.Item.Stats`

## Regeln

### Pickup

- `[quality] == unique`, `set`, `rare`, `magic` oder `crafted` darf ein unidentifiziertes Ground-Item matchen.
- Ein solches Match kann später als Pickup-Kandidat dienen, wenn Inventory- und Safety-Regeln erfüllt sind.
- `[stat:<id>] ...` liefert für `Identified=false` immer `false`, selbst wenn der rohe Memory-Snapshot Stat-Einträge enthält.

Damit trennt der Bot „interessant genug zum Aufheben“ von „nach Stats final behalten“.

### Keep und Stash

Für unidentifizierte Items der Qualitäten Magic, Rare, Set, Unique und Crafted erzeugt die Decision Pipeline:

```text
stage=keep kind=identify_required reason=identify_required
```

Sie erzeugt weder `keep` noch `stash`. Der Personal-Stash-Executor prüft dieselbe Policy nochmals direkt vor Input und lässt das Item im Inventory. Identifizierte Items dürfen anschließend normal über Pickit bewertet werden.

Normal-, Low-Quality- und Superior-Items werden nicht pauschal gegatet; für das aktuelle MVP sind die relevanten Runen, Keys, Gems und Skulls normale, nicht identifikationspflichtige Items.

## Produktiver Mephisto-Sell-Pfad

Ein unidentifiziertes Exceptional-/Elite-Set/Unique mit `sell`-Match aus dem zugeordneten `mephisto-standard`-Profil wird anhand seiner Runtime-UnitID zu Cain geplant. Erst `Identified=true` für dieselbe UnitID gibt den anschließenden Akara-Verkauf frei. Identifizierte Kandidaten überspringen Cain. Nach einer gesendeten Aktion gibt es keinen zweiten Inputversuch; ein ausbleibender Memory-Übergang endet fail-closed. Gems, normale Set-/Unique-Basen und gelockte Items gelangen nie in diesen Pfad.

## Nicht umgesetzt

- Quantity-Management für Identify Scrolls/Tomes.
- Finale Rare/Magic/Set/Unique-Stashstrategie.

Diese Funktionen sind nicht Teil von Phase 6. Sie bleiben einer späteren Town-Service-Phase vorbehalten.

## Verwandte Features

- [Pickit Engine](pickit-engine.md)
- [Loot Decision Pipeline](loot-decision-pipeline.md)
- [Personal-Stash MVP](personal-stash-mvp.md)

---
*Zuletzt aktualisiert: 2026-07-21*
