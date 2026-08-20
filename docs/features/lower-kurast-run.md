# Lower-Kurast-Run

## Überblick

`lower-kurast` farmt die Supertruhen an den Lagerfeuer-Hütten in Area 79. Es gibt keinen Boss und kein globales Route-Clear. Die gemeinsame Pipeline geht nach der gebundenen Einzelroute in `chest_sweep` statt `acquire_boss`. Rückkehr ist Act 3 wie Mephisto. Neu würfeln der Karte ist kein Produktweg: die Aufnahme gilt nur für die bestehende Offline-Karte des Charakters.

## Ort im Code

- **Paket:** `internal/tasks/`
- **Einstieg:** `cmd/d2rbot --run lower-kurast`
- **Wichtige Dateien:** `registry.go`, `chest_select.go`, `chest_sweep.go`, `run_pipeline.go`, `internal/app/chest_operate_adapter.go`, `internal/profile/hammerdin/strategy.go`
- **Config:** `runs.definitions.lower-kurast`, `town.egress.act3`, Pickit-Default `[gems, lk-superchests]`
- **IDs:** Area 79 (`levels.txt`, `*StringName=Lower Kurast`); Objekte aus `objects.txt` Class `JungleChest` 181, `JungleChest2` 183, `ArmorStand1` 104, `WeaponRack2` 107; Stadt-Schlüssel `misc.txt` Code `key`

## Funktionalität

### Town, Schlüssel und Waypoint

Vor dem Handoff `lower-kurast` kauft Akara Stadt-Schlüssel, wenn der Inventarzähler unter 6 liegt. Ziel ist 12, Stückpreis 45 Gold. Gezählt werden persönliche Stapel mit Code `key` über Stat 70; unlesbare Quantity zählt als 0. Uber-Schlüssel `pk1`–`pk3` zählen nicht. Der Kauf bleibt Einzel-RMB. Live erhöht jeder Klick den Zähler um 2 und legt oft einen neuen 2er-Stapel an, statt einen vorhandenen 12er-Stapel zu füllen. Der Verifier stoppt, sobald die Summe das Ziel erreicht.

Ohne Schlüssel überspringt `chest_sweep` eine verschlossene Supertruhe mit `chest_locked_no_key` und beendet den Run deswegen nicht. Der produktive 0-Schlüssel-Lauf ist gestrichen, weil Town unter der Schwelle nachkauft.

Der Act-3-Waypoint wählt Lower Kurast (Tab `273,148`, Zeile 5 / `200,342` bei 1280×720) und erwartet Area 79.

### Route und Lagerfeuer

Aufnahme startet nur am LK-Wegpunkt. Der Vertragstext verlangt die Hüttenreihen bis zum Lagerfeuer: zwei rechteckige Hütten, darunter ein Feuer mit kurzer Mauer. Westliche Hütte zwei große Truhen, östliche eine; dieselben Hütten haben Rüstungs- und Waffengestell. Ein zweites Feuer derselben Anordnung darf mit aufgenommen werden. Truhen und Gestelle werden bei der Aufnahme nicht angeklickt. Ende per F9 in der Nähe der letzten Hüttengruppe.

Playback ist eine Teleport-Einzelroute in Area 79. `RequiresRouteClear()` bleibt `false`. Operate-on-sight hält die Wiedergabe erst, wenn die nächste geschlossene Hütten-Supertruhe höchstens 40 Kacheln entfernt ist. Nach dem Terminal räumt der Rest-Sweep ohne `Route.Hold`. Hover-Misses aus der Fahrt werden dort genau einmal freigegeben.

### Supertruhen und Gestelle

Nur `ObjectKindSuperChest` und `ObjectKindRack` der live belegten Klassen. Andere Act-3-Kisten bleiben unbekannt. Gestelle nur neben einer Supertruhe (Nähe 34/32 Kacheln, Extra-JungleChest 181 an der Westhütte darf mitlaufen). Gestelle auf dem Weg vor dem Feuer bleiben unberührt.

