# Phase-16-Core-Vertrag

## Überblick

Abschnitt 16.0 friert die Phase-15-Charakterauswahl ein und legt die Grenzen für das sichere Charakter-Setup fest. Der Go-Core darf aus aktuellen lokalen D2R-Saves ausschließlich Magic, Save-Version, Name und Klasse lesen. Klasse und gewähltes Kampfprofil werden anschließend charakterbezogen im bestehenden `OperatorSettingsStore` autoritativ; Pickit-Zuordnungen bleiben im `PickitAssignmentStore`.

Der compile-nahe Vertrag definiert die belegten Offsets, Autoritäten, Reason-Codes, Defaults und Nicht-Ziele. Die Abschnitte 16.1 bis 16.5 implementieren darauf den produktiven Präfixleser, den revisionierten Katalog, Profilmetadaten, OperatorSettings Schema 2, die schmale Pickit-Defaultoperation, Setup-/Capture-API, verständliche Onboarding-Oberfläche und die erneuten Laufzeit-Gates. Die automatische Matrix von 16.6 ist grün; die manuelle Produktabnahme bleibt das Abschlussgate.

## Ort im Code

- **Compile-naher Vertrag:** `internal/app/phase16_contract.go`
- **Vertragstests:** `internal/app/phase16_contract_test.go`
- **Baseline-Charakterisierung:** `internal/app/phase16_characterization_test.go`
- **Kanonische Klassenwerte:** `internal/world/game_identity.go`
- **Bestehender Charakterkatalog:** `internal/app/character_catalog.go`
- **Bestehende Persistenz:** `internal/app/operator_settings.go`, `internal/app/pickit_store.go`
- **Config:** `configs/config.example.yaml`, `configs/pickit-assignments.example.yaml`

## Ownership

| Verantwortung | Einziger Owner | Harte Grenze |
|---|---|---|
| D2S-Dateizugriff und Headerauswertung | `internal/app` | Begrenzter read-only v105-Präfix ohne externe Parserdependency |
| Klassen-ID-Zuordnung | `internal/app` mit `world.CharacterClass` | Keine zweite Enumeration oder UI-Zuordnung |
| Profilfreigabe und Entwickler-Default | `config.ProfileConfig.Setup` | Metadaten direkt am bestehenden Kampfprofil |
| Gewähltes Charakterprofil | `OperatorSettingsStore` | Charakterbezogenes Paar aus Klasse und Profil |
| Pickit-Zuordnung | `PickitAssignmentStore` | Geordnete Kette pro Charakter und Run |
| Auswahlbild | bestehender Go-Core-PNG-Writer | Configroot und fester 210×60-Bereich |
| Safety und Input | bestehender Go-Core | Supervisor und Guarded Input Controller |
| Benutzertexte | zentrale React-Projektion | Exhaustive Zuordnung; keine Rohcodes |

`Phase16ContractOwners` hält diese Grenzen compile-nah fest.

## Belegter D2S-v105-Präfix

Koolo und d2go enthalten keinen D2S-Parser; beide Projekte lesen den laufenden Spielprozess beziehungsweise stellen Memory-/Spieldaten bereit. Die alte 41-Byte-Annahme stammt aus dem vor v104 verwendeten Headerlayout. Drei aktuelle unabhängige Parserimplementierungen bestätigen für D2R v105 die verschobene Klasse und den in den Previewbereich verschobenen Namen:

- Savecraft: `plugins/d2r/d2s/header.go`
- D2RuneWizard/d2s: `src/d2/versions/default_header.ts`
- D2SSharp: `src/D2SSharp/Model/Character.cs` und `PreviewData.cs`

Die zusätzliche v105-Spezifikation von d2rr-toolkit dokumentiert dieselben Offsets. Der produktive Vertrag bleibt trotzdem bewusst kleiner als diese Parser.

| Offset | Länge | Feld | Phase-16-Verhalten |
|---|---:|---|---|
| `0x00` | 4 | Magic, little-endian | Exakt `0xAA55AA55` |
| `0x04` | 4 | Save-Version, little-endian | Exakt eine erlaubte Version: `105` |
| `0x08` | 4 | deklarierte Dateigröße | Eingelesen, nicht ausgewertet |
| `0x0C` | 4 | Checksumme | Eingelesen, nicht geprüft |
| `0x10` | 4 | aktive Waffe | Ignoriert |
| `0x14` | 1 | Status | Ignoriert |
| `0x15` | 1 | Progression | Ignoriert |
| `0x16` | 2 | reserviert | Ignoriert |
| `0x18` | 1 | Klassen-ID | `0…7` werden auf `world.CharacterClass` abgebildet |
| `0x12B` | 16 | erster Namensslot | Nicht leer, innerhalb des Slots NUL-terminiert, gültige Projektnamenssyntax |

