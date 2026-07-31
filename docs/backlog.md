# Ideen-Backlog

Sammlung von **späteren** Ideen und Verbesserungen — keine verbindliche Roadmap.
Implementierte Features landen in `docs/features/` und `docs/CHANGELOG.md`.
Die verbindliche Phasenfolge 0–15 steht in [`handoff.html`](../handoff.html). Aktbewusste Town Services sind Phase 9; GUI, Routenverwaltung, Pickit-Editor und Statistiken folgen in den Phasen 11–14.

**Status-Legende:** `idea` → `planned` → `in_progress` → `done` (dann hier entfernen oder nach `done` verschieben)

| Status | Bedeutung |
|--------|-----------|
| `idea` | Grob skizziert, noch nicht eingeplant |
| `planned` | Soll in einer konkreten Phase umgesetzt werden |
| `in_progress` | Aktiv in Arbeit |
| `done` | Umgesetzt — Eintrag streichen oder kurz verlinken |

---

## Map-Reading-Heuristiken

**Status:** `idea`  
**Ziel-Phase:** nach Phase 4.3, sobald Automap-/Room-Graph-Daten verfügbar sind  
**Verwandt:** Pathing, Countess-Run, Route-Cache

### Kontext

Viele D2R-Gebiete folgen bekannten Speedrun-Regeln: Entrance-Tiles, Waypoint-Tiles und Ziel-Tiles stehen oft in festen relativen Richtungen (`Left`, `Straight`, `Right`) oder in stabilen Outdoor-/Fixed-Enough-Layouts. Die Recherche dazu liegt in [`docs/research/d2r-map-reading.html`](research/d2r-map-reading.html).

### Idee

Spätere Navigation kann diese Regeln als Priorisierung nutzen: z. B. Tower Cellar Level 1-4 vom Entrance aus als `Left` behandeln, passende Abzweigungen bevorzugen, Sackgassen markieren und bei "backwards maps" auf allgemeine Exploration oder Route-Cache zurückfallen.

---

## Route-Cache & Route-Recycling

**Status:** `done`
**Abgeschlossen:** Phase 6
**Umsetzung:** [`docs/features/route-recording-playback.md`](features/route-recording-playback.md)
**Verwandt:** Countess-Run, Pathing

### Kontext

In Offline/Singleplayer bleibt das Map-Layout **stabil**, solange sich **Charakter** und **Schwierigkeitsgrad** nicht ändern (neues Spiel mit anderem Char/Diff = neues Seed). Waypoint-Travel ist ohnehin deterministisch; der Gewinn liegt vor allem bei **Outdoor → Tower** und **Tower Cellar 1–5**.

### Idee

Der Bot legt lokal eine Datei an (z. B. unter `configs/routes/` oder `.d2rbot/`), in der der **letzte erfolgreiche Run** referenziert wird:

- Charaktername
- Schwierigkeitsgrad (`normal` / `nightmare` / `hell`)
- optional: `game_version` zum Invalidieren nach Patches
- die **gespeicherte Routen-Sequenz** (siehe gemeinsames Format unten)

**Ablauf beim Run-Start:**

1. Datei vorhanden?  
   - **Nein** → fail-closed abbrechen und den Operator zur manuellen Aufnahme auffordern.
   - **Ja** → Charakter + Diff mit aktuellem Spiel vergleichen.  
     - **Gleich** → gespeicherte Route Memory-verifiziert abspielen.
     - **Unterschiedlich** → fail-closed abbrechen und eine separate Aufnahme verlangen.

Nur **erfolgreiche** Runs persistieren — fehlgeschlagene Exploration nicht cachen.

### Fallback beim Abspielen

- Area weicht von erwarteter Phase ab → Abort
- Position driftet über Schwellwert → begrenzte lokale Korrektur, danach Abort
- Timeout in einer Phase → Abort mit klarem Log

### Offene Punkte

