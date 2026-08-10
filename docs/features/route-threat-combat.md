# Route-Threat-Combat

## Überblick

Route-Threat-Combat schützt das gebundene Summoner- und Cow-Sweep-Playback vor bekannten Gegnern im unmittelbaren Spielerumfeld, im vorausliegenden Korridor und am tatsächlichen nächsten Landepunkt. Die Route bleibt während eines Clears unverändert stehen; das aktive Profil greift stationär an. Cow ergänzt hinter demselben Controller ausschließlich eine direkte Leichen-/CE-Strategie. Das Feature gilt ausschließlich für Offline-/Singleplayer-Runs.

## Ort im Code

- **Pakete:** `internal/tasks`, `internal/profile`, `internal/pathing`, `internal/memory`, `internal/world`
- **Einstieg:** `tasks.runPipeline` im Schritt `play_bound_route`
- **Wichtige Dateien:** `route_threat_assessment.go`, `route_threat_controller.go`, `route_threat_telemetry.go`, `route_segment_player.go`, `executor.go`
- **Config:** `runs.definitions.<run>.route_combat` in `configs/config.example.yaml`

## Funktionalität

### Threat-Geometrie und Coverage

Pro frischem `world.State.At` entsteht höchstens ein O(n)-Assessment ohne zusätzlichen Memory-Read, Sortierung oder temporäre Monsterliste. Die Priorität lautet Immediate, Landing, Corridor. Summoner akzeptiert ausschließlich Specter/Ghost (NPC 40), Hell Clan (56) und Ghoul Lord (131), Cow ausschließlich Hell Bovine (391) und Cow King (735) aus der immutable `RunDefinition`.

Memory hält bis zu 512 nicht priorisierte lebende Monster; Priority-Einheiten bleiben zusätzlich erhalten. `EligibleMonsterCount`, `MonstersTruncated` und `MonsterCoverageRadiusTiles` werden bis `world.State` projiziert. Truncation allein blockiert nicht. Nur unvollständige lokale Coverage aktiviert `density_relief`.

### Hold und stationärer Clear

Ein Blocker führt vor `Route.Tick` zu `Route.Hold`. Hold bewahrt Segment, Transition und Recovery, übernimmt zunächst bereits innerhalb der Toleranz bestätigte Punkte und verlängert die aktive Deadline nur um deduplizierte echte Hold-Zeit. Hat eine ausdrücklich autorisierte Combat- oder Loot-Bewegung den Spieler auf eine spätere Routenpassage versetzt, stimmt Hold das Playback monoton auf die erste passende spätere Kante innerhalb des unveränderten Driftkorridors ab. Normales Playback bleibt strikt sequenziell; die Vorwärtsabstimmung darf weder die Driftgrenze erweitern noch über die früheste passende Kante einer Schleife springen. `RouteClearExecutor` besitzt ausschließlich Combat-Aktionen. Für `necro_bone_spear` ist genau die Strategie `single_target` registriert: Auf den ersten bestätigten Immediate-/Landing-/Corridor-Blocker einer Blockade wird einmal Amplify Damage gewirkt, danach folgt Bone Spear. `density_relief` überspringt den Curse-Opener.

Die Standardbindung für Amplify Damage liegt im Charakter-Loadout (typisch F1). Der Opener verwendet dieselbe schnelle Ziel-, Hover- und Rechtsklickoberfläche wie Bone Spear und erzeugt weder einen zusätzlichen Memory-Read noch eine eigene Hover-Schleife.

