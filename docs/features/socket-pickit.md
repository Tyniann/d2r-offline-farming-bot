# Sockel-Support für Pickit (Phase 19)

## Überblick

Phase 19 ergänzt das bestehende First-Match-Pickit um fail-closed gelesene Gesamtsockel (`[sockets]`, `[flag] ==|!= socketed`) und einen kombinierten Editor-Builder für Typgruppe, Tier, Sockelvergleich und Ätherisch. **Gates 19.0–19.3 sind bestanden** (31. Juli 2026, D2R `3.2.92777`): Stat 194, Flagmaske `0x800`, Memory-/World-Decoder, Parser/Preview und manueller Editor-Gate-Nachweis sind live belegt.

**19.4** (Dokumentation und Feature-Gesamtmatrix) und **19.5** (manuelle Produktabnahme) sind bestanden. Phase 19 ist abgeschlossen; Freigabeziel `v0.17.0`.

Detailplan: [`phase-19-implementation-plan.html`](../../phase-19-implementation-plan.html).

## Ort im Code

| Schicht | Paket / Pfad | Rolle |
|---------|--------------|-------|
| Memory-Decoder | `internal/memory/stats.go`, `internal/memory/items.go`, `internal/memory/entities.go` | Stat 194, Flag `0x800`, Konsistenz → `Sockets` / `SocketsAvailable` / `Socketed` |
| Diagnose | `internal/app/world_log.go` | Verbose Ground-/Inventory-Hints inkl. Active/Base-Stat-194 und produktiver Sockelfelder |
| World | `internal/world/item.go` | Direkte Projektion der Memory-Felder auf `world.Item` |
| Pickit | `internal/loot/pickit.go` | Parser und Evaluation für `[sockets]` und `[flag] socketed` |
| API | `internal/api/pickit_dto.go`, `internal/api/pickit_backend.go` | Preview-DTO mit Tier- und Socket-Feldern |
| Editor | `web/src/features/pickit/pickitRuleBuilder.ts`, `web/src/features/pickit/PickitFeature.tsx` | Kombinierter Regelbuilder und UI-Integration |

- **Einstieg Probe:** `go run ./cmd/d2rbot --probe --verbose`
- **Excel-Quelle (nur compile-time Beleg):** `.tmp/d2r-excel/itemstatcost.txt` — kein Runtime-Zugriff

## Status und Sequenzgrenze

| Stufe | Status |
|-------|--------|
| 19.0 Diagnose + Excel-Beleg | **bestanden** 31.07.2026 |
| 19.1 Memory-/World-Decoder | **bestanden** 31.07.2026 |
| 19.2 Parser / Evaluation / API | **bestanden** (automatisiert) 31.07.2026 |
| 19.3 Editor-Builder | **bestanden** 31.07.2026 (Profil `gate`) |
| 19.4 Docs-Gesamtmatrix | **bestanden** 31.07.2026 (`go test ./…`, Lint, Web-Tests/-Build grün; Defaults ohne Socket-Regeln) |
| 19.5 Produktabnahme | **bestanden** 31.07.2026 |

Phase 19 ist abgeschlossen. Freigabeziel: `v0.17.0`.

### Gate 19.5 – Live-Abnahme

Profil `test` (`tester`), Zuordnung MrBones/Countess. Isolierter Test: `--data-root … --pathing-test pickup:item --probe --verbose`.

| Fall | Regel | Ergebnis | Log |
|------|-------|----------|-----|
| Positiv | `[type] == "pole" && [tier] == "elite" && [sockets] == 4` | Thresher `7s8` aufgenommen (`status=picked_up`, unit 162) | `d2rbot-20260731-213037.log` |
| Negativ | dieselbe Basis mit `[sockets] == 3` | Ground-Item bleibt, `no candidate` (`ground_item_count=1`) | `d2rbot-20260731-213159.log` |

### Dirty Worktree vor 19.0

Am 31. Juli 2026 vor dem ersten Phase-19-Code:

| Pfad | Besitz |
|------|--------|
| Worktree `master` | sauber gegenüber `origin/master` |
| `node_modules/.vite/vitest/.../results.json` | ggf. lokal untracked; nicht anfassen |
| `.tmp/d2r-excel/` | lokale Extrakte, gitignored |

### Belegte Baseline (automatisch)

| Bereich | Ergebnis |
|---------|----------|
| `go test ./internal/loot ./internal/memory ./internal/world ./internal/app ./internal/api -count=1` | grün (vor und nach Diagnose-Code) |
| `go build ./cmd/d2rbot` | grün |
| `StatNumSockets` | `194` = `item_numsockets` in `itemstatcost.txt` (Extrakt 2026-07-13, Katalog D2R `3.2.92777`) |

