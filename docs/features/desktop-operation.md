# Desktop-Betrieb und Einstellungen

## Überblick

Abschnitt 15.6 verbindet die Core-autoritären Operator-Einstellungen mit der installierten Windows-App und definiert das native Verhalten für Fenster, Tray, Benachrichtigungen und Beenden. Electron leitet seine Entscheidungen ausschließlich aus dem letzten Core-Snapshot ab und behandelt einen unbekannten verbundenen Zustand wie einen laufenden Abbruch. Dadurch kann die Desktop-App einen möglicherweise aktiven Core weder versehentlich noch blind beenden.

## Ort im Code

- **Settings-Seite:** `web/src/features/settings/SettingsFeature.tsx`
- **Desktop-Lifecycle:** `web/electron/desktop-lifecycle.ts`
- **Fensterbounds:** `web/electron/desktop-window.ts`
- **Electron Main/Preload:** `web/electron/main.ts`, `web/electron/preload.cts`
- **Core-Steuerung:** `web/electron/core-controller.ts`
- **Persistenz:** `<Datenroot>/desktop-settings.json` und `<Datenroot>/configs/operator-settings.local.yaml`
- **Native Tests:** `web/electron/e2e/desktop-shell.spec.ts`

## Funktionalität

### Settings-Seite

Die Seite lädt die revisionierten Operator-Einstellungen vom Go-Core und die kleinen Desktop-Einstellungen aus Electron. Editierbar sind charakterbezogene Difficulty und Queue, Sessionbudgets, Input-Opt-in, vier verschiedene Hotkeys, History-Retention, Autostart und der Onboarding-Merker. Die effektive Core-Konfiguration und ihr relativer Speicherort bleiben read-only sichtbar.

Operatoränderungen durchlaufen zuerst die Core-Vorschau. Update und Reset senden die erwartete Store-Revision, die aktuelle Supervisorgeneration und den Control-Token. Eine veraltete Revision bleibt als eigener Konfliktzustand sichtbar und kann nur durch Neuladen aufgelöst werden. Input- oder Hotkeyänderungen zeigen `restart_required`; der anschließend angebotene Neustart läuft kontrolliert über Electron und ist nur bei sicher inaktivem Core möglich. Während einer aktiven Session sind alle fachlichen Mutationen gesperrt.

### Desktop-Zustandsmatrix

| Desktopzustand | X im Fenster | Pause/Stop | Emergency | Beenden |
|---|---|---|---|---|
| `idle` | Bestätigung | gesperrt | gesperrt | erlaubt |
| `running` | in Tray | nur im aktiven Run | erlaubt | gesperrt |
| `pause_pending` | in Tray | gesperrt | erlaubt | gesperrt |
| `paused` | in Tray | gesperrt | erlaubt | gesperrt |
| `stop_pending` | in Tray | gesperrt | erlaubt | gesperrt |
| `cancelling` | in Tray | gesperrt | gesperrt | gesperrt |
| `error` | Bestätigung | gesperrt | gesperrt | erlaubt |
| `core_down` | Bestätigung | gesperrt | gesperrt | erlaubt |

Das Tray enthält ausschließlich Öffnen, den nicht interaktiven Status, Pause nach Run, Stop nach Run, Emergency Stop und Beenden. Commands sind single-flight und an den jüngsten Control-Token sowie die jüngste Supervisorgeneration gebunden. Eine zweite App-Instanz aktiviert nur das bestehende Fenster und erzeugt keinen Command.

### Fenster und Autostart

Normale Fensterbounds werden verzögert atomar gespeichert. Beim nächsten Start werden sie auf eine tatsächlich sichtbare Monitor-Arbeitsfläche begrenzt; die Zielgröße beträgt standardmäßig 1440×900 bei einer Mindestgröße von 1100×700. Maximierte, minimierte und Vollbildzustände überschreiben die normalen Bounds nicht.

Autostart ist standardmäßig aus und wird nur in der paketierten App über Windows Login Items gesetzt. Eine beschädigte Desktop-Settings-Datei wird vollständig verworfen; insbesondere bleibt Autostart nach der Recovery deaktiviert.

### Native Benachrichtigungen

Native Benachrichtigungen erscheinen ausschließlich, wenn das Fenster nicht fokussiert ist, und nur für:

- erfolgreich abgeschlossene Session → `#history`;
- terminalen Fehler → `#dashboard`;
- erreichte Pause zwischen Runs → `#dashboard`;
- verfügbare Version → `#settings`.

Ein Klick stellt dasselbe vorhandene Fenster wieder her, fokussiert es und navigiert über die begrenzte Preload-Bridge zum stabilen Hash-Ziel. Abschnitt 15.10 bindet den einmaligen stabilen Versionshinweis an; die Einstellungen bieten den einzigen manuellen Retry und ausschließlich die fest kompilierte Release-Seite.

### Quit- und Cancellation-Sicherheit

Aktive, vorgemerkte, pausierte, abbrechende und unbekannte Corezustände dürfen Electron nicht beenden. Die App führt den Operator stattdessen zum Dashboard und verlangt zuerst Stop nach Run oder den getrennten Emergency Stop. Erst ein vom Core bestätigter sicher inaktiver, terminaler oder getrennter Zustand darf den kontrollierten Kindprozess-Shutdown und anschließend `app.quit()` auslösen.

## Tests

- Unit-Tabellen prüfen alle acht Zustände, Tray-Freigaben, X-/Quit-Verhalten und Notification-Übergänge.
- Controller-Tests prüfen Generation, Control-Token, Pause, Stop, Emergency und doppelte Commands gegen einen Fake-Core.
- Fenster- und Store-Tests prüfen sichtbare Bounds, Defaults, striktes Schema, atomaren Re-Read und beschädigte Dateien.
- React-Tests prüfen Laden, Preview, Reset, Revisionkonflikt, Restart-required und Session-Locks.
- Playwright startet echtes Electron und prüft Bridge, Persistenz, zweite Instanz, aktive X→Tray-Semantik, Crash-Recovery und Offscreen-Clamping.

## Verwandte Features

- [Sichere Electron-Shell und Core-Kindprozess](desktop-shell.md)
- [Persistente Operator-Einstellungen](operator-settings.md)
- [Installierter Datenroot und Desktop-Einstellungen](installed-data-root.md)
- [Desktop-App-Shell und Designsystem](desktop-app-shell.md)

---
*Zuletzt aktualisiert: 26. Juli 2026*
