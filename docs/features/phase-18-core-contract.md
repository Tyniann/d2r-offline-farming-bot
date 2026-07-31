# Phase-18-Core-Vertrag

## Überblick

Abschnitt 18.0 friert die Baseline und den live bestätigten read-only Merc-Evidenzvertrag ein. Abschnitt 18.1 integriert diesen Vertrag in den produktiven Memory-Snapshot und das World Model. Es gibt weiterhin weder Merc-Input noch Town-Heal/Revive oder Shift+Belt.

Detailplan: [`phase-18-implementation-plan.html`](../../phase-18-implementation-plan.html).

## Ort im Code

- **Hireling-Katalog:** `internal/memory/hireling.go`
- **Diagnose-CLI:** `internal/app/mercenary_probe.go`, Flag `--mercenary-probe`
- **Artefakte:** `diagnostics/mercenary/` (gitignored)
- **Vertrag:** diese Datei
- **Detailplan:** `phase-18-implementation-plan.html`

## Status und Sequenzgrenze

18.0 definiert und misst:

- reguläre Hireling-Class-IDs aus lokaler `hireling.txt`;
- eine read-only Diagnoseausgabe für Raw-ID, Corpse, Mode, UI und Life-/MaxLife-Rohwerte;
- Abbruchkriterien für Unknown versus Dead versus NotHired.

Gate 18.0 wurde am 30. Juli 2026 mit fünf kontrollierten Captures bestanden. 18.1 bindet:

- `Snapshot.Mercenary` im bestehenden Monster-Segment-Walk;
- `world.State.Mercenary`, `HPPercent`, Mapping und Reset;
- Merc-Transitionen im bestehenden `--probe`-World-Log.

18.1 bindet weiterhin nicht:

- Shift+Belt, Akara-Heal oder Kashya-Revive;
- Config `resources.mercenary`;
- Town-Demand oder Preflight-Reasons.

**Gate 18.1 ist bestanden.** Live-Abgleich `alive-injured` am 30. Juli 2026: produktiver State `alive=true`, `41/90` (`45%`), Raw `15237/23040`, `hostile_hireling_count=0`.

## Dirty Worktree vor 18.0

Vor dem ersten Phase-18-Code stand der Worktree so:

| Pfad | Besitz |
|---|---|
| `.cursor/rules/00-d2rbot-project.mdc` | Operator / bestehende lokale Änderung |
| `AGENTS.md` | Operator / bestehende lokale Änderung |
| `docs/CHANGELOG.md` | bereits Plan-Eintrag Phase 18 |
| `handoff.html` | bereits Phase-18-Zielbeschreibung |
| `phase-18-implementation-plan.html` | neuer Plan (untracked) |

Phase-18-Code ändert ausschließlich die für 18.0 vorgesehenen Dateien und Docs. Operator-eigene Dateien werden nicht zurückgesetzt.

## Belegte Baseline (automatisch)

Am 30. Juli 2026 vor dem ersten Merc-Diagnosecode:

| Bereich | Ergebnis |
|---|---|
| `go build ./cmd/d2rbot` | grün |
| `go test ./internal/memory ./internal/world ./internal/profile ./internal/input ./internal/town ./internal/app` | grün |
| `go test ./internal/tasks` | **3 vorbestehende Fails** in Route-Recovery-Guard-Tests (`route_threat_state_invalid`, fehlendes `RecoveryOutcomeAt` in Fixtures) |
| `go test ./...` | scheitert ausschließlich an den drei Tasks-Fails |

Die Tasks-Fails gehörten nicht zu Phase 18. Während 18.1 wurden die betroffenen Fixtures durch eine parallele, nicht zu Phase 18 gehörende Worktree-Änderung um `RecoveryOutcomeAt` ergänzt; die anschließende vollständige Go-Matrix war grün.

Nach 18.1-Code:

| Bereich | Ergebnis |
|---|---|
| `go test ./internal/memory ./internal/world ./internal/app -count=1` | grün |
| `go test ./... -count=1` | grün |
| `go build ./cmd/d2rbot` | grün |

## Hireling-Class-IDs

Autoritative Quelle: `.tmp/d2r-excel/hireling.txt`, Spalte `Class`. Keine IDs aus Koolo oder älteren d2go-Katalogen.

| Class | Name | Act | Seller (hireling.txt) |
|---:|---|---:|---:|
| 271 | Rogue Scout | 1 | 150 |
| 338 | Desert Mercenary | 2 | 198 |
| 359 | Eastern Sorceror | 3 | 252 |
| 560 | Barbarian | 5 | 515 |
| 561 | Barbarian | 5 | 515 |

Produktiv abgenommen wird später der tatsächlich von MrBones verwendete Akt-2-Merc (Class `338`). Memory erkennt alle fünf Class-IDs compile-nah. Seller-NPCs sind keine Hirelings.

## Read-only Diagnosevertrag

### CLI

```powershell
go run ./cmd/d2rbot --config configs/config.yaml --mercenary-probe not-hired
go run ./cmd/d2rbot --config configs/config.yaml --mercenary-probe alive-healthy
go run ./cmd/d2rbot --config configs/config.yaml --mercenary-probe alive-injured
go run ./cmd/d2rbot --config configs/config.yaml --mercenary-probe dead
go run ./cmd/d2rbot --config configs/config.yaml --mercenary-probe area-transition
```

- Kein Input, kein Bindings-Precheck, kein Session-/Run-Start.
- Standard: 8 Samples in 45 s; `area-transition` mindestens 24 Samples / 90 s.
- Ausgabe: `diagnostics/mercenary/<utc>-<label>.json`

### Pflichtfelder je Sample

