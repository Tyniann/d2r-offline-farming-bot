# Mephisto-Run

## Überblick

Der Mephisto-Run ist das zweite Farmziel der gemeinsamen Run-Pipeline. Er verwendet dieselben Registry-, Waypoint-, Route-, Boss-, Loot-, Portal-, Town- und Session-Bausteine wie Countess; run-spezifisch sind ausschließlich Definitionsdaten, gebundene Assets und Policies.

## Ort im Code

- **Paket:** `internal/tasks/`
- **Einstieg:** `cmd/d2rbot --run mephisto`
- **Wichtige Dateien:** `registry.go`, `run_pipeline.go`, `phase10_mephisto_run_test.go`
- **Config:** `runs.definitions.mephisto`, `town.egress.act3`
- **Assets:** `configs/routes/farming/mrbones/nightmare/durance-2-mephisto-nightmare-mrbones.yaml`, `configs/routes/town/act3/egress/portal-waypoint.yaml`

## Funktionalität

### Travel und Route

Der Run beginnt am bestätigten Act-1-Stash-/Town-Anker. Die gemeinsame Pipeline läuft zum Waypoint, wählt das registrierte Ziel Durance of Hate Level 2 und spielt die Character-, Difficulty-, Versions- und Layout-gebundene Zwei-Segment-Route bis zur Kampfposition auf Level 3 ab. Der Level-2→Level-3-Übergang verwendet den zentral registrierten Entrance-Kind `durance_down`.

### Boss und Encounter-Sequenz

Die Definition pinnt Mephisto über die eindeutige NPC-ID `242` und die konkrete Runtime-UnitID. Anders als Countess ist der Aktboss kein d2go-Super-Unique-Kandidat; seine Definition setzt deshalb kein Super-Unique-Flag-Gate. Countess behält dieses Gate ausdrücklich, weil ihre Basis-ID auch gewöhnliche Dark Stalker bezeichnet. Vor Bone-Spear-Combat werden genau zwei getrennte `boss_engage`-Aktionen ausgeführt. Beide Aktionen erhalten dieselbe gepinnte UnitID, aber eigene Telemetrieindizes `0` und `1`; der reale Profil-Executor führt für jeden Index eine eigene Delay-/Bone-Prison-/Settle-Sequenz aus. Poll-Retries mit unverändertem Index können keinen zweiten Input auslösen.

Verschwindet der Boss vor Abschluss beider Aktionen oder erscheint Mephisto mit einer anderen UnitID, endet der Run mit `boss_pin_lost`. Erst nach abgeschlossener Sequenz gilt die Abwesenheit der gepinnten UnitID über `kill_confirm_ticks` konsistente Snapshots als Kill-Bestätigung.

Der anschließende Bone-Spear-Angriff verwendet ausschließlich die rechte Skillseite. Nach den Bone Prisons fordert der gemeinsame Combat-Adapter F8 zunächst ohne Mausklick an. Erst ein späterer Memory-Snapshot mit `RightSkillID=84` bestätigt, dass Bone Spear tatsächlich rechts ausgewählt wurde; danach erzeugt jeder freigegebene Combat-Tick einen RMB-Puls an der neu projizierten Bossposition. RMB besitzt für Bone Spear keine Laufsemantik und benötigt deshalb kein synthetisches Stand-still-Modifier. Bleibt F8 links gebunden oder ändert sich nur `LeftSkillID`, endet der Kampf fail-closed ohne Angriffsklick. Die YAML- und D2R-Belegung müssen beide `F8 → Bone Spear auf rechter Skillseite` verwenden; der linke Slot bleibt Attack/Throw.

