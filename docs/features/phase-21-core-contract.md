# Phase-21-Core-Vertrag

## Überblick

Abschnitt 21.0 friert den CASC-Skillkatalog und den Profilvertrag für Pflichtskills sowie Combat-Metadaten ein. Es ändert weder Runtime-Skillauswahl noch OperatorSettings, Bindings oder Inventar.

Detailplan: [`phase-21-implementation-plan.html`](../plans/phase-21-implementation-plan.html).

## Ort im Code

- **Skillkatalog:** `tools/generate-skill-catalog/`, `internal/memory/generate.go`, `internal/memory/skill_catalog*.go`
- **Profilvertrag:** `internal/config/profile.go`, `configs/config.example.yaml`
- **Feature-Doc:** [character-loadouts.md](character-loadouts.md)
- **Detailplan:** `docs/plans/phase-21-implementation-plan.html`

## Status und Sequenzgrenze

21.0 definiert und prüft:

- generierten Skillkatalog aus lokaler `skills.txt` für D2R `3.2.92777`;
- Canonical Keys inklusive TownPortal-Normalisierung;
- Produktaliase `SkillTeleport`…`SkillTownPortal` und `ParseSkillTestName` als Katalogadapter;
- `combat.standard_attack` sowie geordnete `required_skills` am Bone-Spear-Profil;
- Configvalidierung für Teleport/TP, Standardangriff und Dependency-Subset aus Hooks/Maintenance.

21.0 ändert bewusst nicht:

- Strategy Registry (folgt in 21.1);
- OperatorSettings Schema 3;
- SkillsKnown-Gate oder RightSkillSelector (folgt in 21.2);
- produktiven DataRoot.

21.1 ändert bewusst nicht:

- OperatorSettings Schema 3;
- SkillsKnown-Gate oder RightSkillSelector (folgt in 21.2);
- produktiven DataRoot.

21.2 ändert bewusst nicht:

- OperatorSettings Schema 3 / Charaktere-UI;
- produktiven DataRoot.

## Gate 21.5

Gate 21.5 liefert:

- wiederverwendbaren `CharacterSetupWizard` ohne Core-Fachlogik in React;
- Dashboard-CTA „Charakter einrichten“ getrennt von Selection-Apply;
- Profilwechsel mit Erhalt von Queue, Inventar und inaktiven Bindings;
- Fixture-Tests für einen zweiten unterstützten Charakter.

## Gate 21.6

Gate 21.6 liefert:

- synchronisierte Feature-Docs inklusive Future-Abschnitt in `character-loadouts.md`;
- Backlog-Einträge für Multi-Profil, „Bindings kopieren“ und Live-Key-Verifikation;
- Entfernung von `input.bindings` aus dem Configschema und Beispiel;
- Recording-Prerequisites aus Schema-3-Loadouts;
- vollständige automatische Gesamtmatrix aus Abschnitt 16;
- manuelle Responsive-Visual-Abnahme (1280×720 / 390×844, funktional ausreichend);
- installierten Countess-Referenzzyklus mit MrBones (`countess-20260809t232224…-10b08038`, `outcome=success`, Save & Exit; bewusster Not-Aus erst beim Folgestart).

## Gate 21.4

Gate 21.4 liefert:

- Inventar-Aktivierung im LoadoutResolver / Runtime-Loot;
- Queue-Grund `character_inventory_unconfigured` und Farm-Ready-Anreicherung;
- `InventoryLockEditor` mit „Alle schützen“;
- statische Cow-Warnung `inventory_layout_unsuitable_for_cows`;
- Entfernung von `loot.inventory_lock` aus dem Configschema;
- produktiven Schema-3-DataRoot (Revision 16) für MrBones.

## Gate 21.3

Gate 21.3 liefert:

