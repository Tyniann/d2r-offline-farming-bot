# Summoner-Run

## Überblick

Der Summoner-Run farmt den Arcane-Sanctuary-Boss für Key of Hate (`pk2`). Er nutzt dieselbe gemeinsame `runPipeline` wie Countess und Mephisto; run-spezifisch sind Registry-Definition, Act-2-Waypoint, Recording-Contract, leere Boss-Engage-Sequenz und Pickit-Defaults `[gems, keys]`.

## Ort im Code

- **Paket:** `internal/tasks/`
- **Einstieg:** `cmd/d2rbot --run summoner`
- **Wichtige Dateien:** `registry.go`, `run_pipeline.go`, `run_contract.go`
- **Config:** `runs.definitions.summoner`, `town.egress.act2`, `character_setup.pickit_defaults.summoner`
- **IDs:** Boss `*hcIdx` 250 und Area 74 aus `.tmp/d2r-excel` (`monstats.txt` / `levels.txt`)

## Funktionalität

### Travel und Route

Vom Act-1-Hub öffnet die Pipeline den Waypoint und wählt Arcane Sanctuary. Eine character-/difficulty-gebundene Teleport-Route (ein Terminal-Segment in Area 74) führt zur Kampfposition. Es gibt keinen Explorer-Fallback und keinen Suchanker: Die Aufnahme muss den Boss sichtbar machen.

### Boss und Combat

Die Definition pinnt den Summoner über NPC-ID `250` (`world.Summoner`) ohne Super-Unique-Gate. `BossEngageSequence` ist leer — kein Bone Prison; nach Acquire folgt direkt Bone Spear über `necro_bone_spear`. Kill-Confirm wie bei Mephisto über Abwesenheit der gepinnten UnitID.

Im Full Run folgt danach und vor dem Teleport zur Bossleiche der opt-in Step `clear_nearby_hostiles`: Er ermittelt in jedem Snapshot neu den nächsten lebenden Specter-, Hell-Clan- oder Ghoul-Lord-Typ innerhalb von 18 Tiles, zielt auf dessen aktuelle Memory-Position und castet den normalen `attack_skill` erst nach Hover-Bestätigung genau dieser Monster-`UnitID`. Drei gegnerfreie Snapshots oder höchstens 20 tatsächlich gesendete Angriffe beenden die best-effort Bereinigung; anschließend folgt die Loot-Repositionierung auch bei verbleibenden Gegnern. Die isolierte `boss`-Phase endet weiterhin direkt nach Kill-Confirm.

### Loot und Rückkehr

Pickit-Defaults sind `[gems, keys]` (`pk1`/`pk2`/`pk3` im gemeinsamen `keys`-Profil). Rückkehr exakt wie Mephisto: Town Portal → Lut Gholein → Act-2-System-Egress → Rogue Encampment → Stash.

## Datenmodell

- `RunIDSummoner` — `summoner`
- `BossDescriptor` — NPC-ID `250`, keine SearchAnchors, keine Super-Unique-Gates
- `BossEngageSequence` — leer
- `ClearNearbyAfterBoss` — aktiviert die begrenzte Full-Run-Bereinigung
- `ReturnOrigin` — `act2` inkl. `foreign_town_egress`
- `TerminalMaxDistanceTiles` — `60`

## Operator / CLI

```powershell
go run ./cmd/d2rbot --config configs/config.yaml --run summoner --phase boss --probe --verbose
```

Vollständiger Lauf erfordert freigeschalteten Arcane-Waypoint, gebundene Farming-Route und gültigen Act-2-Egress. Session-Queue bleibt spielerseitig; Defaults werden nicht um Summoner erweitert.

## Live-Gate

**Status:** Phase C und der dokumentarische Abschluss in Phase D sind am 28. Juli 2026 abgeschlossen.

Der produktive Hell-Lauf `summoner-20260728t210020999999999z-e529a68f` in Session `session-20260728t210020999999999z-46be6615` belegt das vollständige Live-Gate mit MrBones, Profil `necro_bone_spear`, Route `summoner-mrbones-e59fe08f23` und Pickit-Kette `[gems, keys]`. Der Act-2-Waypoint führte memory-bestätigt in Area 74; das gebundene Arcane-Segment wurde vollständig abgespielt. Der Summoner wurde als UnitID 144 erworben und sein Kill bestätigt.

Anschließend liefen Bereinigung, Drop-Scan, Town Portal, Rückkehr nach Lut Gholein, Act-2-System-Egress, Wechsel ins Rogue Encampment, Personal Stash und `prepare_town_handoff` erfolgreich bis `run_completed`. Der Lauf dauerte zwischen `run_context` und `run_completed` rund 74,8 Sekunden. Ein Key of Hate war für das Bestehen nicht erforderlich und fiel in diesem Lauf nicht.

Damit sind Waypoint-Kalibrierung, Route, Assignment und ein produktiver Full Run nachgewiesen. Später beobachtete Langzeitprobleme durch Gegner auf der Arcane-Route, Mana-Burn und lokale Route-Recovery ändern dieses funktionale Gate nicht; ihre separate Härtung ist als Phase 17 geplant.

## Abhängigkeiten

- `internal/pathing` — `WaypointTargetArcaneSanctuary`, Act-2-Tab
- `internal/town` — Act-2-Egress
- `internal/loot` — Profilkette `gems` + `keys`

## Verwandte Features

- [Mephisto-Run](mephisto-run.md)
- [Nihlathak-Run](nihlathak-run.md)
- [Run Registry](run-registry.md)
- [Route Recording und Playback](route-recording-playback.md)

---
*Zuletzt aktualisiert: 2026-07-28*
