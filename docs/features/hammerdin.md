# Paladin „Hammerdin“

## Überblick

`paladin_hammerdin` ist das zweite freigegebene Kampfprofil. Es registriert ausschließlich Mephisto und friert die Skill-, Slot-, CTA- und Söldnerverträge ein. Phase 22.6 stellt den isolierten CTA-/Holy-Shield-Prebuff bereit und verdrahtet denselben Ablauf produktiv vor dem Abspielen der aufgezeichneten Route. Battle Command und Battle Orders sind CASC-seitig nicht in der Stadt wirkbar. Der produktive Blessed-Hammer-Kampf ist verdrahtet; das manuelle Gate 22.7 ist am 16.08.2026 bestanden.

## Ort im Code

- **Paket:** `internal/profile/hammerdin/`
- **Einstieg:** `hammerdin.NewMephistoFactory`
- **Wichtige Dateien:** `internal/config/profile.go`, `internal/app/combat.go`, `internal/app/combat_strategy_registry.go`, `internal/app/hammerdin_prebuff.go`, `internal/app/hammerdin_town_ready.go`, `internal/tasks/pipeline_boss.go`
- **Config:** `combat_profiles.paladin_hammerdin` in `configs/config.example.yaml`

## Funktionalität

### Profil und erster Run

Das Profil ist für die Klasse `paladin` im Charakter-Setup freigegeben. Die Runtime-Registry enthält genau das Paar `paladin_hammerdin × mephisto`; Countess, Summoner, Nihlathak und Kuh-Level bleiben für dieses Profil nicht verfügbar. Die fünf Pflichtskills sind Teleport, Stadtportal, Gesegneter Hammer, Konzentration und Heiliger Schild.

### Skill-Slots und CTA

Gesegneter Hammer ist trotz seiner beidseitigen CASC-Fähigkeit fest an LMB gebunden. Teleport, Konzentration, Heiliger Schild und die bestehende Stadtportalaktion verwenden RMB. CTA ist kein Pflichtskill und besitzt keinen Aktivierungsschalter: Battle Command und Battle Orders bilden genau ein gemeinsam optionales RMB-Paar. Beide Tasten dürfen fehlen oder müssen gemeinsam gültig belegt sein; jede der beiden Teilbelegungen wird mit der Core-Meldung „Für Call to Arms müssen Battle Command und Battle Orders beide belegt sein.“ abgelehnt.

### Söldner

Ein vorhandener und lebender Söldner ist eine Buildvoraussetzung. Dieses Preflight-Gate bleibt auch dann aktiv, wenn die optionale Söldner-Trankpolicy deaktiviert wurde. Der Bot prüft weder Insight noch Ausrüstung, Aura, Level oder Resistenzen; dafür bleibt der Operator verantwortlich.

### CTA- und Holy-Shield-Prebuff

Der CTA-Zweig verlangt zu Beginn ein frisch bestätigtes Primärset. Er sendet `W` genau einmal, wartet auf eine neuere Snapshot-Generation und bestätigt das Sekundärset. Danach werden Battle Command, Battle Orders, Battle Command und Holy Shield jeweils erst nach bestätigter `RightSkillID` an der neutralen Clientmitte gewirkt. Nach Holy Shield wartet der Bot 1000 ms, bevor ein zweites einzelnes `W` das Primärset wiederherstellt; ein `W` während der Animation wird vom Spiel ignoriert. Das Primärset muss anschließend anhand einer neueren Generation bestätigt werden. Zuletzt werden Gesegneter Hammer auf LMB und Konzentration auf RMB bestätigt.

Der gültige leer/leer-Zweig sendet weder `W` noch Battle-Command-/Battle-Orders-Input. Holy Shield wird einmal im Primärset gewirkt; anschließend werden dieselben Kampfslots wiederhergestellt. Ein unlesbares Waffenset, ein nicht bestätigter Wechsel oder eine nicht bestätigte Skillauswahl beendet den Ablauf ohne Toggle-Schleife.

Die produktive Queue ruft dieselbe Maschine nach der Wegpunkt-Ankunft auf, unmittelbar bevor die aufgezeichnete Route startet. In der Stadt wird nicht gecastet: Battle Command und Battle Orders haben `InTown=false`. CTA vollständig und der 150-Sekunden-Anker noch nicht fällig bedeutet: der Hook endet ohne Input. CTA vollständig und Anker leer oder fällig bedeutet: volle Sequenz inklusive Holy Shield im zweiten Set. Ohne CTA wird Holy Shield einmal pro Spielgeneration im Primärset gewirkt. Der 150-Sekunden-Anker wird erst nach dem tatsächlich autorisierten zweiten Battle-Command-Cast gesetzt. Ein Game- oder Generations-Reset sowie Menu-/Loading-Phasen löschen Timer, Pending Selection und Weapon-Swap-Zustand.

### Mephisto-Standardangriff

Nach dem Wegpunkt wartet die Pipeline auf eine gesetzte Ankunft im Kampfgebiet (InGame, drei frische Snapshots, drei Sekunden Settle), bevor CTA/Holy Shield startet. Sticky `WaypointOpen` blockiert diese Ankunft nicht. Eine nur gemeldete Ziel-Area während des Ladebildschirms reicht nicht.

Nach dem letzten aufgezeichneten Routenpunkt pinnt die gemeinsame Mephisto-Pipeline den Boss über NPC-ID und UnitID. Es gibt keine Bone-Prison- oder sonstigen Encounter-Hooks: `boss_engage` bleibt leer, der Standardangriff beginnt unmittelbar.

