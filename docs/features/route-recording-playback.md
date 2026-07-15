# Route Recording und Playback

## Überblick

Phase 6 baut eine generische, run-unabhängige Infrastruktur für manuell aufgezeichnete und deterministisch wiedergegebene Navigationsrouten. Countess ist der erste vollständig integrierte Anwendungsfall: Der Spieler führt den Weg von Black Marsh über den Forgotten Tower bis Tower Cellar Level 5 einmal selbst aus und der Bot spielt ihn anschließend Memory-verifiziert ab.

`Route`, `Segment`, `Waypoint`, `Transition`, `Recorder`, `Registry`, `Validator` und `Player` dürfen keine Countess-spezifische Fachlogik enthalten. Weitere Runs sollen dieselbe Infrastruktur verwenden können, ohne Recorder oder Playback-Engine zu forken.

Der vorhandene Navigator bleibt für kurze lokale Korrekturen, World-to-Screen-Projektion, Hover-Bestätigung und Area-Übergänge zuständig. Bearing-Explore bleibt Diagnose- und expliziter Fallback-Baustein, ist aber nicht Teil des regulären Phase-6-Erfolgspfads.

## Ort im Code

- **Paket:** `internal/pathing/`
- **Einstieg:** read-only Registry-Kommandos über `--route`; Recorder und Player folgen in späteren Slices
- **Wichtige Dateien:** `route_model.go`, `route_storage.go`, `route_registry.go`, `route_compatibility.go`
- **Config:** invalidierbare Farming-Routen unter `configs/routes/farming/<character>/<difficulty>/`; permanente Town-Routen getrennt unter `configs/routes/town/<act>/`. Run-Adapter referenzieren stabile Route-IDs statt Dateipfade

## Funktionalität

### Speicherstruktur nach Lebenszyklus

```text
configs/routes/
├── town/
│   └── act1/
│       └── waypoint/
│           ├── normal.yaml
│           ├── nightmare.yaml
│           └── hell.yaml
└── farming/
    └── mrbones/
        └── nightmare/
            └── black-marsh-cellar5-nightmare-mrbones.yaml
```

`town/` enthält dauerhafte, fachlich benannte Town-Assets und wird beim Wechsel von Charakter oder Schwierigkeit nicht invalidiert. Bereits separat aufgenommene Varianten bleiben erhalten und werden nicht mit Farming-Dateien vermischt.

`farming/<character>/<difficulty>/` enthält ausschließlich layoutgebundene Route-Contract-Dateien. `routes.directory` zeigt auf genau einen aktiven Unterordner. Bei einem Character-/Difficulty-Wechsel darf nur der betroffene Farming-Unterordner archiviert oder gelöscht und neu aufgenommen werden; `town/` bleibt unangetastet. Registry und Recorder arbeiten ausschließlich innerhalb des konfigurierten aktiven Farming-Verzeichnisses.

Ab Phase 10.8 verwendet der minimale Act-3-Egress denselben vollständigen Route-Contract in einem getrennten Verzeichnis. Er besteht aus genau einem terminalen Walk-Segment innerhalb Kurast-Docks. Der produktive Adapter verlangt die konfigurierte Route-ID sowie alle normalen Binding-, Layout- und Startnähe-Gates. Die Wiedergabe delegiert bewusst nicht an den Teleport-Navigator, sondern an den area-gebundenen Force-Move-Walker; ein als `teleport` deklariertes Egress-Asset wird vor Input abgewiesen.

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

Nach isolierter Live-Validierung ersetzte Playback die früheren Explorer-Schritte im regulären Countess-Run. Die gemeinsame Run-Pipeline enthält keinen Explorer-Fallback mehr; fehlt eine passende Aufnahme, endet sie fail-closed.

### Wiederverwendung und spätere GUI

