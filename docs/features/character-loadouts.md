# Character Loadouts

## Überblick

Phase 21 macht das am Charakter ausgewählte Kampfprofil zur einzigen Combat-Autorität. Pflichtskills, Tastenbelegungen und Inventarschutz bilden einen gemeinsamen fail-closed Loadout-Vertrag. Gates 21.0–21.6 sind bestanden, einschließlich manueller Responsive-Visual- und installierter Countess-Abnahme.

## Ort im Code

- **Paket:** `internal/memory`, `internal/config`, `internal/app`, `internal/profile/necrobonespear`
- **Generator:** `tools/generate-skill-catalog/`
- **Einstieg:** `go generate ./internal/memory`
- **Config:** `configs/config.example.yaml` → `combat_profiles.*.combat`, `combat_profiles.*.required_skills`
- **Vertrag:** [phase-21-core-contract.md](phase-21-core-contract.md)

## Funktionalität

### CASC-Skillkatalog (Gate 21.0)

- Einzige Quelle für Skill-IDs und statische Eigenschaften ist `.tmp/d2r-excel/skills.txt`.
- Runtime liest keine TXT-Datei; der Generator schreibt `internal/memory/skill_catalog_data.go`.
- Canonical Keys sind deterministisch (`TownPortal` → `town_portal`).
- `ParseSkillTestName` und die bestehenden Produktkonstanten sind dünne Alias-/Katalogadapter.

### Profilvertrag (Gate 21.0)

- Freigegebene Profile deklarieren `combat.standard_attack` und eine geordnete `required_skills`-Liste (max. 8).
- Teleport und Stadtportal sind Pflicht. Standardangriff, Hook- und Maintenance-Skills müssen in derselben Liste stehen.
- Jeder Eintrag braucht einen Katalogtreffer und einen getrimmten deutschen Anzeigenamen.

### Strategy Registry und Run-Entkopplung (Gate 21.1)

- `(profileID, runID)` löst über `CombatStrategyRegistry` eine ausführbare Factory auf.
- Bone-Spear besitzt Factories für Countess, Mephisto, Summoner, Nihlathak und Cows.
- Combat-Tuning und Standardangriff kommen aus dem Charakterprofil, nicht aus `runs.definitions.*.combat`.
- Availability und Desktop-Setup blockieren mit `profile_run_strategy_unavailable`, wenn keine Strategy existiert.
- Legacy-Route-`profile_id` bleibt lesbar, steuert keine Kompatibilität mehr und wird bei neuen Recordings weggelassen.

### SkillsKnown-Gate und RightSkillSelector (Gate 21.2)

- Unvollständige Skilllisten markieren Completeness; sie erzeugen keinen Missing-Result.
- Nach bestätigtem Spielstart prüft die Queue einmal pro Session die Profil-Pflichtskills; Missing → Save & Exit ohne produktiven Input.
- Für Stadtportal gilt die Live-Entsprechung aus Aktionsskill `TownPortal` (359), buchgewährtem Listeneintrag `Book of Townportal` (220) und Slings itemgewährtem `Townportal O Skill` (411); alle drei belegen dieselbe Pflichtfähigkeit und dieselbe RMB-Auswahl vor dem Cast.
- `verifyActiveQueueGame` pollt snapshot-only (kein Binding-/Readiness-/Task-Tick).
- Combat, Profile-Casts, Teleport und Town Portal nutzen denselben RightSkillSelector: F-Key nur bei Bedarf, RMB erst nach frischem `RightSkillID` (Stadtportal akzeptiert 359/220/411). Eine laufende Skill-Auswahl wird nicht von einem anderen RMB-Skill (z. B. Teleport während Amplify Damage) verdrängt; erst Timeout oder Bestätigung gibt den Selector frei.
- Setup-Preview/API liefern `standard_attack`, geordnete `required_skills` (Key/ID/Label) und Registry-`supported_runs` read-only.

### OperatorSettings Schema 3 und Loadout-Freeze (Gate 21.3)

- Persistente `profile_bindings` (F1–F8, Gürtel) und presence-sensitives `inventory_lock` (4×10) pro Charakter.
- Partielle Bindings speicherbar (Onboarding „Später“); Queue-Start verlangt vollständige Pflichtskills + vier Gürtelslots.
- `CharacterLoadoutResolver` friert Bindings für Queue-Sessions; `app.New` nimmt keinen globalen BindingSource mehr aus `config.Input.Bindings`.
- Settings-Tab „Charaktere“ und gemeinsamer BindingEditor; Katalog liefert `farm_ready` / `farm_ready_reasons`.

### Inventarschutz und produktive Autorität (Gate 21.4)

