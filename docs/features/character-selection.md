# Read-only Charakterkatalog und Screenshot-gated Selector

## Überblick

Abschnitt 11.4 listet lokale Offline-Charaktere ausschließlich anhand regulärer `*.d2s`-Dateinamen und wählt einen freigegebenen Charakter über eine streng begrenzte, bildbestätigte State-Machine. Abschnitt 11.6 setzt davor eine seiteneffektfreie, revisionsgebundene Lifecycle-Vorschau. Save-Inhalte werden niemals geöffnet oder verändert. Character-Screen-Memory, OCR und Blindklick-Fallbacks sind ausgeschlossen.

## Ort im Code

- **Katalog:** `internal/app/character_catalog.go`
- **Windows Saved Games:** `internal/app/saved_games_windows.go`
- **Selector:** `internal/app/character_selector.go`
- **Bestehender Game-Start:** `internal/app/offline_game.go`
- **API-Projektion:** `internal/api/bootstrap_backend.go`, `internal/api/live_backend.go`
- **Dashboard:** `web/src/app/App.tsx`
- **Anker:** `configs/ui/characters/`, `configs/ui/character-play.png`, `configs/ui/difficulty-dialog.png`

## Funktionalität

### Read-only Katalog

Unter Windows ermittelt der Core `FOLDERID_SavedGames` über `SHGetKnownFolderPath` und betrachtet darunter `Diablo II Resurrected`. Er verwendet nur Dateinamen regulärer `*.d2s`-Dateien. Symlinks, Reparse Points, Verzeichnisse, ungültige Namen und case-insensitive Duplikate werden verworfen. Kein Save wird geöffnet oder geparst.

Der konfigurierte `session.character` bleibt auch bei fehlendem Save sichtbar und dient als Startvorauswahl, aber nicht als Allowlist. Das ist für einen frischen installierten Root erforderlich, dessen Vorauswahl absichtlich leer ist und erst im Onboarding getroffen wird. Ein Charakter ist nur auswählbar, wenn sein reguläres Save existiert, der aktuelle Run einen gültigen Combat-Profil-/Klassenkontext liefert und alle globalen sowie charakterbezogenen versionsgebundenen PNG-Bestätigungen gültig sind. Erst eine gültige charakterbezogene Bestätigung bindet den Save-Namen an den aktuellen unterstützten Profilkontext; ein bloß sichtbarer fremder Save erbt nicht stillschweigend die Klasse des aktuellen Runs. Stabile Sperrgründe sind:

- `character_save_missing`;
- `character_unconfigured`;
- `character_anchor_missing`.

Unvorbereitete Charaktere bleiben im Dashboard und Onboarding mit deutsch erklärtem Sperrgrund sichtbar. Die Oberfläche zeigt keine internen Begriffe oder Reason-Codes, sondern erklärt beispielsweise „Kein unterstütztes Kampfprofil zugeordnet“, nennt die derzeit unterstützte Klasse und trennt dies von „Automatische Auswahl dieses Charakters in D2R ist noch nicht eingerichtet“. Da Save-Inhalte auch für die First-Run-Auswahl nicht gelesen werden, behauptet die App für einen nicht eingerichteten Save keine automatisch erkannte Klasse. Fehlende Farming-Routen sperren später den Queue-Start, nicht die reine D2R-Auswahl.

### Offene Folgearbeit: geführte Charaktereinrichtung

Die Charaktereinrichtung soll ohne zusätzlichen Wizard in den vorhandenen Charakterschritt integriert werden. Für jeden lokalen Charakter muss sie mindestens die Klasse und ein kompatibles Standard-Kampfprofil persistent zuordnen. Die Basisinstallation soll pro unterstützter Klasse ein vorkonfiguriertes Standardprofil mitbringen; beim ersten Einrichten wird dieses Profil automatisch gewählt. Später darf der Benutzer in den Einstellungen ein anderes kompatibles Profil auswählen beziehungsweise die Profildetails bearbeiten.

Zur vollständigen Run-Readiness gehört außerdem eine initiale Pickit-Zuordnung. Die Basisinstallation enthält bereits globale Standard-Lootprofile für Countess und Mephisto; diese sind fachlich primär runbezogen und müssen nicht pro Charakterklasse dupliziert werden. Bei der Charaktereinrichtung soll der Core für jeden unterstützten Run automatisch die vorgesehene Standard-Profilkette als charakterbezogene Zuordnung anlegen. Spätere Änderungen erfolgen weiterhin über den vorhandenen Pickit-Editor. Fehlt ein Default oder ist dessen Profilkette ungültig, bleibt nur der betroffene Run mit verständlichem Grund gesperrt.

