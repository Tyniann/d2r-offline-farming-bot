# Internationalisierung Deutsch und Englisch

## Überblick

Die installierte Windows-App kann ihre Bedienoberfläche ohne Reload oder Core-Neustart zwischen Deutsch und Englisch wechseln. React, Electron Main, Tray, native Dialoge, Benachrichtigungen und Recovery verwenden denselben gespeicherten Sprachwert. Der Go-Core bleibt sprachneutral und liefert Codes, IDs und strukturierte Parameter.

## Ort im Code

- **Renderer-i18n:** `web/src/i18n/`
- **Manuelle Kataloge:** `web/src/i18n/locales/de.json`, `web/src/i18n/locales/en.json`
- **Generierte D2R-Namen:** `web/src/i18n/generated/game.de.json`, `web/src/i18n/generated/game.en.json`
- **Sprachschalter:** `web/src/app/LanguageSwitcher.tsx`
- **Electron-i18n:** `web/electron/i18n.ts`
- **Desktop-Persistenz:** `web/electron/desktop-settings.ts`
- **Recovery:** `web/electron/recovery.html`, `web/electron/recovery.ts`
- **CASC-Generator:** `tools/generate-item-catalog/`
- **Config:** `language` liegt ausschließlich in `<Datenroot>/desktop-settings.json`

## Funktionalität

### Sprachwahl und Persistenz

Ein neuer Desktop-Store leitet `de` für eine deutsche Windows-Sprache ab. Alle anderen Systemsprachen starten mit `en`. Bestehende Schema-1- und Schema-2-Dateien migrieren mit `de` auf Schema 3. Der Sidebar-Schalter speichert `de` oder `en` atomar, bevor React die aktive Sprache wechselt. Bei einem Schreibfehler bleibt die bisherige Sprache aktiv.

Der Renderer lädt die Desktop-Einstellungen vor dem ersten React-Render. Im Entwicklungsbetrieb ohne Electron-Bridge verwendet er die Browsersprache nur für die aktuelle Sitzung. Jeder erfolgreiche Wechsel aktualisiert das `lang`-Attribut des Dokuments und den Fenstertitel.

### Kataloge und Darstellung

Die beiden manuellen JSON-Kataloge besitzen denselben Schlüsselbaum. Komponenten verwenden `react-i18next`; Datum, Zahlen, Prozent, Bytewerte und Dauer laufen über die zentralen Formatter in `web/src/i18n/format.ts`. Presenter ordnen bekannte Problem-, Reason-, Fortschritts- und Routen-Codes festen Katalogschlüsseln zu.

Ein unbekannter Core-Code zeigt eine lokalisierte allgemeine Meldung und den unveränderten Code für Supportzwecke. Rohe Core-Fehler, `stderr`, Stacktraces, Pfade und Request-IDs sind Diagnosewerte und werden nicht als Bedienertext gerendert.

### Electron und Recovery

Electron Main verwendet kein i18n-Paket aus `node_modules`. Der kleine Adapter lädt nach `app.whenReady()` die beiden in `dist-electron/locales/` kopierten JSON-Kataloge und löst ausschließlich `desktop.*` auf. Ein Sprachwechsel baut das Tray sofort neu auf. Spätere Benachrichtigungen und Dialoge verwenden die gespeicherte Sprache.

Recovery erhält Sprache, Titel und Body bereits aufgelöst vom Main-Prozess. Die statische HTML-Datei enthält keine fest verdrahtete deutsche oder englische Meldung. Prozessfehler bleiben im Log; Recovery sieht nur eine stabile `DesktopCoreReason`-ID.

### D2R-Namen

Gebiets-, Skill-, Itembasis-, Unique-, Set- und Setnamen werden aus den lokalen CASC-Extrakten unter `.tmp/d2r-excel` generiert. Die Ausgabe nennt D2R-Build, Quelldateien und stabile Schlüsselarten. Fehlende oder doppelte referenzierte Schlüssel sowie leere Werte für `deDE` oder `enUS` brechen die Generierung ab. Die Runtime liest keine CASC-Datei.

### Installer

Eine Setup-Datei enthält Deutsch und Englisch. Beim interaktiven Start wählt der Operator die Installersprache in einem vorgeschalteten NSIS-Dialog. Der Uninstaller verwendet dieselbe Sprache. Die eigenen Uninstall-Warnungen existieren in beiden Sprachen und verlangen weiterhin zwei Bestätigungen, bevor der feste Datenroot gelöscht wird. Silent-Install und Silent-Uninstall zeigen keinen Sprachauswahldialog; Silent-Uninstall erhält den Datenroot ohne Nachfrage.

## Datenmodell

- `DesktopSettings` Schema 3 enthält `language: "de" | "en"` neben Fensterbounds, Autostart, Onboarding und der letzten Auswahl.
- `ProblemDTO` enthält `code`, erlaubte `params` und bei HTTP-Fehlern `request_id`. Es enthält keine Bedienermeldung.
- Fortschritt, Routenanleitungen und Historie transportieren stabile Codes und strukturierte Parameter.
- Generierte D2R-Kataloge verwenden stabile Area-IDs, Skill-Keys, Itemcodes sowie Identitäts- und Set-Keys.
- Sichtbare `display_name`-Felder sind klassifiziert: Pickit-Identitäten sind generierte D2R-Fallbacks hinter stabilen CASC-Keys; bekannte Kampfprofile werden über ihre ID übersetzt und verwenden `display_name` nur als Fallback für benutzerdefinierte Profile; Routennamen sind unveränderter Benutzerinhalt. Systemnamen für Runs, Schwierigkeiten, Klassen und Skills kommen nicht aus `display_name`, sondern aus stabilen IDs und den Sprachkatalogen.

## Operator / CLI

- Der Sprachschalter steht unten in der Sidebar direkt über Verbindung und Version.
- `DE` und `EN` sind sichtbare, tastaturbedienbare Toggle-Buttons.
- Der Wechsel benötigt weder App-Reload noch Core-Neustart.
- Die Auswahl bleibt nach einem vollständigen App-Neustart erhalten.
- Es gibt keinen zusätzlichen CLI-Sprachparameter und keine Spracheinstellung in der Core-YAML.

## Abhängigkeiten

- `i18next` 26.4.0 und `react-i18next` 17.0.12 nur für den Renderer.
- Node-Standardbibliothek für den frameworkfreien Electron-Adapter.
- Lokale CASC-Extraktdaten als einzige Quelle für D2R-Namen.

## Verwandte Features

- [Lokale Core-API und eingebettete Web-Anwendung](local-core-api.md)
- [Sichere Electron-Shell und Core-Kindprozess](desktop-shell.md)
- [Installierter Datenroot und Desktop-Einstellungen](installed-data-root.md)
- [Persistente Operator-Einstellungen](operator-settings.md)
- [Windows-Installer und lokale Releasepipeline](windows-packaging.md)

---
*Zuletzt aktualisiert: 22. August 2026*