- Presence-sensitives `inventory_lock`: abwesend = Queue-Block und effektiv alles geschützt; explizit alles geschützt ist valide.
- `InventoryLockEditor` (4×10, „Alle schützen“, kein Free-Preset) im Charaktere-Tab.
- Statische Cow-Eignung warnt bei Queue mit Cows (`inventory_layout_unsuitable_for_cows`), blockiert Countess nicht.
- Globales `loot.inventory_lock` ist aus dem Configschema entfernt; Runtime liest nur den Loadout-Snapshot.
- Produktiver DataRoot ist Schema 3 (Revision 16) mit MrBones-Bindings F1–F8, Belt 1–4 und linkem 4-Spalten-Raster.

### Setup-Wizard und Profilwechsel (Gate 21.5)

- Gemeinsamer `CharacterSetupWizard` für Onboarding, Dashboard-CTA und Charaktere-Tab.
- Ein freigegebenes Profil erscheint read-only; mehrere Profile nutzen den Core-Default.
- „Später“ erlaubt Onboarding-Abschluss bei sichtbarem Queue-Block.
- Profilwechsel bewahrt Queue, Inventar und inaktive Profilbindings; unsupported Queue-Einträge bleiben sichtbar.

### Dokumentation und Produktabnahme (Gate 21.6)

- Globales `input.bindings` ist aus dem Configschema entfernt; KnownFields lehnt den Schlüssel ab.
- Recording-Voraussetzungen für Teleport/Town Portal lesen Schema-3-`profile_bindings` des ausgewählten Charakters.
- Produktiver DataRoot bleibt Schema 3 ohne Config-Fallback.
- Manuell bestätigt: Charaktere-Tab bei 1280×720 und 390×844 funktional lesbar; installierter Countess-Zyklus mit MrBones (`outcome=success`, Save & Exit, bewusster F11-Abbruch erst danach).

## Future

Geplante Erweiterungen ohne Implementierungsauftrag in Phase 21:

- **Neue Profilmodule:** Summoner-Necro, Poison-Nova-Necro oder Blizzard-Sorc starten mit ProfileConfig + Modul + genau einem Registry-Eintrag; weitere Runs werden einzeln ergänzt und brauchen Strategy-, Required-Skill- und Availability-Tests.
- **Per-Run Strategy-Einträge:** Ein Profil darf runweise wachsen; fehlende `(profileID, runID)`-Factories bleiben fail-closed mit `profile_run_strategy_unavailable`.
- **„Bindings kopieren“:** BindingEditor-Aktion mit expliziter Preview; kein automatisches Überschreiben bestehender Tasten.
- **Live-Key-Verifikation:** Isolierter „Tasten prüfen“-Flow sendet eine Taste, liest frisches `RightSkillID` und zeigt das Ergebnis; ändert keine Bindings und schreibt keinen Speicher.

## Datenmodell

- `memory.SkillCatalogEntry`: Key, ID, CharClass, Left/Right/InTown/Scroll/Passive
- `memory.PlayerSkills`: `Complete` / `IncompleteReason` neben der bekannten Liste
- `config.ProfileCombatConfig`: `standard_attack`, Intervalle, Distanzwerte
- `config.RequiredSkillConfig`: `skill`, `display_name`
- `profile.RunStrategy` / `app.CombatStrategyRegistry`: ausführbare Profil×Run-Paare
- `app.RightSkillSelector`: pending Select → Snapshot-Confirm → RMB
- `app.OperatorSettings` Schema 3: `profile_bindings`, presence-sensitives `inventory_lock`
- `app.CharacterLoadoutResolver` / `CharacterLoadoutSnapshot`: eingefrorene Runtime-Bindings pro Queue/Workflow

## Operator / CLI

Fehlende Strategies erscheinen als Availability-/Setup-Sperrgrund vor Queue-Start. Unvollständige Profil-Bindings blockieren die Queue mit `profile_bindings_incomplete`. Fehlendes Inventarraster blockiert mit `character_inventory_unconfigured` („Der Inventarschutz wurde noch nicht bestätigt.“). Isolierter Teleportnachweis: `--pathing-test teleport:TX,TY` benötigt DataRoot und Loadout.

## Abhängigkeiten

Lokaler CASC-Extrakt unter `.tmp/d2r-excel`, Go-Generator nach dem Muster der Item-/Monster-/Objektkataloge.

## Verwandte Features

- [Character- und Encounter-Profile](character-encounter-profiles.md)
- [Run-Verfügbarkeit und Inspect](run-availability.md)
- [Input Controller](input-controller.md)
- [Phase-21-Core-Vertrag](phase-21-core-contract.md)

---
*Zuletzt aktualisiert: 2026-08-10*
