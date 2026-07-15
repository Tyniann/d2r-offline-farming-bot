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

Der Executor führt pro Tick höchstens eine Skill-Aktion aus. `once_per_game` und `once_per_encounter` sind explizite Marker; Settle-Zeiten blockieren nachfolgende Hook-Aktionen ohne weitere Inputs. Wiederholt eine Run-Definition denselben Encounter-Hook, wird der stabile Aktionsindex bis zum Executor geführt: Retries desselben Index bleiben idempotent, ein höherer Index startet dagegen eine eigene Delay-/Cast-/Settle-Ausführung auf derselben gepinnten UnitID. `Reset` löscht Game-, Encounter-, Index-, Settle-, Potion- und Throttle-Zustand.

Self-Targets werden an der neutralen geometrischen Client-Mitte gecastet. Der projizierte Spieleranker wird bewusst nicht angeklickt, da D2R diesen Rechtsklick als Bewegung interpretieren kann. Boss-Targets verwenden weiterhin die World-zu-Client-Projektion der gepinnten Unit-Position.

## Resource Policy

Die Resource Policy läuft bei gültigen In-Game-Ticks vor Hooks und Run-Aktionen. Priorität:

1. kritische HP → Rejuvenation;
2. niedrige HP → Healing;
3. niedriges Mana → Mana Potion.

Nur ein tatsächlich modelliertes Belt-Item mit passendem Typ und konfigurierter Spalte autorisiert den Tastendruck. Nach dem Input bleibt die Run-Aktion bis zur bestätigten Abwesenheit der ursprünglichen Item-UnitID gesperrt. Zusätzlich besitzt jeder Tranktyp eine eigene Wirkungs-Sperrzeit: Healing und Mana warten standardmäßig vier Sekunden auf den graduellen Effekt, Rejuvenation nur 1,5 Sekunden wegen der sofortigen Wirkung. Ein leerer oder falsch belegter Slot erzeugt keinen Blindinput und kein Retry-Spamming.

## Konfiguration

```yaml
combat_profiles:
  necro_bone_spear:
    character_class: necromancer
    hooks:
      town_ready:
        - skill: bone_armor
          target: self
          once_per_game: true
          delay_ms: 5000
          settle_ms: 1500
      boss_engage:
        - skill: bone_prison
          target: boss
          once_per_encounter: true
          delay_ms: 750
          settle_ms: 1500
    resources:
      healing: { use_below_percent: 65, belt_slots: [1], cooldown_ms: 4000 }
      mana: { use_below_percent: 35, belt_slots: [2, 3], cooldown_ms: 4000 }
      rejuvenation: { use_below_percent: 35, belt_slots: [4], cooldown_ms: 1500 }
      throttle_ms: 1500
      verify_timeout_ms: 1500
```

Profile referenzieren Skill-Namen. Die tatsächlichen Tasten und Mausbuttons kommen ausschließlich aus `input.bindings.skills`.

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
- Buff-Dauer wird nicht aus Memory gelesen. Bone Armor gilt nach bestätigter Input-Anforderung einmal pro Game-Generation als angefordert.
- Phase 8 verbraucht vorhandene Tränke. Phase 9 kauft nur Healing/Mana nach; Rejuvenation ist nicht kaufbar und kommt vorerst ausschließlich über Pickit-Loot.

## Verwandte Features

- [Task Runner](task-runner.md)
- [Input Controller](input-controller.md)
- [Session Lifecycle](session-lifecycle.md)
- [Countess Run](countess-run.md)

Der isolierte Live-Lauf am 12.07.2026 bestätigte genau einen F5-/Bone-Armor-Cast (Skill 68), Settle und `outcome=success` ohne Town-Navigation. Ein dabei sichtbares CLI-Fallthrough in die aktivierte Session wurde behoben; explizite Runs und Probes besitzen nun Vorrang. Die zweite Live-Abnahme bestätigte Bone Prison (Skill 88) auf Countess Unit 273 und den ersten Bone-Spear-Cast erst 702 ms später; der vollständige Run endete erfolgreich.

Der erste E2E-Abnahmeversuch am 12.07.2026 bestätigte Potion-Cooldowns, Loot, Stash, Save & Exit, Sessionabschluss und die neuen JSONL-Events. Die sichtbare Skill-Abnahme schlug dagegen fehl: Der Settle begann vor dem blockierenden Input und war beim abgeschlossenen Klick fast verbraucht. Town-Walk folgte deshalb 44 ms nach dem Bone-Armor-Klick und F8 100 ms nach Bone Prison. Weitere Versuche zeigten zusätzliche E2E-Unterschiede: Memory meldet den In-Game-State vor der sichtbaren Eingabebereitschaft, ein Self-Cast auf den projizierten Spieleranker kann als Bewegung interpretiert werden und 500 ms Nachlauf reichen nicht zuverlässig für die komplette Castanimation. Deshalb verlangt `town_ready` fünf Sekunden stabilen Town-State, `boss_engage` wartet 750 ms, Self-Targets verwenden die neutrale Client-Mitte und beide Hooks halten nach dem Klick 1,5 Sekunden absolute Aktionsruhe. Eine Input-/JSONL-Anforderung allein gilt ausdrücklich nicht als Beweis eines ausgeführten In-Game-Casts.

Der abschließende E2E-Lauf bestätigte Bone Armor sichtbar nach fünf Sekunden Town-Stabilisierung und 1,67 Sekunden Abstand bis Town-Walk. Bone Prison wurde sichtbar auf Countess Unit 225 ausgeführt; Bone Spear folgte nach 1,81 Sekunden. Der vollständige autonome Run einschließlich Potion-Policy, Loot, Stash und Save & Exit endete erfolgreich. Der 750-ms-Vorlauf vor dem Boss-Cast bleibt für eine spätere Hell-Optimierung explizit beobachtbar.

---
*Zuletzt aktualisiert: 2026-07-12 (Phase 8 abgeschlossen)*