Die Monsterprojektion zielt nicht blind auf die Bodenkachel. Sie beginnt an einem um zwei isometrische Tiles nach oben verschobenen sichtbaren Körperanker und prüft anschließend pro frischem Poll deterministische Spiralpunkte darum. Das ist insbesondere für körperlose oder bewegte Sprites wie Specter relevant. Combat verwendet höchstens fünf Proben oder den kleineren konfigurierten Wert; das allgemeine Budget für statische NPC-, Portal-, Item- und UI-Ziele bleibt unverändert. Bestätigt Memory am gesetzten Cursorpunkt die priorisierte UnitID, darf der Rechtsklick unmittelbar folgen. Bleiben alle spielbaren Hover-Proben ohne Bestätigung, darf ausschließlich der bereits ausgewählte offensive Skill einmal auf die erneut frisch berechnete spielbare Körperprojektion gesendet werden. Dieser Cast trägt `targeting_mode=world_projected` und `hover_confirmed=false`; NPCs, Items, Portale und UI behalten ausnahmslos ihr Hover-Gate. Solange ein Allowlist-Blocker die Route tatsächlich hält, wird außerdem jedes andere aktuell unter dem Cursor Memory-bestätigte lebende Monster sofort angegriffen. Für diesen bereits gehovten Ersatz gelten bewusst keine zusätzliche Allowlist-, Geometrie-, Distanz- oder Pin-Prüfung: Ein sofortiger Angriff ist nützlicher als eine weitere Aim-Schleife auf das verdeckte ursprüngliche Ziel.

Die Threat-Erkennung bleibt von der aktuellen Bildschirmprojektion unabhängig, damit ein noch nicht sichtbares Pack am nächsten Landepunkt keinen Teleport erhält. Liegt ein anhand Tile-Distanz grundsätzlich angreifbarer Blocker richtungsabhängig außerhalb des spielbaren Clientbereichs, bewegt der Combat-Adapter die Maus nicht. Nach drei frischen Bestätigungen derselben UnitID nutzt die Pipeline stattdessen den bereits validierten nächsten Routenpunkt für genau einen Force-Move-Schritt mit der konfigurierten Town-Walk-Taste. Das ist kein Blind-Teleport und kein frei berechnetes Alternativziel: D2R übernimmt das lokale Lauf-Pathfinding, während `Route.Hold` Segment und Punkt unverändert lässt.

Nach mindestens 500 ms und einem neueren Memory-Snapshot zählt jede eindeutig positive Projektion der Spielerbewegung auf den tatsächlich gesendeten Vektor. Das kleine Float-Epsilon toleriert das ganzzahlige D2R-Koordinatengitter; reine Seitwärts-, Stillstands- oder Rückwärtsbewegung zählt nicht. Bestätigter Fortschritt setzt Range-Gate und Clear-Watchdog zurück. Ohne Richtungsfortschritt folgen höchstens drei lokale Annäherungsversuche. Deren Erschöpfung sendet keinen weiteren Movement-Input und beendet keinen Run: `route_threat_out_of_range` bleibt ein internes Recovery-Signal. Nur der gemeinsame zusammenhängende No-Progress-Watchdog darf anschließend `route_clear_no_progress` beziehungsweise `cow_combat_no_progress` terminalisieren.

Drei frische, lokal vollständige und threat-freie Snapshots schließen eine Blockade ab. Zwölf Sekunden ohne objektiven Ziel-, Count- oder Coverage-Fortschritt enden mit `route_clear_no_progress`. Angriffe, Zielwechsel und Trankinput setzen diesen Watchdog nicht zurück.

Ein einzelner gültiger In-Game-Snapshot kann seine Task-seitige RouteProgress-Projektion vorübergehend verlieren, etwa durch einen kurzfristig unvollständigen Identity-Read. Dieser read-only Aussetzer autorisiert keinen Input und ist noch keine interne Vertragsverletzung. Die Pipeline wartet bis zu zwei Sekunden auf neuere Snapshots; eine wieder verfügbare Projektion setzt die Grace sofort zurück. Nur über die Grace hinaus auf frischen Snapshots anhaltende Unverfügbarkeit endet fail-closed mit `route_threat_state_invalid`. Wiederholtes Verarbeiten desselben Snapshot-Zeitstempels verbraucht die Grace nicht.

### Cow-Hold

`cow_sweep` verwendet denselben Hold-, Ressourcen-, Coverage-, Recovery- und Telemetrie-Controller. Anders als Summoner hält Cow auch bei einer beliebigen lebenden allowlist-konformen Kuh innerhalb der Angriffsdistanz, damit der aktuelle lokale Bereich vor dem nächsten Routepunkt vollständig geräumt wird. Hinter dem Controller bindet ein kleiner Cow-Wrapper die aktive Gruppe einmal an den festen Ursprung der spielernächsten lebenden Kuh und nimmt nur Kühe innerhalb von zwölf Tiles dieses Ursprungs auf. Die aktuelle Anker-UnitID bleibt bis zu ihrem Tod oder einem bestätigten Projektionsausschluss gepinnt; erst nach Räumung der gebundenen Gruppe darf eine andere Gruppe übernehmen. Im Standardkampf bis vier Gruppenkühe bleibt die Cow-King-Priorität innerhalb dieser Gruppe erhalten. Der erste erfolgreiche lebende Zielcast eines Holds ist Amplify Damage.

