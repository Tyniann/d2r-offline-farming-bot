# Character- und Encounter-Profile

## Überblick

Phase 8 führt generische, klassenbegrenzte Lifecycle-Hooks und eine profilabhängige Resource Policy ein. Run-State-Machines melden semantische Ereignisse; konkrete Skills, Ziele und Potion-Grenzen bleiben im ausgewählten Profil. Der erste produktive Verbraucher ist `necro_bone_spear`.

## Ort im Code

- **Paket:** `internal/profile/`
- **Wiring:** `internal/app/profile.go`
- **Run-Integration:** `internal/tasks/runner.go`, `internal/tasks/run_pipeline.go`
- **Config:** `combat_profiles` und `runs.definitions.<run-id>.combat.profile`

## Hook-Vertrag

| Hook | Zeitpunkt | Erster Use Case |
|---|---|---|
| `town_ready` | Nach stabilem Town-/Identity-State und vor Town-Walk | Bone Armor auf `self` |
| `boss_engage` | Nach bestätigter Boss-UnitID und vor regulärem Angriff | Bone Prison auf `boss` |

Der Executor führt pro Tick höchstens eine Skill-Aktion aus. `once_per_game` und `once_per_encounter` sind explizite Marker; Settle-Zeiten blockieren nachfolgende Hook-Aktionen ohne weitere Inputs. Bone Armor ist bewusst kein `once_per_game`-Hook, sondern wird zu Beginn jedes Runs aufgefrischt. Bei einem Queue-Folgerun im bereits bestätigten Spiel entfällt nur das New-Game-Delay, nicht der Cast oder dessen Settle-Verifikation. Wiederholt eine Run-Definition denselben Encounter-Hook, wird der stabile Aktionsindex bis zum Executor geführt: Retries desselben Index bleiben idempotent, ein höherer Index startet dagegen eine eigene Delay-/Cast-/Settle-Ausführung auf derselben gepinnten UnitID. `Reset` löscht Game-, Encounter-, Index-, Settle-, Potion- und Throttle-Zustand.

Phase 17.3 ergänzt genau die codegestützte Route-Clear-Strategie `single_target`. `TickRouteClear` erhält pro Snapshot genau ein bereits autorisiertes lebendes Monster und besitzt ausschließlich `CastAttackAtMonster`/`StopAttack`; eine Movement-, Teleport- oder Navigator-Oberfläche existiert dort nicht. Normalerweise ist dies das priorisierte Route-Ziel. Liegt während eines aktiven Route-Holds stattdessen irgendein anderes Memory-bestätigtes lebendes Monster unter dem Cursor, wird diese Unit ohne weitere Allowlist-, Geometrie-, Distanz- oder Pin-Prüfung unmittelbar angegriffen. Für `necro_bone_spear` wirkt die Strategie bei einem echten Route-Threat einmal Amplify Damage und verwendet anschließend Bone Spear; `density_relief` beginnt direkt mit Bone Spear. `paladin_hammerdin` registriert denselben Summoner-Route-Clear ohne Fluch-Opener: `CastAttackAtMonster` erkennt den LMB-Standardangriff und hält Blessed Hammer wie im Mephisto-Bosskampf. `CastAttackAtMonster` meldet für tatsächlich gesendete Inputs explizit `hover_confirmed` oder `world_projected`. Der zweite Modus ist ausschließlich nach erschöpftem Hover-Budget und erneut bestätigter spielbarer Körperprojektion zulässig; er ist kein Interaktionsfallback. Unbekannte Strategien oder Profile werden vor Runtime abgelehnt.

Self-Targets werden an der neutralen geometrischen Client-Mitte gecastet. Der projizierte Spieleranker wird bewusst nicht angeklickt, da D2R diesen Rechtsklick als Bewegung interpretieren kann. Boss-Targets verwenden weiterhin die World-zu-Client-Projektion der gepinnten Unit-Position.

