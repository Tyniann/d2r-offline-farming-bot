# Sichere Electron-Shell und Core-Kindprozess

## Überblick

Die installierte Windows-App führt die bestehende React-Oberfläche in genau einem gehärteten Electron-Fenster aus. Der Electron-Main-Prozess besitzt Desktop-Lifecycle und Fenster, während der gebündelte Go-Core weiterhin alleinige Autorität für Prozessbindung, Memory, World, Tasks, Input, Konfiguration und Statistiken bleibt.

## Ort im Code

- **Electron Main/Preload:** `web/electron/main.ts`, `web/electron/preload.cts`
- **Core-Lifecycle:** `web/electron/core-process.ts`, `web/electron/core-controller.ts`
- **Verträge:** `web/electron/core-contract.ts`
- **Go-Bootstrap:** `internal/app/desktop_handshake.go`, `internal/app/data_root_lock_windows.go`
- **Wiring:** `cmd/d2rbot/main.go`
- **Native Tests:** `web/electron/e2e/desktop-shell.spec.ts`

## Funktionalität

### Einzelinstanz und Datenroot-Ownership

Electron fordert vor dem Fensteraufbau den Single-Instance-Lock an und fokussiert bei einem zweiten Start das vorhandene Fenster. Zusätzlich belegt der Core vor API-, Hotkey- und Inputaufbau einen aus dem kanonischen absoluten Datenroot abgeleiteten Windows-Mutex. Dadurch können auch abweichende Desktop-Prozesse denselben produktiven Root nicht gleichzeitig öffnen.

Das aus dem kanonischen Root abgeleitete Chromium-Laufzeitprofil liegt getrennt im benutzereigenen Windows-Tempbereich. Es ist entbehrlicher Frameworkzustand und keine zweite Datenautorität. Diese Trennung verhindert insbesondere, dass der Renderer den noch unveröffentlichten Datenroot vor der einmaligen Core-Provisionierung selbst anlegt.

### Privater Bootstrap

Electron erzeugt pro Core-Start eine zufällige Named Pipe und startet den Core mit `--data-root` und `--desktop-handshake-pipe`. Die Pipe akzeptiert höchstens einen auf 16 KiB begrenzten JSON-Vertrag. Schema, Child-PID, Generation, exakter IPv4-Loopback-Origin und Bootstrap-Token werden fail-closed geprüft.

Der Control-Token erscheint ausschließlich im URL-Fragment der einmaligen Bootstrap-URL. Die tokenfreie `base_url` wird für Statusabfragen verwendet. Standardausgabe und Standardfehler sind keine Bootstrap-Kanäle und transportieren keinen Token.

### Crash und Shutdown

Ein erwarteter Desktop-Shutdown beendet zuerst den Core. Ein unerwarteter Core-Exit wird nur dann genau einmal automatisch neu gestartet, wenn der letzte autoritative Supervisorzustand sicher inaktiv und der Routenworkflow `idle` war. Bei aktiver, unbekannter oder bereits einmal neu gestarteter Instanz zeigt Electron eine lokale Recovery-Seite und bleibt fail-closed.

### Renderer-Sicherheitsgrenze

Das Browserfenster verwendet `sandbox: true`, `contextIsolation: true`, deaktivierte Node-Integration und deaktivierte Webviews. Navigation, Redirects, neue Fenster und Berechtigungen werden blockiert, sofern sie nicht vom exakten Core-Origin stammen. Eine CSP begrenzt Inhalte und Verbindungen auf diesen Origin.

Der CommonJS-Preload veröffentlicht ausschließlich eng typisierte Desktopoperationen:

- `getProvisioningState()`, `chooseImportRoot()` und `provision()` ausschließlich im lokalen Pre-Core-Provisionierungsmodus
- `getAppInfo()`
- `getDesktopSettings()` und `updateDesktopSettings()` für Autostart und Onboarding
- `restartCore()` für einen kontrollierten Neustart im sicher inaktiven Zustand
- `restartAsAdministrator()` — argumentlos und nur bei Core-bestätigtem `privilege_mismatch`
- `showWindow()`
- `onNavigate()` für die drei stabilen Notification-Ziele Dashboard, Historie und Einstellungen

Jeder IPC-Handler prüft zusätzlich den Sender-Origin. Prozess-, Dateisystem-, Shell-, Token- und frei parametrisierbare IPC-Zugriffe werden nicht in den Renderer gereicht.

## Operator / CLI

Der Core-Desktopmodus verlangt einen absoluten Datenroot:

```text
d2rbot.exe --data-root <absoluter-pfad> --desktop-handshake-pipe <private-pipe>
```

`--desktop-handshake-pipe` ist ausschließlich für den Electron-Elternprozess bestimmt. Es gibt keinen öffentlichen Core-API- oder Browser-Produktstart mehr; ohne privaten Handshake läuft ausschließlich der bestehende Repository-CLI-Betrieb.

## Tests

- Direkte Vertragstests prüfen Handshake, Sandboxoptionen, Navigation, IPC-Sender und Restart-Entscheidung.
- Pipe-Tests prüfen gültigen, verspäteten, falschen und abgebrochenen Handshake.
- Playwright startet echtes Electron mit einem Fake-Core und prüft exakt ein Fenster, Renderer-Isolation, Navigation/Window-Open, zweite Instanz sowie aktive und inaktive Core-Exits.
- Der Fresh-Root-Fall prüft, dass dieselbe gebaute React-App vor dem produktiven Core erscheint und erst nach erfolgreicher Go-Provisionierung in den normalen Handshake wechselt.
- `pnpm test:electron` baut die Electron-Artefakte vor jedem nativen Lauf neu.

### Nativer Desktopbetrieb

Seit Abschnitt 15.6 besitzt Electron zusätzlich Fensterbounds, Autostart, Tray und native Benachrichtigungen. Alle Freigaben stammen aus einer achtstufigen Projektion des autoritativen Corezustands. Aktives X blendet das Fenster aus; Beenden bleibt bis zu einem sicher inaktiven Zustand gesperrt. Details dokumentiert [Desktop-Betrieb und Einstellungen](desktop-operation.md).

## Grenzen

Installer und produktives Packaging sind im Phase-15-Releasepfad gebunden. Die Shell implementiert keine eigene Queue-, Config- oder Statistikengine und führt keine D2R-Aktion aus.

## Verwandte Features

- [Phase-15-Core-Vertrag](phase-15-core-contract.md)
- [Installierter Datenroot und Desktop-Einstellungen](installed-data-root.md)
- [Lokale Core-API und eingebettete Web-Anwendung](local-core-api.md)
- [Live-Dashboard und Session-Steuerung](live-dashboard.md)
- [Desktop-Betrieb und Einstellungen](desktop-operation.md)
- [First Run, Provisionierung und erste Route](first-run-onboarding.md)

---
*Zuletzt aktualisiert: 26. Juli 2026*
