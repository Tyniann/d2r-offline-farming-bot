# Objekt-Inspect

## Überblick

Read-only Diagnose für Gate 23.0: `--object-inspect` schreibt alle sichtbaren Objekte der aktuellen Area mit ID, UnitID, Position, Mode und Katalogname. Der Walk umgeht die produktive Objekt-Allowlist, damit IDs außerhalb des Katalogs sichtbar bleiben. Nach Gate 23.1 tragen die Lagerfeuerklassen `181`/`183`/`104`/`107` den Runtime-Kind `super_chest` bzw. `rack`; andere Kisten bleiben `unknown`. Derselbe Report listet Inventar-Schlüsselstapel und den rohen Quantity-Stat `70`.

## Ort im Code

- **Paket:** `internal/memory/`, `internal/app/`
- **Einstieg:** `cmd/d2rbot/main.go` → `--object-inspect <label>`
- **Wichtige Dateien:**
  - `internal/memory/object_inspect.go` — Objekt-Walk mit Mode `UnitAny+0x0C`; Active/Base-Statlisten für Items
  - `internal/app/object_inspect.go` — CLI, Artefakt, optionale `objects.txt`-Namen
- **Config:** keine neuen Keys
- **Lokale Artefakte:** `diagnostics/objects/*.json` (gitignored)

## Funktionalität

`--object-inspect` deaktiviert die Run-Auswahl, ist mit Run-/Input-/Route-/Town-Testmodi gegenseitig exklusiv und sendet keine Tastatur- oder Mausaktion. Nach dem ersten gültigen In-Game-Snapshot schreibt der Modus ein JSON und beendet sich.

Jedes Objekt enthält:

- `txt_file_no`, `unit_id`, Position, Mode plus Known-Flags
- `catalog_name` / `catalog_class` aus dem bestehenden World-Lookup oder, falls vorhanden, aus `.tmp/d2r-excel/objects.txt`
- `runtime_kind` für katalogisierte Kinds; unbekannte IDs bleiben `unknown`
- Manhattan-Distanz zum Spieler, damit die Hüttenobjekte oben stehen

Schlüsselstapel (`code=key`) erscheinen getrennt. `stats` ist die produktive Active-Liste. `stats_active` und `stats_base` werden extra gelesen, weil Active mit Count 0 Base verdecken kann. `quantity_stat` ist gesetzt, wenn Stat `70` in einer der beiden Layer-0-Listen steht (`quantity_source`: `active` oder `base`). Der produktive Stackzähler liegt seit 23.1 auf `world.Item.Quantity`.

## Operator / CLI

```powershell
go run ./cmd/d2rbot --config configs\config.yaml --object-inspect closed
go run ./cmd/d2rbot --config configs\config.yaml --object-inspect opened
go run ./cmd/d2rbot --config configs\config.yaml --object-inspect locked-with-key
go run ./cmd/d2rbot --config configs\config.yaml --object-inspect locked-no-key
go run ./cmd/d2rbot --config configs\config.yaml --object-inspect keys-in-town
```

Szene zuerst herstellen, dann den Befehl starten. Inspect klickt nicht und wartet nicht. Für Quantity: in der Stadt stehen, Stapelgröße kennen, `keys-in-town` ausführen. In der JSON `quantity_source` und `stats_base` prüfen.

`--object-inspect-timeout-ms` (Default 30000) begrenzt das Warten auf einen In-Game-Snapshot. Labels: `a-z0-9` und Bindestrich, 1–64 Zeichen.

## Abhängigkeiten

Derselbe Unit-Table-Walk und Mode-Offset wie Hireling- und Cow-Diagnose. `objects.txt` ist nur für Anzeigenamen optional.

## Verwandte Features

- [Memory Reader](memory-reader.md)
- [State Probe](state-probe.md)
- [World Model](world-model.md)
- [Item Enumeration Read-Only](item-enumeration.md)

---
*Zuletzt aktualisiert: 2026-08-20*
