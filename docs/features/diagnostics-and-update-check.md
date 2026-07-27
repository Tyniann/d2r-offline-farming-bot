# Lokales Diagnosepaket und Versionshinweis

## Überblick

Abschnitt 15.10 ergänzt zwei strikt getrennte Desktopdienste: Der Go-Core erzeugt ein lokales, redigiertes Diagnose-ZIP aus einer festen Allowlist. Electron Main fragt einmal pro paketiertem App-Start den jüngsten stabilen GitHub-Release ab. Es gibt weder Upload noch automatischen Download oder Installation.

## Ort im Code

- **Core-Collector:** `internal/app/diagnostic_bundle.go`
- **Core-API:** `internal/api/diagnostic_*.go`
- **Update-Prüfung:** `web/electron/update-check.ts`
- **Desktop-Grenze:** `web/electron/main.ts`, `web/electron/preload.cts`
- **Renderer:** `web/src/features/settings/SettingsFeature.tsx`
- **API:** `POST /api/v1/diagnostics/bundle`

## Funktionalität

### Diagnose-ZIP

Der Collector ist an den absoluten installierten Datenroot gebunden. Standardmäßig enthält das ZIP Versions-/Buildmetadaten, redigierte Core-Konfiguration, Lifecycle-/Assignment-Metadaten, die letzten direkten Logdateien und History-Reader-Diagnosen. Tokens, Secrets, Passwörter, Autorisierungswerte und absolute Benutzerpfade werden vor dem Schreiben redigiert.

Vollständige JSONL-Telemetrie und Routenkoordinaten sind zwei getrennte, standardmäßig deaktivierte Opt-ins. Savegames, Speicherabbilder und Screenshots gehören nie zur Allowlist. Quellen müssen reguläre, begrenzte Dateien innerhalb der festen Unterverzeichnisse sein; Symlinks/Reparse-Pfade, übergroße Dateien und unerwartete Typen brechen fail-closed mit einem stabilen Diagnose-Reason ab.

Das ZIP wird unter `diagnostics/diagnose-<UTC>-<Zufall>.zip` atomar veröffentlicht. Die Core-API liefert ausschließlich den neutralen Dateinamen, Größe und die bestätigten Opt-ins. Electron akzeptiert beim Anzeigen im Explorer nur dieses kompilierte Namensformat und löst es selbst gegen den Datenroot auf.

### Versionshinweis

Nur eine paketierte App startet nach `app.ready` genau eine Abfrage an:

`https://api.github.com/repos/Tyniann/d2r-offline-farming-bot/releases/latest`

Ein manueller Retry in den Einstellungen ist die einzige weitere Anfragequelle. Verglichen werden ausschließlich veröffentlichte stabile SemVer-Werte. Gleich oder älter bedeutet „aktuell“, neuer bedeutet „verfügbar“. Prerelease, Draft, private/fehlende Releases, Offlinebetrieb, Timeout, Rate Limit und ungültige Antworten werden neutral als nicht verfügbar projiziert.

Die Netzwerkantwort kann keine externe Ziel-URL bestimmen. Ein Operator-Klick darf ausschließlich die fest kompilierte HTTPS-Seite `https://github.com/Tyniann/d2r-offline-farming-bot/releases` öffnen. Die App lädt oder installiert nichts.

## Tests

- Fake-Fetch deckt gleich, älter, neuer, Prerelease, 404/private, offline, Timeout, Rate Limit und malformed JSON ab.
- ZIP-Tests prüfen Token-/Pfadredaktion, Standardausschluss und getrennte Telemetrie-/Routen-Opt-ins sowie Symlink-Ablehnung.
- API-Tests prüfen Control-Token und die pfadfreie Antwort.
- React-Tests prüfen Status, manuellen Retry, feste Release-Aktion und Diagnose-Defaults.
- Native Electron-Tests prüfen weiterhin die vollständige, explizite Preload-Allowlist bei deaktivierter Node-Integration.

## Verwandte Features

- [Sichere Electron-Shell und Core-Kindprozess](desktop-shell.md)
- [Desktop-Betrieb und Einstellungen](desktop-operation.md)
- [History-Reader und In-Memory-Index](history-reader-index.md)
- [Installierter Datenroot und Desktop-Einstellungen](installed-data-root.md)

---
*Zuletzt aktualisiert: 26. Juli 2026*