Unmittelbar beim bestätigten Opener wird einmal pro Hold die nächste lebende Kuh gegen die nächste verwendbare Leiche verglichen. Eine nähere oder gleich weit entfernte Leiche eröffnet die normale CE-Kette. Ist die Kuh näher oder existiert noch keine Leiche, markiert der Hold alle aktuell sichtbaren Leichen als vorheriges Pack und verwendet zunächst Bone Spear mit dem gemeinsamen Hover-/Projektionsvertrag. Der frühe Schnitt geschieht bewusst vor dem ersten Bone Spear, damit dessen erste neue Leiche nicht nachträglich als altes Pack klassifiziert wird. Bis zur Routenfortsetzung sind ausschließlich Leichen CE-berechtigt, die erstmals nach dieser gelatchten Grenze erscheinen. Die World-Daten bleiben unverändert; es gibt weder eine globale Leichenliste noch eine wiederholte Distanzumschaltung.

CE wird positionsgebunden auf eine konkrete berechtigte Leichen-UnitID gesendet und besitzt keinen Hover-Gate; überlappende Leichen oder lebende Sprites blockieren daher nicht künstlich. Eine Leiche ist für den aktuellen Entscheid nur taktisch passend, wenn sie höchstens zwölf Tiles vom aktuellen Gruppenanker entfernt liegt. Unter allen passenden Leichen gewinnt zuerst die höchste Abdeckung lebender Mitglieder der gebundenen Gruppe, dann der geringste Ankerabstand und schließlich die kleinere UnitID. Ein zuvor dichter Pack darf die CE-Freigabe nach dem ersten Bone-Spear-Kill behalten, aber nur eine Leiche mit mindestens vier abgedeckten lebenden Gruppenkühen tatsächlich sprengen. Sinkt die Gruppe ohne solchen Kandidaten unter fünf, folgt sofort Bone-Spear-Cleanup. 900 Millisekunden plus ein neuerer vollständiger Snapshot verhindern Back-to-back-Casts. Nicht projizierbare Leichen werden im aktuellen Hold übersprungen, wirkungslose Leichen höchstens zweimal versucht.

Nur wenn die sichtbare Körperposition eines lebenden Kuh-Ziels tatsächlich außerhalb des spielbaren Clientbereichs liegt, überspringt der Wrapper diese UnitID und versucht ein anderes Mitglied derselben gebundenen Gruppe. Eine bloß fehlende Hover-Bestätigung ist dagegen kein Projektions- oder Reichweitenfehler und löst den offensiven `world_projected`-Cast aus. Erst wenn alle Gruppenmitglieder wirklich nicht projizierbar sind, darf der bestehende Drei-Snapshot-Range-Gate eine begrenzte Annäherung auslösen. Cow verwendet dafür die vorhandene projektionsgetriebene Combat-Teleportation zur tatsächlich gepinnten Kuh; Summoner behält seinen auf reguläres Movement begrenzten Routepunkt-Force-Move. Der nach dem Input offene Annäherungsnachweis bleibt bis zum frischen Settle-Snapshot autoritativ. Drei wirkungslose Versuche sperren weitere lokale Bewegung für diese UnitID; die übergeordnete No-Progress-Grenze entscheidet danach allein über einen kontrollierten Run-Abbruch.

Der Cow-Watchdog akzeptiert nur weniger lokale Lebende, neue direkte Leichen, bestätigten Verbrauch oder bessere lokale Safe-Coverage. Ein bloßer CE- oder Bone-Spear-Input ist kein Fortschritt. Nach zwölf Sekunden ohne solchen Nachweis folgt zuerst Retarget innerhalb der aktiven Gruppe, danach eine begrenzte Approach-Teleportation; erst ein weiterer Ablauf ohne Fortschritt erzeugt den Soft-Exit `cow_combat_no_progress` und verbraucht das Session-Retry-Budget. Am terminalen Cow-Routenpunkt reichen drei frische Safe-Snapshots auch dann nicht aus derselben Poll-Wiederholung; erst drei verschiedene Zeitstempel erlauben dem RoutePlayer den Abschluss.