Der begrenzte Reader benötigt deshalb exakt `0x12B + 16 = 315` Byte. Er wertet nach dem ersten NUL keine weiteren Namensbytes aus, liest nicht über den Präfix hinaus und prüft weder Checksumme noch Gesamtdatei. Eine Datei mit weniger als 315 Byte ist ungültig.

Dateiname und Headername müssen `strings.EqualFold` erfüllen. Die Header-Schreibweise wird angezeigt; Suche und Persistenzschlüssel bleiben case-insensitiv. Unbekannte Versionen oder Klassen werden nicht geraten.

Windows löst die Suchwurzel primär über `FOLDERID_SavedGames` auf. Die lokale Charakterisierung zeigte einen echten Standardordner bei gleichzeitig fehlender Known-Folder-Registrierung (`FILE_NOT_FOUND`). Nur für diesen Fall ist `FOLDERID_Profile\Saved Games` als vorhandener regulärer, reparse-freier Standardpfad zulässig. Registrierte oder umgeleitete Saved-Games-Pfade behalten Vorrang; Environment-, Registry- und frei konfigurierbare Alternativen sind ausgeschlossen.

### Lokale Charakterisierung

Am 27. Juli 2026 wurden nach ausdrücklicher Freigabe und bei geschlossenem D2R die ersten 315 Byte der drei authentischen lokalen Saves read-only gelesen. Magic, Version, Headername und Klassen-ID wurden validiert; weder Saves noch Kopien oder Binärfixtures entstanden.

| Charakter | Save-Version | Klassen-ID | Klasse |
|---|---:|---:|---|
| MrBones | 105 | 2 | `necromancer` |
| MrHammer | 105 | 3 | `paladin` |
| MrBook | 105 | 7 | `warlock` |

Damit ist ausschließlich Version 105 freigegeben. `CharacterClassWarlock` verwendet den kanonischen Wert `7` und projiziert `warlock`.

## Profil- und Pickit-Defaults

Nur explizit mit `setup.enabled: true` freigegebene Kampfprofile sind im Charakter-Setup sichtbar. Jede Klasse mit mindestens einem freigegebenen Profil besitzt genau einen Entwickler-Default. Zunächst ist ausschließlich `necromancer` mit `necro_bone_spear` fachlich unterstützt; bekannte andere Klassen bleiben sichtbar und gesperrt.

`Phase16DefaultPickitChains` hält die fehlenden Run-Defaults fest:

- Countess: `gems`, `keys`, `countess-standard`
- Mephisto: `gems`, `mephisto-standard`

Eine vorhandene nicht leere Benutzerkette wird niemals ergänzt, sortiert oder ersetzt.

## Reason-Codes

`Phase16CharacterReasonCodes` definiert genau:

- `character_save_missing`
- `character_save_unreadable`
- `character_save_header_invalid`
- `character_save_version_unsupported`
- `character_save_name_mismatch`
- `character_save_name_conflict`
- `character_class_unknown`
- `character_class_unsupported`
- `character_profile_missing`
- `character_profile_incompatible`
- `character_profile_run_incompatible`
- `character_anchor_missing`
- `character_anchor_exists`

Die Auswertung bleibt fail-closed und stufenweise: Ein Save-/Headerfehler erzeugt keine abgeleiteten Klassen- oder Profilgründe; eine unbekannte beziehungsweise nicht unterstützte Klasse erzeugt keinen Profilgrund.

## OperatorSettings Schema 2

Schema 2 ergänzt pro Charakter `character_class` und `combat_profile`. Beide Felder sind gemeinsam leer oder gemeinsam gesetzt. Ein gesetztes Paar muss zum frisch gelesenen Header und zu einem freigegebenen Profil passen.

