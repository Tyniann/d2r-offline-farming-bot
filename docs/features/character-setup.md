# Charaktereinrichtung

## Überblick

Die Charaktereinrichtung verbindet einen read-only erkannten D2R-Offlinespielstand mit genau einem vom Entwickler freigegebenen Kampfprofil und den fehlenden Standard-Lootprofilen. Sie läuft vollständig im bestehenden Charakterschritt des First-Run-Assistenten. Der Go-Core bleibt die einzige Autorität für Saves, Profilkompatibilität, Defaults, Persistenz und Bildbeleg; React zeigt nur die gelieferten Zustände verständlich an.

Ein Kampfprofil ist nur dann auswählbar, wenn es direkt `setup.enabled: true` trägt. Für jede Klasse mit mindestens einem freigegebenen Profil muss die Basisconfig genau ein Profil mit `setup.default: true` festlegen. Bei nur einem kompatiblen Profil gibt es keine Auswahl. Bei mehreren Profilen wählt die Oberfläche den Entwickler-Default vor. Die allgemeinen Einstellungen enthalten bewusst keine Kampfprofilsteuerung.

## Ort im Code

- **Katalog und Saveheader:** `internal/app/character_catalog.go`, `internal/app/character_save.go`
- **Setup-Orchestrierung:** `internal/app/character_setup.go`
- **Bildbeleg:** `internal/app/character_capture.go`
- **Persistenz:** `internal/app/operator_settings.go`, `internal/app/pickit_store.go`
- **API:** `internal/api/character_setup_backend.go`, `internal/api/character_setup_dto.go`, `internal/api/server.go`
- **Maschinenvertrag:** `internal/api/schema/openapi.json`
- **Oberfläche:** `web/src/features/onboarding/OnboardingFeature.tsx`, `web/src/features/characters/CharacterSetupWizard.tsx`, Settings-Tab „Charaktere“
- **Benutzertexte:** `web/src/app/characterReasons.ts`
- **Config:** `configs/config.example.yaml` (Profilfreigabe); Bindings/Inventar in OperatorSettings Schema 3

## Funktionalität

### Read-only Vorschau

`POST /api/v1/characters/setup/preview` lädt den Spielstandkatalog frisch und liefert für genau einen sichtbaren Charakter:

- Headerklasse und verständlichen Klassennamen;
- alle ausdrücklich freigegebenen, klassenkompatiblen Kampfprofile;
- Entwickler-Default und bereits gespeichertes Profil;
- aktuelle Pickit-Zuordnungen und ausschließlich vollständig fehlende Defaults;
- Vorhandensein des namensgebundenen Auswahlbildes;
- Katalog-, OperatorSettings-, Pickit- und Runtimegeneration für einen konfliktfreien Confirm.

Unterstützung wird ausschließlich aus den Profilmetadaten abgeleitet. Ein bekanntes Saveformat allein macht eine Klasse nicht lauffähig. In der Basisinstallation ist zunächst nur der Totenbeschwörer mit `necro_bone_spear` („Knochen-Speer“) freigegeben. Warlock mit Klassen-ID `7` wird korrekt als „Hexenmeister“ erkannt, bleibt ohne freigegebenes Profil aber wie Paladin sichtbar und gesperrt.

### Einrichtung bestätigen

`POST /api/v1/characters/setup/confirm` verlangt Control-Token, eindeutige Command-ID sowie die unveränderten Revisionen und die Runtimegeneration aus der Vorschau. Vor dem ersten Write werden Saveheader, Klasse, Profilfreigabe und alle Zustände erneut geprüft.

Die feste Reihenfolge lautet:

1. `character_class` und `combat_profile` gemeinsam atomar in OperatorSettings speichern.
2. Nur vollständig fehlende Pickit-Zuordnungen in einem atomaren Assignment-Write ergänzen.
3. Beide Stores neu lesen und den Charakterkatalog neu projizieren.

Vorhandene Benutzerzuordnungen werden weder ergänzt noch sortiert noch überschrieben. Scheitert der zweite Store, bleibt das gültige Charakterprofil bestehen und nur der betroffene Run bleibt wegen seiner fehlenden Pickit-Zuordnung gesperrt. Ein erneuter identischer Confirm ist sicher und revisionsneutral.

### Auswahlbild erfassen

Fehlt der Bildbeleg, zeigt die Oberfläche klare Schritte: D2R starten, Offline-Charakterauswahl öffnen, den genannten Charakter markieren, markiert lassen, zur App zurückkehren und die Erfassung bestätigen. Erst nach der ausdrücklichen Checkbox-Freigabe wird der Command aktiv. Ein frisches Default-Bundle enthält absichtlich keinen namensgebundenen Charakterbeleg; nur ein expliziter Benutzer-Capture oder ein bewusst importierter bestehender Datenroot darf ihn bereitstellen.

`POST /api/v1/characters/selection/capture` verlangt einen inaktiven Supervisor, kompatibles D2R, aktivierten Input, das gebundene Fenster mit exakt 1280×720 Clientfläche und den eindeutig bestätigten Charakterbildschirm. Der Core fokussiert D2R, sucht in den neun sichtbaren 210×60-Listenzeilen den goldenen Auswahlrahmen und veröffentlicht genau diese Zeile als valide PNG-Datei atomar im Configroot. Dadurch darf der markierte Charakter an beliebiger sichtbarer Listenposition stehen. Der Core sendet dabei keine Navigations-, Auswahl-, Play- oder Difficulty-Eingabe und überschreibt keinen bereits gültigen Beleg.

