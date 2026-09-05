# Paladin „Hammerdin“

## Überblick

`paladin_hammerdin` ist das zweite freigegebene Kampfprofil. Es registriert Countess, Mephisto, Nihlathak, Summoner, Kuh-Level und Lower Kurast. Countess, Mephisto und Nihlathak teilen denselben Standardangriff. Summoner und der Cow-Sweep nutzen denselben Blessed-Hammer-Hold zusätzlich während der aufgezeichneten Route (Combat-to-go). Lower Kurast besitzt kein globales Route-Clear, kann aber nach einem bestätigten Monster-Hover einmal lokal um eine blockierte Hütten-Truhe kämpfen. Das Profil friert die Skill-, Slot-, CTA- und Söldnerverträge ein. Phase 22.6 stellt den isolierten CTA-/Holy-Shield-Prebuff bereit und verdrahtet denselben Ablauf produktiv vor dem Abspielen der aufgezeichneten Route. Battle Command und Battle Orders sind CASC-seitig nicht in der Stadt wirkbar. Der produktive Blessed-Hammer-Kampf ist verdrahtet; das manuelle Gate 22.7 ist am 16.08.2026 für Mephisto bestanden.

## Ort im Code

- **Paket:** `internal/profile/hammerdin/`
- **Einstieg:** `hammerdin.NewBossFactory`, `hammerdin.NewLowerKurastFactory`
- **Wichtige Dateien:** `internal/config/profile.go`, `internal/app/combat.go`, `internal/app/combat_strategy_registry.go`, `internal/app/hammerdin_prebuff.go`, `internal/app/hammerdin_town_ready.go`, `internal/tasks/pipeline_boss.go`
- **Config:** `combat_profiles.paladin_hammerdin` in `configs/config.example.yaml`

## Funktionalität

### Profil und Bossläufe

Das Profil ist für die Klasse `paladin` im Charakter-Setup freigegeben. Die Runtime-Registry enthält die Paare `paladin_hammerdin × countess|cows|lower-kurast|mephisto|nihlathak|summoner`. Die fünf Pflichtskills sind Teleport, Stadtportal, Gesegneter Hammer, Konzentration und Heiliger Schild.

### Skill-Slots und CTA

Gesegneter Hammer ist trotz seiner beidseitigen CASC-Fähigkeit fest an LMB gebunden. Teleport, Konzentration, Heiliger Schild und die bestehende Stadtportalaktion verwenden RMB. CTA ist kein Pflichtskill und besitzt keinen Aktivierungsschalter: Battle Command und Battle Orders bilden genau ein gemeinsam optionales RMB-Paar. Beide Tasten dürfen fehlen oder müssen gemeinsam gültig belegt sein; jede der beiden Teilbelegungen wird mit der Core-Meldung „Für Call to Arms müssen Battle Command und Battle Orders beide belegt sein.“ abgelehnt.

### Söldner

Ein vorhandener und lebender Söldner ist eine Buildvoraussetzung. Dieses Preflight-Gate bleibt auch dann aktiv, wenn die optionale Söldner-Trankpolicy deaktiviert wurde. Der Bot prüft weder Insight noch Ausrüstung, Aura, Level oder Resistenzen; dafür bleibt der Operator verantwortlich.

### CTA- und Holy-Shield-Prebuff

Der CTA-Zweig verlangt zu Beginn ein frisch bestätigtes Primärset. Er sendet `W` genau einmal, wartet auf eine neuere Snapshot-Generation und bestätigt das Sekundärset. Danach werden Battle Command, Battle Orders, Battle Command und Holy Shield jeweils erst nach bestätigter `RightSkillID` an der neutralen Clientmitte gewirkt. Nach Holy Shield wartet der Bot 1000 ms, bevor ein zweites einzelnes `W` das Primärset wiederherstellt; ein `W` während der Animation wird vom Spiel ignoriert. Das Primärset muss anschließend anhand einer neueren Generation bestätigt werden. Zuletzt werden Gesegneter Hammer auf LMB und Konzentration auf RMB bestätigt.

