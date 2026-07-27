# Installierter Datenroot und Desktop-Einstellungen

## Überblick

Abschnitt 15.1 trennt installierte Benutzerdaten vom Repository und vom Installationsverzeichnis. Der Go-Core erhält seinen Root im Desktopbetrieb ausschließlich explizit über `--data-root`; ohne dieses Flag bleibt der bisherige Repository-CLI-Kontext erhalten. Ein frischer Root oder ein Import wird zunächst vollständig in einem benachbarten Stagingverzeichnis aufgebaut, mit den produktiven Core-Loadern validiert und erst danach atomar veröffentlicht.

## Ort im Code

- **Pakete:** `internal/app/`, `internal/config/`, `cmd/d2rbot/`
- **Einstieg:** `app.NewDataRootManager`, `config.LoadFromDataRoot`, CLI-Flag `--data-root`
- **Wichtige Dateien:** `internal/app/data_root.go`, `internal/app/data_root_reparse_windows.go`, `internal/config/config.go`, `web/electron/desktop-settings.ts`
- **Persistenz:** `%LOCALAPPDATA%\D2ROfflineFarmingBot\`

## Funktionalität

### Expliziter Core-Root

`config.LoadFromDataRoot` verlangt einen absoluten Root, lädt ausschließlich `configs/config.yaml` darunter und bindet den Telemetriepfad an `logs/telemetry`. Das YAML-Schema ist strikt: unbekannte Felder und weitere YAML-Dokumente werden abgelehnt. Ein gemeinsam angegebenes `--config` und `--data-root` ist verboten, damit im Desktopbetrieb keine zweite Pfadautorität entsteht.

### Defaultbundle und Veröffentlichung

`BuildDefaultBundle` erzeugt ein read-only Bundle aus den versionierten Beispieldateien. `bundle.json` listet jeden relativen Pfad mit SHA-256 auf; unbekannte Manifestfelder, ungebundene Dateien, Hashdrift, Traversal, Symlinks und Windows-Reparse-Points werden abgelehnt. Farming-Routen werden absichtlich nicht als Defaults verteilt, weil die erste Route später über denselben produktiven Recording-Workflow entsteht.

Der Manager legt Staging im Elternverzeichnis des Zielroots an. Config, Offsets, Pickit, Route-Assignments, Route-Lifecycle, Kandidaten, Town-Routen und History werden durch ihre produktiven Loader geprüft. Erst dann wird das gesamte Verzeichnis mit einem Rename veröffentlicht und erneut gelesen. Jeder Fehler entfernt nur das eindeutig eigene Staging beziehungsweise den gerade veröffentlichten ungültigen Stand; ein bereits vorhandener Zielroot wird weder gemergt noch überschrieben.

### Import

Ein Import kopiert `configs/` und direkte `.jsonl`-Dateien aus `logs/telemetry/`. Alte Laufzeitlogs, Backups und Diagnose-ZIPs bleiben ausgeschlossen. Die Quelle wird nur gelesen und nie verändert. Fachkonfiguration muss vollständig valide sein; beschädigte Telemetriedateien dürfen entsprechend dem vorhandenen History-Vertrag isoliert als Diagnose erscheinen.

### Provisionierung vor dem produktiven Core

Bei einem wirklich frischen installierten Root läuft der produktive Core noch nicht. Electron lädt dieselbe gebaute React-App lokal in einem schmalen Modus mit ausschließlich „Neu“ und „Import“. Nach der nativen Auswahl führt ein kurzlebiger Go-Core genau eine `DataRootManager`-Operation aus und beendet sich. Erst nach erfolgreicher atomarer Veröffentlichung startet der produktive Core mit API, Hotkeys und Runtime. Der Renderer erhält keinen Importpfad und keinen Dateisystemzugriff.

Das entbehrliche Chromium-Laufzeitprofil liegt dabei getrennt im benutzereigenen Windows-Tempbereich und wird stabil aus dem kanonischen Datenroot abgeleitet. Es ist weder Config- noch Datensicherung und darf den Zielroot vor der Core-Provisionierung nicht anlegen. Auch Fensterbounds werden im Pre-Core-Modus noch nicht gespeichert. Dadurch bleibt der Zielroot bis zum atomaren Rename des validierten Core-Stagings tatsächlich abwesend.

### Desktop-Einstellungen

`DesktopSettingsStore` liegt im Electron-Main-Bereich und besitzt ausschließlich `window_bounds`, `autostart` und `onboarding_completed` im strikten JSON-Schema 1. Fachwerte wie Input, Queue oder Budgets sind ausdrücklich ausgeschlossen. Ein Save schreibt eine eindeutige Temp-Datei im selben Verzeichnis, flusht sie, ersetzt atomar und liest den persistierten Vertrag erneut. Unbekannte Felder, unbekannte Versionen oder ungültige Bounds setzen den gesamten effektiven Desktopzustand fail-closed auf Defaults zurück; Autostart ist dabei aus.

Bei einem Import liest Electron vor der Core-Provisionierung ausschließlich `onboarding_completed` aus dem streng validierten Desktop-Store der Quelle. Nach der atomaren Veröffentlichung lädt es den Ziel-Store erneut und persistiert einen importierten Abschluss, bevor der produktive Renderer startet. Autostart und Fensterbounds werden bewusst nicht übernommen. Fehlt der Quelldatensatz oder ist er ungültig, bleibt der Abschluss fail-closed auf `false` und der Assistent wird regulär angeboten. Der Go-Core kopiert weiterhin ausschließlich seine eigenen fachlichen Daten und erhält keine Ownership über `desktop-settings.json`.

## Datenmodell

- `DataRootStatus`: `published` oder `existing`
- `DataRootResult`: kanonischer Root, Status und isolierte History-Diagnosen
- `DefaultBundleManifest`: Schema-Version und SHA-256-gebundene Dateien
- `DesktopSettings`: Schema 1, optionale Fensterbounds, Autostart und Onboardingstatus

## Operator / CLI

- Repositorybetrieb: `d2rbot` beziehungsweise `--config <pfad>` wie bisher.
- Desktop-Core: `d2rbot --data-root <absoluter-pfad>`.
- Einmalige Desktop-Provisionierung: `d2rbot --provision-data-root --data-root <ziel>` mit exakt einem von `--defaults-root` oder `--import-root`.
- Die CLI protokolliert Importfehler mit stabilem Phase-15-Reason-Code; eine lokale JSON-Diagnose liegt neben dem vorgesehenen Zielroot.

## Abhängigkeiten

- Go-Standardbibliothek für kanonische Pfade, SHA-256, atomisches Rename und Flush.
- Windows-Dateiattribute für Reparse-Point-Erkennung.
- Node-Standardbibliothek ausschließlich im Electron-Main-Modul für `desktop-settings.json`.

## Verwandte Features

- [Phase-15-Core-Vertrag](phase-15-core-contract.md)
- [Session-Konfiguration und Inspect](session-configuration.md)
- [History-Reader und In-Memory-Index](history-reader-index.md)
- [First Run, Provisionierung und erste Route](first-run-onboarding.md)

---
*Zuletzt aktualisiert: 26. Juli 2026*
