# Windows-Installer und lokale Releasepipeline

## Überblick

Abschnitt 15.11 liefert die bestehende Electron-App und den Go-Core als per-user NSIS-Installer für Windows x64. Ein expliziter Releaseparameter speist Electron-Metadaten, Windows-Dateiversion, Core-Ldflags und Installername. Die lokale Pipeline veröffentlicht ausschließlich einen Installer und seine SHA-256-Datei; es gibt kein CI-Publishing, Code Signing oder Auto-Update. Dieselbe Setup-Datei enthält Deutsch und Englisch.

## Ort im Code

- **Pipeline:** `scripts/build-release.ps1`
- **Electron-Builder-Konfiguration:** `web/package.json`
- **NSIS-Erweiterung:** `web/build/installer.nsh`
- **App-Icon:** `web/electron/app-icon.ico` (16/32/48/256, dasselbe Town-Portal-PNG)
- **Build-Icon:** `web/electron/create-build-icon.mjs`
- **Paketprüfung:** `web/electron/verify-package.mjs`
- **Installierter Smoke-Test:** `web/electron/package-smoke.mjs`
- **Defaults-Builder:** `tools/build-default-bundle/main.go`
- **Installationshinweis:** `docs/INSTALLATION.md`

## Funktionalität

### Artefakt und Inhalt

Der Aufruf

```powershell
.\scripts\build-release.ps1 -Version 0.23.0
```

verlangt ein stabiles SemVer und einen lesbaren Git-Commit. Der Core wird mit `-trimpath` sowie `version.Version` und `version.Commit` gebaut. Electron Builder erhält dieselbe Version als `extraMetadata`; Produktdateiversion, `app.asar/package.json`, Core-`--version`, Core-Sidebarprojektion und Installername werden gegeneinander geprüft. Ein `dev`-Commit bricht den Build ab.

`extraResources` enthält nur:

- `core/d2rbot.exe`;
- das vom produktiven Go-Builder erzeugte hashgebundene `defaults/`-Bundle;
- `INSTALLATION.md`.

Renderer, Preload und Electron Main liegen im ASAR. Die Paketprüfung verlangt zusätzlich `dist-electron/locales/de.json`, `dist-electron/locales/en.json`, Recovery und den Renderer-Einstieg. Ein eigener ASAR-Audit lehnt `node_modules`, Test-/Buildwerkzeuge, Workspacepfade, Logs, Diagnosen und lokale Umgebungsdateien ab. Der entpackte Produktbaum wird zusätzlich gegen lokale Config, Telemetrie, Secrets und fehlende Pflichtdateien geprüft.

### Installer und Deinstallation

NSIS installiert ohne Maschinenkontext für den aktuellen Benutzer, legt einen Startmenüeintrag an und erzwingt weder Desktopshortcut noch Administratorrechte. Das Produkt besitzt eine feste App-ID und ein Windows-Icon mit 16-, 32-, 48- und 256-Pixel-Kacheln desselben Town-Portal-Zeichens. `installerLanguages` enthält `de_DE` und `en_US`; die früher erzwungene deutsche LCID entfällt. Beim interaktiven Start fragt dieselbe Setup-Datei nach Deutsch oder Englisch. Silent-Installationen überspringen diese Auswahl.

Die Deinstallation erhält `%LOCALAPPDATA%\D2ROfflineFarmingBot\` standardmäßig. Nur im interaktiven Uninstaller kann der Operator nach zwei standardmäßig auf "Nein" stehenden Bestätigungen exakt diesen festen Root zusätzlich löschen. Beide eigenen Warnungen existieren als deutsche und englische `LangString`-Werte. Silent-Uninstall fragt nicht und erhält die Daten.

### Reproduzierbare Prüfkette

Die Pipeline führt mit frozen Lockfile nacheinander Clientgeneration, alle Vitest-Fälle, Renderer-/Electron-Typecheck, Produktionsbuild, native Electron-Tests, alle Go-Tests und Lint aus. Danach folgen Core-/Defaults-Build, Installer, Inhaltsscan und ein temporärer Installations-Smoke:

1. per-user Silent-Install in einen eindeutigen Workspace-Tempordner;
2. Start der installierten App gegen einen nachweislich noch nicht existierenden Datenroot und Auswahl „Neu“ in derselben React-Shell;
3. frische Core-Provisionierung aus den installierten Defaults und anschließender Start der echten React-Shell mit realem Core;
4. App-, Core- und Sidebarversionsvergleich ohne `dev` sowie frische Provisionierung mit beiden globalen UI-Belegen, aber ohne namensgebundenen Charakterbeleg;
5. erneute Installation als Upgrade bei unverändertem Datenroot;
6. Silent-Uninstall mit nachgewiesen erhaltenem Datenroot.

Der manuelle Sprach-Smoke startet dieselbe Setup-Datei zweimal und wählt im vorgeschalteten Dialog einmal Deutsch und einmal Englisch. Er prüft anschließend beide Uninstall-Warnungen. Die Texte müssen in der gewählten Sprache erscheinen, beide Dialoge müssen standardmäßig "Nein" wählen und ein Abbruch muss den Datenroot erhalten.

Temporäre Builder-, Ressourcen-, Installations- und Profilverzeichnisse werden anschließend entfernt. Unter `dist/release/` verbleiben exakt `D2R-Offline-Farming-Bot-<Version>-Setup.exe` und die zugehörige `.sha256`.

Für eine ausdrücklich angeordnete manuelle Gate-Iteration darf derselbe Builder nach separat dokumentierten grünen Prüfungen mit `-SkipAutomatedChecks -SkipProductSmoke` ausschließlich Build und statische Inhaltsaudits ausführen. Ohne diese Schalter bleibt die vollständige Prüfkette unverändert verpflichtend. Der Renderer-Zielordner wird vor jedem Build geleert, damit das ASAR keine nicht mehr referenzierten gehashten Altbundles enthält.

## Operator / Release

- Zielsystem: Windows 10/11 x64.
- D2R bleibt manuell gestartet und Offline/Singleplayer-only.
- Der unsignierte Installer kann SmartScreen auslösen; es gibt keine Umgehungsautomatik.
- GitHub benötigt als eigene Releaseassets nur Installer und SHA-256. Sourcearchive entstehen aus dem Tag.

## Verwandte Features

- [Sichere Electron-Shell und Core-Kindprozess](desktop-shell.md)
- [Installierter Datenroot und Desktop-Einstellungen](installed-data-root.md)
- [First Run, Provisionierung und erste Route](first-run-onboarding.md)
- [Lokales Diagnosepaket und Versionshinweis](diagnostics-and-update-check.md)
- [Internationalisierung Deutsch und Englisch](internationalization.md)

---
*Zuletzt aktualisiert: 22. August 2026*