Der gültige leer/leer-Zweig sendet weder `W` noch Battle-Command-/Battle-Orders-Input. Holy Shield wird einmal im Primärset gewirkt; anschließend werden dieselben Kampfslots wiederhergestellt. Ein unlesbares Waffenset, ein nicht bestätigter Wechsel oder eine nicht bestätigte Skillauswahl beendet den Ablauf ohne Toggle-Schleife.

Die produktive Queue ruft dieselbe Maschine nach der Wegpunkt-Ankunft auf, unmittelbar bevor die aufgezeichnete Route startet, und danach weiter in jedem `play_route`-Tick. In der Stadt wird nicht gecastet: Battle Command und Battle Orders haben `InTown=false`. CTA vollständig und der 150-Sekunden-Anker noch nicht fällig bedeutet: der Hook endet ohne Input. CTA vollständig und Anker leer oder fällig bedeutet: volle Sequenz inklusive Holy Shield im zweiten Set; ein gehaltener Hammer-Angriff wird vorher gelöst, danach wartet der Bot 250 ms bevor `W` das Sekundärset anwählt. Ohne CTA wird Holy Shield einmal pro Spielgeneration im Primärset gewirkt. Der 150-Sekunden-Anker wird erst nach dem tatsächlich autorisierten zweiten Battle-Command-Cast gesetzt. Ein Game- oder Generations-Reset sowie Menu-/Loading-Phasen löschen Timer, Pending Selection und Weapon-Swap-Zustand.

### Standardangriff

Nach dem Wegpunkt wartet die Pipeline auf eine gesetzte Ankunft im Kampfgebiet (InGame, drei frische Snapshots, drei Sekunden Settle), bevor CTA/Holy Shield startet. Sticky `WaypointOpen` blockiert diese Ankunft nicht. Eine nur gemeldete Ziel-Area während des Ladebildschirms reicht nicht.

Nach dem letzten aufgezeichneten Routenpunkt pinnt die gemeinsame Boss-Pipeline den Boss über NPC-ID und UnitID. Es gibt keine Bone-Prison- oder sonstigen Encounter-Hooks: `boss_engage` bleibt leer, der Standardangriff beginnt unmittelbar. Countess und Mephisto schließen die leeren Registry-Hooks in denselben Ticks ab; Nihlathak und Summoner haben keine Engage-Sequenz.

Jeder Kampftick prüft Spieler- gegen Bossposition. Liegt die Distanz über 3 Tiles, teleportiert der Bot auf 1 Tile und prüft erneut. Ist die Distanz in Reichweite, zielt der Cursor auf den sichtbaren Boss-Körper. Eine andere Hover-ID auf demselben Sprite (überlagerndes Monster) ist erlaubt; die Mausposition hat Vorrang vor der Monster-ID. Anschließend bleibt `LMB` auf diesem Punkt gedrückt, ohne Shift. Alle drei World-Snapshots folgt eine Distanzprüfung. Über 5 Tiles oder bei totem Boss geht LMB hoch; ein Teleport folgt erst im nächsten Snapshot, damit ein sterbender Boss keinen Lauf vom Leichnam weg auslöst. Lebt das gepinnte Ziel nach zwei Sekunden stationärem Hold noch, teleportiert der Bot stattdessen zum nächsten anderen lebenden Monster innerhalb von 18 Tiles zum gepinnten Ziel, hält aber die alte UnitID als Angriffsziel fest. Dadurch entsteht eine neue Hammerspirale auf einer tatsächlich im Pack belegten Passage, während das alte Ziel zum Paladin nachziehen kann. Bereits verwendete Ausweichziele bleiben ausgeschlossen; fehlt ein nahes anderes Monster, läuft der Hold bis zum endlichen Boss-Watchdog weiter. Nach bestätigter Bewegung wird vor jeder normalen Distanz-Rückannäherung zuerst der Hover und Hammer-Hold auf dem alten Ziel versucht. Eine bestätigte Positionsänderung beendet das Teleport-Settle bereits im ersten frischen Snapshot, während 500 ms nur noch die Frist für eine blockierte Landung bilden. Nach einem Teleport muss Konzentration erneut auf RMB bestätigt sein, bevor der Hold startet. Die Konzentrationsauswahl verschiebt den Cursor bereits im selben Tick zum Ziel; LMB folgt erst nach der Memory-Bestätigung von Aura und Hover.