- Phase, Valid/Reason, AreaID, Player-HP
- UI: `NPCInteractOpen`, Shop, Inventory, Stash, Waypoint, Quit
- Produktiver `Mercenary`-Wert sowie `monster_count`, `eligible_monster_count` und `hostile_hireling_count`
- Je Hireling-Kandidat: `npc_id`, `unit_id`, `corpse`, `mode`/`mode_known`, Position, Raw Life/MaxLife aus Base- und Active-Liste
- Kandidaten-Interpretationen: `*_shift8` (`>> 8`) und `*_frac_32768` (`raw / 32768`)

### Harte Leseregeln

1. Abwesenheit allein beweist weder Tod noch „nicht angeheuert“.
2. `Alive` und `Dead` schließen einander aus; Unknown ist der Fail-closed Default.
3. Area-Transition / Loading / unlesbare Stats → Unknown, nie Dead.
4. Mehrere widersprüchliche Hirelings → Unknown.
5. Hirelings gehören nie in Hostile-/Coverage-Reservoire.

## Evidenztabelle (Gate 18.0 bestanden)

| Zustand | Label | Beobachtung | Artefakt |
|---|---|---|---|
| Kein Merc angeheuert | `not-hired` | MrBook: acht valide In-Game-Samples in Area 1 ohne eine reguläre Hireling-Class; keine lebende oder tote Hireling-Unit. | `20260730T201814.787902600Z-not-hired.json` |
| Lebend, gesund | `alive-healthy` | MrHammer: Rogue Scout 271, Unit 1, Corpse 0, Mode 1; Raw Life 32768, Raw MaxLife 23040. | `20260730T202011.663964300Z-alive-healthy.json` |
| Lebend, verletzt | `alive-injured` | MrHammer: Rogue Scout 271, Unit 1, Corpse 0, Mode 2; Raw Life 4096, Raw MaxLife 23040. | `20260730T202800.307287400Z-alive-injured.json` |
| Tot | `dead` | MrHammer: dieselbe Class/Unit bleibt enumeriert; Corpse 1 und Mode 12 in acht Samples, Life fehlt. | `20260730T202450.794081300Z-dead.json` |
| Alive → Areawechsel → Alive | `area-transition` | Area 32 → 5; Hireling bleibt Unit 1/Corpse 0, Mode wechselt bewegungsbedingt 1/2. Ein Zielarea-Sample besitzt kurz Position 0/0, erzeugt aber keinen Dead-State. | `20260730T202850.315856600Z-area-transition.json` |

Die NotHired-Evidenz ist nicht „keine Living-Unit“, sondern ein vollständiger Monster-Segment-Walk ohne irgendeine reguläre Hireling-Class (einschließlich Corpse) über drei frische, identitätsbestätigte In-Game-Snapshots. Loading, invalid, unbestätigte Identity und Segmentfehler setzen die Bestätigung zurück.

### Eingefrorener Decoder

| Signal | Eingefrorene Bedeutung | Live-Beleg |
|---|---|---|
| MaxLife | `rawMaxLife >> 8` | 23040 → 90 |
| Life | `MaxHP * clamp(rawLife, 0, 32768) / 32768` | 32768 → 90/90; 4096 → 11/90 |
| Mode `0x0C` | Diagnose-/Death-Evidenz; Mode 12 ist tot | lebend 1/2, tot 12 |
| Corpse `0x1AE` | `!= 0` ist direkte Dead-Evidenz | lebend 0, tot 1 |
| Active-Stats | für den beobachteten Hireling nicht vorhanden | Base-Liste ist Vitals-Autorität |

Player-Vitals und ihr Normalizer bleiben unverändert.

## 18.1 Memory- und World-Vertrag

- Hirelings werden im vorhandenen Monster-Segment-Walk vor Corpse- und Hostile-Filter abgezweigt.
- Es gibt keinen zweiten produktiven Segment-Walk; der zusätzliche Raw-Walk existiert nur im expliziten Diagnosemodus.
- Hirelings gelangen nie in `Snapshot.Monsters`, `EligibleMonsterCount` oder Coverage.
- Mehrere Hirelings, unlesbare Identität/Mode, invalid/loading und inkonsistente Zustände ergeben Unknown.
- Eine lebende Unit mit unlesbaren Vitals bleibt `Alive=true`, `VitalsKnown=false`.
- Dead besitzt keine Vitals. `world.FromSnapshot` verwirft widersprüchliche Alive/Dead-Kombinationen.
- `world.Model.Reset` und invalid Snapshots nullen Merc vollständig.
- `--mercenary-probe` schreibt ab 18.1 Raw-Evidenz und den gleichzeitig produktiv gemappten semantischen Merc-State.

## Ownership

| Verantwortung | Owner | Grenze |
|---|---|---|
| Raw Hireling-/Corpse-/Stat-Evidenz | `internal/memory` | Kein Task-/Town-Process-Read |
| Diagnose-Artefakt | `internal/app` `--mercenary-probe` | Kein Input |
| Semantischer Merc-State | `internal/world` | Kein Downstream-Code deutet Abwesenheit selbst |
| Combat-/Town-Aktionen | ab 18.2 / 18.3 | vor Gate 18.0 verboten |

## Nicht-Ziele in 18.0

- Shift+Belt, Rejuvenation-Fallback, Inventartränke
- Akara-Heal, Kashya-Revive, Gold-Precheck
- Merc-Anheuern, Ausrüstung, Aura, Pet-AI
- Neue Desktop-UI oder Settings-Seite
- Automatische Aktivierung in Runs

## Verwandte Features

- [Memory Reader](memory-reader.md)
- [State Probe](state-probe.md)
- [World Model](world-model.md)
- [UI-State-Probe](ui-state-probe.md)

---
*Zuletzt aktualisiert: 2026-07-31 · Gate 18.0–18.6 bestanden · Phase 18 abgeschlossen*
