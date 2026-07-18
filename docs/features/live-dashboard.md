# Live-Dashboard und Session-Steuerung

## Überblick

Abschnitt 11.3 zeigt den laufenden Core, den D2R-Attach, die Input-Safety-Gates und den letzten gültigen World-Zustand live im lokalen Browser. Abschnitt 11.8 ergänzt den Runtime-Queue-Builder und sichere Session-Controls. Nur explizite, token-geschützte Befehle dürfen den Core mutieren.

## Ort im Code

- **Runtime-Projektion:** `internal/app/ui_monitor.go`
- **API-Projektion:** `internal/api/live_backend.go`
- **Live-Publisher:** `internal/telemetry/publisher.go`
- **SSE-Transport:** `internal/api/server.go`
- **Dashboard:** `web/src/app/App.tsx`
- **CLI:** `cmd/d2rbot/main.go` mit `--ui`

## Funktionalität

### Statusprojektion

Der UI-Monitor verwendet den vorhandenen Runtime-Snapshot-Tick, damit Prozess-, Memory- und World-Verhalten nicht parallel neu implementiert werden. Er projiziert nur unveränderliche DTO-Werte: Prozesszustand und PID, Fensterbindung und Clientgröße, die drei Input-Safety-Gates sowie World-Gültigkeit, Phase und Gebiet. Handles, Pointer und mutable World-Slices verlassen den Core nicht.

Snapshot-Fehler werden als deutscher `last_error` sichtbar und beim nächsten Tick erneut versucht. Attach/Detach räumt über dieselben bestehenden Reset- und Unbind-Grenzen auf. Der Monitor registriert keine Hotkeys und ruft weder Task- noch Session-Ausführung auf.

### Live-Ereignisse

`LivePublisher` hält einen begrenzten In-Memory-Ring, monotone Sequenzen und pro Client eine begrenzte Queue. Publishing wartet nie auf Browser. Langsame Clients werden getrennt; Area- und Step-Wiederholungen werden dedupliziert. Der Publisher ist additiv und verändert die persistente JSONL-Telemetrie nicht.

Der SSE-Endpunkt sendet zuerst einen vollständigen `snapshot`, optional Replay ab `Last-Event-ID` und danach Live-Deltas. Heartbeats halten inaktive Verbindungen erkennbar. Jedes Delta stößt im Browser eine serialisierte Status-Neuabfrage an; bei einem Burst folgt nach der laufenden Abfrage genau eine weitere, sodass keine ältere HTTP-Antwort einen neueren Zustand überschreibt. Browser-`EventSource` reconnectet automatisch und reicht seine letzte Event-ID weiter; ein kompletter Refresh rekonstruiert alles aus Status-, Katalog- und Stream-Snapshot. Die manuelle Abnahme bestätigte zusätzlich einen echten Verbindungsverlust durch Deaktivieren des LAN-Adapters und die automatische Wiederverbindung nach etwa 30–60 Sekunden.

### Dashboard und Queue Builder

Die Oberfläche zeigt:

- Live-Verbindungszustand;
- Core-/Supervisor-Zustand und Generation;
- D2R-Attach und Fensterbindung;
- Input enabled/paused/stopped;
- World-Phase und Gebiet;
- read-only Run-Katalog;
- die letzten 40 Live-Ereignisse.

Availability-Karten können jeden Run genau einmal in die „Run-Reihenfolge pro Spiel“ aufnehmen. Bereits enthaltene Runs sind mit Erklärung deaktiviert; der Core lehnt Duplikate unabhängig davon mit `queue_duplicate_run` ab. Die Liste lässt sich per beschrifteten Auf-/Ab-Buttons verschieben, einzeln entfernen und auf `session.queue` zurücksetzen. Während `starting_game`, `starting_run`, `running_run`, `paused_between_runs`, `exiting_game` und `cancelling` sind Auswahl und Queue-Änderungen gesperrt. Browser-Refresh rekonstruiert Queue, Game-ID, Run-ID, Lifecycle-Phase, Index, Spielzyklus, Retry, Budgets, Step und terminales Ergebnis aus dem Core-Snapshot.

Start führt zuerst den tokenfreien Gesamt-Preflight und danach denselben Kontext als token-geschützten `start_queue`-Command aus. Nach der Core-seitigen Memory-Verifikation aktivieren Start und Resume das gebundene D2R-Fenster über den gemeinsamen gegateten Fokuspfad; React aktiviert kein Fenster selbst. Pause wartet auf Loot und Town, lässt das Spiel geöffnet und benötigt durch den globalen Hotkey keinen Browserfokus. Resume revalidiert dasselbe Spiel. Geordneter Stopp verlässt es danach genau einmal; der natürliche Wrap verlässt es erst nach der vollständigen Folge. Emergency Stop bleibt visuell getrennt, bestätigt und F11-identisch. Sichtbarer mutierter Zustand folgt ausschließlich der Core-Antwort; ein lokaler Command-Lock verhindert Doppelklick-Starts.

Alle Operator-Texte und Fehler sind deutsch. Der Control-Token bleibt ausschließlich im Browser-Memory und ist für den read-only SSE-GET nicht erforderlich.

## Operator / CLI

```powershell
go run ./cmd/d2rbot --config configs/config.yaml --ui
```

Für die manuelle Abnahme D2R zunächst geschlossen lassen, dann starten und wieder beenden. Das Dashboard muss `detached` → `attached` → `detached`, Fensterbindung und eine Gebietsänderung live zeigen. Browser-Refresh und eine kurz getrennte Netzwerkverbindung müssen den aktuellen Snapshot wiederherstellen. In den Logs darf keine Gameplay-Input-Aktion erscheinen.

## Abhängigkeiten und Grenzen

- Ausschließlich lokaler Loopback-Transport; keine Remote-Freigabe.
- Queue-Entwürfe sind Runtime-only; ein Prozessneustart lädt den YAML-Default bei Index 0.
- Keine Persistenz des Eventrings; JSONL bleibt autoritativ.
- Ein fehlender Browser oder ein langsamer SSE-Client beeinflusst den Core nicht.

## Verwandte Features

- [Lokale Core-API](local-core-api.md)
- [Run-Telemetrie](run-telemetry.md)
- [Input Controller](input-controller.md)
- [State Probe](state-probe.md)

---
*Zuletzt aktualisiert: 17. Juli 2026*