Phase 20.1 ergänzt ausschließlich für `necro_bone_spear` den engen Vertrag `TickAuthorizedCorpseExplosion`. Die Task-Schicht muss eine konkrete direkte `CowCorpse.UnitID` aus genau dem aktuellen vollständigen State autorisieren. Profile löst diese UnitID erneut auf, verlangt übereinstimmende Snapshot-Zeit und -Generation, bekannte unverbrauchte CE-Bits und eine spielbare Position. Der App-Adapter bestätigt Fokus und Clientprojektion, bevor Skill 74 genau einmal gesendet wird. Anschließend blockieren 900 Millisekunden Mindest-Settle und der erste neuere vollständige Snapshot den nächsten Cow-Combat-Input. Die Grenze entspricht der langsameren der beiden in Gate 20.0 beobachteten Verbrauchszeiten. Ist CE im aktuellen Player-State bereits als rechter Skill bestätigt, überspringt der Adapter die redundante Skill-Auswahl und sendet nur den positionsgebundenen Rechtsklick. Es gibt keinen Hover-Gate: überlappende Leichen oder lebende Sprites dürfen CE nicht festfahren lassen; der Folgesnapshot entscheidet, welche Leiche tatsächlich verbraucht wurde.

Phase 20.5 kombiniert diese zwei bereits engen Profiloberflächen ausschließlich im Cow-Task. `cowHoldExecutor` lässt `TickRouteClear` den einmaligen Amplify-Damage-Opener und Bone Spear über denselben expliziten Hover-/Projektionsvertrag ausführen und ruft CE nur mit einer im selben vollständigen State ausgewählten Leichen-UnitID auf. Bei weniger als fünf lebenden Kühen innerhalb der Angriffsdistanz bleibt CE unabhängig von vorhandenen Leichen gesperrt; der Standard-Clear verwendet AD und Bone Spear bis zum Ende der lokalen Gruppe. Profile entscheidet weder über Routenbewegung noch über Dichteschwelle, Leichenpriorität, Versuchsbudget oder Fortschritt. `ResetRouteClear` löscht neben dem AD-Opener auch ein eventuell wartendes CE-Settle, sodass kein Pending-Cast in einen späteren Hold überlebt.

Phase 20.7 ergänzt `TickRouteMaintenance` als bewusst nicht generische Necromancer-Regel. Auf bereits aktiven Combat-Routen wird Bone Armor 60 Sekunden nach dem letzten erfolgreichen Cast oder nach neuem HP-Verlust bei höchstens 65 Prozent fällig. Zehn Sekunden Mindestabstand, Ressourcenpriorität und getrennte StopAttack-/Self-Cast-Ticks verhindern Spam und konkurrierende Eingaben. Nach dem Cast gelten 750 ms Settle. Der erfolgreiche `town_ready`-Cast setzt denselben Zeitanker; Town-, Wirt-Leg- und reine Playback-Ticks führen keine Maintenance aus.

Die Live-Abnahme `cows-20260808t213733999999999z-6ddb2272` schließt das Bone-Armor-Teilgate: Auf den bestätigten `town_ready`-Cast um 23:37:57.384 folgten im Cow-Sweep zwei `route_maintenance`-Self-Casts mit Skill 68 um 23:38:57.484 und 23:39:57.602. Die rund 60-sekündigen Abstände bestätigen den Timervertrag ohne Spam.

## Resource Policy

Die Resource Policy läuft bei gültigen In-Game-Ticks vor Hooks und Run-Aktionen. Priorität:

1. kritische HP → Rejuvenation;
2. Route-Emergency-Mana bzw. Mobility-Mana (nur mit `ResourceContext`);
3. niedrige HP → Healing;
4. niedriges Mana → Mana Potion;
5. nur wenn kein Player-Demand und `AllowMercenary=true`: lebender Merc mit bekannten Vitals und `HPPercent < use_below_percent` → Healing-Potion per Shift+Belt.

Nur ein tatsächlich modelliertes Belt-Item mit passendem Typ und konfigurierter Spalte autorisiert den Tastendruck. Nach dem Input bleibt die Run-Aktion bis zur bestätigten Abwesenheit der ursprünglichen Item-UnitID gesperrt. Zusätzlich besitzt jeder Tranktyp eine eigene Wirkungs-Sperrzeit: Healing und Mana warten standardmäßig vier Sekunden auf den graduellen Effekt, Rejuvenation nur 1,5 Sekunden wegen der sofortigen Wirkung. Merc-Heilung führt einen eigenen 4-Sekunden-Cooldown und teilt den globalen Potion-Throttle. Ein leerer oder falsch belegter Slot erzeugt keinen Blindinput und kein Retry-Spamming. Merc verwendet niemals Rejuvenation und trinkt nicht bei exakt 50&nbsp;% (strict `<`).