Schema 1 wird weder gelesen noch migriert. Nur der dedizierte Setup-Pfad darf das Paar gemeinsam atomar ändern. Die allgemeine Settings-Mutation projiziert es read-only und lehnt Änderungen ab. PreviewReset und Reset bewahren beide Werte, während sie die bisherigen Operatorwerte auf sichere Defaults setzen.

## Setup und Schreibreihenfolge

Preview ist read-only und entsteht aus frisch gelesenem Save, Katalogrevision, OperatorSettings-Revision, Pickit-Revision und validierter Basisconfig. Confirm prüft Token, Command-ID, Generation, Idle-/Workflowzustand und alle Revisionen vor dem ersten Write.

Die Reihenfolge ist verbindlich:

1. Save, Profil und alle Gates erneut prüfen.
2. Klasse und Profil gemeinsam atomar in OperatorSettings schreiben.
3. Fehlende Pickit-Defaults in genau einem atomaren Assignment-Write ergänzen.
4. Stores erneut lesen und Katalog neu aufbauen.

Scheitert der zweite Write, bleibt der Charakter sicher profiliert und der betroffene Run wegen fehlender Pickit-Zuordnung gesperrt. Ein Retry ist idempotent; es gibt kein Cross-Store-Journal und keinen Rollback.

## Capture- und Runtimegrenze

Capture verwendet ausschließlich den bestehenden 1280×720-Screenshotpfad und den Bereich `(1035,48)–(1245,108)`. Der Benutzer bestätigt den aktuell markierten Charakter. Der Core prüft Version, Fenster, Screen, Input und Workflowzustand, fokussiert D2R und schreibt das PNG atomar. Capture sendet kein Home, Down, Enter, Play oder anderes Navigationsinput und überschreibt keinen gültigen 210×60-Beleg.

D2S bleibt Setup- und Preflightquelle. Nach Spieleintritt bestätigt der bestehende Memory-Flow weiterhin Name und Klasse. Produktive Desktoppfade dürfen Klasse oder Profil niemals aus `session.run`, Queue, Route oder aktivem Run ableiten. CLI-Probe-, Test- und Recorderpfade ohne OperatorSettings behalten ihren ausdrücklich configgebundenen Profilpfad.

## Quellenaudit der Phase-15-Semantik

Der Audit aus 16.0 ist abgeschlossen:

- `configuredCharacterClass`: Funktion und Katalogaufruf sind entfernt. Produktive Desktoppfade verwenden den frisch gelesenen Saveheader und das gespeicherte Profil; bewusst operatorlose CLI-/Diagnosepfade verwenden weiterhin ihre konkrete Config.
- `CharacterReasonUnconfigured`: Core-Konstante, Katalogprojektion und React-Sondertext sind entfernt. Die Oberfläche übersetzt die präzisen Phase-16-Gründe zentral.
- `ExpectedClass`: `CharacterCatalogEntry`, `CharacterSelectionRequest`, API-DTO/-Backend, `RecordingPreflight` und `RecordingCoordinator`.
- Direkte Run-Profilverwendung: `internal/app/profile.go`, `run_mode.go`, `town_preparation.go`, `run_availability.go`, `guided_route_record.go` sowie Configvalidierung in `internal/config/config.go`.
- 210×60-Beleg und Crop: Größenprüfung in `internal/app/character_catalog.go`; Crop in `character_selector.go` und `offline_game.go`.
- OperatorSettings-Schema: Der in 16.0 gefundene Schema-1-Pfad ist in 16.2 vollständig durch `OperatorSettingsSchemaVersion = 2` ersetzt; es existiert kein Schema-1-Decoder und kein Lifecycle-Migrationsaufruf mehr.

## Stand Abschnitt 16.1

`internal/app/character_save.go` implementiert den produktiven read-only Reader mit genau einem 315-Byte-`io.ReadFull`, Versionsallowlist, Namensabgleich und kanonischem Klassenmapping. `character_catalog.go` isoliert Einzelfehler, lehnt Namenskollisionen global ab und veröffentlicht über `CharacterCatalogStore` nur vollständig gelesene immutable Projektionen. Identischer Reload behält die Revision; Änderung erhöht sie einmal; Fehler bewahrt den letzten erfolgreichen Stand.

Die Memory-Identitätsgrenze akzeptiert nun ebenfalls `0…7` und weist `8` weiterhin ab. Bis Schema 2 in 16.2 existiert, ist ein valider Necromancer bewusst `character_profile_missing`; bekannte andere Klassen sind `character_class_unsupported`. Kein Katalogeintrag erbt noch Klasse oder Profil aus Run, Queue oder Route.