### Kataloginvalidierung

Ein fachlich unveränderter Reload, Confirm oder Capture erzeugt kein Ereignis. Ändert sich die Katalogprojektion, veröffentlicht der Core genau ein `catalog_changed`-Event. React lädt daraufhin Katalog, Setup-Vorschau, OperatorSettings und Recording-Voraussetzungen neu, ohne den aktuellen Assistentenschritt zu verlassen. Es gibt keinen Watcher oder Hintergrundcache.

## Datenmodell

- `ProfileSetupConfig`: `enabled` und `default` direkt am bestehenden Kampfprofil.
- `OperatorCharacterSettings`: autoritatives Paar `character_class` und `combat_profile`.
- `CharacterSetupPreview`: Core-berechnete Profile, Default, Zuordnungen, fehlende Teile und Revisionen.
- `CharacterCatalogEntry`: read-only Savezustand, erkannte Klasse, gespeichertes Profil, Bildbeleg und Sperrgründe.
- `PickitAssignmentStore`: geordnete Profilkette pro `(character, run)`.

Stabile technische Gründe bleiben Teil des API-Vertrags, werden aber in der Oberfläche ausschließlich in einfachen deutschen Text übersetzt. Rohcodes, Profil-IDs und interne Pfade erscheinen nicht in Fehlermeldungen.

## Operator / Desktop

- „Spielstände neu laden“ führt genau einen begrenzten Scan aus.
- „Profil und Lootprofile bestätigen“ speichert die vom Core validierte Einrichtung.
- „Auswahlbild jetzt speichern“ erfasst nach der Benutzerbestätigung den markierten Eintrag.
- Der Assistent bleibt bei Fehlern im Charakterschritt und zeigt eine konkrete nächste Handlung.
- Die normalen Einstellungen bieten keine Profil- oder Defaultauswahl.

Vor einer produktiven Auswahl sowie vor Queue-Validierung, Queue-Start und jedem Supervisor-Run wird der Save erneut gelesen. Headerklasse, gespeichertes Profil, Profilfreigabe, Run-Profil und Pickit-Zuordnung müssen weiterhin zusammenpassen. Ein Mismatch stoppt vor Input. CLI-Diagnose- und Testpfade ohne OperatorSettings behalten ihren ausdrücklich configgebundenen Profilpfad.

## Produktabnahme

Die installierte Phase-16-Abnahme vom 28. Juli 2026 bestätigte den vollständigen Ablauf auf einem frischen Datenroot:

- MrBones wurde read-only als Totenbeschwörer erkannt und mit dem Entwickler-Default `necro_bone_spear` sowie den fehlenden Countess-/Mephisto-Pickit-Ketten eingerichtet.
- MrHammer blieb als Paladin und MrBook mit der empirisch bestätigten Klassen-ID `7` als Hexenmeister sichtbar, verständlich gesperrt und ohne Setup- oder Capture-Aktion.
- Der frische Default-Bundle enthielt keinen namensgebundenen Charakterbeleg. Der explizite Capture speicherte die tatsächlich gold markierte MrBones-Zeile unabhängig von ihrer Position in der sichtbaren Liste.
- Die spätere Auswahl ignorierte volatile Level- und Klassentexte, verlangte aber Namenszeile und goldenen Auswahlrahmen derselben Zeile vor dem Play-Klick sowie Name und Klasse aus Memory nach Spieleintritt.
- Setup und Auswahl überlebten den kontrollierten Core-Neustart; Mephisto erreichte den grünen Preflight.
- Ein produktiver Countess-Run verwendete nachweislich dasselbe Profil, die Route `countess-mrbones-b801e63e3c` und die Pickit-Kette `[gems, keys, countess-standard]`.

## Abhängigkeiten

- Windows Known Folder und read-only Dateizugriff für lokale D2R-Saves.
- Bestehende OperatorSettings- und Pickit-Assignment-Stores.
- Bestehender D2R-Versions-, Fenster-, Screen-, Input- und Workflowvertrag.
- Bestehender Screenshotpfad; keine OCR- oder Parserdependency.

## Grenzen

- Kein vollständiger D2S-Parser und keine Savegame-Schreiboperation.
- Keine Legacy-Migration für OperatorSettings Schema 1.
- Kein zusätzlicher Setup-Store, Watcher, Headercache oder Cross-Store-Journal.
- Kein Kampfprofil-Editor und keine UI-Verwaltung des Entwickler-Defaults.
- Keine vorsorgliche Freigabe unvollständiger oder experimenteller Profile.

## Verwandte Features

- [Phase-16-Core-Vertrag](phase-16-core-contract.md)
- [Charakterauswahl](character-selection.md)
- [Charakter- und Encounter-Profile](character-encounter-profiles.md)
- [Operator-Einstellungen](operator-settings.md)
- [Pickit-Profile](pickit-profiles.md)
- [First-Run-Onboarding](first-run-onboarding.md)
- [Lokale Core-API](local-core-api.md)

---
*Zuletzt aktualisiert: 28. Juli 2026*