Auf einer Combat-Route gilt zusätzlich `FailOnUnavailable`: Ist eine bereits fällige Spielerressource oder Merc-Heilung nicht im zugewiesenen Belt vorhanden, stoppt der Executor zuerst den Combat-Zustand und liefert `combat_resource_exhausted`. Eine tatsächliche Potion-Aktion bleibt höher priorisiert; passive Verifikation sendet weiterhin keinen Input und blockiert den Route-Tick nicht. Die Queue beendet den erschöpften Versuch kontrolliert, kehrt nach Town zurück und beginnt den endlichen Retry mit regulärem Restock; das Profil implementiert weder eigenen Town-Refill noch Route-Checkpoint-Resume.

`AllowMercenary` setzen nur `engage_boss`, `clear_nearby_hostiles` und ein aktiver Route-Clear von Summoner oder Cow-Sweep. Dead/Unknown/NotHired senden keinen Merc-Input.

Phase 17.4 ergänzt `ResourceContext` ausschließlich für den opt-in Summoner-Route-Step. Dort gilt die engere Priorität: HP-kritische Rejuvenation, bei Immediate-Threat und höchstens 10 % Mana eine vorhandene Mana-Potion, danach Rejuvenation als Fallback und erst anschließend die normalen Regeln. Dadurch verbraucht reiner Mana-Drain den nur über Pickit ersetzbaren Rejuvenation-Vorrat nicht vor den in Town nachkaufbaren Mana-Tränken. `StatusAction` bleibt ein vollständig verbrauchter Input-Tick; `StatusPending` bedeutet weiterhin nur passive Memory-Verifikation und autorisiert selbst keinen zweiten Input.

## Konfiguration

```yaml
combat_profiles:
  necro_bone_spear:
    character_class: necromancer
    display_name: Knochen-Speer
    setup:
      enabled: true
      default: true
    hooks:
      town_ready:
        - skill: bone_armor
          target: self
          once_per_game: false
          delay_ms: 5000
          settle_ms: 1500
      boss_engage:
        - skill: bone_prison
          target: boss
          once_per_encounter: true
          delay_ms: 250
          settle_ms: 1000
    resources:
      healing: { use_below_percent: 65, belt_slots: [1], cooldown_ms: 4000 }
      mana: { use_below_percent: 35, belt_slots: [2, 3], cooldown_ms: 4000 }
      rejuvenation: { use_below_percent: 35, belt_slots: [4], cooldown_ms: 1500 }
      mercenary: { enabled: true, use_below_percent: 50, belt_slots: [1], cooldown_ms: 4000 }
      throttle_ms: 1500
      verify_timeout_ms: 1500
    route_maintenance:
      bone_armor:
        enabled: true
        skill: bone_armor
        refresh_interval_ms: 60000
        refresh_after_damage_below_percent: 65
        minimum_recast_interval_ms: 10000
        settle_ms: 750
```

Fehlt `resources.mercenary`, gilt presence-sensitiv `enabled=true` mit Defaults 50/`[1]`/4000. Explizites `enabled:false` deaktiviert Combat- und spätere Town-Merc-Aktionen. Merc-Slots müssen eine Teilmenge von `healing.belt_slots` sein. `shift` ist feste D2R-Semantik und kein Config-Wert.
Die Spaltenzuordnung aus `combat_profiles.*.resources.*.belt_slots` ist der Profil-Default. OperatorSettings Schema 3 darf unter `characters.*.profile_bindings.<profil>.belt_layout` eine vollständige Spaltenkarte (`healing`/`mana`/`rejuvenation` für Slot 1–4) speichern; fehlt sie, bleiben die YAML-Defaults. Gespeicherte Layouts überschreiben zur Runtime Pickup, Trinken und Town-Restock für genau diesen Charakter und bleiben fail-closed bei Teilbelegung oder fehlender Heilspalte bei aktivem Merc.
Profile referenzieren Skill-Namen. Produktive Tasten kommen aus OperatorSettings Schema 3 (`characters.*.profile_bindings` für das aktive Kampfprofil); `config.yaml` besitzt keinen Binding-Fallback. Phase 21 liefert den CASC-Skillkatalog, geordnete `required_skills`, die Strategy Registry und den Charaktere-Loadout-Vertrag.