### Pickit während der Kampfroute

Nach der frischen Threat- und Ressourcenprüfung, aber vor `Route.Tick`, wertet der opt-in Route-Step einmal pro aktivem Routenpunkt die bereits für den Run gebundene Benutzer-Pickit-Kette aus. Es gibt kein separates Combat-Pickit und keine hardcodierte Itemliste. Ausschließlich `keep`-Treffer innerhalb von 30 Tiles werden berücksichtigt; `sell`-Treffer bleiben dem normalen Town-Workflow vorbehalten.

Erst wenn kein reguläres `keep`-Ziel vorliegt, prüft derselbe gehaltene Route-Loot-Zyklus die profilzugewiesenen Gürtelspalten. Für tatsächlich freie Healing-, Mana- oder Rejuvenation-Plätze kommt ausschließlich der jeweils nächstgelegene exakte Drop `hp5`, `mp5` oder `rvl` infrage. Die Auswahl umgeht bewusst nur die Inventarplatzprüfung, nicht aber Distanz, Hover-Bestätigung, Monster-Abbruch, Annäherungsbudget oder frische Verifikation. Nach jeder Aufnahme wird der aktuelle Gürtel neu gezählt; bei vollem Ziel oder ohne passenden Bodentrank läuft die Route sofort weiter. Das ist keine globale Pickit-Regel und betrifft nur Runs mit aktivierter Kampfroute.

Alle passenden Items werden in deterministischer Distanzreihenfolge nacheinander aufgehoben. Während Auswahl, begrenzter Teleport-Annäherung und hover-bestätigtem Pickup bleibt der RoutePlayer per `Route.Hold` am selben Segment und Punkt. Ein neuer Threat verdrängt Loot sofort und erlaubt in diesem Tick keinerlei Loot-Input. Nach dem Clear wird am gehaltenen Fortschritt erneut gescannt, damit Kampf-Drops nicht verloren gehen. Die reguläre Vorannäherung bleibt auf 20 Tiles und drei Versuche begrenzt. Meldet der Pickup für ein bereits threat-frei ausgewähltes `keep`-Ziel innerhalb des bestehenden 30-Tile-Route-Scans dennoch `too_far`, darf die vorhandene UnitID-gebundene Einmal-Recovery genau einen Teleport zum Ziel und einen frischen Pickup-Versuch ausführen. Dieses erweiterte Budget gilt nur für den gehaltenen Route-Loot-Zyklus; gewöhnliches Boss-Loot bleibt bei 20 Tiles. Monster-Nähe, Projektion, Hover-Bestätigung und frische Verifikation bleiben autoritativ. Nicht passende, nicht passende Kapazität besitzende oder danach weiterhin nicht erreichbare Items stoppen die Kampfroute nicht.

### Mana und Recovery

Unter 20 % Mana beginnt ein Route-Hold; ein begonnener Hold endet erst ab 35 %. Immediate-Threat bei höchstens 10 % Mana priorisiert nach kritischer HP-Safety eine Mana-Potion; nur wenn keine passende Mana-Potion verfügbar ist, darf Rejuvenation als Fallback dienen. So leert wiederholter Mana-Drain nicht zuerst den nicht in Town nachkaufbaren Rejuvenation-Vorrat. Fehlt ein geeigneter Trank oder wird innerhalb von fünf Sekunden keine Erholung bestätigt, endet der Run mit `route_mana_recovery_failed`.

Seit Phase 20.7 stoppt das Profil bei einer bereits fälligen, aber im konfigurierten Belt nicht mehr verfügbaren Spielerressource oder Merc-Heilung zuerst den Angriff und liefert `combat_resource_exhausted`; bei aktiver Mana-Hysterese bleibt der speziellere Grund `route_mana_recovery_failed`. Beide Gründe verwenden den kontrollierten Town-/Exit-Retry statt weiterzukämpfen. Passive Potion-Verifikation behält den bestehenden Vertrag: Sie sendet keinen Input und verbraucht deshalb den Route-Tick nicht.

