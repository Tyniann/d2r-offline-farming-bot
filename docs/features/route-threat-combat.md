# Route-Threat-Combat

## Überblick

Route-Threat-Combat schützt das gebundene Summoner-Playback vor bekannten Gegnern im unmittelbaren Spielerumfeld, im vorausliegenden Korridor und am tatsächlichen nächsten Landepunkt. Die Route bleibt während eines Clears unverändert stehen; das aktive Profil greift stationär an. Das Feature gilt ausschließlich für Offline-/Singleplayer-Runs.

## Ort im Code

- **Pakete:** `internal/tasks`, `internal/profile`, `internal/pathing`, `internal/memory`, `internal/world`
- **Einstieg:** `tasks.runPipeline` im Schritt `play_bound_route`
- **Wichtige Dateien:** `route_threat_assessment.go`, `route_threat_controller.go`, `route_threat_telemetry.go`, `route_segment_player.go`, `executor.go`
- **Config:** `runs.definitions.<run>.route_combat` in `configs/config.example.yaml`

## Funktionalität

### Threat-Geometrie und Coverage

Pro frischem `world.State.At` entsteht höchstens ein O(n)-Assessment ohne zusätzlichen Memory-Read, Sortierung oder temporäre Monsterliste. Die Priorität lautet Immediate, Landing, Corridor. Summoner akzeptiert ausschließlich Specter/Ghost (NPC 40), Hell Clan (56) und Ghoul Lord (131) aus der immutable `RunDefinition`.

Memory hält bis zu 512 nicht priorisierte lebende Monster; Priority-Einheiten bleiben zusätzlich erhalten. `EligibleMonsterCount`, `MonstersTruncated` und `MonsterCoverageRadiusTiles` werden bis `world.State` projiziert. Truncation allein blockiert nicht. Nur unvollständige lokale Coverage aktiviert `density_relief`.

### Hold und stationärer Clear

Ein Blocker führt vor `Route.Tick` zu `Route.Hold`. Hold bewahrt Segment, Transition und Recovery, übernimmt ausschließlich bereits innerhalb der Toleranz bestätigte Punkte und verlängert die aktive Deadline nur um deduplizierte echte Hold-Zeit. `RouteClearExecutor` besitzt ausschließlich Combat-Aktionen. Für `necro_bone_spear` ist genau die Strategie `single_target` registriert: Auf den ersten bestätigten Immediate-/Landing-/Corridor-Blocker einer Blockade wird einmal Amplify Damage gewirkt, danach folgt Bone Spear. `density_relief` überspringt den Curse-Opener.

Die Standardbindung für Amplify Damage ist `input.bindings.skills.amplify_damage: {key: f1, button: right}`. Bestehende Installationen erhalten diesen fehlenden Default beim Laden nur im Speicher; eine bereits vom Operator konfigurierte Bindung bleibt unverändert und die lokale YAML wird nicht umgeschrieben. Der Opener verwendet dieselbe schnelle Ziel-, Hover- und Rechtsklickoberfläche wie Bone Spear und erzeugt weder einen zusätzlichen Memory-Read noch eine eigene Hover-Schleife.

Die Monsterprojektion zielt nicht blind auf die Bodenkachel. Sie beginnt an einem um zwei isometrische Tiles nach oben verschobenen sichtbaren Körperanker und prüft anschließend pro frischem Poll deterministische Spiralpunkte darum. Das ist insbesondere für körperlose oder bewegte Sprites wie Specter relevant. Bestätigt Memory am gesetzten Cursorpunkt die priorisierte UnitID, darf der Rechtsklick folgen. Solange ein Allowlist-Blocker die Route tatsächlich hält, wird aber jedes andere aktuell unter dem Cursor Memory-bestätigte lebende Monster sofort angegriffen. Für diesen bereits gehovten Ersatz gelten bewusst keine zusätzliche Allowlist-, Geometrie-, Distanz- oder Pin-Prüfung: Ein sofortiger Angriff ist nützlicher als eine weitere Aim-Schleife auf das verdeckte ursprüngliche Ziel.