Fehlt `route_maintenance.bone_armor` bei einem bestehenden `necro_bone_spear`-Profil, ergänzt der Loader den dokumentierten aktiven Standard nur im Speicher. Ein ausdrücklich gespeichertes `enabled: false` bleibt deaktiviert; die lokale YAML wird nicht automatisch umgeschrieben.

Abschnitt 16.2 ergänzt ausschließlich Setup-Metadaten am bestehenden Profil. `setup.enabled` ist die ausdrückliche Produktfreigabe; `setup.default` markiert den Entwickler-Default der Klasse. Ein freigegebenes Profil benötigt einen getrimmten, steuerzeichenfreien Anzeigenamen mit höchstens 64 Zeichen. Jede Klasse mit mindestens einem freigegebenen Profil besitzt exakt einen freigegebenen Default; Klassen ohne Freigabe bleiben valide und im Charakter-Setup nicht unterstützt. `necro_bone_spear` ist als „Knochen-Speer“ freigegeben und der einzige Necromancer-Default. Experimentelle, nicht freigegebene Profile bleiben außerhalb der Setup-Projektion.

Phase 21 ergänzt am freigegebenen Profil `combat.standard_attack`, Combat-Tuningwerte und eine geordnete `required_skills`-Liste. Teleport und Stadtportal sind Pflicht. Hook- und Maintenance-Skills müssen in derselben Liste stehen; jeder Eintrag wird gegen den generierten Skillkatalog geprüft. Die ausführbare Combat-Autorität pro Run ist die Code-`CombatStrategyRegistry` für `(profileID, runID)`; Run-YAML pinnen kein Profil mehr.

### Modulregel für neue Profile

Ein neues Kampfprofil darf runweise wachsen: ProfileConfig + Profilmodul + genau ein Registry-Eintrag reichen für den ersten Run. Jeder weitere Registry-Eintrag braucht Strategy-, Required-Skill- und Availability-Tests. Fehlende Factories bleiben fail-closed (`profile_run_strategy_unavailable`). Siehe auch [Character Loadouts](character-loadouts.md) und den Backlog-Eintrag „Multi-Profil-Erweiterungen“.

Beim Hinzufügen des ersten tatsächlich lauffähigen Profils einer neuen Klasse setzt der Entwickler daher direkt `setup.enabled: true` und `setup.default: true`. Weitere Profile derselben Klasse dürfen freigegeben werden, aber genau eines bleibt der feste Default. Vor Selection, Queue-Start und jedem Run wird erneut geprüft, dass das gespeicherte Profil weiterhin freigegeben und klassenkompatibel ist.

## Operator und Sicherheit

- Isoliertes erstes Live-Gate: `go run ./cmd/d2rbot --config configs/config.yaml --run countess --phase town-ready`.
- Der Full-Countess-Preflight verlangt alle Hook-Skills und alle konfigurierten Belt-Spalten vor Runtime-Input.
- Eine falsche Character Class endet mit `profile_class_mismatch`.
- Ein fehlendes Bossziel endet mit `profile_target_missing`.
- Ein fehlgeschlagener Skill- oder Potion-Input sowie ein Verify-Timeout sind stabile Fehlergründe.
- Hook-Aktion, Potion-Anforderung und Memory-Bestätigung werden synchron im Run-JSONL erfasst. Ein Schreibfehler endet exakt mit `profile_telemetry_failed` vor jeder weiteren Aktion.
- Der Session-Reset enthält den Profil-Executor.

## Telemetrie und Reset

Jeder frische Session-Run bindet seinen eigenen Run-Recorder an den Profil-Executor und entfernt ihn beim Run-Ende wieder. `profile_hook_action` enthält Profil, Hook, Skillname/-ID, Ziel und Boss-UnitID. `resource_potion_requested` und `resource_consumption_confirmed` enthalten Ressourcenart, Schwellwert, Belt-Slot, Potion-UnitID und den Bestätigungsstatus. `profile_action_failed` trägt einen stabilen Reason-Code.

