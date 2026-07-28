# Pickit-Profile und Assignments

## Überblick

Abschnitt 13.4 ersetzt die drei run-spezifischen NIP-Dateien durch globale, revisionierte YAML-Profile und genau eine lokale Assignment-Autorität pro Charakter und Run. Profile enthalten geordnete Regeln mit stabilen IDs und den Aktionen `keep` oder `sell`; die geordnete Assignment-Kette bleibt First-Match-autoritativ.

## Ort im Code

- **Paket:** `internal/app/`
- **Einstieg:** `NewPickitProfileService`, `NewPickitAssignmentStore`
- **Wichtige Dateien:** `pickit_store.go`, `phase13_contract.go`
- **Profile:** `configs/pickit/profiles/*.yaml`
- **Assignment:** `configs/pickit-assignments.local.yaml`
- **Schema-Beispiel:** `configs/pickit-assignments.example.yaml`

## Funktionalität

### Profilservice

`PickitProfileService` lädt YAML mit strikter Feldprüfung und validiert Schema, positive Revision, unveränderliche Slug-ID, eindeutige Regel-IDs, Aktionen, Parserausdruck und Katalogreferenzen vor jedem Schreiben. Create, Update, Duplicate und Delete verwenden dieselbe Validierung. Neue und duplizierte Profile beginnen bei Revision `1`; eine echte Änderung erhöht genau um eins. Eine idempotent wiederholte, bereits bestätigte Änderung erzeugt keine weitere Revision.

Schreiben erfolgt im Zielverzeichnis über temporäre Datei, Flush, Close, atomischen Replace und anschließendes Re-Read. Ein Fehler vor dem Replace lässt die vorige Datei unverändert. Eine Profil-ID kann nicht umbenannt werden. Delete ist nur möglich, wenn keine Assignment-Liste die ID referenziert.

### Assignment-Store

`PickitAssignmentStore` persistiert eine globale positive Revision und pro `(character, run)` eine nicht leere, geordnete, duplikatfreie Profilliste. Charaktere sind case-insensitiv eindeutig; Run-IDs stammen aus der Run Registry und jede Profilreferenz muss beim Laden und Schreiben existieren. Replace ist revisionsgebunden, atomisch und bei bereits erreichtem Zielzustand idempotent.

Abschnitt 16.2 ergänzt `EnsureMissingDefaults`. Die Operation lädt und validiert das Manifest genau einmal unter dem Storelock, verwendet einen vorhandenen Charakterschlüssel case-insensitiv oder legt die bestätigte Header-Schreibweise neu an und kopiert ausschließlich vollständig fehlende Run-Ketten. Jede vorhandene nicht leere Benutzerkette bleibt strukturell und in ihrer Reihenfolge unverändert. Mehrere Ergänzungen erhöhen die globale Revision zusammen exakt einmal; ohne Ergänzung bleibt auch bei einem idempotenten Retry die Revision unverändert. Ein bereits ungültiges Manifest mit leerer Kette wird nicht repariert.

`Resolve` kompiliert die Profil- und YAML-Regelreihenfolge zu genau einer gemeinsamen Action Policy. Pickup, Stash und Cain/Akara werten denselben First-Match-Gewinner und dessen `keep`- oder `sell`-Aktion aus; eine zweite Sell-Teilpolicy existiert nicht.

### Einmalige Migration

Die versionierten Startprofile sind:

- `gems`: makellose/perfekte Gems und Schädel, `keep`
- `keys`: Key of Terror (`pk1`), Key of Hate (`pk2`) und Key of Destruction (`pk3`), jeweils `keep`
- `countess-standard`: Runen und Rejuvenation Potions, `keep`
- `mephisto-standard`: Exceptional-/Elite-Set/Unique, `sell`
- Das optionale Abnahmeprofil `tal-rasha` ist nach Phase 13 nicht Teil der versionierten Produktprofile. Es kann bei Bedarf über den Editor neu angelegt werden.

Die Entwickler-Defaults für `MrBones` lauten Countess `[gems, keys, countess-standard]`, Mephisto `[gems, mephisto-standard]` sowie Summoner und Nihlathak jeweils `[gems, keys]`. Damit bleibt die charakterisierte Pickup-/Keep-/Sell-Matrix der bestehenden Runs erhalten und beide Key-Runs verwenden dasselbe Profil für `pk1`/`pk2`/`pk3`. Das versionierte Beispiel referenziert ausschließlich vorhandene Produktprofile.

`runs.definitions.*.loot.pickup_file`, `sell_file` und die drei alten NIP-Policy-Dateien sind entfernt. Ein altes `loot`-Run-Schema wird mit einem konkreten Migrationshinweis abgelehnt und niemals still als Fallback gelesen.

## Datenmodell

| Typ | Rolle |
|---|---|
| `PickitProfileDocument` | Persistentes Profil mit Schema, Revision, ID, Name und Regeln |
| `PickitProfileRuleDocument` | Stabile Regel-ID, Aktion und kanonischer Ausdruck |
| `PickitAssignmentManifest` | Globale Revision und geordnete Zuordnungen |
| `EffectivePickitPolicy` | Eine kompilierte Action Policy plus Assignment-Revision, Profilrevisionen und Profilreihenfolge |

## Operator / CLI

Die lokale Core-API besitzt CRUD, Validierung, Vorschau, Assignment sowie Import/Export. Vor jeder Session-Run-Generation wird eine neue Policy vollständig kompiliert und erst danach atomar aktiviert; ein Reload-Fehler lässt den vorherigen Snapshot unverändert und stoppt den nächsten Run fail-closed. Passive Diagnosemodi ohne bestätigten Charakterkontext erhalten weiterhin eine leere Policy und lesen keine Legacy-Datei.

## Abhängigkeiten

- `internal/loot` für Parser, Aktionen und Trace
- `internal/world` indirekt über kataloggebundene Parserreferenzen
- `gopkg.in/yaml.v3` für striktes YAML
- vorhandenes Windows-sicheres `writeAtomicYAML`/`replaceFile`

## Grenzen

- Ein laufender Run übernimmt keine Profiländerung; Aktivierung erfolgt ausschließlich an der nächsten validierten Run-Grenze.
- NIP-Import/-Export ist ein begrenzter Transport für unterstützte Ausdrücke und niemals eine persistente zweite Autorität.
- Profilhistorie, Undo, Cloud-Sync und Queue-Positions-spezifische Overrides sind nicht Bestandteil von Phase 13.

## Verwandte Features

- [Phase-13-Core-Vertrag](phase-13-core-contract.md)
- [Pickit Engine](pickit-engine.md)
- [Pickit-API und sichere Run-Grenze](pickit-api.md)
- [Loot Decision Pipeline](loot-decision-pipeline.md)
- [Session-Lifecycle](session-lifecycle.md)
- [Charaktereinrichtung](character-setup.md)

---
*Zuletzt aktualisiert: 2026-07-28*
