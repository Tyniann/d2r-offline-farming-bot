# Live-Dashboard

## Überblick

Das Dashboard ist die produktive Startansicht der Desktop-App. Es verbindet die app-weite Charakterauswahl, die persistente Farm-Queue, read-only Historienkennzahlen und den Core-autoritativen Fortschritt eines aktiven Runs. Technische Runtime-Zustände und direkte Session-Schaltflächen bleiben aus der Hauptfläche heraus; während einer Session verweist das Dashboard auf die globalen Safety-Hotkeys.

## Ort im Code

- **Run-Fortschritt:** `internal/tasks/run_progress.go`
- **Runtime-Projektion:** `internal/app/ui_monitor.go`
- **API und SSE:** `internal/api/live_backend.go`, `internal/api/schema.go`
- **Dashboard:** `web/src/features/dashboard/`
- **UI-Plan:** [Dashboard-UI-Redesign](../plans/ui/dashboard-ui-redesign-spec.html)
- **App-weite Auswahl:** `web/src/app/AppSelectionContext.tsx`
- **Desktop-Persistenz:** `web/electron/desktop-settings.ts`
- **Shell-Wiring:** `web/src/app/App.tsx`

## Funktionalität

### App-weite Auswahl und Queue

Charakter und Schwierigkeit bilden eine einzige Auswahl für Dashboard, Routen, Pickit, Historie und Einstellungen. Diese App-Präferenz ist von der in D2R bestätigten Auswahl getrennt: Ein Wechsel aktualisiert zuerst nur die Oberfläche. Erst „In D2R anwenden“ führt den vorhandenen Core-Preflight und die Memory-bestätigte Auswahl aus.

Electron speichert die zuletzt gewählte Kombination als `selected_character` und `selected_difficulty` im Desktop-Settings-Schema 2. Vorhandene Schema-1-Dateien werden beim Laden übernommen und atomar in das neue Schema geschrieben. Fach- und Safetywerte bleiben weiterhin im Core-eigenen Operator-Settings-Store.

Das Dashboard zeigt die Core-persistierte Queue des gewählten Charakters read-only. Bearbeitet wird die Reihenfolge ausschließlich unter „Einstellungen“. Ein Start verwendet weiterhin den tokenfreien Gesamt-Preflight und danach den token-geschützten `start_queue`-Command. Auswahl, Queue-Verfügbarkeit, Versionsgate, Input-Safety und laufende Routenarbeit bleiben Core-seitige Start-Gates.

### Leerlauf und Historie

Im Leerlauf gliedert sich die Ansicht in Auswahl, Queue, verfügbare Runs und Historienkennzahlen. Der gemeinsame Zeitraumfilter „7 Tage“, „30 Tage“ oder „Gesamt“ steuert Zusammenfassung und Run-Vergleich; die letzten Runs werden unabhängig geladen. Die Oberfläche aggregiert keine Telemetriedaten selbst, sondern verwendet die read-only Historien-API.

Laden, Fehler und leere Historie haben stabile eigene Zustände. Ein Historienfehler kann direkt erneut geladen werden. Die sichtbaren Run-Namen werden in verständliches Deutsch übersetzt; interne Reason-Codes und Core-Zustandsnamen erscheinen nicht als Produkttext.

### Aktiver Run

Während einer Session ersetzt die aktive Run-Karte die Startaktion. Sie zeigt den aktuellen Queue-Eintrag, die beobachtete Laufzeit, einen Core-projizierten Etappennamen und den Fortschritt `current/total`. Countess unterscheidet dabei die fünf Kellergeschosse über die semantische Area; Mephisto, Beschwörer, Nihlathak, Unteres Kurast und Kuh-Level besitzen jeweils stabile fachliche Etappen. Ein kurz unvollständiger Loading-Snapshot darf die sichtbare Etappe nicht zurücksetzen.

Der Fortschritt wird im Status-Snapshot als `run_progress` ausgeliefert. `run_progress_changed` löst über SSE eine serialisierte Status-Neuabfrage nur dann aus, wenn sich die sichtbare Etappe tatsächlich ändert. Ein Renderer-Reload rekonstruiert denselben Stand aus dem Core-Snapshot.

Nach einem terminalen Sessionende öffnet die Shell einen Dialog mit Sitzungsdauer sowie aufklappbaren Listen der aufgehobenen und verkauften Items. Details: [Session-Zusammenfassung](session-summary.md).

Pause, Stopp nach dem Run und Notstopp bleiben auf dem Dashboard keine klickbaren Aktionen. Die Karte zeigt die in den Operator-Einstellungen konfigurierten Hotkeys sowie eine bereits vorgemerkte Absicht. Damit bleiben die globalen Safety-Wege auch ohne Rendererfokus maßgeblich. Sobald der Core `paused_between_runs` erreicht, blendet die Karte den Pause-Hinweis aus und zeigt an derselben Stelle Fortsetzen. Resume hat keinen Hotkey; der Button ruft denselben `resume`-Command wie die Loopback-API.

### Live-Verbindung und Darstellung

Das Dashboard bezieht seinen vollständigen Zustand über die Loopback-API. SSE meldet relevante Änderungen; der Browser lädt danach den autoritativen Snapshot nach. Reconnect und Reload erzeugen deshalb keinen zweiten UI-eigenen Sessionzustand.

Die Ansicht wechselt bei 1180 Pixeln von drei auf zwei Spalten und unter 1000 Pixeln auf eine Spalte. Fokusziele, Statusmeldungen und Diagramme besitzen semantische Beschriftungen. Animationen laufen einmalig und werden bei `prefers-reduced-motion` deaktiviert.

## Operator / Desktop

- Im Leerlauf Charakter und Schwierigkeit wählen, bei Bedarf in D2R anwenden und anschließend die gespeicherte Queue starten.
- Die Queue unter „Einstellungen“ ändern; das Dashboard selbst bleibt read-only.
- Während eines Runs ausschließlich die angezeigten globalen Hotkeys für Pause, Stopp nach dem Run und Notstopp verwenden. Nach einer Pause die Session über Fortsetzen auf der Run-Karte wieder aufnehmen.
- Bei unterbrochener Verbindung auf „Verbindung getrennt“ achten; nach dem Reconnect wird der Core-Snapshot erneut geladen.

## Abhängigkeiten und Grenzen

- Ausschließlich lokaler Loopback-Transport; keine Remote-Freigabe.
- JSONL-Telemetrie und Core-Historienanalyse bleiben für Kennzahlen autoritativ.
- Die Desktop-Präferenz bestätigt weder einen Charakter noch eine Schwierigkeit in D2R.
- Die UI projiziert Run-Fortschritt, steuert aber keine Task-Schritte.
- Ein langsamer oder neu geladener Renderer beeinflusst den Core nicht.

## Verwandte Features

- [Lokale Core-API](local-core-api.md)
- [FarmQueue-Scheduler](farm-queue-scheduler.md)
- [Run-Historie im Dashboard](run-history.md)
- [Persistente Operator-Einstellungen](operator-settings.md)
- [Desktop-App-Shell und Designsystem](desktop-app-shell.md)
- [Session-Zusammenfassung](session-summary.md)

---
*Zuletzt aktualisiert: 28. August 2026*