`Reset` verwirft Once-Marker, Encounter-Pinning, Settle-Zeit, pending Potion-Verifikation, Verfügbarkeitsstatus sowie globale und typspezifische Cooldowns. Tests beweisen, dass nach Reset, Stop oder terminalem Fehler kein verzögerter Profilinput nachläuft.

## Grenzen

- `town_ready` und `boss_engage` sind produktiv in die gemeinsamen Countess-/Mephisto-Flows integriert. Der Boss bleibt über seine bestätigte UnitID an jede indexierte Encounter-Aktion gebunden.
- Buff-Dauer wird nicht aus Memory gelesen. Bone Armor wird deshalb zu Beginn jedes Runs und auf Combat-Routen nach dem dokumentierten Zeit-/Schadensvertrag neu angefordert; nur das New-Game-Delay entfällt beim Same-game-Folgerun.
- Phase 8 verbraucht vorhandene Tränke. Phase 9 kauft nur Healing/Mana nach; Rejuvenation ist nicht kaufbar und kommt vorerst ausschließlich über Pickit-Loot.

## Verwandte Features

- [Task Runner](task-runner.md)
- [Input Controller](input-controller.md)
- [Session Lifecycle](session-lifecycle.md)
- [Countess Run](countess-run.md)
- [Phase-18-Core-Vertrag](phase-18-core-contract.md)

Der isolierte Live-Lauf am 12.07.2026 bestätigte genau einen F5-/Bone-Armor-Cast (Skill 68), Settle und `outcome=success` ohne Town-Navigation. Ein dabei sichtbares CLI-Fallthrough in die aktivierte Session wurde behoben; explizite Runs und Probes besitzen nun Vorrang. Die zweite Live-Abnahme bestätigte Bone Prison (Skill 88) auf Countess Unit 273 und den ersten Bone-Spear-Cast erst 702 ms später; der vollständige Run endete erfolgreich.

Der erste E2E-Abnahmeversuch am 12.07.2026 bestätigte Potion-Cooldowns, Loot, Stash, Save & Exit, Sessionabschluss und die neuen JSONL-Events. Die sichtbare Skill-Abnahme schlug dagegen fehl: Der Settle begann vor dem blockierenden Input und war beim abgeschlossenen Klick fast verbraucht. Town-Walk folgte deshalb 44 ms nach dem Bone-Armor-Klick und F8 100 ms nach Bone Prison. Weitere Versuche zeigten zusätzliche E2E-Unterschiede: Memory meldet den In-Game-State vor der sichtbaren Eingabebereitschaft, ein Self-Cast auf den projizierten Spieleranker kann als Bewegung interpretiert werden und 500 ms Nachlauf reichen nicht zuverlässig für die komplette Castanimation. Deshalb verlangte `town_ready` fünf Sekunden stabilen Town-State, `boss_engage` zunächst 750 ms, Self-Targets verwenden die neutrale Client-Mitte und beide Hooks halten nach dem Klick 1,5 Sekunden absolute Aktionsruhe. Eine Input-/JSONL-Anforderung allein gilt ausdrücklich nicht als Beweis eines ausgeführten In-Game-Casts.

Der abschließende E2E-Lauf bestätigte Bone Armor sichtbar nach fünf Sekunden Town-Stabilisierung und 1,67 Sekunden Abstand bis Town-Walk. Bone Prison wurde sichtbar auf Countess Unit 225 ausgeführt; Bone Spear folgte nach 1,81 Sekunden. Der vollständige autonome Run einschließlich Potion-Policy, Loot, Stash und Save & Exit endete erfolgreich. Beim installierten Hell-Gate am 26.07.2026 benötigte die Boss-Erkennung nach Ende der Route nur 72 ms; der historische 750-ms-Vorlauf dominierte den verbleibenden Leerlauf. Er ist deshalb auf 250 ms reduziert. Der kurze Puffer schützt weiterhin den Übergang aus dem letzten Teleport. Die sichtbare Nachruhe vor Bone Spear wurde anschließend kontrolliert von 1,5 auf 1,0 Sekunden reduziert.

---
*Zuletzt aktualisiert: 2026-08-17*