Die Threat-Erkennung bleibt von der aktuellen Bildschirmprojektion unabhängig, damit ein noch nicht sichtbares Pack am nächsten Landepunkt keinen Teleport erhält. Liegt ein anhand Tile-Distanz grundsätzlich angreifbarer Blocker richtungsabhängig außerhalb des spielbaren Clientbereichs, bewegt der Combat-Adapter die Maus nicht. Nach drei frischen Bestätigungen derselben UnitID nutzt die Pipeline stattdessen den bereits validierten nächsten Routenpunkt für genau einen Force-Move-Schritt mit der konfigurierten Town-Walk-Taste. Das ist kein Blind-Teleport und kein frei berechnetes Alternativziel: D2R übernimmt das lokale Lauf-Pathfinding, während `Route.Hold` Segment und Punkt unverändert lässt.

Nach mindestens 500 ms und einem neueren Memory-Snapshot muss die Distanz zur beim Input beobachteten Monsterposition um mindestens ein Tile gesunken sein. Bestätigter Fortschritt setzt Range-Gate und Clear-Watchdog zurück; erst ein neuer dreifacher Projektionsbefund darf erneut annähern. Ohne Distanzgewinn folgen höchstens drei Force-Move-Versuche, erst danach entsteht `route_threat_out_of_range`. Ein kleinerer kreisförmiger Erkennungsradius wäre ungeeignet, weil isometrische Projektion und untere UI-Grenze keinen kreisförmigen sichtbaren Bereich bilden.

Drei frische, lokal vollständige und threat-freie Snapshots schließen eine Blockade ab. Zwölf Sekunden ohne objektiven Ziel-, Count- oder Coverage-Fortschritt enden mit `route_clear_no_progress`. Angriffe, Zielwechsel und Trankinput setzen diesen Watchdog nicht zurück.

### Pickit während der Kampfroute

Nach der frischen Threat- und Ressourcenprüfung, aber vor `Route.Tick`, wertet der opt-in Route-Step einmal pro aktivem Routenpunkt die bereits für den Run gebundene Benutzer-Pickit-Kette aus. Es gibt kein separates Combat-Pickit und keine hardcodierte Itemliste. Ausschließlich `keep`-Treffer innerhalb von 30 Tiles werden berücksichtigt; `sell`-Treffer bleiben dem normalen Town-Workflow vorbehalten.

Alle passenden Items werden in deterministischer Distanzreihenfolge nacheinander aufgehoben. Während Auswahl, begrenzter Teleport-Annäherung und hover-bestätigtem Pickup bleibt der RoutePlayer per `Route.Hold` am selben Segment und Punkt. Ein neuer Threat verdrängt Loot sofort und erlaubt in diesem Tick keinerlei Loot-Input. Nach dem Clear wird am unveränderten Punkt erneut gescannt, damit Kampf-Drops nicht verloren gehen. Die bestehende Pickup-Distanz, maximal drei Annäherungsversuche, Monster-Nähe, Projektion, Hover-Bestätigung und Skip-Logik bleiben autoritativ. Nicht passende, nicht passende Kapazität besitzende oder nicht erreichbare Items stoppen die Kampfroute nicht.

### Mana und Recovery

Unter 20 % Mana beginnt ein Route-Hold; ein begonnener Hold endet erst ab 35 %. Immediate-Threat bei höchstens 10 % Mana priorisiert nach kritischer HP-Safety eine Mana-Potion; nur wenn keine passende Mana-Potion verfügbar ist, darf Rejuvenation als Fallback dienen. So leert wiederholter Mana-Drain nicht zuerst den nicht in Town nachkaufbaren Rejuvenation-Vorrat. Fehlt ein geeigneter Trank oder wird innerhalb von fünf Sekunden keine Erholung bestätigt, endet der Run mit `route_mana_recovery_failed`.

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

