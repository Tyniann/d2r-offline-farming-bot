# Read-only Charakterkatalog und Screenshot-gated Selector

## Überblick

Abschnitt 16.1 listet lokale Offline-Charaktere anhand regulärer `*.d2s`-Dateien und liest read-only exakt den für Magic, Version, Klasse und Namen nötigen 315-Byte-v105-Präfix. Die eigentliche Auswahl bleibt die streng begrenzte, bild- und anschließend Memory-bestätigte State-Machine aus Phase 11. Savegames werden niemals geschrieben oder über den Präfix hinaus gelesen. Character-Screen-Memory, OCR und Blindklick-Fallbacks sind ausgeschlossen.

## Ort im Code

- **Katalog:** `internal/app/character_catalog.go`
- **D2S-Präfixleser:** `internal/app/character_save.go`
- **Windows Saved Games:** `internal/app/saved_games_windows.go`
- **Selector:** `internal/app/character_selector.go`
- **Bestehender Game-Start:** `internal/app/offline_game.go`
- **API-Projektion:** `internal/api/bootstrap_backend.go`, `internal/api/live_backend.go`
- **Dashboard:** `web/src/app/App.tsx`
- **Anker:** `configs/ui/characters/`, `configs/ui/character-play.png`, `configs/ui/difficulty-dialog.png`

## Funktionalität

### Read-only Katalog

Unter Windows ermittelt der Core zuerst `FOLDERID_SavedGames` über `SHGetKnownFolderPath` und betrachtet darunter `Diablo II Resurrected`. Liefert Windows für diesen Known Folder `FILE_NOT_FOUND`, obwohl das lokale Profil den physischen Standardordner besitzt, wird ausschließlich `FOLDERID_Profile\Saved Games` als validiertes reguläres, reparse-freies Fallback verwendet. Ein registrierter oder umgeleiteter Saved-Games-Pfad behält immer Vorrang.

Der Core betrachtet nur direkte reguläre Dateien mit case-insensitiver Erweiterung `.d2s`. Symlinks, Reparse Points, Verzeichnisse und ungültige Namen bleiben unsichtbar. Case-insensitive Namenskollisionen lehnen den gesamten Reload mit `character_save_name_conflict` ab. Nach dem read-only Öffnen werden Handle und Pfad erneut als dieselbe reguläre, reparse-freie Datei bestätigt.

Der Reader verwendet genau ein begrenztes `io.ReadFull` über 315 Byte. Für die allein erlaubte Save-Version 105 gelten Magic `0xAA55AA55`, Klasse bei `0x18` und der erste NUL-terminierte Namensslot ab `0x12B`. Dateiname und Headername müssen case-insensitiv übereinstimmen. Checksumme, deklarierte Größe, Status, Progression und der restliche Save werden nicht ausgewertet. Die acht IDs `0…7` verwenden direkt `world.CharacterClass`; ID 7 ist `warlock`. Auch die Memory-Projektion akzeptiert diese kanonische Obergrenze, während ID 8 fail-closed bleibt.

Ein Fehler eines einzelnen gültig benannten Saves sperrt nur diesen sichtbaren Eintrag. Die Präzedenz unterscheidet fehlend, nicht lesbar, ungültiger Header, nicht unterstützte Version, Namensabweichung und unbekannte Klasse. Ein bekannter Paladin oder Warlock bleibt mit `character_class_unsupported` sichtbar. Ein unterstützter Charakter ohne gespeichertes Profil erhält `character_profile_missing`; ein nicht mehr zur Klasse oder Config passendes gespeichertes Profil erhält `character_profile_incompatible`. Kein Eintrag erbt Klasse oder Profil aus `session.run`.

`CharacterCatalogStore` hält die letzte erfolgreiche immutable Projektion. Ein expliziter Reload liest vollständig neu, behält bei identischem Fachzustand dieselbe Revision und erhöht sie bei Änderung genau einmal. Ein global fehlgeschlagener Reload veröffentlicht keinen Teilstand. Es gibt keinen Watcher, Hintergrundscan oder Headercache.

### Charaktereinrichtung und frische Auswahlprüfung

Das charakterbezogen in OperatorSettings persistierte Profil muss ausdrücklich für Setup freigegeben sein und zur frisch gelesenen Headerklasse passen. Nur dann und mit vorhandenem Auswahlbild ist der Katalogeintrag auswählbar. Setup, Pickit-Defaults und die sichere Erfassung des namensgebundenen Bildbelegs laufen im bestehenden Onboarding-Charakterschritt.

