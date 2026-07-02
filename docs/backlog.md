# Ideen-Backlog

Sammlung von **späteren** Ideen und Verbesserungen — keine verbindliche Roadmap.
Implementierte Features landen in `docs/features/` und `docs/CHANGELOG.md`.

**Status-Legende:** `idea` → `planned` → `in_progress` → `done` (dann hier entfernen oder nach `done` verschieben)

| Status | Bedeutung |
|--------|-----------|
| `idea` | Grob skizziert, noch nicht eingeplant |
| `planned` | Soll in einer konkreten Phase umgesetzt werden |
| `in_progress` | Aktiv in Arbeit |
| `done` | Umgesetzt — Eintrag streichen oder kurz verlinken |

---

## Route-Cache & Route-Recycling

**Status:** `idea`  
**Ziel-Phase:** nach Phase 4 (Exploration funktioniert) oder frühes Phase 5  
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
   - **Nein** → normal erkunden; bei Erfolg Route + Metadaten speichern.  
   - **Ja** → Charakter + Diff mit aktuellem Spiel vergleichen.  
     - **Gleich** → gespeicherte Route abspielen (mit Fallback bei Abweichung).  
     - **Unterschiedlich** → erkunden, alte Route verwerfen oder separat pro Key ablegen.

Nur **erfolgreiche** Runs persistieren — fehlgeschlagene Exploration nicht cachen.

### Fallback beim Abspielen

- Area weicht von erwarteter Phase ab → erkunden oder Abort  
- Position driftet über Schwellwert → erkunden  
- Timeout in einer Phase → erkunden oder Abort mit klarem Log  

### Offene Punkte

- Charaktername und Difficulty aus Memory lesen (aktuell nicht im Snapshot)  
- Cache-Key und Dateipfad-Konvention  
- Gemeinsames Routen-Dateiformat mit Run Recorder (siehe #2)

---

## Run Recorder (manuelle Route aufzeichnen)

**Status:** `idea`  
**Ziel-Phase:** nach Phase 4/5, wenn Bot-UI existiert (oder vorher minimal per CLI)  
**Verwandt:** Route-Cache (#1), Input Controller, später Dashboard/UI

### Kontext

Funktional **gleiches Ziel** wie Route-Cache (#1): eine abspielbare Sequenz von Aktionen für wiederholbare Runs. Unterschied: Die Route kommt **nicht vom Bot-Exploration**, sondern vom **Spieler**, der einmal manuell den Weg vormacht.

**Befund aus Phase-4.3-Testlauf (2026-07-02):** Bearing-Explore erreicht Outdoor-Zonenübergänge nicht zuverlässig — der Bot deckt den Kartenrand ab, teleportiert aber nie gezielt in den schmalen Übergangsbereich hinein und dreht stattdessen Kreise am Rand. Der Run Recorder ist damit der bevorzugte Weg für schnelle, effiziente Navigation über längere Strecken (siehe [`docs/features/pathing.md`](features/pathing.md), Grenzen).

### Idee

Modus „Run Recorder“ (später im Bot-UI startbar):

- Spieler führt den Countess-Weg einmal selbst aus (Bewegung, Klicks, Teleports, Interact, ggf. Skills).  
- Bot zeichnet auf: Timestamp oder Tick, Area, Welt-/Client-Koordinaten, Aktionstyp (`teleport`, `click`, `interact`, `skill`, …).  
- Speichern in eine Datei (gleiches Format wie Cache-Route).  
- **Replay:** Bei gleichem Charakter + Schwierigkeitsgrad Route abspielen — analog zum automatischen Cache.

Vorteil für den Nutzer: Kein Warten auf zuverlässige Bot-Exploration; „ich zeig es dir einmal“.  
Vorteil für uns: Recorder und Cache-Player können **dieselbe Abspiel-Engine** nutzen.

### Minimaler CLI-Umweg (optional, vor UI)

Z. B. `--record-route countess` / `--play-route configs/routes/my-sorc-hell.yaml` — nur wenn UI noch fehlt.

### Offene Punkte

- Welche Events genau aufzeichnen (nur Input vs. zusätzlich World-State pro Schritt)  
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

*(Neue Einträge unten anfügen — kurzer Titel, Status `idea`, Kontext, Ziel-Phase.)*
