# Mercenary Support (Phase 18)

## Überblick

Fail-closed Söldner-Support für Offline-Farming: Merc-Zustand aus Memory, Combat-Heilung per Shift+Belt nach allen Spielertränken, Town-Heilung bei Akara und Revive bei Kashya. Kein Anheuern, keine Ausrüstung, keine Pet-AI, kein Gold-Precheck.

## Ort im Code

- **Paket:** `internal/memory`, `internal/world`, `internal/config`/`internal/profile`, `internal/input`, `internal/tasks`, `internal/town`, `internal/app`
- **Einstieg:** Session-Preflight, Resource-Executor, Town-Preparation, `--mercenary-probe`, `--town-test mercenary-heal|mercenary-revive`
- **Config:** `combat_profiles.*.resources.mercenary` in `configs/config.example.yaml`
- **Graph:** `configs/routes/town/act1/graph/waypoint-kashya-*.yaml` + Variante in `graph.yaml`

## Funktionalität

### Memory / World

Hireling wird im bestehenden Monster-Segment gelesen. Zustände: NotHired, Alive, Dead, Unknown. Abwesenheit allein beweist keinen Tod. `HPPercent` nur bei `VitalsKnown`. Decoder und Evidenztabelle: [Phase-18-Core-Vertrag](phase-18-core-contract.md).

### Combat-Heal

Default: `enabled=true`, `use_below_percent=75`, `belt_slots=[1]`, `cooldown_ms=4000`. Strikt `< 75` (exakt 75 trinkt nicht). Nur `hpot`, Slots ⊆ Healing. Input: `CastBeltWithModifier(shift, slot)`. Erlaubt nur in `engage_boss`, Route-Clear und Post-Boss-Cleanup; Spielerressourcen haben Vorrang. Route-Mana-Hold (`MobilityCritical`) blockiert Merc-Heilung nicht.

### Town

Planner: `identify → mercenary_revive → mercenary_heal → potions/sell → repair`. Heal: Akara einmal klicken, Full-HP bestätigen; Dialog wird nicht geschlossen. Revive: Kashya Home/Down/Enter je Tick einmal; weiterhin Dead nach Enter → `mercenary_revive_insufficient_gold` (Queue nicht retrybar). KISS-Graph: eine reversible Kante `waypoint-kashya` je Layout.

### Preflight

Vor dem ersten Queue-Run: NotHired / Dead-at-start / Invalid stoppen terminal. Disabled Policy ignoriert Merc.

## Datenmodell

| Key | Default | Regel |
|-----|---------|--------|
| `enabled` | true (presence) | explizit `false` deaktiviert Combat+Town-Merc |
| `use_below_percent` | 75 | 1..100; Combat strikt `<` |
| `belt_slots` | `[1]` | unique 1..4, ⊆ healing |
| `cooldown_ms` | 4000 | > 0 |

Telemetrie: `recipient=player|mercenary`, `mercenary_died` (Alive→Dead mit letztem HP%), `town_mercenary_heal_*`, `town_mercenary_revive_*`. Keine Goldschätzung.

## Operator / CLI

```text
--mercenary-probe <label>
--town-test mercenary-heal
--town-test mercenary-revive
--pathing-test record-town-edge:waypoint-kashya
--pathing-test play-town-graph:waypoint,kashya,waypoint
--pathing-test play-town-graph:stash,kashya,waypoint
```

Deutsche Reasons u. a.: „Zu wenig Gold, um den Söldner wiederzubeleben.“, „Für dieses Profil ist kein Söldner angeheuert.“

## Abhängigkeiten

Bestehende Snapshot-/Resource-/Town-/Graph-Verträge. Koolo/d2go nur Recherche.

## Verwandte Features

- [Phase-18-Core-Vertrag](phase-18-core-contract.md)
- [Memory Reader](memory-reader.md)
- [World Model](world-model.md)
- [Character- und Encounter-Profile](character-encounter-profiles.md)
- [Input Controller](input-controller.md)
- [Task Runner](task-runner.md)
- [Town Services](town-services.md)

---
*Zuletzt aktualisiert: 2026-07-31 · Gate 18.0–18.6 bestanden · Phase 18 abgeschlossen*