### Bone-Armor-Maintenance

Combat-Routen des Profils `necro_bone_spear` prüfen nach Ressourcen und vor jedem Clear- oder Movement-Input genau eine schmale Maintenance-Regel. Bone Armor wird spätestens 60 Sekunden nach dem letzten erfolgreichen Self-Cast oder nach neu beobachtetem HP-Verlust bei höchstens 65 Prozent fällig. Zehn Sekunden Mindestabstand verhindern Back-to-back-Casts. Ein fälliger Cast stoppt im ersten Tick ausschließlich den Combat-Zustand; erst der nächste frische Tick sendet den Self-Cast, danach gelten 750 ms Settle. Town, die combatfreie Wirt-Leg-Route und reine Recording-/Playback-Tests werten diese Regel nicht aus.

Der Cow-Hold verwendet innerhalb der bestehenden Angriffsdistanz drei explizite Kampfphasen. `await_density` verwendet Amplify Damage und Bone Spear, bis die gebundene 12-Tile-Gruppe erstmals mindestens fünf Mitglieder besitzt. `corpse_explosion` hält diese Freigabe über den ersten Kill hinweg, sendet CE aber nur bei mindestens vier abgedeckten lebenden Gruppenmitgliedern. Ein bestätigter nützlicher CE-Settle oder eine ohne passenden Kandidaten unter fünf gesunkene Gruppe schaltet in `cleanup`. So kann Bone Spear die erste lohnende CE nicht verhindern, erzwingt aber niemals eine Explosion mit nur einem Restziel. Wächst eine neu gebundene Gruppe im Cleanup erneut auf mindestens fünf, beginnt wieder `corpse_explosion`. Ein wartender CE-Cast muss 900 Millisekunden sowie einen danach frischen vollständigen Snapshot abwarten; nach der Bestätigung kann der nächste Entscheid noch im selben Tick erfolgen. Ein bereits Memory-bestätigter rechter CE-Skill wird nicht erneut per F2 ausgewählt.

Das tatsächlich projizierte Recovery-Ziel durchläuft dieselbe Threat-Prüfung. Nach einem wirklich gesendeten Recovery-Teleport unterscheidet RouteProgress den throttle-seitig frühesten Folgeinput vom autoritativen 700-ms-Teleport-Settle-Zeitpunkt des Navigators. Eine noch unveränderte Position ist vor diesem Settle-Zeitpunkt kein Fehlschlag. Bleibt der Fortschritt danach unter `pathing.stuck_progress_tiles`, wird ein zweiter identischer Cast mit `route_recovery_unsafe` verhindert. Das gilt auch für die erwartete Rückkehr zum Routenanker nach einem erfolgreichen Route-Pickit-Teleport. Es gibt kein Jitter- oder Alternativziel; `max_local_corrections` bleibt die absolute Obergrenze.

## Datenmodell

| Effektiver Wert | Default |
|---|---:|
| Immediate-Radius | 18 Tiles |
| Corridor-Halbbreite | 7 Tiles |
| Landing-Radius | 10 Tiles |
| Angriffsdistanz | 30 Tiles |
| No-Progress-Timeout | 12.000 ms |
| Mana-Hold / Resume | 20 % / 35 % |
| Emergency-Mana | 10 % |
| Mana-Recovery-Timeout | 5.000 ms |

`enabled` ist presence-sensitiv: fehlend aktiviert Route-Combat weiterhin nur für Summoner; die Cow-Beispielkonfiguration setzt den Sweep ausdrücklich auf `enabled: true`. `enabled: false` ist der Rollbackschalter. Die private `leg_acquisition`-Route erhält unabhängig davon eine deaktivierte Configkopie. Andere Runs bleiben standardmäßig und fachlich deaktiviert. Die effektiven, vom Core validierten Werte erscheinen read-only unter **Einstellungen → Effektive Route-Combat-Werte**.