Nihlathak und Summoner verwenden genau diesen Pfad, nicht den Necro-Projektionsansatz und nicht Bone Spear. Nach dem Kill folgt direkt die Loot-Repositionierung; `clear_nearby_hostiles` entfällt, weil Blessed Hammer als Flächenangriff die meisten Gegner bereits wegräumt.

### Summoner-Route (Combat-to-go)

Summoner registriert denselben Route-Clear wie der Necromancer: Threat-Hold, Hostile-Allowlist, Mana-Reserve, endlicher No-Progress-Watchdog und Pickit unterwegs bleiben unverändert. Statt Amplify Damage und Bone Spear teleportiert der Hammerdin vor dem ersten Angriff bei mehr als 3 Tiles Distanz auf 1 Tile. Während eines Mana-Holds ist dieser Teleport gesperrt. Anschließend zielt der Cursor auf den sichtbaren Körper; befindet sich dort irgendein lebendes Monster, startet der Blessed-Hammer-LMB-Hold auf genau dieser UnitID. Aim-only-Ticks bleiben noch nicht gepinnt, damit ein überlagerndes Monster unter dem Cursor das eigentliche Angriffsziel werden kann.

Nach Beginn des Holds bleibt die bestätigte UnitID bis zu ihrem Tod gepinnt, auch wenn sie den ursprünglichen Threat-Korridor verlässt. Wie im Mephisto-Pfad wird die Distanz nur alle drei frischen Snapshots erneut bewertet; erst über 5 Tiles geht LMB hoch und die begrenzte Teleport-Annäherung beginnt erneut. Bleibt das Ziel zwei Sekunden am Leben, wählt Route-Clear das nächste andere lebende Allowlist-Monster innerhalb von 18 Tiles zum gepinnten Ziel als Teleportziel, behält aber die alte UnitID für Hover und Hammer-Hold. Gibt es kein nahes Ausweichmonster, teleportiert der Bot höchstens 8 Tiles in Richtung des bereits validierten nächsten Routenpunkts; ein weit entferntes Monster wird dafür nicht gewählt. Ein bestätigter Teleport wird ohne feste 500-ms-Wartezeit übernommen; nur eine ausbleibende Positionsänderung wartet bis zur Terrain-Frist. Vor einer normalen Rückannäherung muss der Bot von der neuen Position mindestens einen Angriff auf das alte Ziel versuchen. Ein bereits verwendetes Ausweichmonster wird beim nächsten Versuch übersprungen. Erst nach bestätigtem Tod wird das reguläre Angriffsziel freigegeben. Der Summoner-Boss am Routenende nutzt denselben Standardangriff; es gibt kein gesondertes Encounter-Verhalten.

Ohne weiteres Ausweichmonster bleibt der Hold bestehen, damit ein langsamer Boss weiter Schaden nehmen kann. Spätestens nach 25 Sekunden ohne bestätigten Fortschritt oder nach 12 wirkungslosen Teleports endet der Bosskampf mit `boss_combat_no_progress`. Frische und unveränderte Default-Retry-Listen behandeln diesen Grund wie andere kontrollierte Recoveries: Town Portal, Save & Exit, derselbe Queue-Index. Das bloße Starten oder Fortsetzen eines LMB-Holds zählt nicht als Fortschritt. Distanz, Cursorpunkt und Hold sind live bestätigt: 1/3/5 Tiles, Sprite-Aim, LMB ohne Shift.

### Lower-Kurast-Objektblocker

