# Pickit Engine

## Überblick

Phase 5.3 führt eine kleine Pickit-Engine ein, die einen bewusst begrenzten NIP-Subset lädt und gegen Items aus dem bestehenden World Model auswertet. Abschnitt 13.3 ergänzt exakt aufgelöste Set-/Unique-Identitäten, die Aktionen `keep`/`sell`, stabile Regelherkunft und einen geordneten First-Match-Trace. Die Engine hebt weiterhin nichts auf, bewegt keine Items und führt selbst weder Identify-, Stash- noch Vendor-Input aus.

Die Item-Identität kommt ausschließlich aus dem generierten `internal/world`-Katalog:

```text
memory raw item -> world.Item via generated base/identity catalog -> loot.Pickit evaluation
```

Die lokalen Dateien unter `.tmp/d2r-excel` sind nur Eingabe für die Katalog-Regeneration. Runtime-Code liest diese Dateien nicht.

## Ort im Code

- **Paket:** `internal/loot/`
- **Einstieg:** `loot.CompilePickitRules` über den Assignment-Resolver in `internal/app/pickit_store.go`
- **Wichtige Dateien:** `internal/loot/pickit.go`, `internal/loot/loot.go`
- **Config:** `configs/pickit/profiles/*.yaml` und `configs/pickit-assignments.local.yaml`

## Funktionalität

### Profile und Pfadauflösung

Globale Profile liegen relativ zur geladenen Config unter `pickit/profiles/`; die lokale, gitignorierte Zuordnung liegt in `pickit-assignments.local.yaml`. Beide werden strikt und kataloggebunden geladen. Fehlende, nicht lesbare, syntaktisch ungültige oder unbekannt referenzierte Profile brechen den autorisierten Kontext fail-closed ab. Die frühere Run-Config mit `pickup_file`/`sell_file` und deren NIP-Dateien wurde in 13.4 entfernt.

### Unterstützter NIP-Subset

Unterstützte Felder:

- `[name]` gegen `world.Item.Code`
- `[type]` gegen `world.Item.Type`
- `[quality]` gegen `world.ItemQuality.String()`
- `[tier]` gegen die generierte `world.BaseTier` (`unknown`, `normal`, `exceptional`, `elite`)
- `[setitem]` gegen den stabilen Schlüssel einer konsistent aufgelösten Set-Identität
- `[uniqueitem]` gegen den stabilen Schlüssel einer konsistent aufgelösten Unique-Identität
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

`[setitem]` und `[uniqueitem]` akzeptieren ausschließlich `==` und `!=`. Jede Referenz wird bereits beim Kompilieren case-insensitiv eindeutig im patchgenauen World-Katalog aufgelöst und kanonisch serialisiert. Ein Item matcht nur bei `IdentityValid=true` und passender Identitätsart. Unavailable, unbekannte oder widersprüchliche Identität bleibt auch bei einer `!=`-Bedingung fail-closed. Dadurch erzeugt weder ein Katalogdrift noch ein fehlgeschlagener Raw-Read ein positives Match.

Der kanonische Serializer verwendet doppelte Anführungszeichen. Apostrophe bleiben literal; doppelte Anführungszeichen und Backslashes werden eindeutig als `\"` beziehungsweise `\\` geschrieben. Nur Quote und Backslash besitzen Escape-Bedeutung, andere bestehende Backslash-Folgen bleiben literal. `#` wird kanonisch als `&&` ausgegeben.

Nicht unterstützt sind unter anderem `[maxquantity]`, Socket-/Sockelbedingungen, Prefix/Suffix, Charged Skills, Advanced Aliases und mehrteilige NIP-Sektionen. Solche Konstrukte schlagen beim Parsen mit Datei- und Zeilenkontext fehl. „Elite Polearm mit vier Sockeln“ bleibt eine spätere Erweiterung, bis Socket-Datenquelle, Sichtbarkeit vor Identifikation, Typvererbung und UI-Semantik gemeinsam spezifiziert sind.

### Ergebnis

`Pickit.Evaluate` liefert den ersten Treffer:

```go
type PickitResult struct {
    Matched   bool
    RuleIndex int
    Line      int
    Rule      string
    RuleID    string
    ProfileID string
    Action    Action
    ProfileRevision    int
    AssignmentRevision int
    Trace     []PickitTraceEntry
}
```

`RuleIndex` ist nullbasiert innerhalb der geparsten Regeln. `Line` ist die einbasierte Zeile in der Pickit-Datei. `CompilePickitRules` bindet jede Regel an stabile Profil-/Regel-IDs, die aktiven Revisionen und genau eine Aktion. Der Trace enthält ausschließlich tatsächlich in Reihenfolge ausgewertete Regeln und endet beim ersten Treffer; Katalogvorschau und Runtime verwenden dieselbe Auswertung. Der Legacy-Dateiloader wurde mit der Migration in 13.4 entfernt.

## Default Countess Pickit

Die Assignment-Kette `[gems, keys, countess-standard]` reproduziert die bisherige Countess-Policy:

- Runen per `[type] == rune`
- Key of Terror per `[name] == pk1`
- Rejuvenation und Full Rejuvenation per `[type] == rpot`, da sie nicht bei Town-Vendoren gekauft werden können
- Flawless und Perfect Gems per expliziten Item-Codes

Gold sowie Healing-/Mana-Potion-Regeln bleiben kommentierte Beispiele. Rejuvenation bildet die bewusste Ausnahme: Sie wird als seltener Loot aufgenommen und kann zentral gestasht werden; Crafting und automatisches Auffüllen aus dem 99er-Stash-Slot sind nicht Bestandteil von Phase 9.

## Operator / CLI

Es gibt in 13.4 noch keine neue CLI-Oberfläche. Profile und Zuordnung werden beim Start über die App-Services geladen. Für manuelle Validierung bleiben die bestehenden read-only Probe-Logs und Loot-Testmodi relevant.

## Abhängigkeiten

- [Item Enumeration Read-Only](item-enumeration.md) - beschreibt den generierten Item-Katalog und die lokale D2R-Extraktion
- [Inventory Model und Lock Grid](inventory-lock-grid.md) - liefert spätere Kapazitätsgrenzen
- [Loot- und Recovery-Loop](loot-recovery-loop.md) - ordnet Pickit in die Phase-5-Slices ein
- [Pickit-Profile und Assignments](pickit-profiles.md) - persistente Source of Truth und effektive Policy

## Verwandte Features

- [World Model](world-model.md)
- [Countess-Run](countess-run.md)

---
*Zuletzt aktualisiert: 2026-07-21*
