# Zauberin mit Blizzard

## Überblick

`sorceress_blizzard` ist das vollständig implementierte und freigegebene Standardprofil der Zauberin. Es verwendet die vorhandene Task-Pipeline, Skill-Bestätigung, Ressourcenpolicy und Routenwiedergabe. Unterstützt sind Gräfin, Mephisto, Beschwörer, Nihlathak, Lower Kurast und Kuh-Level.

## Ort im Code

- Paket: `internal/profile/sorceressblizzard/`
- Einstieg: `NewFactory` und `NewEncounterExecutor`
- Verdrahtung: `internal/app/combat_strategy_registry.go`, `internal/app/app.go`
- Angriffe: `internal/app/combat.go`, `internal/profile/executor.go`
- Config: `combat_profiles.sorceress_blizzard` in `configs/config.example.yaml`, Defaults in `internal/config/profile.go`

## Funktionalität

### Pflichtskills und Standardangriff

| Skill | Aufgabe | Maustaste |
|---|---|---|
| Blizzard | Standardangriff auf die aktuelle Monsterposition | Rechts |
| Eisstoß | Einzelzielangriff während Blizzards Cooldown | Rechts |
| Teleport | Routenwiedergabe und Kampfannäherung | Rechts |
| Statikfeld | Drei Casts gegen Mephisto vor Blizzard | Rechts |
| Stadtportal | Gemeinsame Rückkehr- und Recovery-Abläufe | Rechts |
| Eisrüstung | Prebuff beim Town-Ready-Schritt | Rechts |

Alle sechs Skills benötigen eine eigene gültige Tastenbelegung. Die vorhandene Skill-Auswahl wartet auf die Bestätigung im Memory, bevor sie klickt. Blizzard verwendet einzelne Rechtsklicks standardmäßig im Abstand von 1800 ms. Die Config akzeptiert keine kürzeren Intervalle, entsprechend `Blizzard.localdelay=45` Frames aus dem lokalen CASC-Extrakt. Während des Cooldowns greift Eisstoß das vom Task im aktuellen Snapshot gewählte lebende Ziel an. Jeder Tick bewertet dieses Ziel neu; eine Burst-Sequenz speichert kein möglicherweise inzwischen totes Monster. Auswahl und erster Zielversuch können im selben Tick erfolgen. Nach bestätigter Skillauswahl folgen einzelne Eisstoß-Klicks ohne zusätzliche künstliche Wartefrist, höchstens einer pro Tick. Die tatsächliche Zauberrate hängt weiterhin von Spielanimation, Ausrüstung und Polling ab. Sobald Blizzard bereit ist, erhält er Vorrang, auch vor einer noch unbestätigten Eisstoß-Auswahl.

Nur ein gesendeter Blizzard-Cast startet dessen Cooldown. Teleport und Skillauswahl starten ihn nicht erneut; sonst könnte das lokale Drei-Sekunden-Budget schon vor dem ersten Angriff verstreichen. Ein Route-Clear-Reset hebt das Angriffsintervall nicht auf.

Die gemeinsame Boss-Pipeline nähert sich bei mehr als 22 Tiles auf 15 Tiles an. Nihlathak verwendet die bestehende Annäherung bis zur spielbaren Zielprojektion. Route-Clear greift das vom Task autorisierte lebende Ziel über die vorhandene Hover- oder Projektionsprüfung an. Blizzard trifft dort eine Fläche; das Profil führt keine eigene Gruppen- oder Routenplanung ein.

### Zielwahl und Recovery bei Untätigkeit

Im regulären Route-Clear von Beschwörer und Cow-Sweep bevorzugt die gemeinsame Pipeline lebende, anvisierbare Gegner. Die Priorität der Bedrohungszonen bleibt erhalten; innerhalb einer Zone zählen der vorübergehende Zielausschluss nach Untätigkeit, bestätigter Hover und Entfernung. Die Auswahl verwendet jeden frischen World-Snapshot. Nicht anvisierbare Gegner bleiben in der Bedrohungs- und Coverage-Bewertung und gelten nicht als erledigt. Eine gültige Bildschirmprojektion beweist keine freie Schussbahn.

Nach **zwei Sekunden ohne gesendeten Angriff** bei vorhandenen Gegnern beginnt eine begrenzte Recovery:

1. Zielwahl und ausstehende Angriffsauswahl zurücksetzen. Ein anderes anvisierbares Ziel derselben Priorität erhält Vorrang.
2. Nach weiteren zwei Sekunden ohne Angriff die vorhandene sichere Annäherung versuchen. Im Cow-Sweep bleiben Projektions- und Landeplatzprüfung verbindlich; beim Beschwörer bleibt die Bewegung an die freigegebene Routenkante gebunden.
3. Bleiben Angriffe weiterhin aus, endet die Recovery kontrolliert mit `route_clear_no_progress`.