Lower Kurast registriert den Blessed-Hammer-Route-Clear nur als interne Recovery für `chest_sweep`; `RequiresRouteClear()` bleibt `false`. Erst wenn eine laufende Objekt- oder Pickup-Hover-Suche einen Monster-Hover beobachtet und trotzdem erschöpft, hält die Route und greift lebende Gegner im 12-Kachel-Kreis um die gepinnte Truhe, das Gestell oder das Item an. Sitzt der Hover auf einem Söldner oder einer Leiche, kämpft der Clear gegen den nächsten lebenden Gegner in dem Kreis. Der Clear endet nach drei monsterfreien Snapshots oder spätestens nach zwölf Aktionen, sechs Sekunden beziehungsweise drei Sekunden ohne gesendete Aktion. Anschließend löst der Executor den LMB-Hold; Objekt-Suche und Pickup erhalten jeweils genau einen neuen Versuch. Ohne lebenden Gegner im Kreis startet kein Kampf.

### Kuh-Level

Kuh-Level bleibt ein gemeinsamer Zwei-Rollen-Run (`leg_acquisition` / `cow_sweep`). Hammerdin registriert dieselbe Cow-Pipeline wie der Necromancer: Preflight, Town-Ready, Stony-Field-Wegpunkt, combatfreie Wirt-Route, Bein, Tome, Cube-Rezept und Sweep. Der Preflight übernimmt Klasse und Pflichtskills aus der Hammerdin-Cow-Strategie; Amplify Damage, Corpse Explosion, Bone Spear und Bone Armor werden nicht verlangt. CTA und Holy Shield laufen über `field_ready` nach gesetzter Ankunft in Stony Field bzw. Moo Moo Farm, nicht im Town-Ready-Schritt. Derselbe Hook bleibt während des Cow-Sweeps aktiv, damit der 150-Sekunden-CTA-Anker die Buffs erneuert, bevor Battle Orders auslaufen.

Der Sweep bindet denselben Blessed-Hammer-Route-Clear wie Summoner und schaltet den Necromancer-CE-Hold nicht ein. Annäherung bleibt die profildefinierte 1-Tile-Distanz. Beide Routenrollen müssen für den Paladin-Charakter aufgenommen und veröffentlicht werden; Necromancer-Aufnahmen sind nicht übertragbar.

## Datenmodell

- `config.RequiredSkillConfig.Slot`: erzwungener Profilslot `left` oder `right`
- `config.OptionalSkillPairConfig`: genau zwei gemeinsam optionale Skills
- `config.ProfileConfig.RequiresMercenary`: von der Trankpolicy unabhängiges Preflight-Gate
- `profile.RunStrategy`: registriert Countess, Mephisto, Nihlathak, Summoner, Lower Kurast und Kuh-Level mit denselben fünf Pflichtskills; Summoner und Cow-Sweep binden Travel-RouteClear, Lower Kurast nur die lokale Objektblocker-Recovery

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

Live-Beleg 16.08.2026, Charakter `MrHammer`: `d2rbot-20260816-184632.log` und `d2rbot-20260816-185534.log` schließen beide mit `outcome=success`. Hämmer entstehen durch gehaltenes LMB auf dem Boss-Sprite, Teleport und Konzentration jeweils bestätigt, LMB geht bei Distanzverlust oder Kill hoch. Nach dem Loslassen wartet ein Snapshot, bevor ein Teleport folgen darf. Der Zwei-Sekunden-Ausweichfallback über ein anderes Monster und der endliche Abbruch bei ausbleibendem Fortschritt sind codegedeckt.

## Abhängigkeiten

- lokaler CASC-Skillkatalog aus `.tmp/d2r-excel/skills.txt`
- vollständige `SkillsKnown`-Evidenz und `ActiveWeaponSet` aus dem bestehenden Snapshot-/World-Modell
- gemeinsame Strategy Registry, slotbewusster Skill-Selector und Mercenary-Preflight

## Verwandte Features

- [Character Loadouts](character-loadouts.md)
- [Mercenary Support](mercenary-support.md)
- [Input Controller](input-controller.md)
- [Mephisto-Run](mephisto-run.md)
- [Countess-Run](countess-run.md)
- [Summoner-Run](summoner-run.md)
- [Nihlathak-Run](nihlathak-run.md)
- [Cow Level / Moo Moo Farm](cow-level-run.md)
- [Lower-Kurast-Run](lower-kurast-run.md)
- [Route-Threat-Combat](route-threat-combat.md)

---
*Zuletzt aktualisiert: 2026-08-29*