## Stand Abschnitt 16.2

`ProfileConfig` trägt nun den deutschen Anzeigenamen und die kleine Freigabe aus `ProfileSetupConfig`. Die vollständige Configvalidierung erzwingt gültige Klassen, Anzeigenamen, `default ⇒ enabled` und exakt einen Entwickler-Default pro freigegebener Klasse. Die festen Countess-/Mephisto-Ketten werden zusätzlich gegen die Run Registry, ihre exakte Reihenfolge und tatsächlich ladbare Pickit-Profile geprüft.

`OperatorSettingsStore` unterstützt ausschließlich Schema 2. Klasse und Profil sind gemeinsam leer oder gemeinsam gesetzt; ein gesetztes Paar muss ein bekanntes, freigegebenes und klassenkompatibles Profil referenzieren. Allgemeine Settings-Mutationen dürfen weder Werte noch Charakterschlüssel dieser Projektion ändern. Der dedizierte gemeinsame Write kann einen neu gefundenen Charakter mit sicheren Queue-/Difficulty-Defaults anlegen. Reset und Resetvorschau bewahren alle Setup-Paare.

`PickitAssignmentStore.EnsureMissingDefaults` ergänzt nur fehlende Ketten, erhält abweichende Benutzerketten unverändert, ist ohne Änderung revisionsneutral und fasst mehrere Ergänzungen in genau einer Revision und einem atomaren Write zusammen. Schema-1-Settings und der frühere Lifecycle-Auswahlfallback sind entfernt.

## Stand Abschnitte 16.3 bis 16.5

`CharacterSetupService` berechnet Preview, Confirm und Capture aus den bestehenden Stores und Runtimebausteinen. Vier streng dekodierte Endpunkte ergänzen Reload, Setup-Vorschau, Setup-Bestätigung und Auswahlbilderfassung. Fachlich unveränderte Operationen erzeugen kein SSE-Ereignis; eine veränderte Katalogprojektion erzeugt genau ein `catalog_changed`.

Der vorhandene Onboarding-Charakterschritt zeigt Klasse, Unterstützung, Profil, Lootprofile und Bildstatus in Alltagssprache. Ein einzelnes kompatibles Profil wird fest angezeigt; mehrere Profile verwenden ein Dropdown mit vorausgewähltem Entwickler-Default. Nicht unterstützte Klassen erhalten keine falsche Setup- oder Capture-Aktion. Die normale Settings-Seite besitzt keine Profilwahl.

Selection Preview und Apply lesen den Save erneut und verlangen das gespeicherte kompatible Profil. Queue-Validierung, Queue-Start und jeder Supervisor-Run prüfen zusätzlich, dass genau dieses Profil vom angeforderten Run verwendet wird und für diesen Run eine Pickit-Zuordnung existiert. Falsche Klasse, fehlendes oder inkompatibles Profil und Profil-/Run-Mismatch stoppen vor Input; ein fehlendes Pickit-Assignment betrifft nur den angeforderten Run. Nach Spieleintritt bleibt die bestehende Memory-Bestätigung von Name und Klasse autoritativ.

## Nicht-Ziele

`Phase16NonGoals` verbietet insbesondere Vollparser, externe Parserdependency, Checksumprüfung, Save-Mutation, zweite Klassen- oder Persistenzautorität, Legacy-Migration, Cross-Store-Transaktion, Pickit-Merge, Workflowframework, Renderer-Dateizugriff, Profilparametereditor, OCR, Blindklick, Skalierung, Combat-Refactor, Watcher, Headercache sowie echte Saves oder Hexdumps im Repository.

## Verwandte Features

- [Phase-15-Core-Vertrag](phase-15-core-contract.md)
- [Charaktereinrichtung](character-setup.md)
- [Charakterauswahl](character-selection.md)
- [Charakter-Encounter-Profile](character-encounter-profiles.md)
- [First-Run-Onboarding](first-run-onboarding.md)
- [Operator-Einstellungen](operator-settings.md)
- [Pickit-Profile](pickit-profiles.md)
- [Lokale Core-API](local-core-api.md)
- [Run-Verfügbarkeit](run-availability.md)

---
*Zuletzt aktualisiert: 28. Juli 2026*
