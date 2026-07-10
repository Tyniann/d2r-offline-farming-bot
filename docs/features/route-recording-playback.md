# Route Recording und Playback

## Überblick

Phase 6 baut eine generische, run-unabhängige Infrastruktur für manuell aufgezeichnete und deterministisch wiedergegebene Navigationsrouten. Countess ist der erste vollständig integrierte Anwendungsfall: Der Spieler führt den Weg von Black Marsh über den Forgotten Tower bis Tower Cellar Level 5 einmal selbst aus und der Bot spielt ihn anschließend Memory-verifiziert ab.

`Route`, `Segment`, `Waypoint`, `Transition`, `Recorder`, `Registry`, `Validator` und `Player` dürfen keine Countess-spezifische Fachlogik enthalten. Weitere Runs sollen dieselbe Infrastruktur verwenden können, ohne Recorder oder Playback-Engine zu forken.

Der vorhandene Navigator bleibt für kurze lokale Korrekturen, World-to-Screen-Projektion, Hover-Bestätigung und Area-Übergänge zuständig. Bearing-Explore bleibt Diagnose- und expliziter Fallback-Baustein, ist aber nicht Teil des regulären Phase-6-Erfolgspfads.

## Ort im Code

- **Paket:** geplant unter `internal/pathing/`
- **Einstieg:** geplanter CLI-Record-/Playback-Modus in `cmd/d2rbot`
- **Wichtige bestehende Dateien:** `internal/pathing/navigator.go`, `internal/pathing/town_route.go`, `internal/app/pathing_test_mode.go`
- **Config:** verwaltete Routen unter `configs/routes/`; Run-Adapter referenzieren stabile Route-IDs statt Dateipfade

## Funktionalität

### Aufnahme

Der Recorder beobachtet den World-State, während der Spieler die Route manuell durchläuft. Er zeichnet keine unkontrollierte Folge roher Mauspositionen auf, sondern versionierte Segmente mit World-Koordinaten, Area, Bewegungsart und erwarteten Übergängen. Loading-Snapshots und inkonsistente Reads werden nicht als Routenpunkte übernommen.

Die erste Ausbaustufe deckt ausschließlich diese Strecke ab:

1. Black Marsh zum Forgotten Tower.
2. Forgotten Tower zu Tower Cellar Level 1.
3. Tower Cellar Level 1 bis Tower Cellar Level 5.

### Wiedergabe

Der Player prüft vor dem ersten Input die Metadaten und den aktuellen Startzustand. Innerhalb eines Segments arbeitet er die aufgezeichneten World-Koordinaten ab. Übergänge werden nicht zeitgesteuert angenommen, sondern über erwartete Areas und hover-bestätigte Entrance-Interaktionen verifiziert.

Abweichungen führen nur innerhalb enger Grenzen zu einer lokalen Korrektur durch den Navigator. Eine falsche Area, ein unbekannter Zustand, ein überschrittenes Drift-Limit oder ein Timeout beendet das Playback fail-closed.

### Integration in den Countess-Run

Nach isolierter Live-Validierung ersetzt Playback die Explorer-Schritte `find_tower` und `enter_cellar_1` bis `enter_cellar_5` im regulären Countess-Run. Fehlt eine passende Aufnahme, startet der Bot nicht stillschweigend eine globale Erkundung. Der Operator muss Explorer-Fallback ausdrücklich als Test- oder Diagnosemodus wählen.

### Wiederverwendung und spätere GUI

Eine Route beschreibt ausschließlich Navigation. Kampf, Loot, Town und Stash bleiben fachliche Run-Schritte. Eine spätere Run-Definition kann eine oder mehrere Routen über stabile IDs referenzieren und mit anderen Task-Schritten kombinieren.

Die Registry bietet bereits ohne GUI eine listen- und filterbare Sicht auf Route-ID, Anzeigename, Tags, Charakter, Schwierigkeit, Spielversion, Aufnahmezeitpunkt und Validitätsstatus. Eine spätere GUI kann darauf aufbauen, um Aufzeichnungen auszuwählen, umzubenennen, zu duplizieren, neu aufzunehmen oder Run-Definitionen beziehungsweise Playlists zuzuordnen.