Eine Route beschreibt ausschließlich Navigation. Kampf, Loot, Town und Stash bleiben fachliche Run-Schritte. Eine spätere Run-Definition kann eine oder mehrere Routen über stabile IDs referenzieren und mit anderen Task-Schritten kombinieren.

Die Registry bietet bereits ohne GUI eine listen- und filterbare Sicht auf Route-ID, Anzeigename, Tags, Charakter, Schwierigkeit, Spielversion, Aufnahmezeitpunkt und Validitätsstatus. Eine spätere GUI kann darauf aufbauen, um Aufzeichnungen auszuwählen, umzubenennen, zu duplizieren, neu aufzunehmen oder Run-Definitionen beziehungsweise Playlists zuzuordnen.

## Datenmodell

### Route Contract v1

Phase 6.0 legt folgendes logisches YAML-Schema fest. Nach der vorgeschalteten Game Identity implementiert Phase 6.2 die Go-Typen, den Loader und die vollständige Validierung.

```yaml
version: 1
id: black-marsh-to-cellar-5-hell-necro
name: Black Marsh bis Tower Cellar 5
kind: navigation
tags: [countess, act1, hell]

binding:
  character_name: MyNecro
  character_class: necromancer
  difficulty: hell
  map_seed: 466817790
  game_version: 3.2.92777
  profile_id: necro-hell
  layout_fingerprint:
    version: 1
    area_id: 6
    anchor_count: 2
    hash: c233f9b137a09e07e3b8d0d2fc02c74103bbc54e42ff89e57d9592a6024fb51c

recording:
  recorded_at: 2026-07-10T20:00:00Z
  sample_distance_tiles: 4.0

playback:
  waypoint_tolerance_tiles: 3.0
  max_drift_tiles: 8.0
  max_local_corrections: 2
  segment_timeout_ms: 30000
  transition_timeout_ms: 10000

segments:
  - id: black-marsh
    from_area_id: 6
    to_area_id: 20
    movement: teleport
    points:
      - { x: 1234, y: 5678 }
      - { x: 1260, y: 5690 }
    transition:
      type: entrance
      entrance_kind: wilderness_to_tower

  - id: forgotten-tower
    from_area_id: 20
    to_area_id: 21
    movement: teleport
    points:
      - { x: 410, y: 220 }
    transition:
      type: entrance
      entrance_kind: unknown_antechamber
```

Die Beispielkoordinaten sind nicht produktiv. Numerische Area-IDs sind die persistente Identität; Namen dienen nur der Anzeige. Unit-IDs sind laufzeitabhängig und werden niemals als alleinige persistente Routenanker verwendet.

### Feldregeln

| Feld | Vertrag |
|------|---------|
| `version` | Muss in Phase 6 exakt `1` sein. Unbekannte Versionen werden nicht best-effort geladen. |
| `id` | Stabile, nach Veröffentlichung unveränderliche ID: Regex `^[a-z0-9][a-z0-9-]{2,63}$`. Die Registry verlangt globale Eindeutigkeit. |
| `name` | Anzeigename mit 1–80 Zeichen; darf geändert werden, ohne Referenzen zu brechen. |
| `kind` | In v1 ausschließlich `navigation`; fachliche Run-Aktionen gehören nicht in eine Route. |
| `tags` | Optional, maximal 16 eindeutige lower-kebab-case Werte; nur Suche und GUI-Organisation. |
| `binding.character_name` | Erforderlich und muss exakt dem Memory-bestätigten aktiven Offline-Charakter entsprechen. |
| `binding.character_class` | Erforderlicher Plausibilitätswert; ersetzt den Charakternamen nicht. |
| `binding.difficulty` | Erforderliches Operator-/GUI-Label: `normal`, `nightmare` oder `hell`. Es ist keine alleinige Sicherheitsquelle. |
| `binding.map_seed` | Optionaler Diagnosewert. Die 6.1a-Live-Matrix zeigte denselben Wert über Charaktere und Difficulties hinweg; er besitzt keine Sicherheitswirkung. |
| `binding.game_version` | Erforderlich und muss standardmäßig exakt `memory.game_version` entsprechen. |
| `binding.profile_id` | Optionales Organisationsfeld für CLI/GUI; besitzt keine Sicherheitswirkung. |
| `binding.layout_fingerprint` | Verpflichtender autoritativer Startlayout-Nachweis aus Phase 6.1d; Version, Area, Ankerzahl und SHA-256 werden validiert. |
| `recording.recorded_at` | Erforderlicher RFC-3339-Zeitpunkt in UTC. |
| `recording.sample_distance_tiles` | Dokumentiert den Recorder-Schwellwert; `> 0` und höchstens `20`. |
| `playback.*` | Positive, konservative Sicherheitslimits. `max_drift_tiles` muss größer als `waypoint_tolerance_tiles` sein. |
| `segments` | Nicht leer, geordnet und mit innerhalb der Route eindeutigen Segment-IDs. |
| `movement` | In v1 `teleport` oder `walk`; ein Segment mischt keine Bewegungsarten. |
| `points` | Mindestens ein valider World-Punkt pro Segment; keine Client-/Screen-Koordinaten. |
| `transition` | In v1 `entrance`; enthält semantischen Entrance-Kind und wird durch den erwarteten Area-Wechsel bestätigt. |