Stabile Fehlergründe sind `route_clear_no_progress`, `cow_combat_no_progress`, `route_threat_out_of_range`, `route_mana_recovery_failed`, `route_recovery_unsafe` und `route_threat_state_invalid`. Die retryfähigen Gründe gehören nur in frischen oder exakt unveränderten früheren Default-Konfigurationen zur erweiterten Retry-Liste; angepasste Listen werden nicht überschrieben. `route_threat_out_of_range` entsteht erst nach der oben beschriebenen begrenzten Annäherung. Bei einer anschließend retryfähigen Störung castet die Runtime aus einem für die veröffentlichte Route erlaubten Gebiet ein Town Portal, bestätigt die Rückkehr in die Herkunftsstadt, verwendet bei fremden Akten den bestehenden System-Egress nach Akt 1 und setzt `SafeToExit` erst nach bestätigtem Rogue Encampment. Danach führt der Supervisor exakt dieselbe Save-&-Exit-Routine wie am normalen Queue-Ende aus und startet denselben Queue-Eintrag innerhalb der endlichen Restart-Budgets neu. Die gemeinsame Routine startet nach der ersten validen Akt-1-/Identity-Bestätigung genau ein nicht zurückgesetztes Drei-Sekunden-Settle-Fenster, bevor sie Escape sendet. Bleibt das Quit-Menü danach Memory-bestätigt geschlossen, fokussiert sie D2R erneut und wiederholt Escape nach 1,5 Sekunden genau einmal.

## Operator / Telemetrie

- `route_threat_detected`: neues Ziel oder Zonenwechsel
- `route_clear_started`: einmal pro Blockade
- `route_monster_snapshot_saturated`: Eintritt, Coverage-Wechsel und Ende
- `route_clear_action`: tatsächlich gesendeter `curse`-, `attack`-, `corpse_explosion`-, `force_move`- oder Cow-`teleport`-Input; Cow-Aktionen tragen additiv `cow_group_anchor_unit_id` und `cow_group_living_count`, CE zusätzlich `cow_corpse_anchor_distance_tiles` und `cow_corpse_coverage_count`; CE trägt weiterhin die konkrete Leichen-UnitID, Annäherung Versuch 1–3
- `route_clear_progress`: akzeptierter objektiver Clear- oder Memory-bestätigter Annäherungsfortschritt
- `route_clear_completed`: einmalige Aggregate für Dauer, Hold, Aktionen und Ziele
- `route_mana_hold`: Start, materielle Änderung und Ende
- `route_recovery_suppressed`: verhinderter unsicherer Recovery-Input

Die additiven Schema-3-Ereignisse erzeugen keine Per-Tick-Flut und keine neue History-Ansicht. Die Cow-Debugfelder hängen nur an einem tatsächlich gesendeten bestehenden Aktionsereignis; es gibt dafür weder ein neues Event noch einen neuen Stream. Deutsche Fehlertexte und die effektive Configprojektion bilden die Operatoroberfläche.

## Abhängigkeiten und Grenzen

- `memory` und `world` sind alleinige Quelle für Living-Monster, Coverage und Hover.
- `tasks.RunDefinition` ist alleinige Hostile-Allowlist.
- `pathing` besitzt Routefortschritt, Deadlines und Recovery-Budget.
- `profile` besitzt Buildstrategie und Ressourcenwahl.
- `app.CombatActions` besitzt Skill, Aim, Hover und Klick.

Candidate-/Raw-Playback, Recording und Guided Validation bleiben reine Navigation. Countess, Mephisto und Nihlathak erhalten keine neuen Holds; die enge Bone-Armor-Regel gilt nur in bereits vorhandenen Combat-Routen. Es gibt keine Rotations-DSL, universelle Multi-Route-/Combat-AI oder zusätzliche Nebenläufigkeit. Phase 20.7 ergänzt ausdrücklich keinen Abbruch allein aufgrund von Laufweglänge, D2R-Pfadwahl oder Force-Move-Anzahl.

## Verwandte Features

- [Summoner-Run](summoner-run.md)
- [Route Recording und Playback](route-recording-playback.md)
- [Task Runner](task-runner.md)
- [Character- und Encounter-Profile](character-encounter-profiles.md)
- [World Model](world-model.md)

---
*Zuletzt aktualisiert: 2026-08-10*