Selection Preview lädt den Savekatalog erneut. Apply wiederholt denselben Reload unmittelbar vor Fokus, Screenshot und erstem Auswahlinput und lehnt eine geänderte Klasse, ein entferntes Profil oder eine stale Katalogrevision ab. Der Selector erhält die erwartete Klasse ausschließlich aus diesem frischen Eintrag. Nach dem Spieleintritt bestätigt Memory weiterhin Name und Klasse.

### Begrenzter Selector

Vor dem ersten Input verlangt der Core:

- gebundenes D2R-Fenster und Input-Opt-in;
- exakt 1280×720 Clientfläche;
- eine eindeutige Zwei-Anker-Klassifikation, bei der der Character-Play-Anker mit Sicherheitsabstand besser als der Difficulty-Dialog-Anker passt;
- einen freigegebenen Zielanker aus dem unveränderten Katalog.

Der Core aktiviert anschließend zuerst das gebundene D2R-Fenster und wartet erneut auf UI-Settle, bevor er den ersten Screenshot erfasst. Dadurch darf das Dashboard beim Klick im Vordergrund stehen, ohne dass dessen Pixel irrtümlich gegen den D2R-Anker geprüft werden. Danach sendet der Selector genau einmal `Home`, wartet erneut auf UI-Settle und sendet höchstens `character_count - 1` einzelne `Down`-Tasten. Die geprüfte 210×60-Zeile folgt dabei mit jedem `Down` der sichtbaren D2R-Auswahl; nach dem letzten sichtbaren Eintrag bleibt sie an der unteren Listenzeile. Für die Charakteridentität wird aus dem Beleg ausschließlich die stabile Namenszeile mit einem eigenen strengen Grenzwert und höchstens drei Pixeln Schrifttoleranz verglichen. Titel, Level und Klassenzeile sind veränderlich und dürfen die Auswahl nicht beeinflussen. Zusätzlich muss dieselbe Zeile den goldenen D2R-Auswahlrahmen tragen. Drei stabile gemeinsame Treffer aus Name und Auswahlrahmen sind nötig. No-Match und Timeout enden ohne Play-Klick.

Der produktive Queue-Start verwendet denselben Selector erneut, wenn noch kein passiv bestätigtes Spiel übernommen werden darf. Er löst den konfigurierten Charakter frisch aus dem lokalen Save-Katalog auf und führt die begrenzte Home-/Down-Suche unabhängig von der zufällig in D2R markierten Zeile aus. Erst der bestätigte Charakteranker erlaubt den bestehenden Play-/Difficulty-Ablauf; nach Eintritt bleiben Character Name, Klasse und Rogue Encampment per Memory autoritativ. Der Queue-Start besitzt keine zweite Navigationslogik und setzt niemals voraus, dass der gewünschte Charakter bereits ausgewählt ist. Nach Save & Exit wartet bereits der Selector vor seinem ersten Screenshot das begrenzte 1,2-Sekunden-Renderfenster ab. Bleiben Play- und Dialog-Anker danach in einem einzelnen Renderframe uneindeutig, prüft er innerhalb des bestehenden Gesamt-Timeouts im normalen 350-ms-Takt erneut; ein klar erkannter Difficulty-Dialog bleibt terminal. Memory-`menu` allein darf weder einen noch im Übergang gezeichneten Charakterbildschirm ablehnen noch Home/Down freigeben. Fehler protokollieren zusätzlich die konkrete Startstufe, bevor der Supervisor den stabilen Grund `game_start_failed` projiziert.

Der allgemeine Anker-Grenzwert ist für positive Erkennung bewusst tolerant. Er darf deshalb nicht umgekehrt als Abwesenheitsnachweis verwendet werden: Auf den dunklen Frontend-Flächen können Play- und Dialog-Anker isoliert beide unter dem Grenzwert liegen. Der Selector vergleicht stattdessen beide Scores und akzeptiert nur einen klaren Play-Sieg; ein klarer Dialog-Sieg bricht vor `Home` ab, während ein uneindeutiger Übergangsframe nur die nächste begrenzte Prüfung abwartet. Der spätere bewährte Difficulty-Flow bestätigt den Dialog weiterhin positiv nach dem Play-Klick.