## Gate 19.0 – eingefrorene Live-Evidenz

Logs: `logs/d2rbot-20260731-200915.log`, `…-201546.log`, `…-202335.log`. Build: D2R `3.2.92777`.

### Beobachtungsmatrix

| Fall | Item | Qualität | identified | flags | active_stat194 | base_stat194 | Bedeutung |
|------|------|----------|------------|-------|----------------|--------------|-----------|
| A ungesockelt | Skullder's Ire (`xpl`) | unique | true | `0x800010` | absent | absent | Flag `0x800` aus; Stat fehlt (kein `value:0`) |
| B leer gesockelt | Thresher (`7s8`) 4os | normal | true | `0x800810` | absent | value:4 | Flag an; Gesamtzahl 4 |
| C belegt gesockelt | Bone Wand (`bwn`) Runewort White | normal | true | `0x4800810` | absent | value:2 | Flag an; Gesamtzahl 2 trotz belegter Sockel |
| D weiß/grau | Elegant Blade (`7sb`) 1os leer | normal | true | `0x800810` | absent | value:1 | Weiße/graue Base; Sockel vor Identify-Flow lesbar. `identified=true` ist bei Normal-Bases erwartetes D2-Verhalten |
| E Ground→Inventory | Thresher unit 162 | normal | true | `0x800810` | absent | value:4 | Identische Maske und Zahl nach Aufheben; nach Drop wieder am Boden gleich |

Zusatz Identify-Bit (kein Socket-Fall, aber Flag-Kontrolle):

| Item | quality | identified | flags | Socket-Evidenz |
|------|---------|------------|-------|----------------|
| Light Gauntlets (`tgl`) | magic | true | `0x800010` | absent/absent, kein `0x800` |
| Tome (`wa2`) | magic | **false** | `0x800000` | absent/absent, kein `0x800` |

Damit ist Bit `0x10` (Identified) live gegen Magic bestätigt. Ein unidentifiziertes **gesockeltes** Magic/Rare blieb in 19.0 unbelegt; das Produktziel „weiße/graue Bases am Boden“ ist über Fall D abgedeckt.

### Eingefrorene Konstanten und Quellenstrategie

| Entscheidung | Wert |
|--------------|------|
| Stat-ID | `StatNumSockets = 194` (`item_numsockets`) |
| Socketed-Flag | `itemFlagSocketed = 0x800` (Live 31.07.2026, D2R `3.2.92777`) |
| Stat-Quelle | In allen Socket-Fällen lag Stat 194 in **Base**; Active war `absent`. Produktiver Decoder muss Base prüfen; Active zusätzlich, falls später vorhanden. Ein erfolgreicher Active-Parse ohne Stat 194 darf Base nicht verdecken |
| Nullfall `value:0` | Live **nicht** beobachtet. Ungesockelt = Flag aus + Stat `absent` → produktiv `SocketsAvailable=false` (nicht „bekannt 0“) |
| Identify | Socket-Evidenz ist nicht an Identify gebunden; Normal-Bases tragen typischerweise `identified=true` |

## Datenmodell

### Truth Table (`Sockets` / `SocketsAvailable` / `Socketed`)

| Stat 194 | Flag `0x800` | `SocketsAvailable` | `Sockets` | `Socketed` |
|----------|--------------|--------------------|-----------|------------|
| fehlt / unlesbar | beliebig | false | 0 | false |
| 0 | aus | true nur falls live belegt (bisher nicht) | 0 | false |
| 1..6 | an | true | Statwert | true |
| 0 | an | false | 0 | false |
| 1..6 | aus | false | 0 | false |
| außerhalb 0..6 | beliebig | false | 0 | false |

`memory.ItemUnit` und `world.Item` tragen `Sockets`, `SocketsAvailable`, `Socketed`. Der Decoder in `internal/memory` ist die einzige Autorität; World mappt nur. Ein optionaler Decode-Fehler verwirft das Item nicht — es bleibt mit `SocketsAvailable=false` sichtbar.

Active und Base werden jeweils höchstens einmal gelesen. Stat 194 kommt aus Active falls vorhanden, sonst Base. Widersprüchliche Werte → unavailable.

## Funktionalität

### Memory- und World-Decoder (19.1)

Neben den Diagnose-Tokens erscheinen in verbose Hints `sockets=`, `sockets_available=`, `socketed=`. `--probe --verbose` ergänzt Ground- und Inventory-Hints um `identified`, `flags=0x…`, `active_stat194`, `base_stat194` (`unreadable` / `absent` / `value:N`).