Unbekannte Felder dürfen für Forward Compatibility eingelesen, aber nicht interpretiert werden. Unbekannte Werte in sicherheitsrelevanten Enums oder eine unbekannte `version` führen immer zum Abbruch.

### Segmentinvarianten

1. Das erste Segment beginnt in der World-Area, in der die Aufnahme gestartet wurde.
2. `segments[n].to_area_id` muss `segments[n+1].from_area_id` entsprechen.
3. Aufeinanderfolgende Punkte müssen plausibel, endlich und innerhalb des konfigurierten Maximalabstands liegen.
4. Loading- und inkonsistente Snapshots erzeugen keine Punkte.
5. Ein Segment endet erst mit einem Memory-bestätigten Wechsel in `to_area_id`.
6. Replay darf nur am ersten Segment oder an einer eindeutig bestätigten Segmentgrenze starten.

### Route, Run-Definition und Run-Ausführung

- Eine **Route** enthält ausschließlich Navigation und kann von mehreren Run-Definitionen referenziert werden.
- Eine spätere **Run-Definition** ordnet Schritte wie Waypoint, `play_route`, Combat, Loot und Town an. `play_route` referenziert ausschließlich `route_id`, niemals einen Dateipfad.
- Die **Run-Ausführung** bleibt eine Task-State-Machine und delegiert Navigation an den generischen Player.

Damit kann eine spätere GUI eine Liste von Run-Definitionen oder Playlists anbieten, ohne Routeninhalt mit Kampf- oder Loot-Konfiguration zu vermischen.

## Identitäts- und Gültigkeitsprüfung

Phase 6.1 führt vor Recorder und Playback eine read-only `GameIdentity` ein. Contract v1 verwendet folgende Bindung:

1. Character Name und Character Class werden aus dem aktiven Prozesszustand gelesen und über mehrere Snapshots bestätigt.
2. Registry und Run-Adapter verlangen exakte Übereinstimmung von `character_name`, `character_class` und `game_version`.
3. Vor dem ersten Input muss der gespeicherte Layout-Fingerprint aus Area und stabilen World-Ankern exakt reproduziert werden; aktuelle Area und Distanz zum ersten Routenpunkt bleiben zusätzliche Plausibilitätsgrenzen.
4. Jedes weitere Segment wird durch Area-Kette, Position und Transition bestätigt.

