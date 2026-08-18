# Live-Dashboard und Session-Steuerung

## Überblick

Abschnitt 11.3 zeigt den laufenden Core, den D2R-Attach, die Input-Safety-Gates und den letzten gültigen World-Zustand im Electron-Renderer. Abschnitt 11.8 ergänzt sichere Session-Controls. Nur explizite, token-geschützte Befehle dürfen den Core mutieren.

## Ort im Code

- **Runtime-Projektion:** `internal/app/ui_monitor.go`
- **API-Projektion:** `internal/api/live_backend.go`
- **Live-Publisher:** `internal/telemetry/publisher.go`
- **SSE-Transport:** `internal/api/server.go`
- **Dashboard:** `web/src/app/App.tsx`
- **Desktop-Wiring:** `cmd/d2rbot/main.go` mit privater Electron-Handshake-Pipe

## Funktionalität

### Statusprojektion

Der UI-Monitor verwendet den vorhandenen Runtime-Snapshot-Tick, damit Prozess-, Memory- und World-Verhalten nicht parallel neu implementiert werden. Er projiziert nur unveränderliche DTO-Werte: Prozesszustand und PID, Fensterbindung und Clientgröße, die drei Input-Safety-Gates sowie World-Gültigkeit, Phase und Gebiet. Handles, Pointer und mutable World-Slices verlassen den Core nicht.

Snapshot-Fehler werden als deutscher `last_error` sichtbar und beim nächsten Tick erneut versucht. Attach/Detach räumt über dieselben bestehenden Reset- und Unbind-Grenzen auf. Der Monitor registriert keine Hotkeys und ruft weder Task- noch Session-Ausführung auf.

### Live-Ereignisse

`LivePublisher` hält einen begrenzten In-Memory-Ring, monotone Sequenzen und pro Client eine begrenzte Queue. Publishing wartet nie auf Browser. Langsame Clients werden getrennt; Area- und Step-Wiederholungen werden dedupliziert. Der Publisher ist additiv und verändert die persistente JSONL-Telemetrie nicht.

Der SSE-Endpunkt sendet zuerst einen vollständigen `snapshot`, optional Replay ab `Last-Event-ID` und danach Live-Deltas. Heartbeats halten inaktive Verbindungen erkennbar. Jedes Delta stößt im Browser eine serialisierte Status-Neuabfrage an; bei einem Burst folgt nach der laufenden Abfrage genau eine weitere, sodass keine ältere HTTP-Antwort einen neueren Zustand überschreibt. Browser-`EventSource` reconnectet automatisch und reicht seine letzte Event-ID weiter; ein kompletter Refresh rekonstruiert alles aus Status-, Katalog- und Stream-Snapshot. Die manuelle Abnahme bestätigte zusätzlich einen echten Verbindungsverlust durch Deaktivieren des LAN-Adapters und die automatische Wiederverbindung nach etwa 30–60 Sekunden.

Das kleine `history_changed`-Delta enthält nur die geänderte Indexgeneration. Es fordert die Historienseite zum serialisierten Nachladen über die read-only API auf; einzelne JSONL-Ereignisse werden niemals über SSE dupliziert.

### Dashboard und Queue Builder

Die Oberfläche zeigt:

- Live-Verbindungszustand;
- Core-/Supervisor-Zustand und Generation;
- D2R-Attach und Fensterbindung;
- Input enabled/paused/stopped;
- World-Phase und Gebiet;
- read-only Run-Katalog;
- die letzten 40 Live-Ereignisse.