Hammerdin folgt derselben Pin- und Killbestätigung, überspringt die Bone-Prison-Sequenz aber vollständig. Sobald die aufgezeichnete Route endet, prüft jeder Tick die Distanz zum gepinnten Mephisto: über 3 Tiles folgt ein Teleport auf 1 Tile, innerhalb der Toleranz Blessed Hammer als LMB-Hold auf dem sichtbaren Boss-Sprite. Overlay-Hover auf demselben Körper ist erlaubt. Nach Teleport wird Konzentration auf RMB erneut bestätigt. Ein aktiver Hold wird erst nach drei Snapshots und mehr als 5 Tiles Distanz gelöst; der nächste Teleport wartet den folgenden Snapshot ab. Ohne Hammer-Fortschritt innerhalb von 25 Sekunden oder nach 12 Teleports endet der Versuch mit `boss_combat_no_progress` und verbraucht bei Default-Retry-Listen das Session-Restart-Budget statt die Queue kalt zu stoppen. Gate 22.7 hat diesen Pfad am 16.08.2026 live abgenommen. Countess und Nihlathak verwenden denselben Standardangriff; Nihlathak überspringt zusätzlich den Necro-Post-Boss-Cleanup.

### Loot und Rückkehr

Nach bestätigtem Kill teleportiert die gemeinsame Run-Pipeline zunächst bis auf Pickup-Distanz zur letzten Memory-bestätigten Bossposition. Diese Repositionierung ist für alle registrierten Boss-Runs verbindlich und läuft vor Drop-Wartezeit, Pickit-Scan und Pickup; ein fehlender Positionspin oder Teleportfehler stoppt fail-closed.

Die Zuordnung `[gems, mephisto-standard]` nimmt geschützte makellose/perfekte Gems sowie Exceptional-/Elite-Set/Unique auf. Ausschließlich die `sell`-Regel aus `mephisto-standard` autorisiert letztere Gruppe für den späteren UnitID-gepinnten Cain→Akara-Service. No-Drop wechselt nach stabiler Leerscan-Bestätigung direkt zum Portal. `inventory_full` beendet weitere Pickups ebenfalls und kehrt ohne erneuten Bodenloot-Versuch zurück.

Nach Eintritt in das eigene, hover-bestätigte Town Portal wird Kurast-Docks erwartet. Endet `enter_town_portal` mit `too_far` oder `hover_not_found` (häufig hinter Mephistos Bone Prison), teleportiert die produktive Pipeline einmalig pro Portal-`UnitID` auf die Portalposition und wiederholt den Hover-Click; Guided Recording bleibt unverändert fail-closed. Die gebundene Act-3-Egress-Walkroute führt zum lokalen Waypoint; der gemeinsame Waypoint-Executor wählt Rogue Encampment genau einmal. Erst nach bestätigter Act-1-Ankunft beginnen Stash und optionale Item-Services.

## Datenmodell

- `RunIDMephisto` — stabile Run-ID `mephisto`.
- `BossDescriptor` — NPC-ID `242`, `RequireSuperUnique=false`, kein Super-Unique-Fallback.
- `BossEngageSequence` — zwei Einträge mit `boss_engage`.
- `ReturnOrigin` — `act3`; verlangt `foreign_town_egress`.
- `RunConfig.Loot` — getrennte Pickup- und Sell-Policy.

## Operator / CLI

Isolierter Boss-Test nach manueller Positionierung auf Durance Level 3:

```powershell
go run ./cmd/d2rbot --config configs/config.yaml --run mephisto --phase boss --probe --verbose
```

Der vollständige produktive Lauf wurde am manuellen Session-Gate 10.11 erfolgreich abgenommen. Binding-, Capability-, Route- und Egress-Fehler stoppen vor dem jeweils folgenden Input.

## Abhängigkeiten

- `internal/pathing` — Waypoint, Farming-Route, Entrance-Transition und Act-3-Egress.
- `internal/profile` — zwei geordnete Bone-Prison-Hooks.
- `internal/loot` — Mephisto-Pickup-/Sell-Policies und Inventory-Full-Signal.
- `internal/town` — Act-3-Normalisierung und zentraler Act-1-Serviceplan.

## Verwandte Features

- [Task Runner](task-runner.md)
- [Route Recording und Playback](route-recording-playback.md)
- [Character Encounter Profiles](character-encounter-profiles.md)
- [Town Services](town-services.md)
- [Paladin „Hammerdin“](hammerdin.md)

---
*Zuletzt aktualisiert: 2026-08-29*