Söldner-Kills, Zielwechsel, Hover-Versuche und Skill-Anforderungen setzen diesen eigenen Aktivitätstimer nicht zurück. Ein gesendeter Angriff beendet die Recovery; eine bestätigte Annäherung startet die Wartefrist neu, erhält aber die bereits verbrauchten Recovery-Stufen. Mana-Erholung und Profilwartung pausieren die Prüfung. Die vorhandenen Mana- und Gesamtfristen bleiben bestehen. Fehlender Schaden allein gilt nicht als Untätigkeit; Kälteimmune können weiterhin vom Söldner bekämpft werden.

Zusätzlich prüft ein eigener Zwei-Sekunden-Wächter die Clear-Wirkung nach gesendeten Angriffen. Klicks, Zielwechsel, wechselnde Bedrohungszonen und größere Snapshot-Abdeckung gelten dabei nicht als beseitigter Gegner. Das Verschwinden des vorherigen lebenden Ziels oder eine verringerte erfasste Gegnerzahl startet diese Frist neu. Das World Model liefert keine Monster-Lebenspunkte und keine freie Schussbahn; der Wächter erkennt daher ausbleibenden Clear-Fortschritt, ohne einen Treffer oder eine konkrete Mauer zu behaupten.

Bei ausbleibender Wirkung werden Angriffsauswahl und Zielsuche zurückgesetzt. Die bestehende Teleportsteuerung versucht sofort eine Verschiebung von höchstens vier Tiles in Richtung des nächsten aufgezeichneten Routenpunkts. Die Route bleibt angehalten. Im Cow-Level gelten weiterhin Coverage- und Gegnerabstandsprüfung für den Landeplatz. Auswahl, Cast und Positionsbestätigung behalten ihre bestehenden Fristen. Anschließend erhält ein anderes anvisierbares lebendes Ziel Vorrang. Nach höchstens zwei Repositionierungsstufen und einer weiteren Zwei-Sekunden-Frist ohne Clear-Fortschritt endet der Task mit `route_clear_no_progress`; abgelehnte Landeplätze verbrauchen ebenfalls eine Stufe. Bestätigte Bewegung beginnt die Wartefrist neu, setzt aber das Stufenbudget nicht zurück. Mana-Erholung und Profilwartung pausieren auch diesen Wächter.

Die gemeinsame Annäherung besitzt den Ablauf bereits während der Skillauswahl. Erst danach folgen Teleport-Klick und Bestätigung der Positionsänderung in einem neueren Snapshot. Eine Auswahl darf höchstens zwei Sekunden warten. Stirbt das Ziel vor dem Klick, wird die Annäherung verworfen und neu bewertet. Während einer laufenden Annäherung kann die Angriffsrotation den rechten Skill nicht überschreiben. Zauberin-Teleports verwenden das bestehende `pathing.move_interval_ms` statt Blizzards 1800-ms-Angriffsintervall. Skillauswahl-Wartezyklen gelten nicht als neue Bewegung oder neuer Angriff.

Die Telemetrie unterscheidet `idle_retarget`, `idle_reposition` und `idle_exhausted` als `progress_kind` von `route_clear_progress`. Diese Diagnoseereignisse setzen den objektiven Fortschrittswächter nicht zurück. `route_clear_action` meldet weiterhin ausschließlich gesendete Eingaben; eine bestätigte Bewegung wird separat als `approach` erfasst. Ein Angriffsklick ist kein Nachweis eines Treffers oder einer tatsächlich gestarteten Spielanimation.

`cast_no_progress_reposition` und `cast_no_progress_exhausted` kennzeichnen den neuen Wirkungswächter. Ein gesendeter Teleport erscheint als `cast_no_progress_teleport`. Bei der Zauberin wird die Zielprojektion bereits vor der Blizzard-/Eisstoß-Auswahl geprüft. Ein Ziel außerhalb des sichtbaren Bereichs löst zunächst die bestehende Annäherung aus und verbraucht keinen Blizzard-Cooldown.

Boss-Zielbindung, Nihlathaks Anker und die begrenzten lokalen Clears in Lower Kurast und bei Objektinteraktionen behalten ihre eigenen Verträge. Der neue Zwei-Sekunden-Wächter gehört zum regulären Route-Clear.

### Statikfeld gegen Aktbosse

Mephisto ist der einzige Aktboss unter den aktuellen Routen. Der Profil-Executor prüft seine gepinnte UnitID gegen die lebenden Monster im aktuellen World State. Gräfin, Beschwörer, Nihlathak und Kühe erhalten keinen Statikfeld-Opener.