## Datenmodell

Das endgültige Schema wird in Phase 6.0 festgelegt. Es muss mindestens enthalten:

- Formatversion, stabile Route-ID, Anzeigename, Routentyp und optionale Tags.
- Charakter- und Schwierigkeitsbindung.
- D2R-Spielversion sowie eine belastbare Layout-/Startzustandsprüfung.
- Geordnete Segmente mit Start- und Ziel-Area.
- Ausgedünnte World-Koordinaten und Bewegungsart.
- Erwartete Entrance-Art und Verifikationsbedingungen.
- Aufnahmezeitpunkt und optionale Diagnosemetadaten.

Unit-IDs sind laufzeitabhängig und daher keine alleinigen persistenten Routenanker.

## Operator / CLI

Phase 6 soll getrennte, explizite Oberflächen für Aufnahme, isoliertes Playback und den integrierten Countess-Run bieten. Die konkreten Flag-Namen werden erst mit dem CLI-Vertrag festgelegt. Pause und Stop müssen in Aufnahme und Wiedergabe jederzeit wirksam sein.

Zusätzlich muss eine CLI-Listenansicht dieselben Verwaltungsmetadaten liefern, die später eine GUI konsumiert. Core-APIs dürfen weder CLI-Strings noch GUI-Annahmen enthalten.

Jede Playback-Aktion loggt Ziel, Grund, erwarteten Zustand und Ergebnis. Eine Aufnahme wird zunächst in eine temporäre Datei geschrieben und erst nach erfolgreicher Strukturvalidierung veröffentlicht.

## Herausforderungen und Sicherheitsgrenzen

- Offline-Layouts bleiben nur unter den vereinbarten Bedingungen stabil; Charakter- oder Schwierigkeitswechsel dürfen keine fremde Route auswählen.
- Replay darf keine zeitbasierte Blindwiedergabe von Eingaben sein.
- Loading, unstabile Snapshots und UI-Phasen dürfen keine Bewegung auslösen.
- Positionsdrift braucht enge Korrekturgrenzen und ein hartes Abbruchlimit.
- Aufzeichnungen müssen nach Schema- oder Spielversionsänderungen explizit invalidiert werden können.
- D2R-Installations- und Savegame-Dateien werden niemals verändert.

## Abnahme

Phase 6 ist abgeschlossen, wenn:

1. eine Route Black Marsh → Tower Cellar Level 5 vollständig aufgezeichnet und validiert werden kann;
2. jedes Segment isoliert wiedergegeben und über Area-/Entrance-Signale bestätigt wird;
3. mindestens zehn vollständige Playbacks im gebundenen Singleplayer-Layout ohne globale Zufallserkundung erfolgreich sind;
4. falscher Startzustand, falsche Route, Drift, Timeout, Pause und Stop kontrolliert und ohne nachlaufende Inputs enden;
5. der reguläre Countess-Run die passende Aufnahme verwendet und bei fehlender Aufnahme fail-closed endet.
6. eine zweite synthetische Route ohne Änderung an Recorder, Registry, Validator oder Player geladen und abgespielt werden kann.

## Nicht Teil von Phase 6

- autonomes Verlassen und Erstellen von Spielen;
- Multi-Run-Dauerbetrieb;
- die fachliche Integration zusätzlicher Farmziele; ihre spätere Nutzung der generischen Infrastruktur ist ausdrücklich vorgesehen;
- Map-Server, Seed-basierte Kartengenerierung oder vollständiger Tower-Solver;
- Identify-, Vendor-, Repair- oder Mercenary-Automation.

## Verwandte Features

- [Pathing](pathing.md)
- [Countess-Run](countess-run.md)
- [Task Runner](task-runner.md)
- [Run-Telemetrie](run-telemetry.md)

---
*Zuletzt aktualisiert: 2026-07-10*
