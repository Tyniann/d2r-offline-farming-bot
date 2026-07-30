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

Die Definition pinnt den Summoner über NPC-ID `250` (`world.Summoner`) ohne Super-Unique-Gate. `BossEngageSequence` ist leer — kein Bone Prison; nach Acquire folgt direkt Bone Spear über `necro_bone_spear`. Kill-Confirm wie bei Mephisto über Abwesenheit der gepinnten UnitID. Als bewusst enger Summoner-Sonderfall gilt der Boss nach vollständig abgespielter Route auch dann als erledigt, wenn die Priority-Enumeration ihn beim unmittelbar folgenden Acquire nicht mehr enthält: Der schwache Boss kann bereits durch den Mercenary oder einen durchschlagenden Route-Clear-Angriff sterben. Die Pipeline überspringt dann den Bossangriff und setzt an der aktuellen terminalen Spielerposition mit Restgegnern und Loot fort; andere Runs behalten ihre strikte Boss-Akquise.

Im Full Run folgt danach und vor dem Teleport zur Bossleiche der opt-in Step `clear_nearby_hostiles`: Er ermittelt in jedem Snapshot neu den nächsten lebenden Specter-, Hell-Clan- oder Ghoul-Lord-Typ innerhalb von 18 Tiles, zielt auf dessen aktuelle Memory-Position und castet den normalen `attack_skill` erst nach Hover-Bestätigung genau dieser Monster-`UnitID`. Drei gegnerfreie Snapshots oder höchstens 20 tatsächlich gesendete Angriffe beenden die best-effort Bereinigung; anschließend folgt die Loot-Repositionierung auch bei verbleibenden Gegnern. Die isolierte `boss`-Phase endet weiterhin direkt nach Kill-Confirm.

### Loot und Rückkehr

Pickit-Defaults sind `[gems, keys]` (`pk1`/`pk2`/`pk3` im gemeinsamen `keys`-Profil). Rückkehr exakt wie Mephisto: Town Portal → Lut Gholein → Act-2-System-Egress → Rogue Encampment → Stash.

Während `play_bound_route` verwendet Summoner ebenfalls die beim Runstart unveränderlich gebundene, vom Spieler gewählte Pickit-Kette. Nach einer threat-freien Route-Bewertung und vor dem nächsten Routeninput hebt die Pipeline alle `keep`-Treffer innerhalb von 30 Tiles nacheinander auf. `sell`-Treffer werden unterwegs nicht angefasst. Für entfernte Treffer wird die vorhandene begrenzte Teleport-zum-Loot-Annäherung wiederverwendet; Route-Hold bewahrt dabei Segment, Punkt und Deadline. Kampf besitzt auf jedem frischen Snapshot Vorrang und unterdrückt sämtliche Loot-Eingaben.

## Datenmodell

- `RunIDSummoner` — `summoner`
- `BossDescriptor` — NPC-ID `250`, keine SearchAnchors, keine Super-Unique-Gates
- `BossEngageSequence` — leer
- `ClearNearbyAfterBoss` — aktiviert die begrenzte Full-Run-Bereinigung
- `RunCapabilityRouteClear` — ausschließlich für Summoner registriert
- `RouteHostileNPCIDs` — Specter `40`, Hell Clan `56`, Ghoul Lord `131`; immutable, nicht in YAML

Seit Phase 17.2 ist `runs.definitions.summoner.route_combat` presence-sensitiv validiert und standardmäßig aktiv; `enabled: false` bleibt der Rollback. Seit Phase 17.3 prüft der produktive Route-Step vor jedem Movement Immediate-, Corridor- und Landing-Zone. Bei einem Blocker hält er den unveränderten RoutePlayer, wirkt einmal Amplify Damage (`F1` im Default) auf das erste hover-bestätigte Ziel und greift anschließend stationär per `single_target`/Bone Spear an. Drei lokal vollständige freie Snapshots und ein weiterer frischer Tick sind für die Fortsetzung nötig. Candidate-Playback, Recording sowie Countess, Mephisto und Nihlathak bleiben davon unberührt.

Phase 17.4 bindet die Mana-Reserve an denselben Route-Step: Ein Hold beginnt unter 20 % und endet erst ab 35 %. Zwischen 20 und 34 % darf die Route weiterlaufen, solange zuvor kein Mana-Hold begonnen hat. Immediate-Threat bei höchstens 10 % priorisiert bei sicheren HP eine vorhandene Mana-Potion und verwendet Rejuvenation nur als Fallback; kritische HP behalten Rejuvenation-Vorrang. Ein Belt-Input verbraucht den Tick; während passiver Verbrauchsverifikation darf stationärer Threat-Clear weiterarbeiten. Ohne geeigneten Trank scheitert der Route-Step sofort, ohne bestätigte Erholung spätestens nach fünf Sekunden.

Phase 17.5 behandelt eine lokale Drift-Korrektur wie jedes andere effektive Routenziel: Ein Pack am Previous Point wird vor Recovery gehalten und geräumt. Wurde ein Recovery-Teleport tatsächlich gesendet und bleibt der Positionsfortschritt bis zum frühesten Folgecast unter `pathing.stuck_progress_tiles`, bricht die Pipeline vor einem zweiten identischen Cast mit `route_recovery_unsafe` ab. Es gibt weder alternatives Ziel noch Jitter; das vorhandene Route-Korrekturbudget bleibt über Combat-Holds erhalten.
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

Damit sind Waypoint-Kalibrierung, Route, Assignment und ein produktiver Full Run nachgewiesen. Später beobachtete Langzeitprobleme durch Gegner auf der Arcane-Route, kritisches Mana und lokale Route-Recovery ändern dieses historische funktionale Gate nicht. Die automatische Härtung ist umgesetzt; nach zwei realen Range-Abbrüchen nähert sich der Summoner einem nicht projizierbaren Blocker nun per begrenztem Force Move zum nächsten validierten Routenpunkt. Ein frischer Snapshot muss nach 500 ms mindestens ein Tile Distanzgewinn bestätigen, andernfalls endet erst der dritte wirkungslose Versuch mit `route_threat_out_of_range`. Die neue manuelle Hell-Abnahme steht noch aus.

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
*Zuletzt aktualisiert: 2026-07-30*