Als bevorzugte Verbesserung ist eine eng begrenzte read-only Erkennung der Charakterklasse aus dem D2S-Header zu untersuchen. Sie darf ausschließlich die für Name/Version/Klasse nötigen Headerfelder lesen, niemals schreiben und keine Items, Skills oder Statistiken auswerten. Reguläre Datei, Reparse-Schutz, maximale Dateigröße, unterstützte Save-Versionen, feste Headergrenzen und synthetische Testfixtures sind vor einer Umsetzung verbindlich festzulegen. Das wäre eine bewusste Änderung des bisherigen Vertrags „Save-Inhalte werden nicht geöffnet“ und benötigt deshalb einen eigenen geprüften Implementierungsabschnitt samt Aktualisierung der Architekturunterlagen.

Damit kann der bestehende Onboarding-Schritt sofort zwischen „unterstützte Klasse mit verfügbarem Standardprofil“, „nicht unterstützte Klasse“, „Klasse nicht sicher ermittelbar“ und „Standardprofil fehlt“ unterscheiden. Ein kompletter Charakter-Wizard ist dafür nicht nötig. Die weiterhin erforderliche visuelle, namensgebundene D2R-Bestätigung wird im selben Schritt als kurze manuelle Einrichtung angeboten: Benutzer markiert den Charakter auf dem Offline-Charakterbildschirm, der Core erzeugt beziehungsweise bestätigt die begrenzte Auswahlreferenz und prüft sie vor dem ersten Spieleintritt. Erst Klasse, Profil und sichere Auswahl zusammen machen einen Charakter auswählbar.

### Begrenzter Selector

Vor dem ersten Input verlangt der Core:

- gebundenes D2R-Fenster und Input-Opt-in;
- exakt 1280×720 Clientfläche;
- eine eindeutige Zwei-Anker-Klassifikation, bei der der Character-Play-Anker mit Sicherheitsabstand besser als der Difficulty-Dialog-Anker passt;
- einen freigegebenen Zielanker aus dem unveränderten Katalog.

Der Core aktiviert anschließend zuerst das gebundene D2R-Fenster und wartet erneut auf UI-Settle, bevor er den ersten Screenshot erfasst. Dadurch darf das Dashboard beim Klick im Vordergrund stehen, ohne dass dessen Pixel irrtümlich gegen den D2R-Anker geprüft werden. Danach sendet der Selector genau einmal `Home`, wartet erneut auf UI-Settle und sendet höchstens `character_count - 1` einzelne `Down`-Tasten. Drei stabile Treffer des Zielankers sind nötig. No-Match und Timeout enden ohne Play-Klick.

Der allgemeine Anker-Grenzwert ist für positive Erkennung bewusst tolerant. Er darf deshalb nicht umgekehrt als Abwesenheitsnachweis verwendet werden: Auf den dunklen Frontend-Flächen können Play- und Dialog-Anker isoliert beide unter dem Grenzwert liegen. Der Selector vergleicht stattdessen beide Scores und akzeptiert nur einen klaren Play-Sieg; ein klarer Dialog-Sieg oder ein uneindeutiges Ergebnis bricht vor `Home` ab. Der spätere bewährte Difficulty-Flow bestätigt den Dialog weiterhin positiv nach dem Play-Klick.

Nach dem Treffer übernimmt der unveränderte Phase-7.3-Flow: Ziel- und Play-Anker bestätigen vor dem Play-Klick, der Difficulty-Dialog bestätigt vor genau einem Difficulty-Klick und Memory bestätigt anschließend Name, erwartete Klasse und Rogue Encampment. F11 und Prozess-Cancellation bleiben auch während der Listen-Navigation wirksam.

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
- Kein Lesen oder Schreiben von Save-Inhalten.
- Kein Difficulty-Commit ohne aktuelle Vorschau, explizite Bestätigung der Auswirkung und Memory-bestätigten Spieleintritt.
- Kein Fallback ohne gültigen Zielanker.

## Verwandte Features

- [Verifizierter Offline-Game-Start](offline-difficulty-selection.md)
- [Lokale Core-API](local-core-api.md)
- [Phase-11-Core-Vertrag](phase-11-core-contract.md)

---
*Zuletzt aktualisiert: 26. Juli 2026*