**Gate 19.1 – bestanden 31.07.2026** (`logs/d2rbot-20260731-203653.log`):

| Item | sockets / available / socketed |
|------|--------------------------------|
| Skull Cap Lore (`skp`) 2os | `2` / true / true |
| Thresher (`7s8`) 4os | `4` / true / true |
| Skullder's (`xpl`) | `0` / false / false |

`item_count=234`, `ground_item_count=3` unverändert plausibel.

### Pickit-Parser und Evaluation (19.2)

- `[sockets]` mit `==`, `!=`, `>`, `>=`, `<`, `<=` und Integerliteral; eigenes Feld, **kein Identify-Gate**
- `[flag] == socketed` und `[flag] != socketed`; bei `SocketsAvailable=false` liefern beide Prädikate immer `false` (fail-closed, auch für `!=`)
- `[stat:<id>]` behält das bestehende Identify-Gate unverändert

Die Zahl bezeichnet **alle** Sockelplätze (Gesamtsockel), nicht freie oder belegte Sockel.

### API-Preview (19.2)

Der kontrollierte Preview-Endpunkt akzeptiert und liefert zusätzlich:

| Feld | Bedeutung |
|------|-----------|
| `base_tier` | Generiertes Tier des Basisitems |
| `sockets` | Gesamtsockelzahl (0 wenn unavailable) |
| `sockets_available` | Ob konsistente Socket-Evidenz vorliegt |
| `socketed` | Konsistentes Ergebnis aus Flag und Stat |

Widersprüchliche `sockets_available`-Fixtures werden abgelehnt.

### Editor-Builder (19.3)

Der kombinierte Builder in `pickitRuleBuilder.ts` / `PickitFeature.tsx` erzeugt genau eine Core-validierte Regel aus:

1. durchsuchbarer Mehrfachauswahl expliziter Ausrüstungstypen (geklammerte OR-Gruppe),
2. optionalem Tier,
3. verpflichtendem Sockeloperator/-anzahl,
4. optionalem Ätherisch (`[flag] == ethereal`, kanonisch auch im Basis-Schnellpfad).

**Typliste (Auszug):**

| Katalogcode | Anzeige im Editor |
|-------------|-------------------|
| `grim` | Schilde – Hexenmeister (Grimoires) |
| `head` | Schilde – Nekromant (Köpfe) |
| `h2h`, `h2h2` | Assassinenklauen expandieren sichtbar zu `h2h \|\| h2h2` |

Der Hexenmeister hat im Base-Katalog **keinen** eigenen Waffentyp; klassegebunden ist nur Offhand `grim`. Pflichtfeldfehler bleiben lokal im Builder und verändern den Profilentwurf nicht.

**Gate 19.3 – bestanden 31.07.2026:** Profil `%LOCALAPPDATA%\D2ROfflineFarmingBot\configs\pickit\profiles\gate.yaml` mit Ausdruck:

```text
([type] == "shie" || [type] == "ashd") && [tier] == "elite" && [sockets] == 4
```

## Bewusste Nicht-Ziele (Phase 19 gesamt)

- `[emptysockets]`, Socket-Inhalte (Runen, Gems, Jewels), Larzuk-/Max-Sockel
- Typvererbung / Obertypen — siehe optionaler Dokumentationsanker unten
- Änderungen an ausgelieferten Default-Profilen

### Später optional: Typvererbung

Eine spätere Erweiterung könnte `[type]`-Regeln über Parent-Typen aus einer lokalen `itemtypes.txt` auflösen (z. B. „alle Polearm-Bases“ ohne manuelle OR-Liste). Phase 19 implementiert das **nicht**; der Anker dient nur der späteren Spezifikation. Runtime liest diese Datei heute nicht.

## Operator / CLI

```powershell
go run ./cmd/d2rbot --probe --verbose
```

Verbose Hints zeigen Socket-Diagnose und produktive Felder read-only; normale Logs werden nicht pro Poll erweitert.

## Abhängigkeiten

- Windows Memory-Read über `internal/memory`
- Generierter `internal/world`-Basiskatalog für `[type]` und `[tier]`
- Bestehende Pickit-Pipeline (First-Match, Profilrevisionen, Run-Snapshot)

## Verwandte Features

- [Item Enumeration Read-Only](item-enumeration.md)
- [Pickit Engine](pickit-engine.md)
- [Pickit Editor](pickit-editor.md)
- [Pickit API](pickit-api.md)
- [Phase-13-Core-Vertrag](phase-13-core-contract.md) — historische Non-Goals; Phase 19 hebt Socket-Parser-Nachzug auf

---
*Zuletzt aktualisiert: 2026-07-31*
