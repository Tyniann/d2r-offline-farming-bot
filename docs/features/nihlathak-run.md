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

NPC-ID `526` (`world.Nihlathak`), kein Super-Unique-Gate, leere `BossEngageSequence` → direkt Bone Spear. Die aufgezeichnete terminale Spielerposition ist der bevorzugte Kampfanker: Ist Nihlathaks sichtbarer Körper von dort in den spielbaren Clientbereich projizierbar, erfolgt kein Annäherungsteleport – auch dann nicht, wenn der allgemeine Boss-Distanzschwellwert überschritten ist. Nur bei tatsächlich unspielbarer Projektion wird entlang der bestehenden Linie zum Boss die entfernteste noch projizierbare Distanz bestimmt und genau ein Teleport gesendet. Danach sperren 700 ms Settle und ein frischer Snapshot jeden Folgeinput; ein zweiter Annäherungsteleport ist ausgeschlossen. Bleibt der Boss danach weiterhin unzielbar – etwa nach einem leichten Schub während des Kampfs – endet der Engage nicht mehr als kalter Queue-Abbruch mit `combat_action_failed`, sondern mit dem retryfähigen Grund `boss_combat_unprojectable`. Die Runtime nutzt denselben kontrollierten Rückweg wie bei Route-Störungen: Town Portal aus den Halls of Vaught, Harrogath, Act-5-Egress, Rogue Encampment, Save & Exit und Neustart desselben Queue-Eintrags innerhalb der Restart-Budgets.

Die körperversetzte Hover-Spirale beginnt immer an Nihlathaks aktueller Position. Ein anderes lebendes Monster unter dem Cursor darf erst als Bone-Spear-Klickfläche dienen, nachdem der Combat-Adapter den Cursor nachweislich auf den gepinnten Boss projiziert hat und ein danach neuerer Memory-Snapshot den überlagernden Gegner bestätigt. Ein zufälliger Hover vom letzten Routenteleport oder von der bloßen Skillauswahl reicht nicht. Die Freigabe bleibt außerdem an Boss-UnitID, Bossposition und Spielerposition gebunden; eine Bewegung oder ein Annäherungsteleport erzwingt einen neuen Zielversuch. Dadurch fliegt Bone Spear weiterhin durch ein Nihlathak verdeckendes Rudel in Richtung Boss, erzeugt aber nicht vor dem ersten Boss-Aim willkürlich eine Corpse-Explosion-fähige Leiche. Boss-Anwesenheit und Kill-Bestätigung bleiben ausschließlich an die ursprünglich gepinnte Nihlathak-UnitID gebunden. Countess, Mephisto und das Summoner-Route-Interleave bleiben unverändert.

Nach Memory-bestätigtem Bosskill ist diese CE-Gefahr beendet. Nihlathak verwendet deshalb anschließend den vorhandenen begrenzten `clear_nearby_hostiles`-Schritt innerhalb von 30 Tiles. Die Nihlathak-Strategy (`NewNihlathakFactory`) bindet dafür `ConfigureRouteClear` mit Amplify Damage und Bone Spear, ohne die Run-Capability `route_clear` und ohne Travel-Route-Combat: `RequiresRouteClear()` bleibt bewusst `false`. Das Profil wirkt einmal Amplify Damage auf das erste hover-bestätigte lebende Ziel und räumt danach mit Bone Spear. Die Memory-Erfassung enthält dafür den vollständigen gebietseigenen Halls-of-Vaught-Hostile-Katalog, lässt Spieler-Summons jedoch weiterhin aus. Während dieses Post-Boss-Clears darf jedes bereits Memory-bestätigt gehovte lebende Monster sofort angegriffen werden, damit ein überlagerndes Rudel keine neue Aim-Schleife erzeugt. Wird ein Ziel nach der Hover-Suche nicht mehr in den spielbaren Clientbereich projiziert, merkt sich der Cleanup dessen UnitID als unbrauchbar und wählt im nächsten Tick den nächsten Gegner. Drei gegnerfreie beziehungsweise nur noch übersprungene Snapshots, drei Sekunden ohne gesendeten Combat-Input oder das Nihlathak-spezifische Budget von 40 tatsächlich gesendeten Aktionen geben Loot und Town Portal frei.

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

**Status:** Das ursprüngliche Phase-F-Gate wurde am 28. Juli 2026 abgeschlossen. Boss-Targeting und AD-/Bone-Spear-Post-Clear benötigen nach der Memory-Allowlist-Korrektur erneut eine fokussierte Sichtprüfung.

Der produktive Hell-Lauf `nihlathak-20260728t213428999999999z-32fbc15b` in Session `session-20260728t213428999999999z-b6a82ca8` belegt das vollständige Live-Gate mit MrBones, Profil `necro_bone_spear`, Route `nihlathak-mrbones-8e0752f325` und Pickit-Kette `[gems, keys]`. Der Act-5-Waypoint führte memory-bestätigt in Area 123; die gebundene Multi-Segment-Route wechselte nach Halls of Vaught (Area 124) und endete mit sichtbarem Boss. Nihlathak wurde als UnitID 130 erworben und sein Kill bestätigt.

Anschließend liefen Drop-Scan, Town Portal, Rückkehr nach Harrogath, Act-5-System-Egress, Wechsel ins Rogue Encampment, Personal Stash und `prepare_town_handoff` erfolgreich bis `run_completed`. Der Lauf dauerte zwischen `run_context` und `run_completed` rund 53,0 Sekunden. Ein Key of Destruction war für das Bestehen nicht erforderlich und fiel in diesem Lauf nicht.

Damit sind Waypoint-Kalibrierung, Multi-Segment-Route, Assignment und ein produktiver Full Run nachgewiesen. Der unmittelbar folgende Queue-Zyklus derselben Session wurde per F11 (`emergency_stop_requested`) abgebrochen und zählt nicht gegen dieses Gate.

Der spätere Run `nihlathak-20260730t013139999999999z-9c9cea99` erwarb den Boss innerhalb von 55 ms korrekt, schoss danach aber 70-mal im 400-ms-Abstand auf dieselbe projizierte Bodenkachel `(779,285)`. Drei Rejuvenations und zwei Heiltränke wurden verbraucht, bevor der Charakter starb. Das war kein Phase-17-Route-Combat-Eingriff: Nihlathak besitzt keine `route_clear`-Capability. Ursache war der ältere gemeinsame Boss-Pfad ohne Hover-Bestätigung; der oben dokumentierte Nihlathak-spezifische Wechsel behebt genau diese Zieloberfläche.

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
*Zuletzt aktualisiert: 2026-08-13*