- OperatorSettings Schema 3 mit `profile_bindings` und presence-sensitivem `inventory_lock`;
- `CharacterLoadoutResolver` und Queue-Freeze ohne Config-`input.bindings`-Autorität;
- statischen Queue-Grund `profile_bindings_incomplete`;
- API/OpenAPI-Transport für Bindings und Inventar;
- Settings-Tab „Charaktere“ mit BindingEditor;
- Katalog-`farm_ready` sowie Onboarding-„Später“ ohne Queue-Freigabe.

## Dirty Worktree vor 21.0

Vor dem ersten Phase-21-Code stand der Worktree sauber auf `master` (`c272039`). `.vscode/extensions.json` war vorhanden und blieb unverändert.

## Belegte Baseline (automatisch)

Am 9. August 2026 vor dem ersten Skillkatalog-Code:

| Bereich | Ergebnis |
|---|---|
| `go test ./...` | grün |
| `go build ./cmd/d2rbot` | grün |
| `web/`: `pnpm generate`, `--check`, `pnpm test`, `pnpm typecheck` | grün (26 Dateien / 171 Tests) |

## Skillkatalog-Vertrag

Autoritative Quelle: `.tmp/d2r-excel/skills.txt`. Keine IDs aus d2go/Koolo.

| Key | ID | Hinweis |
|---|---:|---|
| `teleport` | 54 | Pflicht |
| `amplify_damage` | 66 | Combat |
| `bone_armor` | 68 | Hook / Maintenance |
| `corpse_explosion` | 74 | Cow |
| `bone_wall` | 78 | Alias vorhanden |
| `bone_spear` | 84 | Standardangriff |
| `bone_prison` | 88 | Boss-Hook |
| `town_portal` | 359 | Pflicht; SourceName `TownPortal` |

Doppelte IDs/Keys, fehlende Header oder unlesbare Zeilen stoppen die Generierung. Runtime besitzt keinen TXT-Zugriff.

## Profilvertrag

Für freigegebene Profile:

1. `combat.standard_attack` existiert im Katalog und steht in `required_skills`.
2. `required_skills` ist geordnet, eindeutig, höchstens acht Einträge und besitzt deutsche Labels.
3. `teleport` und `town_portal` sind enthalten.
4. Hook- und aktive Maintenance-Skills sind Teilmenge von `required_skills`.

`necro_bone_spear` migriert die bisherigen Run-Combat-Tuningwerte in `combat_*` und deklariert die sieben Pflichtskills.

## Gate 21.1

Gate 21.1 macht das Charakterprofil zur Runtime-Autorität:

- Strategy Registry besitzt `(necro_bone_spear × countess|mephisto|summoner|nihlathak|cows)`;
- Run-YAML darf keine Combat-Profil-/Attack-Keys mehr tragen;
- Availability und Desktop-Setup prüfen Strategy-Präsenz (`profile_run_strategy_unavailable`);
- Route-`profile_id` wird tolerant gelesen, für Kompatibilität ignoriert und bei neuen Aufnahmen nicht geschrieben.

## Gate 21.2

Gate 21.2 (automatisch bestanden; manueller Teleportnachweis offen) liefert:

- vollständige `PlayerSkills`-Evidenz (`Complete` / `IncompleteReason`) und World-`SkillsKnown`-Clone;
- snapshot-only Queue-Poller für `VerifySameGame` ohne produktiven Input;
- einmaligen Session-Skillgate vor dem ersten produktiven Input (`ExitRequired` bei Missing/Read-Fail);
- gemeinsamen `RightSkillSelector` für Combat, Profile, Teleport und Town Portal;
- Pathing-Teleports mit Selection-Pending bis zur RightSkillID-Bestätigung.

## Gate 21.0

Bestanden, wenn:

- jeder vom Bone-Spear-Profil verwendete Skill ausschließlich aus dem generierten Katalog aufgelöst wird;
- Configvalidierung Teleport/TP, Standardangriff und Dependency-Subset einfriert;
- bestehende Go-/Web-Tests und `go build ./cmd/d2rbot` grün bleiben;
- kein manueller D2R-Test nötig ist.

---
*Zuletzt aktualisiert: 2026-08-09*
