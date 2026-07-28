# Run Registry und gemeinsames Run-Schema

## Überblick

Die Run-Registry hält stabile, typisierte Definitionen für `countess`, `mephisto`, `summoner` und `nihlathak`. Sie enthält ausschließlich unveränderliche Produktmetadaten und Capabilities; operatorabhängige Route, Combat-Tuning sowie Pickup-/Sell-Policies kommen für alle IDs aus demselben Config-Typ.

Alle Runs teilen die gemeinsame Pipeline ohne Run-ID-Switch. Fremdakt-Runs (Mephisto Act 3, Summoner Act 2, Nihlathak Act 5) verlangen `foreign_town_egress`. Ohne live verfügbaren Layout-Fingerprint meldet der Resolver für die gebundene Route `route_runtime_validation_required`.

## Ort im Code

- **Paket:** `internal/tasks/`
- **Vertrag:** `internal/tasks/run_contract.go`
- **Registry und Resolver:** `internal/tasks/registry.go`
- **Config:** `internal/config/config.go`
- **Beispiel:** `configs/config.example.yaml`

## Funktionalität

### Definitionen und Registry

`RunDefinition` beschreibt stabile ID, Anzeigename, Entry-/Terminal-Area, Waypoint-Ziel, Boss, geordnete Encounter-Aktionen, Rückkehrakt und erforderliche Capabilities. `RunRegistry` validiert Definitionen, verwirft ungültige oder doppelte IDs und liefert defensive Kopien in deterministischer ID-Reihenfolge.

Seit Phase 12.0 enthält jede Definition zusätzlich einen typisierten `RecordingContract`. Er bindet deutsche Anleitung, Startwegpunkt und -gebiet, erlaubte Routengebiete, Terminalgebiet, denselben autoritativen `BossDescriptor` wie Combat, großzügige Maximaldistanz, Teleport-Bewegung, Town-Portal-Rückweg und Herkunftsakt. Countess akzeptiert Black Marsh bis Cellar 5 mit maximal 80 Tiles Bossdistanz; Mephisto/Summoner/Nihlathak maximal 60 Tiles. Bossnähe beendet keine Aufnahme automatisch.

Countess besitzt eine `boss_engage`-Aktion, verlangt wegen ihrer geteilten Dark-Stalker-Basis-ID das Super-Unique-Flag und kehrt direkt in Act 1 zurück. Mephisto besitzt zwei getrennte `boss_engage`-Aktionen und die NPC-ID `242`. Summoner (`250`) und Nihlathak (`526`) haben eine **leere** Engage-Sequenz (direkt Bone Spear, kein Bone Prison) und verlangen Fremdakt-Egress.

### Gemeinsames Config-Schema

```yaml
runs:
  active: ""
  step_timeout_ms: 45000
  definitions:
    countess:
      combat: { profile: necro_bone_spear, attack_skill: bone_spear }
    mephisto:
      combat: { profile: necro_bone_spear, attack_skill: bone_spear }
    summoner:
      combat: { profile: necro_bone_spear, attack_skill: bone_spear }
    nihlathak:
      combat: { profile: necro_bone_spear, attack_skill: bone_spear }
```

Das frühere `runs.countess`-Sonderschema und ab Abschnitt 13.4 auch run-spezifische `pickup_file`-/`sell_file`-Felder sind entfernt. Alte Blöcke werden ausdrücklich mit Migrationshinweis abgelehnt, statt still ignoriert zu werden. Globale Pickup-Budgets, Stash-Geometrie und Inventory-Lock bleiben unter `loot`; fachliche Policies liegen in globalen Profilen und werden pro Charakter/Run zugeordnet.

## Datenmodell

- `RunID`: stabile Registry-Identität.
- `RunCapability`: deklarative Preflight-Anforderung ohne Fallback-Semantik.
- `RunDefinition`: immutable Produktdaten.
- `config.RunConfig`: identischer Operatorvertrag für jede Run-ID.
- `tasks.RunConfig`: aufgelöste Runtime-Sicht des ausgewählten Runs.
- `ResolvedRun`: Paar aus Definition und exakt derselben ID zugeordneter Config.

Fehlende Config liefert `run_config_missing`, unbekannte ID `run_unknown`, ungültige Definition beziehungsweise Capability-Kombination `run_definition_invalid` oder `run_capability_missing`.

## Operator / CLI

`--run countess|mephisto|summoner|nihlathak` verwendet die gemeinsame Pipeline. Die optionalen Werte für `--phase` sind definitionsneutral: `travel-entry`, `play-route`, `boss`, `loot-and-return`, `stash-personal` und `town-ready`.

## Abhängigkeiten

- `internal/world` für Areas und Boss-Metadaten.
- `internal/pathing` für stabile Waypoint-Ziel-IDs.
- `internal/profile` für semantische Encounter-Hooks.
- `internal/town` für den Rückkehrakt.

## Verwandte Features

- [Task Runner](task-runner.md)
- [Session-Konfiguration und Inspect](session-configuration.md)
- [Character- und Encounter-Profile](character-encounter-profiles.md)
- [Route Recording und Playback](route-recording-playback.md)
- [Summoner-Run](summoner-run.md)
- [Nihlathak-Run](nihlathak-run.md)

---
*Zuletzt aktualisiert: 2026-07-28*