`difficulty` ist ein erforderliches Operator-/GUI-Label und wird bei automatischem Spielstart durch den eigenen Menü-Klick kontrolliert. Es ist trotzdem kein Sicherheitsnachweis. Der gelesene `map_seed` ist ebenfalls kein Ersatz: Er blieb in den Live-Tests über Charakter- und Difficulty-Wechsel konstant. Autoritativ ist der reproduzierte Layout-Fingerprint; ein fehlender oder abweichender Fingerprint sperrt Playback vor dem ersten Input. Es gibt keinen persistenten Cache der zuletzt gewählten Difficulty. `profile_id` dient nur der Organisation.

## Sampling-Vertrag

- Recorder liest nur valide `in_game`-World-States.
- Der erste valide Punkt einer Area wird immer übernommen.
- Ein weiterer Punkt wird übernommen, wenn die Distanz zum letzten Sample mindestens `sample_distance_tiles` erreicht oder eine relevante Richtungsänderung sonst verloren ginge.
- Der letzte valide Punkt vor einer Transition wird immer übernommen.
- Doppelte Punkte, Teleport-Animationen ohne Positionsfortschritt und Loading werden verworfen.
- Nach der Aufnahme darf eine Vereinfachung nur Punkte entfernen, wenn Start, Ende, Area, Reihenfolge und maximale geometrische Abweichung erhalten bleiben.
- Recording erzeugt niemals Bewegungsinput; der Spieler steuert vollständig manuell.

## Fehlerklassen

| Code | Bedeutung | Reaktion |
|------|-----------|----------|
| `route_not_found` | Route-ID ist in der Registry unbekannt. | Vor Input abbrechen. |
| `route_duplicate_id` | Mehrere Dateien veröffentlichen dieselbe ID. | Betroffene ID sperren. |
| `route_unsupported_version` | `version` wird nicht unterstützt. | Nicht laden. |
| `route_invalid` | Schema oder Segmentinvarianten sind verletzt. | Nicht laden; Feldkontext loggen. |
| `game_identity_unavailable` | Charaktername oder Klasse sind nicht stabil bestätigt. | Recording/Playback nicht starten. |
| `route_character_mismatch` | Charaktername oder Klasse passen nicht. | Vor Input abbrechen. |
| `route_layout_unverified` | Es sind nicht genügend stabile Layoutanker für einen Fingerprint sichtbar. | Vor Input abbrechen. |
| `route_layout_mismatch` | Der aktuelle Layout-Fingerprint entspricht nicht der Aufnahme. | Vor Input abbrechen. |
| `route_game_version_mismatch` | Spielversion passt nicht. | Vor Input abbrechen. |
| `route_start_mismatch` | Start-Area oder Startdistanz ist unplausibel. | Vor Input abbrechen. |
| `route_unexpected_area` | Replay befindet sich außerhalb der erwarteten Segment-Area. | Sofort abbrechen. |
| `route_drift_exceeded` | Position überschreitet die lokale Korrekturgrenze. | Abbrechen; kein Explorer-Fallback. |
| `route_segment_timeout` | Segment erreicht sein Ende nicht rechtzeitig. | Abbrechen. |
| `route_transition_failed` | Erwartete Entrance/Area-Transition wird nicht bestätigt. | Nach begrenzten lokalen Retries abbrechen. |
| `route_recording_incomplete` | Aufnahme endet ohne vollständige, valide Segmentkette. | Temporäre Datei verwerfen. |
| `route_storage_failed` | Lesen, temporäres Schreiben oder atomare Veröffentlichung schlägt fehl. | Fehler mit Dateikontext zurückgeben. |

Pause ist kein Fehler und friert den Zustand ohne neue Inputs ein. Stop liefert ein kontrolliertes Operator-Ergebnis und veröffentlicht keine unvollständige Aufnahme.

## Operator / CLI

Phase 6 verwendet einen eigenen, generischen Route-Modus. Phase 6.2 implementiert `list`, `inspect` und `validate`; Record und Play folgen in den späteren Slices:

```powershell
go run ./cmd/d2rbot --route list
go run ./cmd/d2rbot --route inspect:<route-id>
go run ./cmd/d2rbot --route validate:<route-id>
go run ./cmd/d2rbot --route record:<route-id> --route-name "Anzeigename" --route-difficulty hell
go run ./cmd/d2rbot --route play-segment:<route-id>/<segment-id>
go run ./cmd/d2rbot --route play:<route-id>
go run ./cmd/d2rbot --route inspect-egress:act3
go run ./cmd/d2rbot --route record-egress:act3 --route-name "Kurast-Docks Portal bis Waypoint" --route-difficulty nightmare
go run ./cmd/d2rbot --route validate-egress:act3
go run ./cmd/d2rbot --route play-egress:act3
```

- `list`, `inspect` und `validate` erzeugen keinen Gameplay-Input.
- `record` beobachtet ausschließlich die manuelle Bewegung und benötigt eine valide Memory-bestätigte Game Identity, einen stabilen Startanker sowie Pause/Stop-Hotkeys. `--route-difficulty` ist als nicht autorisierendes Label verpflichtend.
- `play` ist input-aktiv, verlangt `input.enabled: true` und alle Pathing-Prechecks.
- `play-segment` ist der isolierte Phase-6.4-Testmodus. Segment 1 verlangt den vollständigen Layout-Precheck; spätere Segmente sind explizite Diagnoseaufrufe mit Character-, Versions-, Area- und Startdistanzprüfung.
- `--route-name` ist nur mit `record` gültig. Ohne Wert wird aus der ID ein Anzeigename abgeleitet.
- `--route-difficulty normal|nightmare|hell` ist nur mit `record` gültig und wird niemals anstelle des Layout-Fingerprints vertraut.
- Unbekannte Commands, leere IDs und gleichzeitige `--route`-/`--run`-/Testmodi werden vor Runtime-Start abgelehnt.
- Das aktive Farming-Routenverzeichnis wird über `routes.directory` festgelegt. Es zeigt auf genau einen Character-/Difficulty-Kontext, beispielsweise `configs/routes/farming/mrbones/nightmare` relativ zur Config-Datei.
- Die vier `*-egress:act3`-Kommandos verwenden ausschließlich `town.egress.act3.routes_directory` und `route_id`. Inspect beobachtet Kurast read-only, Record akzeptiert nur einen gleichbleibenden Kurast-Walk, Validate verlangt genau ein terminales Walk-Segment und Play kombiniert den Lauf mit dem registrierten Transfer nach Rogue Encampment.

## Implementierungsstand Phase 6.2

Phase 6.2 ist abgeschlossen:

- Godoc-dokumentierte, run-unabhängige Route-v1-Typen und Enum-Grenzen;
- vollständige Feld-, Segmentketten-, Punktabstands- und Sicherheitslimit-Validierung;
- YAML-Load mit tolerierten unbekannten Feldern sowie validate-before-publish Speicherung über temporäre Datei und atomare Umbenennung;
- Registry mit stabiler ID-Auflösung, defensiver Metadatensicht und sichtbaren `valid`, `invalid` sowie `duplicate_id` Status;
- fail-closed Precheck für bestätigten Character, Game Version, Layout-Fingerprint, Start-Area und Startdistanz;
- read-only CLI `--route list`, `inspect:<id>` und `validate:<id>`;
- `routes.directory` mit Default `configs/routes/farming` und produktiver Auswahl eines konkreten `<character>/<difficulty>`-Unterordners.

## Implementierungsstand Phase 6.3

Der read-only Recorder ist technisch implementiert:

- generische State-Machine ohne Countess- oder Ziel-Area-Annahmen;
- Start erst bei bestätigter Character Identity und verfügbarem Layout-Fingerprint;
- Distanz-Sampling in World-Koordinaten, keine Maus-/Screen-Aufzeichnung;
- Loading und ungültige Snapshots werden ignoriert;
- Area-Wechsel schließen ein Segment und speichern den letzten validen Punkt vor der Transition;
- semantisch nächster Entrance-Kind wird als Transition-Metadatum übernommen;
- Character-Wechsel und Prozessverlust brechen fail-closed ab;
- Pause friert Sampling ein; ausschließlich der Stop-Hotkey veröffentlicht eine vollständige, erneut validierte Route;
- das unvollständige aktuelle Area-Endstück wird beim Stop verworfen, da ihm keine bestätigte Transition folgt.