Jeder Kampftick prüft Spieler- gegen Bossposition. Liegt die Distanz über 3 Tiles, teleportiert der Bot auf 1 Tile und prüft erneut. Ist die Distanz in Reichweite, zielt der Cursor auf den sichtbaren Boss-Körper wie beim Nihlathak-Aim. Eine andere Hover-ID auf demselben Sprite (überlagerndes Monster) ist erlaubt; die Mausposition hat Vorrang. Anschließend bleibt `LMB` auf diesem Punkt gedrückt, ohne Shift. Alle drei World-Snapshots folgt eine Distanzprüfung. Über 5 Tiles oder bei totem Boss geht LMB hoch; ein Teleport folgt erst im nächsten Snapshot, damit ein sterbender Boss keinen Lauf vom Leichnam weg auslöst. Nach einem Teleport muss Konzentration erneut auf RMB bestätigt sein, bevor der Hold startet. Auswahl, Aim und Hold liegen nie im selben Tick.

Ohne bestätigten Hammer innerhalb von 25 Sekunden oder nach 12 wirkungslosen Teleports endet der Run mit `boss_combat_no_progress`. Distanz, Cursorpunkt und Hold sind live bestätigt: 1/3/5 Tiles, Sprite-Aim, LMB ohne Shift.

## Datenmodell

- `config.RequiredSkillConfig.Slot`: erzwungener Profilslot `left` oder `right`
- `config.OptionalSkillPairConfig`: genau zwei gemeinsam optionale Skills
- `config.ProfileConfig.RequiresMercenary`: von der Trankpolicy unabhängiges Preflight-Gate
- `profile.RunStrategy`: registriert aktuell nur Mephisto und seine fünf Pflichtskills

Die Skill-IDs stammen aus dem lokal extrahierten `.tmp/d2r-excel/skills.txt`: Teleport 54, Blessed Hammer 112, Concentration 113, Holy Shield 117, Battle Orders 149, Battle Command 155 und TownPortal 359. Der eingebettete Katalog bleibt auf stabile `skill`-/`*Id`-Schlüssel zurückführbar. Für Hammerdin aktiviert die Runtime die vorhandene Weapon-Set-Evidenz über dieselben CASC-IDs 149 und 155.

## Operator / CLI

OperatorSettings Schema 3 bleibt die einzige Binding-Autorität. Der Core projiziert Pflichtskills samt Slot, das optionale CTA-Paar, Söldnerpflicht, Binding-Readiness und Gründe über die Charakter-Setup-API; Webtypen und UI übernehmen diese Werte unverändert.

Das Charaktersetup zeigt den Hammerdin-Profilnamen, LMB/RMB-Rollen und die Core-Readiness. Der optionale Call-to-Arms-Block erklärt ausdrücklich Waffenset II, dass ein Holy-Shield-Schild dort liegen darf, und dass der Bot Set und Skill prüft, nicht Runenwort oder Söldnerausrüstung. Das Badge bleibt „Waffenset II · beide oder keine“. Die UI verspricht keine Item- oder Insight-Erkennung und weist nur darauf hin, dass ein lebender Söldner erforderlich ist, dessen Ausrüstung nicht geprüft wird. Native Select-Felder bleiben per Tastatur bedienbar; das Layout wurde bei 1280×720 und 390×844 ohne horizontalen Überlauf geprüft.

Die isolierte manuelle Abnahme verwendet den eingefrorenen Charakter-Loadout aus dem installierten DataRoot. Der Charakter muss außerhalb der Stadt stehen, zum Beispiel am Zielwegpunkt oder im Kampfgebiet:

```powershell
$dataRoot = Join-Path $env:LOCALAPPDATA 'D2ROfflineFarmingBot'
.\d2rbot.exe --data-root $dataRoot --town-test hammerdin-prebuff:cta
.\d2rbot.exe --data-root $dataRoot --town-test hammerdin-prebuff:no-cta
```

Der CTA-Test verlangt beide optionalen F-Tasten; der No-CTA-Test verlangt beide leer.

Die bestandene Combat-Abnahme 22.7 verwendet denselben eingefrorenen Loadout und die veröffentlichte Mephisto-Route, ohne Combat-Test-Bypass:

```powershell
$dataRoot = Join-Path $env:LOCALAPPDATA 'D2ROfflineFarmingBot'
.\d2rbot.exe --data-root $dataRoot --run mephisto
```

Live-Beleg 16.08.2026, Charakter `MrHammer`: `d2rbot-20260816-184632.log` und `d2rbot-20260816-185534.log` schließen beide mit `outcome=success`. Hämmer entstehen durch gehaltenes LMB auf dem Boss-Sprite, Teleport und Konzentration jeweils bestätigt, LMB geht bei Distanzverlust oder Kill hoch. Nach dem Loslassen wartet ein Snapshot, bevor ein Teleport folgen darf. Endlicher Abbruch bei ausbleibendem Fortschritt bleibt codegedeckt.

## Abhängigkeiten

- lokaler CASC-Skillkatalog aus `.tmp/d2r-excel/skills.txt`
- vollständige `SkillsKnown`-Evidenz und `ActiveWeaponSet` aus dem bestehenden Snapshot-/World-Modell
- gemeinsame Strategy Registry, slotbewusster Skill-Selector und Mercenary-Preflight

## Verwandte Features

- [Character Loadouts](character-loadouts.md)
- [Mercenary Support](mercenary-support.md)
- [Input Controller](input-controller.md)
- [Mephisto-Run](mephisto-run.md)

---
*Zuletzt aktualisiert: 2026-08-16*
