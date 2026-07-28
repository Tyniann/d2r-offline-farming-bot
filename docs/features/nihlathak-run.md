# Nihlathak-Run

## Überblick

Der Nihlathak-Run farmt den Boss in den Halls of Vaught für Key of Destruction (`pk3`). Er folgt dem Mephisto-/Summoner-Muster der gemeinsamen Pipeline: Registry-Definition, Act-5-Waypoint Halls of Pain, Multi-Segment-Route, leere Engage-Sequenz und Pickit-Defaults `[gems, keys]`.

## Ort im Code

- **Paket:** `internal/tasks/`
- **Einstieg:** `cmd/d2rbot --run nihlathak`
- **Wichtige Dateien:** `registry.go`, `run_pipeline.go`, `internal/world/entrance_ids.go`
- **Config:** `runs.definitions.nihlathak`, `town.egress.act5`, `character_setup.pickit_defaults.nihlathak`
- **IDs:** Boss `*hcIdx` 526 (`nihlathakboss`, nicht Town `514`); Areas 123→124; Warps 76/77/78 aus `.tmp/d2r-excel/levels.txt`

## Funktionalität

### Travel und Route

Waypoint-Ziel ist Halls of Pain (UI-Name „Halls of Death's Calling“). Die gebundene Route führt nach Halls of Vaught; Entrance-Kinds `halls_down` / `halls_up` / `halls_entrance` klassifizieren die Excel-Warps 77/78/76. Beim Areawechsel bestimmt der Recorder die erwartete Richtung aus dem bestätigten From-/To-Area-Paar; fehlt der passende Entrance im Snapshot, speichert er fail-closed `unknown` statt einen näheren Eingang der Gegenrichtung. Kein Explorer, kein Suchanker, kein CE-Corner-AI.

### Boss und Combat

NPC-ID `526` (`world.Nihlathak`), kein Super-Unique-Gate, leere `BossEngageSequence` → direkt Bone Spear. Kampfverhalten kann nach manuellem Live-Test nachgezogen werden (Corpse Explosion).

### Loot und Rückkehr

`[gems, keys]`; Rückkehr TP → Harrogath → Act-5-Egress → Act 1 → Stash.

## Datenmodell

- `RunIDNihlathak` — `nihlathak`
- Entry `HallsOfPain` (123), Terminal `HallsOfVaught` (124)
- `ReturnOrigin` — `act5`
- `TerminalMaxDistanceTiles` — `60`

## Operator / CLI

```powershell
go run ./cmd/d2rbot --config configs/config.yaml --run nihlathak --phase boss --probe --verbose
```

## Live-Gate

**Status:** Phase F ist am 28. Juli 2026 abgeschlossen.

Der produktive Hell-Lauf `nihlathak-20260728t213428999999999z-32fbc15b` in Session `session-20260728t213428999999999z-b6a82ca8` belegt das vollständige Live-Gate mit MrBones, Profil `necro_bone_spear`, Route `nihlathak-mrbones-8e0752f325` und Pickit-Kette `[gems, keys]`. Der Act-5-Waypoint führte memory-bestätigt in Area 123; die gebundene Multi-Segment-Route wechselte nach Halls of Vaught (Area 124) und endete mit sichtbarem Boss. Nihlathak wurde als UnitID 130 erworben und sein Kill bestätigt.

Anschließend liefen Drop-Scan, Town Portal, Rückkehr nach Harrogath, Act-5-System-Egress, Wechsel ins Rogue Encampment, Personal Stash und `prepare_town_handoff` erfolgreich bis `run_completed`. Der Lauf dauerte zwischen `run_context` und `run_completed` rund 53,0 Sekunden. Ein Key of Destruction war für das Bestehen nicht erforderlich und fiel in diesem Lauf nicht.

Damit sind Waypoint-Kalibrierung, Multi-Segment-Route, Assignment und ein produktiver Full Run nachgewiesen. Der unmittelbar folgende Queue-Zyklus derselben Session wurde per F11 (`emergency_stop_requested`) abgebrochen und zählt nicht gegen dieses Gate.

## Abhängigkeiten

- `internal/pathing` — `WaypointTargetHallsOfPain`
- `internal/world` — Halls-Entrance-Kinds aus `levels.txt`
- `internal/town` — Act-5-Egress

## Verwandte Features

- [Summoner-Run](summoner-run.md)
- [Mephisto-Run](mephisto-run.md)
- [Run Registry](run-registry.md)
- [System-Egress](system-egress.md)

---
*Zuletzt aktualisiert: 2026-07-28*
