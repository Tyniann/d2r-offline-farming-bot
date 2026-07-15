# Pickit Engine

## Überblick

Phase 5.3 führt eine kleine Pickit-Engine ein, die einen bewusst begrenzten NIP-Subset lädt und gegen Items aus dem bestehenden World Model auswertet. Die Engine entscheidet nur, ob ein Item zu einer Regel passt. Sie hebt nichts auf, bewegt keine Items, identifiziert nichts und führt keine Stash- oder Quantity-Logik aus.

Die Item-Identität kommt ausschließlich aus dem generierten `internal/world`-Katalog:

```text
memory.TxtFileNo -> world.Item via generated itemCatalog -> loot.Pickit evaluation
```

Die lokalen Dateien unter `.tmp/d2r-excel` sind nur Eingabe für die Katalog-Regeneration. Runtime-Code liest diese Dateien nicht.

## Ort im Code

- **Paket:** `internal/loot/`
- **Einstieg:** `loot.LoadPickit` beim App-Wiring in `internal/app/app.go`
- **Wichtige Dateien:** `internal/loot/pickit.go`, `internal/loot/loot.go`
- **Config:** `runs.definitions.<run-id>.loot.pickup_file`; Countess verwendet `pickit/countess.nip`

## Funktionalität

### Datei und Pfadauflösung

Die Pickup-Policy des ausgewählten Runs wird relativ zur geladenen Config-Datei aufgelöst. Beim normalen Layout bedeutet das:

```text
configs/config.yaml -> configs/pickit/countess.nip
```

Wenn ein Operator eine Config an einem anderen Ort nutzt, z. B. `C:\somewhere\custom.yaml`, zeigt der Standard entsprechend auf `C:\somewhere\pickit\countess.nip`.

Fehlende, nicht lesbare oder syntaktisch ungültige Pickit-Dateien brechen den Start mit `pickit config invalid` ab. Eine leere Datei ist gültig und matcht keine Items.

### Unterstützter NIP-Subset

Unterstützte Felder:

- `[name]` gegen `world.Item.Code`
- `[type]` gegen `world.Item.Type`
- `[quality]` gegen `world.ItemQuality.String()`
- `[tier]` gegen die generierte `world.BaseTier` (`unknown`, `normal`, `exceptional`, `elite`)
- `[flag]` mit `identified` und `ethereal`
- `[stat:<id>]` gegen `world.Item.Stats`, ab Phase 5.9 ausschließlich bei `Identified=true`

Quality-Regeln dürfen unidentifizierte Items weiterhin für einen späteren Pickup auswählen. Stat-Regeln matchen bis zur Identifikation nie; Keep/Stash wird zusätzlich über die [Identification-Strategie](identification-strategy.md) gegatet.

Unterstützte Literale:

- Bare identifiers wie `rune`, `pk1`, `unique`
- Quoted strings wie `"pk1"`
- Integer für Stats

Operatoren:

- String-Felder: `==`, `!=`
- Stats: `>`, `>=`, `<`, `<=`, `==`, `!=`
- Logik: `&&`, `||`, Klammern
- `#` wird im MVP wie `&&` behandelt

String-Vergleiche sind case-insensitive. Die originale Regel bleibt im `PickitResult.Rule` erhalten. `[tier]` ist ein rein read-only Prädikat; unbekannte Literale werden abgewiesen und Misc-Items werden niemals heuristisch als Elite behandelt.

Nicht unterstützt sind unter anderem `[maxquantity]`, Prefix/Suffix, Charged Skills, Advanced Aliases und mehrteilige NIP-Sektionen. Solche Konstrukte schlagen beim Parsen mit Datei- und Zeilenkontext fehl.

### Ergebnis

`Pickit.Evaluate` liefert den ersten Treffer:

```go
type PickitResult struct {
    Matched   bool
    RuleIndex int
    Line      int
    Rule      string
}
```

`RuleIndex` ist nullbasiert innerhalb der geparsten Regeln. `Line` ist die einbasierte Zeile in der Pickit-Datei.

## Default Countess Pickit

`configs/pickit/countess.nip` startet bewusst klein:

- Runen per `[type] == rune`
- Key of Terror per `[name] == pk1`
- Rejuvenation und Full Rejuvenation per `[type] == rpot`, da sie nicht bei Town-Vendoren gekauft werden können
- Flawless und Perfect Gems per expliziten Item-Codes

Gold sowie Healing-/Mana-Potion-Regeln bleiben kommentierte Beispiele. Rejuvenation bildet die bewusste Ausnahme: Sie wird als seltener Loot aufgenommen und kann zentral gestasht werden; Crafting und automatisches Auffüllen aus dem 99er-Stash-Slot sind nicht Bestandteil von Phase 9.

## Operator / CLI

Es gibt keine neue CLI-Oberfläche. Pickit wird beim Start geladen, sobald die App-Komponenten verdrahtet werden. Für manuelle Validierung bleiben die bestehenden read-only Probe-Logs und spätere Loot-Phasen relevant.

## Abhängigkeiten

- [Item Enumeration Read-Only](item-enumeration.md) - beschreibt den generierten Item-Katalog und die lokale D2R-Extraktion
- [Inventory Model und Lock Grid](inventory-lock-grid.md) - liefert spätere Kapazitätsgrenzen
- [Loot- und Recovery-Loop](loot-recovery-loop.md) - ordnet Pickit in die Phase-5-Slices ein

## Verwandte Features

- [World Model](world-model.md)
- [Countess-Run](countess-run.md)

---
*Zuletzt aktualisiert: 2026-07-14*