Außerhalb von vier Tiles teleportiert der bestehende Kampfadapter auf drei Tiles an Mephisto heran. Vor dem Cast müssen ein neuerer Snapshot und 500 ms Settle vorliegen. Der World State enthält keine effektiven Skill-Level; die konservative Reichweite stützt sich deshalb auf `Static Field.aurarangecalc=ln12`, `Param1=5`, `Param2=1` aus `skills.txt`.

Anschließend führt der gemeinsame Hook-Executor drei selbstzentrierte Statikfeld-Casts mit jeweils 500 ms Settle aus. Zwei Casts sind ebenfalls konfigurierbar. Mephistos zweiter Encounter-Hook führt die Sequenz nicht erneut aus. Ein unlesbarer State wartet ohne Input; Zielverlust, Inputfehler oder 20 Sekunden ohne Abschluss beenden den Opener. Der bestehende Task-Abbruch bleibt wirksam. Reset löscht Annäherung, Fristen und Cast-Zustand.

### Eisrüstung in der Stadt

Eisrüstung verwendet denselben `town_ready`-Hook wie Knochenrüstung beim Totenbeschwörer. Der Default wartet 5000 ms, bestätigt den rechten Skill, wirkt auf die neutrale Clientmitte und wartet 1500 ms. `once_per_game: false` erlaubt den erneuten Prebuff beim nächsten Town-Ready-Ablauf. Die bestehende Queue-Fortsetzung kann die anfängliche Wartezeit überspringen. Während einer Route gibt es keine automatische Eisrüstungs-Erneuerung.

### Routen und lokale Kämpfe

| Route | Verhalten |
|---|---|
| Gräfin | Kein Kampf unterwegs. Boss-Akquise mit bestehendem Superunique-Gate, Blizzard, drei Bestätigungsticks für den Kill, anschließend Loot ohne weiteren Gebiets-Clear. |
| Mephisto | Kein Kampf unterwegs. Einmal drei Statikfeld-Casts, danach Blizzard auf den gepinnten Boss; drei Bestätigungsticks für den Kill, anschließend Loot. |
| Beschwörer | Blizzard gegen relevante Routenblocker und den Boss. Danach höchstens 20 Angriffe auf erlaubte Restgegner innerhalb von 18 Tiles; drei freie Snapshots beenden den Clear früher. |
| Nihlathak | Kampf vom aufgezeichneten Anker, höchstens eine Annäherung bei unspielbarer Zielprojektion. Nach dem Bosskill Cleanup innerhalb von 30 Tiles mit höchstens 40 Aktionen. |
| Lower Kurast | Gemeinsamer Truhen-Sweep. Nur ein bestätigter Objektblocker löst den lokalen Clear aus; danach genau ein erneuter Interaktionsversuch. |
| Kuh-Level | Gemeinsame Bein-/Buch-/Würfel-Vorbereitung. Der Cow-Sweep hält auch für nahe Gegner außerhalb des unmittelbaren Laufkorridors und prüft die sichere Endposition. |

Nur Beschwörer und Cow-Sweep erlauben regulären Kampf während der Routenwiedergabe. Kuh-Level nutzt keinen Fluch und keine Kadaverexplosion. Mana-Reserve, Threat-Holds, Beuteaufnahme, Town-Dienste und endliche Recovery-Budgets bleiben beim jeweiligen bestehenden Task.

Ein Clear bedeutet keine vollständige Kartenräumung. Der Beschwörer räumt die Gegner, die den nächsten Routenschritt bedrohen. Nach vollständig abgespielter Route darf die bestehende Sonderregel einen bereits verschwundenen Beschwörer als erledigt behandeln; die isolierte Bossphase bleibt strikt. Der Cow-Sweep berücksichtigt zusätzlich die erlaubten Gegner im lokalen Angriffsradius von 30 Tiles. Unvollständige Monsterabdeckung gibt die Bewegung nicht frei. Die Wiederaufnahme verlangt drei vollständige freie Snapshots und einen weiteren frischen Tick. Fehlender Fortschritt bleibt durch die gemeinsamen Recovery-Budgets begrenzt.

Nihlathak bleibt über seine UnitID gepinnt. Ist er vom aufgezeichneten Anker erreichbar, bleibt die Zauberin dort, auch jenseits der allgemeinen Distanzschwelle. Sonst folgt genau ein Teleport entlang der bestehenden Linie, danach 700 ms Settle und ein neuer Snapshot. Bleibt er unzielbar, greift `boss_combat_unprojectable` mit dem vorhandenen kontrollierten Rückweg. Ein überlagerndes Monster darf erst nach einem nachgewiesenen Zielversuch auf den Boss und einem neueren Snapshot als Klickfläche dienen. Der Cleanup nach dem Kill überspringt unprojizierbare Ziele und endet auch nach drei freien beziehungsweise nur noch übersprungenen Snapshots oder drei Sekunden ohne gesendete Aktion. Er bewegt die Zauberin nicht.