`enabled` ist presence-sensitiv: fehlend aktiviert Route-Combat nur für Summoner; `enabled: false` ist der Rollbackschalter. Andere Runs bleiben standardmäßig und fachlich deaktiviert. Die effektiven, vom Core validierten Werte erscheinen read-only unter **Einstellungen → Effektive Route-Combat-Werte**.

Stabile Fehlergründe sind `route_clear_no_progress`, `route_threat_out_of_range`, `route_mana_recovery_failed`, `route_recovery_unsafe` und `route_threat_state_invalid`. Die ersten vier gehören nur in frischen oder unveränderten Default-Konfigurationen zur Retry-Liste; angepasste Listen werden nicht überschrieben. `route_threat_out_of_range` entsteht erst nach der oben beschriebenen begrenzten Annäherung. Bei einer anschließend retryfähigen Störung castet die Runtime aus einem für die veröffentlichte Route erlaubten Gebiet ein Town Portal, bestätigt die Rückkehr in die Herkunftsstadt, verwendet bei fremden Akten den bestehenden System-Egress nach Akt 1 und setzt `SafeToExit` erst nach bestätigtem Rogue Encampment. Danach führt der Supervisor exakt dieselbe Save-&-Exit-Routine wie am normalen Queue-Ende aus und startet denselben Queue-Eintrag innerhalb der endlichen Restart-Budgets neu. Die gemeinsame Routine startet nach der ersten validen Akt-1-/Identity-Bestätigung genau ein nicht zurückgesetztes Drei-Sekunden-Settle-Fenster, bevor sie Escape sendet. Bleibt das Quit-Menü danach Memory-bestätigt geschlossen, fokussiert sie D2R erneut und wiederholt Escape nach 1,5 Sekunden genau einmal.

## Operator / Telemetrie

- `route_threat_detected`: neues Ziel oder Zonenwechsel
- `route_clear_started`: einmal pro Blockade
- `route_monster_snapshot_saturated`: Eintritt, Coverage-Wechsel und Ende
- `route_clear_action`: tatsächlich gesendeter `curse`-, `attack`- oder `force_move`-Input; Annäherung trägt Versuch 1–3
- `route_clear_progress`: akzeptierter objektiver Clear- oder Memory-bestätigter Annäherungsfortschritt
- `route_clear_completed`: einmalige Aggregate für Dauer, Hold, Aktionen und Ziele
- `route_mana_hold`: Start, materielle Änderung und Ende
- `route_recovery_suppressed`: verhinderter unsicherer Recovery-Input

Die additiven Schema-3-Ereignisse erzeugen keine Per-Tick-Flut und keine neue History-Ansicht. Deutsche Fehlertexte und die effektive Configprojektion bilden die Operatoroberfläche.

## Abhängigkeiten und Grenzen

- `memory` und `world` sind alleinige Quelle für Living-Monster, Coverage und Hover.
- `tasks.RunDefinition` ist alleinige Hostile-Allowlist.
- `pathing` besitzt Routefortschritt, Deadlines und Recovery-Budget.
- `profile` besitzt Buildstrategie und Ressourcenwahl.
- `app.CombatActions` besitzt Skill, Aim, Hover und Klick.

Candidate-/Raw-Playback, Recording und Guided Validation bleiben reine Navigation. Countess, Mephisto und Nihlathak erhalten weder neue Holds noch geänderte Ressourcenpriorität. Es gibt keine Rotations-DSL, Leichenlogik, universelle Combat-AI oder zusätzliche Nebenläufigkeit.

## Verwandte Features

- [Summoner-Run](summoner-run.md)
- [Route Recording und Playback](route-recording-playback.md)
- [Task Runner](task-runner.md)
- [Character- und Encounter-Profile](character-encounter-profiles.md)
- [World Model](world-model.md)

---
*Zuletzt aktualisiert: 2026-07-30*