- Charaktername und Difficulty aus Memory lesen (aktuell nicht im Snapshot)  
- Cache-Key und Dateipfad-Konvention  
- Gemeinsames Routen-Dateiformat mit Run Recorder (siehe #2)

---

## Run Recorder (manuelle Route aufzeichnen)

**Status:** `done`
**Abgeschlossen:** Phase 6; Dashboard und Guided Validation in Phase 12
**Umsetzung:** [`docs/features/route-recording-playback.md`](features/route-recording-playback.md), [`docs/features/guided-route-recording.md`](features/guided-route-recording.md), [`docs/features/route-dashboard.md`](features/route-dashboard.md)
**Verwandt:** Route-Cache (#1), Input Controller, Run-Definitionen, Dashboard/UI

### Kontext

Funktional **gleiches Ziel** wie Route-Cache (#1): eine abspielbare Sequenz von Aktionen für wiederholbare Runs. Unterschied: Die Route kommt **nicht vom Bot-Exploration**, sondern vom **Spieler**, der einmal manuell den Weg vormacht.

**Befund aus Phase-4.3-Testlauf (2026-07-02):** Bearing-Explore erreicht Outdoor-Zonenübergänge nicht zuverlässig — der Bot deckt den Kartenrand ab, teleportiert aber nie gezielt in den schmalen Übergangsbereich hinein und dreht stattdessen Kreise am Rand. Der Run Recorder ist damit der bevorzugte Weg für schnelle, effiziente Navigation über längere Strecken (siehe [`docs/features/pathing.md`](features/pathing.md), Grenzen).

### Idee

Modus „Run Recorder“ (später im Bot-UI startbar):

- Spieler führt den Countess-Weg einmal selbst aus.
- Bot zeichnet semantische World-Koordinaten, Area-Segmente, Bewegungsart und erwartete Übergänge auf; keine rohe zeitgesteuerte Input-Makrofolge.
- Speichern in eine Datei (gleiches Format wie Cache-Route).  
- **Replay:** Bei gleichem Charakter + Schwierigkeitsgrad Route abspielen — analog zum automatischen Cache.

Recorder, Registry, Validator und Player sind generisch. Countess ist nur der erste Live-Use-Case. Stabile Route-IDs, Anzeigenamen, Tags und Validitätsmetadaten erlauben einer späteren GUI, Aufzeichnungen zu verwalten und Run-Definitionen oder Playlists zuzuordnen. Navigation bleibt von fachlichen Schritten wie Kampf, Loot und Stash getrennt.

Vorteil für den Nutzer: Kein Warten auf zuverlässige Bot-Exploration; „ich zeig es dir einmal“.  
Vorteil für uns: Recorder und Cache-Player können **dieselbe Abspiel-Engine** nutzen.

### Minimaler CLI-Umweg (optional, vor UI)

Z. B. `--record-route countess` / `--play-route configs/routes/my-sorc-hell.yaml` — nur wenn UI noch fehlt.

### Offene Punkte

- Sampling-Distanz und Regeln zur Reduktion der World-State-Punkte
- Pause/Stop während Recording  
- Validierung vor Replay (Char, Diff, Version)  
- UI-Integration vs. reine Config/CLI

---

## Verworfene Alternative: Map-Server-Navigation (Koolo-Ansatz)

**Status:** `idea` (bewusst zurückgestellt — Run Recorder bevorzugt)  
**Entschieden:** 2026-07-02 (nach Phase-4.3-Testlauf)  
**Verwandt:** Run Recorder (#2), Pathing

### Wie Koolo navigiert

Koolo hat keinen öffentlichen Map-Dienst — es startet einen **lokalen** Map-Generierungsprozess:

1. **Map Seed aus dem Speicher lesen:** D2R generiert Karten prozedural aus einem Seed pro Spiel; d2go liest ihn aus dem Prozess.
2. **Lokaler Map-Server:** `koolo-map.exe` (Fork von [blacha/diablo2, packages/map](https://github.com/blacha/diablo2/tree/master/packages/map), MIT) lädt die **originalen Diablo II LoD 1.13c Game-DLLs** und lässt deren Map-Generierung mit dem Seed laufen — D2R nutzt dieselbe Generierungslogik wie das klassische D2. Läuft als lokale REST-API (`localhost:8899/v1/map/:seed/:difficulty/:act.json`) und liefert Kollisionsgrid, Area-Übergänge, Waypoint- und Objekt-Positionen als JSON.
3. **Gerichtetes Pathfinding:** Mit vollständigem Layout kann der Bot Zonenübergänge gezielt ansteuern (A* auf dem Kollisionsgrid) — genau das, was Bearing-Explore fehlt (Kreise am Kartenrand, Übergänge werden verpasst).

### Warum wir stattdessen den Run Recorder wählen

- **Harte Voraussetzung:** Der Map-Server braucht eine lokale **Diablo II LoD 1.13c Installation** (Koolo-Config `D2LoDPath`), weil die Generierung aus den Legacy-DLLs kommt. Unser Projekt braucht bisher nur D2R — eine erhebliche Setup-Hürde für Nutzer.
- **Zusätzliche Infrastruktur:** Separater Prozess (bzw. Docker/Wine), Seed-Read als weiterer patch-sensitiver Memory-Offset, plus eine komplette Pathfinding-Schicht obendrauf.
- **Run Recorder erreicht dasselbe Ziel einfacher:** Offline-Layouts bleiben pro Charakter+Schwierigkeit stabil; eine einmal manuell aufgezeichnete Route ist zuverlässig wiederholbar — ohne Legacy-Installation, ohne Map-Server, ohne Pathfinding.

### Wann neu bewerten

Falls der Run Recorder an Grenzen stößt (z. B. Layouts ändern sich doch, oder vollautomatische Exploration ohne manuelles Vormachen wird gewünscht), ist der Map-Server-Ansatz die belastbare Alternative — als lokale Komponente mit LoD-Abhängigkeit, nicht als öffentlicher Dienst.

---

## Gemeinsames Routen-Format (für #1 und #2)

Beide Ideen sollten **eine** Sequenz-Definition teilen, z. B. YAML:

```yaml
meta:
  character_name: "MySorc"
  difficulty: hell
  game_version: "3.2.92777"
  recorded_at: "2026-06-26T12:00:00Z"
  source: auto_explore | manual_record
  run: countess

steps:
  - area: BlackMarsh
    action: teleport_to
    x: 12345
    y: 6789
  - area: BlackMarsh
    action: interact
    object: stairs_down
    near: { x: 100, y: 200 }
```

Weltkoordinaten bevorzugen; Screen-Pixel nur wenn Transform noch unsauber ( dann pro Auflösung fragil).

---

## Weitere Ideen

### Gebietsabhängige Monster-Interest-Kataloge

**Status:** `idea`

**Ziel-Phase:** Beim nächsten neuen Combat-/Sweep-Run, spätestens mit dem Cow Level

**Verwandt:** Memory Reader, World Model, Run Registry, Route-Threat-Combat, Nihlathak-Run

#### Kontext

Die Runtime-Monstererfassung verwendet derzeit einen globalen NPC-ID-Filter für alle unterstützten Runs. Das ist für die vorhandenen Countess-, Summoner- und Nihlathak-Kataloge noch überschaubar, würde mit weiteren Farmzielen jedoch stetig wachsen. Monster eines Gebiets könnten dadurch auch in anderen Runs unnötig in das begrenzte Memory-/World-Reservoir gelangen.

Ein Filter direkt pro Route oder Run würde dieses Problem zwar begrenzen, aber `internal/memory` unerwünscht an fachliche Task- und Run-Definitionen koppeln.

#### Idee

Die Auswahl in zwei Verantwortungen aufteilen:

1. `internal/memory` enumeriert anhand der aktuellen `AreaID` nur den gebietseigenen Monster-Interest-Katalog. Die Kataloge enthalten reguläre Hostiles und notwendige erzeugte Minions, schließen Spieler-Summons aber weiterhin aus.
2. `internal/tasks` entscheidet unverändert anhand der Run-Capability und des aktuellen Zustands, welche der im World Model sichtbaren Monster tatsächlich Route-Threat, Bossziel oder Post-Boss-Cleanup-Ziel sind.

Die Gebietskataloge sollten nach Möglichkeit aus den patchgenauen Spieldaten erzeugt oder zentral gepflegt werden. Ein neuer Combat-Run ergänzt damit einen Area-Katalog und eine Task-Policy, ohne den Memory-Layer mit Run-IDs oder Routentypen zu versehen.

#### Auslöser für die Umsetzung

- Ein weiterer größerer Combat-/Sweep-Run kommt hinzu.
- Der globale Filter belastet das Monsterreservoir außerhalb seines Zielgebiets.
- Dieselben NPC-IDs benötigen in unterschiedlichen Gebieten verschiedene Erfassungsregeln.

Bis einer dieser Fälle eintritt, bleibt der bestehende globale Filter als bewusste KISS-/YAGNI-Lösung erhalten.

---

### Konfigurierbare Offline-Players-Anzahl (`/players 1–8`)

**Status:** `idea`

**Ziel-Phase:** nach stabiler Session-/Town-Orchestrierung; Session-Config zuerst, Desktop-UI optional danach

**Verwandt:** Session-Config, Input Controller (Chat), Game-Join / neues Spiel, Telemetrie

#### Kontext

Im Solo-Offline-Modus steuert der Chat-Befehl `/players 1`–`/players 8` Monster-Schwierigkeit und Erfahrung so, als wären entsprechend viele Spieler im Spiel. Typischer Use Case: Powerleveling oder härtere Farm-Runs (z. B. `/players 8` Richtung Level 99). Ohne Befehl bleibt das Spiel unverändert (effektiv players 1).

#### Idee

Der Bot-Benutzer kann optional eine Players-Anzahl `1`–`8` konfigurieren. Der Bot setzt den Wert per Chat-Befehl **einmal pro neuem Game** (nach Town-/Game-Join), nicht dauerhaft im Chat. Bei jedem Game-Reset erneut.

**Default / Opt-in:**

- Config weggelassen oder `0` = Bot fasst Players **nicht** an (unverändertes Spiel).
- `1`–`8` = aktiv; Bot erzwingt diesen Wunschwert nach Join.
- `0`/unset ist bewusst besser als stillschweigendes `1`, damit „Bot hat nichts angefasst“ explizit bleibt.

**Ort:**

- Primär Session-/Run-Config (YAML).
- Später optional Desktop-UI: einfache Zahl `1`–`8` plus Hinweis „nur Offline“.

**Telemetrie (v1):**

- gewünschter Wert
- ob der Befehl in diesem Game gesendet wurde  
Memory-Verify des aktiven Players-Werts ist nice-to-have, kein Muss für v1.

**Verhalten / Grenzen:**

- Chat öffnen, Befehl tippen, Chat schließen — über den bestehenden Input-Pfad, geloggt wie andere Eingaben.
- Wenn der Nutzer Players schon manuell gesetzt hat und Config aktiv ist: Bot erzwingt den konfigurierten Wunschwert einmalig nach Join (klar dokumentieren).
- Höhere Players machen Runs härter → mehr Timeouts/Deaths sind erwartetes Verhalten, kein Bug.
- Priorität hinter Town-/Run-Stabilität; kein Core-Contract-Thema.

#### Offene Punkte

- Exakter Config-Key (z. B. `session.players`) und Validierung `0|1–8`
- Timing relativ zu Town-Prep / erstem Waypoint
- UI-Darstellung und Warnhinweis bei hohen Werten

---

*(Neue Einträge unten anfügen — kurzer Titel, Status `idea`, Kontext, Ziel-Phase.)*