Nach dem Treffer übernimmt der Phase-7.3-Flow: Namenszeile, Auswahlrahmen und Play-Anker bestätigen unmittelbar vor dem Play-Klick dieselbe gefundene Listenzeile, der Difficulty-Dialog bestätigt vor genau einem Difficulty-Klick und Memory bestätigt anschließend Name, erwartete Klasse und Rogue Encampment. F11 und Prozess-Cancellation bleiben auch während der Listen-Navigation wirksam.

### Zweistufige Auswahl und Lifecycle-Commit

`POST /api/v1/selection/preview` vergleicht Charakter und Difficulty mit dem bestätigten Lifecycle-Kontext. Die Vorschau schreibt nichts, nennt bei einem Difficulty-Wechsel alle betroffenen Farming-Route-IDs und liefert ein zufälliges, nur im Prozessspeicher gehaltenes Confirmation-Token. Das Token bindet Charakter, neue Difficulty, Katalogrevision, Manifestrevision und die vollständige Auswirkung. Eine veränderte Revision wird vor dem ersten D2R-Input mit `selection_confirmation_invalid` abgewiesen.

Ohne Invalidation darf das Dashboard die Vorschau direkt anwenden. Bei `difficulty_changed` zeigt es zuerst einen modalen Dialog mit der vollständigen Routenliste. Abbrechen sendet kein Apply und verändert weder D2R noch Manifest. Nach Bestätigung führt der bestehende Selector den verifizierten Menüablauf aus. Erst wenn Memory den erwarteten Charakter, seine Klasse und den Spieleintritt bestätigt, committed der Core die Lifecycle-Änderung atomisch. Timeout, falscher Charakter und jeder frühere Fehler lassen das Manifest unverändert. Ein Manifest-Write-Fehler bleibt sichtbar und sperrt Farming fail-closed.

Gleiche Difficulty aktualisiert ausschließlich `confirmed_at`. Ein Charakterwechsel invalidiert weder den alten noch den neuen Charakter. Ein bestätigter Difficulty-Wechsel markiert alle Farming-Routen genau des gewählten Charakters als `stale`; Route-Dateien werden nicht verändert.

Vor der ersten Bildschirmprüfung aktiviert der Selector D2R und bestätigt das tatsächliche Foreground-Fenster. Da der Dashboard-Browser beim Klick den Windows-Foreground-Lock besitzt, wird die Aktivierung begrenzt wiederholt und nutzt bei Bedarf verbundene GUI-Input-Queues. Live-Events verändern den Betriebssystemfokus nicht. Ohne bestätigten D2R-Fokus folgen weder Capture noch Home/Down oder Mausklicks.

## API und Dashboard

`GET /api/v1/catalog` liefert `characters`, `difficulties`, `profiles`, `default_difficulty` und die bestehenden Runs. Dateipfade bleiben intern. `POST /api/v1/selection/preview` ist read-only und benötigt keinen Control-Token. `POST /api/v1/selection/apply` verlangt zusätzlich zu Charakter, Difficulty, Katalogrevision, Command-ID und erwarteter Generation das exakt passende Confirmation-Token sowie den Control-Token.

Während der synchron verifizierten Auswahl zeigt der Core `activating_selection`; Erfolg endet in `idle_in_game`, ein sicherer Abbruch wieder in `idle` mit strukturiertem Fehler. Derselbe Command ist idempotent. Der passive UI-Monitor wird für die Auswahl vollständig gestoppt und danach neu gestartet, sodass niemals zwei Poll-/Input-Pipelines konkurrieren.

## Operator / Desktop

D2R muss bereits auf dem Offline-Charakterbildschirm bei 1280×720 stehen. Das Dashboard der installierten App zeigt deaktivierte Charaktere einschließlich Reason-Codes und bietet für den vorbereiteten Kontext „Auswahl in D2R anwenden“.

## Grenzen

- Kein D2R-Start und keine Main-Menu-Automation.
- Keine Auflösung außer 1280×720.
- Kein Lesen über den festen 315-Byte-v105-Präfix und keinerlei Schreiben von Save-Inhalten.
- Kein Difficulty-Commit ohne aktuelle Vorschau, explizite Bestätigung der Auswirkung und Memory-bestätigten Spieleintritt.
- Kein Fallback ohne gültigen Zielanker.

## Verwandte Features

- [Verifizierter Offline-Game-Start](offline-difficulty-selection.md)
- [Lokale Core-API](local-core-api.md)
- [Phase-11-Core-Vertrag](phase-11-core-contract.md)
- [Phase-16-Core-Vertrag](phase-16-core-contract.md)

---
*Zuletzt aktualisiert: 9. August 2026*