### Live-Abnahme Phase 6.3

Am 11.07.2026 wurde `black-marsh-cellar5-nightmare-mrbones` vollständig read-only aufgezeichnet und anschließend unabhängig über Registry, `inspect` und `validate` geladen:

- Character `MrBones`, Klasse `necromancer`, Difficulty-Label `nightmare`;
- Start-Fingerprint `54f310a416649e926dca8ba067e30223d71b22043b753097a6065ffe82aec5be`;
- sechs bestätigte Segmente von Black Marsh bis Tower Cellar Level 5;
- bekannte Cellar-Abgänge als `tower_cellar_down`, Forgotten-Tower-Antechamber konservativ als `unknown`;
- Veröffentlichung erst nach F11 auf Level 5;
- gespeicherte Datei `configs/routes/farming/mrbones/nightmare/black-marsh-cellar5-nightmare-mrbones.yaml` besteht die vollständige Route-v1-Validierung.

Phase 6.3 ist damit abgeschlossen.

## Implementierungsstand Phase 6.4

Der isolierte Segment-Player ist technisch implementiert:

- genau ein benanntes Segment pro CLI-Aufruf, keine automatische Verkettung;
- vollständiger Character-/Version-/Layout-/Start-Precheck vor Input in Segment 1;
- spätere isolierte Diagnose-Segmente verlangen Character, Version, exakte Start-Area und Startdistanz;
- punktweise Wiedergabe über den vorhandenen Navigator mit Memory-bestätigter Ankunftstoleranz;
- Driftmessung gegen die aktive Kante zwischen letztem und nächstem Routenpunkt;
- begrenzte lokale Korrekturen aus `max_local_corrections`;
- strikter Entrance-Übergang ohne Bearing-Explore-Fallback;
- Erfolg ausschließlich nach Memory-bestätigtem Wechsel in `to_area_id`;
- separate Segment- und Transition-Timeouts; Pause friert beide Fristen ein;
- Stop, Prozessverlust, Identity-/Area-Abweichung, Drift, Stuck und fehlender Entrance brechen fail-closed ab.

CLI: `--route play-segment:<route-id>/<segment-id>`.

### Live-Abnahme Phase 6.4

Am 11.07.2026 wurden alle sechs Segmente der `MrBones`-Nightmare-Route als getrennte CLI-Aufrufe erfolgreich wiedergegeben. Jeder Aufruf bestätigte seinen Startzustand neu und endete erst nach dem erwarteten Area-Wechsel. Der erste Versuch offenbarte eine unterschiedliche allgemeine Navigator- und Route-Ankunftstoleranz; der Goal-Vertrag übergibt seitdem explizit `waypoint_tolerance_tiles`. Der korrigierte Wiederholungslauf sowie alle fünf Folgesegmente waren erfolgreich.

Ein absichtlicher Start von Segment `black-marsh` auf Tower Cellar Level 5 endete mit `route start mismatch` vor dem ersten Gameplay-Input. Stop-/Cancel-Tests beweisen, dass der Navigator zurückgesetzt und kein nachlaufendes Goal gestartet wird. Phase 6.4 ist damit abgeschlossen.

## Implementierungsstand Phase 6.5

Area-Transitionen sind als eigener strikter Handler gehärtet:

- Auswahl ausschließlich nach dem in der Route gespeicherten semantischen `entrance_kind`;
- Bindung genau einer aktuellen Entrance-UnitID pro Versuch;
- kein Wechsel auf eine andere Unit innerhalb desselben Versuchs;
- falsche Entrance-Kinds erzeugen kein Navigator-Goal und keinen Input;
- fehlende Entrance wartet bis zur Transition-Frist, ohne Bearing-Explore;
- verschwundene Unit oder Navigator-/Hover-Fehler erlauben höchstens `max_local_corrections` lokale Neuselektionen;
- erschöpfte Recovery oder Timeout liefert `route_transition_failed`;
- nach hover-bestätigtem Klick wartet der Navigator auf Loading beziehungsweise Area-Wechsel, statt denselben Entrance pro Poll erneut zu klicken;
- ausschließlich `to_area_id` bestätigt Erfolg; jede andere Area ist `route_unexpected_area`.

### Live-Abnahme Phase 6.5

Am 11.07.2026 wurden Tower-Eingang, Forgotten-Tower-Antechamber und alle vier Cellar-Abgänge erneut als sechs isolierte Aufrufe durchlaufen. Jeder Übergang verwendete genau einen hover-bestätigten Entrance-Klick, meldete keinen Retry/Failure und endete in der erwarteten Area. Unit-Tests beweisen zusätzlich, dass eine falsche Entrance-Art kein Goal startet, eine korrekte Laufzeit-Unit fest gepinnt bleibt und Recovery nach dem konfigurierten Limit mit `route_transition_failed` endet.

Phase 6.5 ist damit abgeschlossen.

## Implementierungsstand Phase 6.6

Das vollständige Route Playback ist technisch implementiert:

- `RoutePlayer` startet immer mit Segment 0 und lebt für die gesamte Route in derselben überwachten Session;
- der vollständige Character-/Version-/Layout-/Start-Precheck erfolgt genau einmal vor dem ersten Input;
- der nächste Segment-Player wird erst nach bestätigtem `to_area_id` des Vorgängers erzeugt;
- Segment-, Punkt- und Transition-State bleiben fail-closed und verwenden unverändert die 6.4-/6.5-Grenzen;
- Pause friert die aktive Segment- oder Transition-Frist ein;
- Stop setzt RoutePlayer und Navigator zurück und erzeugt ein kontrolliertes `route_playback_stopped`-Ereignis;
- Prozessverlust, Identity-Wechsel, unerwartete Area, Drift, Stuck und Timeout beenden die gesamte Route;
- kein Resume nach Prozessneustart oder aus beliebiger späterer Area; damit bleibt der autoritative Start-Fingerprint an dieselbe Session gebunden;
- `--route play:<route-id>` startet die vollständige Wiedergabe;
- JSONL-Telemetrie korreliert Route-ID, Segment-ID/-Index, Punktindex/-ziel, Transition-Ziel-Area, Abschluss, Stop und Fehler.

### Live-Abnahme Phase 6.6

Am 11.07.2026 wurden zehn vollständige Wiedergaben im gebundenen Nightmare-Layout erfolgreich vom Black-Marsh-Wegpunkt bis Tower Cellar Level 5 ausgeführt: neun direkte `--route play`-Replays und ein Playback über den Countess-Adapter. Jede erfolgreiche JSONL-Datei endet mit `route_playback_completed` und enthält sechs `route_segment_completed`-Ereignisse.

### Aktive Nightmare-Neuaufnahme vom 13.07.2026

Nach zwischenzeitlichen Difficulty-Wechseln wurden die alten Countess-Aufzeichnungen bewusst verworfen. Die Town-Walk-Dateien bleiben davon unabhängig erhalten; nur die Nightmare-Town-Walk-Datei wurde zusätzlich vom aktuellen Spawn-/Stashbereich neu aufgenommen und einmal erfolgreich bis `waypoint_visible` abgespielt.

Nach den Town-Preset-Abnahmen wurde die aktive Route `black-marsh-cellar5-nightmare-mrbones` erneut vollständig von Black Marsh bis in den Countess-Raum aufgezeichnet:

- Character `MrBones`, Difficulty `nightmare`, Game-Version `3.2.92777`;
- sieben Segmente mit neuem Start-Fingerprint `56035675f9c30f9c11bfdea89e1da882d48e95f8423822bd2e95c01291619e37`;
- sechs bestätigte Area-Übergänge bis Tower Cellar Level 5 sowie ein terminaler Level-5-Pfad mit zehn Punkten bis `(12547,11065)` im Countess-Raum;
- Countess- und Monsterzustand werden nicht im Navigationsasset gespeichert;
- die ungültige Altdatei mit derselben internen ID wurde entfernt; Registry-, Route-v1- und Session-Validierung sind erfolgreich;
- das isolierte Gesamt-Playback vom 13.07.2026 schloss alle sieben Segmente ohne Drift-, Recovery-, Stuck- oder Fehlerereignis ab und endete mit `route_playback_completed` am terminalen Punkt im Countess-Raum.

Die alte Hell-Countess-Route wurde entfernt. Normal-/Hell-Town-Walk-Aufzeichnungen bleiben erhalten, da ihr fester Rogue-Encampment-Vertrag nicht an den Countess-Layout-Fingerprint gekoppelt ist.

Zwei frühe Entwicklungsversuche endeten reproduzierbar und fail-closed auf Cellar 1, nachdem ein randgeklemmter Teleport seitlich mehr als `max_drift_tiles` von der aktiven Routenkante landete. Ein weiterer Wiederholungslauf brach auf dem letzten Segment nach zwei lokalen Korrekturen am Drift-Limit ab. Die daraus abgeleitete Recovery erhöht das Drift-Limit nicht: Sie kehrt höchstens `max_local_corrections`-mal zum letzten bestätigten aufgezeichneten Punkt zurück und versucht danach erneut ausschließlich den unveränderten nächsten Routenpunkt. Alle drei Fehler bleiben als `route_playback_failed` mit Segment- und Punktkontext in JSONL nachvollziehbar; sie lösten keinen Explorer-Fallback aus.

Phase 6.6 ist damit abgeschlossen.

## Implementierungsstand Phase 6.7

Der Countess-Adapter verwendet die generische Route-Infrastruktur ohne eigene Routenlogik:

- `runs.definitions.countess.route_id` referenziert ausschließlich eine stabile Registry-ID, niemals einen Dateipfad;
- der neue Task-Schritt `play_bound_route` ersetzt im regulären Full Run und in `play-route` die sechs best-effort Explorer-Schritte;
- App-seitig lädt der Adapter Registry und Route, bildet den aktuellen Layout-Fingerprint und führt den vollständigen Route-Precheck aus;
- anschließend delegiert er an denselben generischen `RoutePlayer` aus Phase 6.6;
- Route-, Segment-, Transition-, Abschluss- und Fehlerereignisse werden in die bestehende Countess-Run-Telemetrie geschrieben;
- fehlende ID, unbekannte/ungültige Route, Identity-/Layout-Mismatch oder Playback-Fehler beenden den Task fail-closed;
- kein stiller Explorer-Fallback im regulären Erfolgspfad;
- `play-route` darf nach Prozessstart nur in Act-1-Town, am verifizierten Black-Marsh-Start oder bereits auf Cellar 5 beginnen. Mittlere Route-Areas sind kein zulässiges Resume.

Die Live-Abnahme des Countess-Adapters wurde am 11.07.2026 mit `play-route` durchgeführt. Die konfigurierte Nightmare-Route startete am verifizierten Black-Marsh-Wegpunkt, schloss alle sechs Segmente ab, erreichte Tower Cellar Level 5 und beendete den Task mit `outcome=success`. Der Player wird zusätzlich mit einer zweiten synthetischen Route getestet, damit die Infrastruktur nicht unbemerkt an Countess-Metadaten gekoppelt wird.

Pause und Stop müssen in Aufnahme und Wiedergabe jederzeit wirksam sein.

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