Der Klick zielt auf den Objektkörper (`anchor_offset_tiles` Default 2), nicht auf die Bodenkachel. Höchstens sechs Approach-Teleports. Mode bekannt: geschlossen klicken, geöffnet nicht erneut. Ein Retry ohne Mode-Änderung, dann Skip. Telemetrie: `chest_opened`, `rack_operated`, `chest_skipped` — keine Boss-Kills.

Verdeckt ein Monster-Hover die Objektsuche oder das Pickup, folgt genau ein lokaler Hammerdin-Clear im 12-Kachel-Kreis. Sitzt der Hover auf einem Söldner oder einer Leiche, gilt der nächste lebende Gegner in dem Kreis. Ohne lebenden Gegner kein Kampf. Danach ein neuer Hover- bzw. Pickup-Versuch statt Teleport-Recovery.

### Loot und Rückkehr

Pickit-Default `[gems, lk-superchests]`: Pul bis Ber (`r21`–`r30`) sowie Elite-Unique und Elite-Set, `keep`. Nach jedem Hütten-Cluster wartet die Pipeline Drops und hebt Keep-Treffer auf. Town Portal, Kurast Docks, Act-3-Egress, Rogue Encampment, Personal Stash.

## Datenmodell

- `RunIDLowerKurast` — `lower-kurast`
- `RunCapabilityChestSweep` — ersetzt Boss-Acquire; Fake-NPC unzulässig
- `RecordingTerminalEndpoint` — Area 79, Maximaldistanz 60
- `ReturnOrigin` — `act3`
- `world.JungleChestID` / `JungleChest2ID` / `ArmorStand1ID` / `WeaponRack2ID`
- `town.KeyRestockThreshold` 6, `KeyRestockTarget` 12, `KeyItemCode` `key`

## Operator / CLI

```powershell
go run ./cmd/d2rbot --run lower-kurast --probe --verbose
go run ./cmd/d2rbot --run lower-kurast --phase boss --probe --verbose
go run ./cmd/d2rbot --town-test waypoint:lower_kurast
```

`--phase boss` ist hier `precheck -> chest_sweep` und startet in Area 79. Vollständiger Lauf verlangt Act-1-Town, Akara-Shop, LK-Wegpunkt, gebundene Route und Act-3-Egress. Isoliertes `--object-inspect` bleibt die Diagnose vor Produkt-IDs.

## Live-Gate

**Status:** Phase 23 Gates 23.0–23.9 sind am 20.08.2026 abgeschlossen. Der 0-Schlüssel-Lauf ist gestrichen.

Session `session-20260820t183916999999999z-a7b983e6`, MrHammer Hell, Route `lower-kurast-mrhammer-d48622a56f`, Seed `466817790`. Fünf produktive Läufe endeten jeweils mit `run_completed`: vier Supertruhen (181/183) und die beiden Hüttengestelle (107 und 104), keine `chest_skipped`, kein lokaler Clear. Run 4 kaufte bei Akara nach (`current_count` 5, Schwelle 6, `verified_final_count` 12). Pickit-Treffer gingen in den Stash. Session-Ende per Stop (`game_start_failed`) zählt nicht gegen das Gate.

## Abhängigkeiten

- `internal/pathing` — `WaypointTargetLowerKurast`
- `internal/town` — `RestockKey`, Act-3-Egress
- `internal/loot` — Profilkette `gems` + `lk-superchests`
- `internal/world` — Objektkatalog aus `.tmp/d2r-excel/objects.txt`

## Verwandte Features

- [Task Runner](task-runner.md)
- [Run Registry](run-registry.md)
- [Town Services](town-services.md)
- [Paladin „Hammerdin“](hammerdin.md)
- [Objekt-Inspect](object-inspect.md)
- [Pickit-Profile und Assignments](pickit-profiles.md)
- [Mephisto-Run](mephisto-run.md)

---
*Zuletzt aktualisiert: 2026-08-20*
