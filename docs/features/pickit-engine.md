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
- **Config:** `loot.pickit_file`, Standard `pickit/countess.nip`

## Funktionalität

### Datei und Pfadauflösung

`loot.pickit_file` wird relativ zur geladenen Config-Datei aufgelöst. Beim normalen Layout bedeutet das:

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
- `[flag]` mit `identified` und `ethereal`
- `[stat:<id>]` gegen rohe `world.Item.Stats`

Unterstützte Literale:

- Bare identifiers wie `rune`, `pk1`, `unique`
- Quoted strings wie `"pk1"`
- Integer für Stats

Operatoren:

- String-Felder: `==`, `!=`
- Stats: `>`, `>=`, `<`, `<=`, `==`, `!=`
- Logik: `&&`, `||`, Klammern
- `#` wird im MVP wie `&&` behandelt

String-Vergleiche sind case-insensitive. Die originale Regel bleibt im `PickitResult.Rule` erhalten.

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
- Flawless und Perfect Gems per expliziten Item-Codes

Gold und Potion-Regeln sind nur kommentierte Beispiele. Potion-Aufnahme braucht später Belt-Zustand und gehört deshalb nicht in Phase 5.3.

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
*Zuletzt aktualisiert: 2026-07-05*