Availability-Karten und die Queue-Liste gehören zum in D2R bestätigten Farm-Charakter. Die Daten kommen aus `GET /api/v1/runs` bzw. der gespeicherten Queue in `operator-settings.local.yaml`. Die Dropdowns starten mit dieser bestätigten Auswahl, nicht mit dem ersten Katalogeintrag. Ein Wechsel im Dropdown ändert nur, wen Apply in D2R anwählt; Queue, Run-Karten und Start bleiben beim bestätigten Charakter. Nach erfolgreichem Apply lädt das Dashboard die gespeicherte Queue des neuen Charakters. Apply bestätigt visuell und per Memory auf dem Offline-Charakterbildschirm bei 1280 × 720. Ein Timeout mit fehlender Clientfläche bedeutet, dass das D2R-Fenster 0×0 war. Weicht die Clientgröße beim Queue-Start von 1280 × 720 ab, zeigt das Dashboard die gemessene Größe und den Hinweis, dass der Bot nur in dieser Auflösung arbeitet, statt einer allgemeinen Spielstart-Meldung. Das Dashboard startet exakt die persistente Reihenfolge des bestätigten Charakters und verweist für Änderungen auf den einzigen Editor unter „Einstellungen“. Die Queue-Überschrift nennt den bestätigten Charakter und die Anzeigenamen der Runs. Nicht unterstützte oder fremde Saves stehen nur im Auswahl-Dropdown als „nicht verfügbar“ sowie unter Einstellungen → Charaktere, nicht unter der aktuellen Auswahlkarte. Der Start bleibt gesperrt, solange die Dropdown-Auswahl noch nicht angewendet ist, ein Queue-Eintrag nicht `available` oder `runtime_validation_required` ist, Pause oder Notstopp aktiv ist oder ein Routenvorgang läuft. Ein fehlgeschlagenes Spielstart bleibt als deutsche Meldung sichtbar; der kurze Monitor-Übergang vor dem Worker gilt nicht als fehlende Spielsteuerung. Reason-Texte sind 1:1; Core-Codes erscheinen nicht in der Fläche. Der Core lehnt Duplikate unabhängig von der UI mit `queue_duplicate_run` ab. Während `starting_game`, `starting_run`, `running_run`, `paused_between_runs`, `exiting_game` und `cancelling` sind Settings-Mutationen gesperrt. Ein Renderer-Reload rekonstruiert Queue, Game-ID, Run-ID, Lifecycle-Phase, Index, Spielzyklus, Retry, Budgets, Step und terminales Ergebnis aus dem Core-Snapshot.

Nach einer erfolgreichen Settings-Mutation veröffentlicht der Core genau ein Änderungsevent und projiziert Queue sowie Budgets sofort in den inaktiven Dashboard-Status. Die Availability-Karten übersetzen interne Reason-Codes in kurze Benutzertexte. Eine veröffentlichte Route mit `runtime_validation_required` wird als bereit angezeigt; der Hinweis erklärt lediglich den sicheren Abgleich mit dem aktuellen Spiellayout beim Start.

Start führt zuerst den tokenfreien Gesamt-Preflight und danach denselben Kontext als token-geschützten `start_queue`-Command aus. Nach der Core-seitigen Memory-Verifikation aktivieren Start und Resume das gebundene D2R-Fenster über den gemeinsamen gegateten Fokuspfad; React aktiviert kein Fenster selbst. Pause wartet auf Loot und Town, lässt das Spiel geöffnet und benötigt durch den globalen Hotkey keinen Rendererfokus. Resume revalidiert dasselbe Spiel. Geordneter Stopp verlässt es danach genau einmal; der natürliche Wrap verlässt es erst nach der vollständigen Folge. Emergency Stop bleibt visuell getrennt, bestätigt und F11-identisch. Sichtbarer mutierter Zustand folgt ausschließlich der Core-Antwort; ein lokaler Command-Lock verhindert Doppelklick-Starts.

Alle Operator-Texte und Fehler sind deutsch. Der Control-Token bleibt ausschließlich im Renderer-Memory und ist für den read-only SSE-GET nicht erforderlich.

### Historie

Abschnitt 14.6 ergänzt den eigenen Navigationspunkt `Historie`. Zeitraum-, Kontext-, Ergebnis-, Reason- und Pickit-Filter, Übersicht, Core-sortierter Boss-/Routenvergleich, Item- und Run-Pagination, semantischer Drill-down und Exporte bleiben vollständig read-only. Tabellen wechseln auf kleinen Viewports in beschriftete Zeilengruppen. Details stehen in [Run-Historie im Dashboard](run-history.md).

## Operator / Desktop

Das Dashboard ist das Startziel der installierten App. Für eine Lifecycle-Abnahme D2R zunächst geschlossen lassen, dann starten und wieder beenden. Das Dashboard muss `detached` → `attached` → `detached`, Fensterbindung und eine Gebietsänderung live zeigen. Renderer-Reload und eine kurz getrennte Netzwerkverbindung müssen den aktuellen Snapshot wiederherstellen. In den Logs darf ohne expliziten Command keine Gameplay-Input-Aktion erscheinen.

## Abhängigkeiten und Grenzen

- Ausschließlich lokaler Loopback-Transport; keine Remote-Freigabe.
- Die Queue wird ausschließlich in den Core-eigenen Operator-Einstellungen persistiert und vom Dashboard nur projiziert.
- Keine Persistenz des Eventrings; JSONL bleibt autoritativ.
- Ein langsamer oder neu geladener Renderer beeinflusst den Core nicht.

## Verwandte Features

- [Lokale Core-API](local-core-api.md)
- [Run-Telemetrie](run-telemetry.md)
- [Input Controller](input-controller.md)
- [State Probe](state-probe.md)
- [Historien-API und Export](history-api-export.md)

---
*Zuletzt aktualisiert: 18. August 2026*
