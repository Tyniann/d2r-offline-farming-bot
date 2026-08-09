# Mercenary Support (Phase 18)

## Überblick

Fail-closed Söldner-Support für Offline-Farming: Merc-Zustand aus Memory, Combat-Heilung per Shift+Belt nach allen Spielertränken, sofortiger Offensivstopp bei Tod sowie Town-Heilung bei Akara und Revive bei Kashya. Kein Anheuern, keine Ausrüstung, keine Pet-AI und keine Rückkehr an denselben Routenpunkt.

## Ort im Code

- **Paket:** `internal/memory`, `internal/world`, `internal/config`/`internal/profile`, `internal/input`, `internal/tasks`, `internal/town`, `internal/app`
- **Einstieg:** Session-Preflight, Resource-Executor, Town-Preparation, `--mercenary-probe`, `--town-test mercenary-heal|mercenary-revive`
- **Config:** `combat_profiles.*.resources.mercenary` in `configs/config.example.yaml`
- **Graph:** `configs/routes/town/act1/graph/waypoint-kashya-*.yaml` + Variante in `graph.yaml`

## Funktionalität

### Memory / World

Hireling wird im bestehenden Monster-Segment gelesen. Zustände: NotHired, Alive, Dead, Unknown. Abwesenheit allein beweist keinen Tod. `HPPercent` nur bei `VitalsKnown`. Decoder und Evidenztabelle: [Phase-18-Core-Vertrag](phase-18-core-contract.md).

### Combat-Heal

Default: `enabled=true`, `use_below_percent=50`, `belt_slots=[1]`, `cooldown_ms=4000`. Strikt `< 50` (exakt 50 trinkt nicht). Nur `hpot`, Slots ⊆ Healing. Input: `CastBeltWithModifier(shift, slot)`. Erlaubt nur in `engage_boss`, Route-Clear und Post-Boss-Cleanup; Spielerressourcen haben Vorrang. Route-Mana-Hold (`MobilityCritical`) blockiert Merc-Heilung nicht. Wird eine Merc-Heilung auf einer Combat-Route fällig und ist kein erlaubter Healing-Trank mehr vorhanden, stoppt das Profil den Angriff und verwendet denselben kontrollierten `combat_resource_exhausted`-Retry wie bei erschöpften Spielerressourcen. Außerhalb dieses Route-Kontexts bleibt der fehlende Merc-Trank inputfrei und nicht terminal.

### Town

Planner: `identify → mercenary_revive → mercenary_heal → potions/sell → repair`. Heal: Akara hover-bestätigt anklicken, Full-HP bestätigen; Dialog wird nicht geschlossen. Revive: Kashya hover-bestätigt anklicken, dann Home/Down/Enter je Tick einmal; weiterhin Dead nach Enter → `mercenary_revive_insufficient_gold` (Queue nicht retrybar). Öffnet ein erster bestätigter NPC-Klick innerhalb von 750 ms keinen Dialog, darf dieselbe gepinnte Runtime-UnitID an ihrer aktuellen Position genau einmal neu erfasst und erneut hover-bestätigt geklickt werden. Es gibt keinen Blindklick und keinen dritten Versuch. KISS-Graph: eine reversible Kante `waypoint-kashya` je Layout.

### Preflight

Vor dem produktiven Run bleiben NotHired und Invalid terminal. Ein nachweislich angeheuerter, aber toter Merc läuft dagegen durch dieselbe zentrale Town-Vorbereitung wie nach einem Run: Stash → Kashya, bestehendes `mercenary_revive`, Lebendbestätigung und Handoff zum Waypoint. Fehlende Layoutroute, ungültige Evidenz und unzureichendes Gold bleiben fail-closed. Dieser Readiness-Schritt läuft erst nach der Spielverifikation und besitzt bereits den aktiven Run-Recorder. Ein tatsächlich neu gestartetes und verifiziertes Spiel aktiviert ihn erneut, auch bei Wiederverwendung derselben Retry-Runtime; ein Run-Handoff oder Resume im weiterhin offenen Spiel nicht. Disabled Policy ignoriert Merc.

Die produktive Live-Abnahme `cows-20260808t213233999999999z-815d1ccc` bestätigte den Dead-at-start-Pfad: Kashya wurde über UnitID 5 hover-bestätigt geöffnet, die Wiederbelebung angefordert und 263 Millisekunden später derselbe Merc mit UnitID 1 und 1870 HP als lebend bestätigt. Der Revive-Schritt schloss terminal ab. Ein erst danach beim separaten Potion-Restock ausgelöstes `town_gold_unavailable` ändert diesen Wiederbelebungsnachweis nicht.

### Tod während eines Runs

Eine bestätigte Alive→Dead-Kante stoppt den Combat-Adapter vor jeder weiteren Task-Aktion und schließt den offenen Run-Schritt mit `mercenary_died_during_run`. Die Queue verwendet danach ihren vorhandenen kontrollierten Town-Portal-Rückweg, Save & Exit und denselben endlichen Retry-Index. Der nächste Versuch belebt den Merc vor dem ersten produktiven Task-Tick. Der Tod stoppt daher nicht permanent die gesamte Queue; eine riskante Fortsetzung am Cow-Routenpunkt findet ebenfalls nicht statt.

Run `cows-20260808t213733999999999z-6ddb2272` bestätigte die produktive Alive→Dead-Kante: `mercenary_died` und der Abbruch von `cow_play_cow_sweep` entstanden im selben Snapshot; danach wurde keine weitere Offensivaktion geschrieben. Der Supervisor rief anschließend die bestehende kontrollierte Retry-Rückkehr auf. Der dabei noch installierte Zwischenbuild enthielt den lokalen Cow-Dispatch-Fix für die gemeinsame Phase `retry-return` nicht und blieb deshalb bis zum manuellen F11 ohne Portalinput stehen. Dieser Lauf bestätigt den Offensivstopp, nicht den noch ausstehenden Live-Abschluss der korrigierten Rückkehr.

## Datenmodell

| Key | Default | Regel |
|-----|---------|--------|
| `enabled` | true (presence) | explizit `false` deaktiviert Combat+Town-Merc |
| `use_below_percent` | 50 | 1..100; Combat strikt `<` |
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
*Zuletzt aktualisiert: 2026-08-09 · Readiness an verifizierten Folgespielstart gebunden*