Lower Kurast und die lokale Portal-Recovery verwenden denselben Clear mit zwölf Tiles Radius um das blockierte Objekt, höchstens zwölf Aktionen, sechs Sekunden Gesamtdauer und drei Sekunden ohne gesendete Aktion. Nach Ende des Budgets folgt der begrenzte Wiederholungsversuch auch bei überlebenden Gegnern. Es entsteht keine dauerhafte Clear-Schleife.

### Automatisierte Prüfung und Spielabnahme

`internal/tasks/sorceress_recovery_test.go` prüft den Zwei-Sekunden-Trigger trotz Söldner-Fortschritt, anvisierbare und sterbende Ziele, die exklusive Annäherung, Mana-Holds und endliche Recovery. `internal/tasks/sorceress_blizzard_test.go` prüft die tatsächlichen Task-Übergänge mit dem gemeinsamen Profil-Executor und simulierten Kampfaktionen: Boss-Akquise, Statikfeld nur bei Mephisto, Kill-Bestätigung, Cleanup-Auswahl und -Budgets, Nihlathaks einzelne Annäherung, Lower-Kurast-Blocker sowie Beschwörer-/Cow-Holds bei Gegnern und unvollständiger Abdeckung. Die App-Tests prüfen zusätzlich den echten Kampfadapter, einschließlich Blizzard nach Teleport innerhalb des lokalen Clear-Budgets, Eisstoß-Bursts mit wechselnden Zielen und Blizzards Vorrang nach 1800 ms. Profil-, Config- und UI-Tests prüfen Hooks, CASC-Annahmen, Bindings und Setup.

Der Operator hat die Zauberin am 1. Oktober 2026 als fertig implementiert bestätigt. Das Profil ist für alle sechs Routen freigegeben. Ausrüstung, Söldner und persönliche Routenaufzeichnungen bleiben Voraussetzungen des jeweiligen Charakters.

## Datenmodell

Der Profilvertrag verlangt Klasse `sorceress`, Standardangriff `blizzard`, die sechs rechten Pflichtskills und einen lebenden Söldner. Setup und Bindings werden über die bestehenden Charakter- und OperatorSettings-Daten gespeichert. Es gibt kein neues Config-Schema und keine zweite Binding-Datei.

Die Skill-IDs stammen aus dem generierten CASC-Katalog. `testdata/skills.tsv` im Profilpaket hält die verwendeten Spalten aus `.tmp/d2r-excel/skills.txt` anhand der stabilen `skill`-Zeilenschlüssel fest. Tests prüfen IDs, Slot-Fähigkeiten, Town-Cast und die Annahmen zu Verzögerung und Reichweite.

## Operator / CLI

Im Charaktersetup die Zauberin bestätigen, alle sechs Skills belegen und die benötigten Routenaufzeichnungen für diesen Charakter veröffentlichen. Für den Kuh-Level werden beide Rollen `leg_acquisition` und `cow_sweep` benötigt. Danach startet die bestehende Queue oder `--run cows` mit dem eingefrorenen Charakter-Loadout.

Die Electron-Oberfläche verwendet denselben Setup-Wizard, Binding-Editor und Charakterbereich wie die anderen Klassen. Profilname und Klassenname sind auf Deutsch und Englisch verfügbar. Das vorhandene Blizzard-Medaillon erscheint in den Einstellungen auch bei allein bekannter Profil-ID. Skillnamen stammen über den bestehenden Generator aus den lokalen `skills.txt`, `skilldesc.txt` und `skills.json`.

Ein lebender Söldner ist Voraussetzung. Kälteimmune Gegner bleiben Aufgabe der ausgerüsteten Sunder Charm und des Söldners. Der Bot prüft weder Charm noch Ausrüstung oder aufgebrochene Immunität. Beide Angriffe verursachen Kälteschaden. Ohne ausreichenden Schaden können die bestehenden Kampf- und Recovery-Fristen auslaufen.

Stop/Pause und Input-Logging verwenden die vorhandenen zentralen Mechanismen. Bei einer falschen Entscheidungsreihenfolge den Ablauf mit `--runtime-trace-capture` aufzeichnen.

## Abhängigkeiten

Lokale CASC-Extrakte und generierter Skill-/Monsterkatalog, World Model, gemeinsame Profile- und Task-Executor sowie bestätigter Windows-Input. Es gibt keinen Schreibzugriff auf Spiel- oder Savegame-Dateien.

## Verwandte Features

- [Charakter- und Encounter-Profile](character-encounter-profiles.md)
- [Charakter-Setup](character-setup.md)
- [Kuh-Level](cow-level-run.md)
- [Söldnerunterstützung](mercenary-support.md)

---
*Zuletzt aktualisiert: 2026-10-01*
