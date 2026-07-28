# First Run, Provisionierung und erste Route

## Überblick

Abschnitt 15.7 führt einen frischen installierten Datenroot sicher bis zum ersten echten Farming-Run. Vor der Veröffentlichung eines Datenroots läuft dieselbe gebaute React-App in einem schmalen Provisionierungsmodus, während der produktive Core noch vollständig aus bleibt. Nach erfolgreicher Anlage oder einmaligem Import startet der normale Core; der anschließende neun Schritte umfassende Assistent projiziert ausschließlich Core-Zustände und führt optional in denselben Routenworkflow wie die Routenbibliothek.

## Ort im Code

- **Go-Provisionierung:** `cmd/d2rbot/main.go`, `internal/app/data_root.go`
- **Electron Main/Preload:** `web/electron/main.ts`, `web/electron/preload.cts`
- **Kindprozess:** `web/electron/core-process.ts`
- **Renderer:** `web/src/features/onboarding/ProvisioningFeature.tsx`, `web/src/features/onboarding/OnboardingFeature.tsx`
- **Routenübergabe:** `web/src/features/routes/RouteFeature.tsx`
- **Core-Readiness:** `internal/api/live_backend_routes.go`, `internal/api/route_dto.go`

## Funktionalität

### Provisionierung vor dem produktiven Core

Fehlt `configs/config.yaml` im kanonischen Datenroot, lädt Electron den normalen gebauten Renderer per `file:` und startet weder API, Hotkeys noch D2R-/Input-Laufzeit. React bietet ausschließlich „Neu beginnen“ und „Bestehende Daten importieren“ an. Die native Ordnerauswahl bleibt im Electron-Main-Prozess; der Renderer erhält weder den Pfad noch eine Dateisystembrücke.

Electron verwendet für diesen lokalen Renderer ein getrenntes entbehrliches Chromium-Laufzeitprofil im Windows-Tempbereich. Weder dieses Profil noch Fensterbounds oder `desktop-settings.json` dürfen den kanonischen Zielroot vorab erzeugen. Der Core erhält deshalb bei „Neu“ garantiert einen nicht existierenden Zielroot und kann seinen vollständig validierten Stagingstand weiterhin ohne Merge atomar veröffentlichen.

Electron startet für genau eine Operation denselben gebündelten Go-Core mit `--provision-data-root` und entweder `--defaults-root` oder `--import-root`. Dieser kurzlebige Prozess verwendet den produktiven `DataRootManager`, gibt einen streng begrenzten JSON-Vertrag aus und beendet sich. Erst nach atomarer Veröffentlichung und erfolgreicher Validierung startet Electron den produktiven Core mit privatem Handshake.

Importiert die App eine bestehende Phase-15-Installation mit abgeschlossenem Onboarding, übernimmt Electron diesen einen eigenen Desktopwert vor dem Start des produktiven Renderers. Dadurch erscheint der Assistent nicht einmalig zwischen Import und erstem Neustart. Autostart und Fensterposition der Quelle bleiben unberührt; ein älterer Import ohne validen Desktopabschluss durchläuft weiterhin die normale First-Run-Führung.

### Neun Schritte

Der First-Run-Assistent umfasst Willkommen, System, D2R, Safety, Input, Charakter, Readiness, erste Route und Abschluss. Safety wird bewusst vor dem Input-Opt-in erklärt; die Input-Freigabe liegt wiederum vor der D2R-Charakterbestätigung, weil deren verifizierter Home-/Down-/Play-Ablauf kontrollierten Input benötigt. Versionszustand, Privilegienkonflikt, bestätigter Charakter und Difficulty, Inputfreigabe, Hotkeys, Pickit-Zuordnung sowie Routen-Voraussetzungen stammen aus bestehenden Core- und API-Verträgen.

Im Charakterschritt ist `session.character` nur eine optionale Startvorauswahl. „Spielstände neu laden“ stößt einen neuen begrenzten Scan an. Jeder sichtbare Charakter zeigt seine read-only erkannte Klasse, Unterstützung, Profil, Lootprofile und den Status der automatischen D2R-Auswahl. Nicht vorbereitete Saves bleiben sichtbar und erben weder Klasse noch Profil vom aktuellen Run. Stabile technische Gründe werden zentral in klare deutsche Handlungsanweisungen übersetzt; rohe IDs und interne Pfade erscheinen nicht.

Für eine unterstützte Klasse liefert der Core ausschließlich freigegebene kompatible Profile. Bei genau einem Profil zeigt React festen Text, bei mehreren ein Dropdown mit dem Entwickler-Default. „Profil und Lootprofile bestätigen“ speichert Klasse und Profil gemeinsam und ergänzt ausschließlich vollständig fehlende Standard-Pickit-Ketten. Die normale Settings-Seite enthält keine Profil- oder Defaultauswahl.

Fehlt der Auswahlbeleg, erklärt derselbe Schritt: D2R starten, Offline-Charakterauswahl öffnen, den genannten Charakter markieren, markiert lassen und zur App zurückkehren. Erst nach einer ausdrücklichen Checkbox-Bestätigung wird „Auswahlbild jetzt speichern“ aktiv. Der Core erfasst nur den bestehenden Auswahlbereich und sendet außer dem protokollierten Fokus kein Input. Setup und Capture laden Katalog, Vorschau, OperatorSettings und Recording-Voraussetzungen neu, ohne den aktuellen Schritt zu verlassen.

Ein inkompatibler D2R-Build blockiert den fachlichen Fortschritt ohne Override. Input wird nur über die revisions- und generationsgebundene Operator-Settings-API freigegeben. Der Assistent unterscheidet den gespeicherten Operatorwert vom effektiven `StatusDTO.input` des laufenden Controllers. Charakterauswahl, Weiter-Schaltfläche und serverseitiger Apply bleiben gesperrt, bis der kontrollierte Core-Neustart tatsächlich `enabled=true`, `paused=false` und `stopped=false` projiziert. Überspringen persistiert zuerst `input.enabled=false` und markiert erst danach das getrennte Desktop-Onboarding als abgeschlossen.

Ein durch die Input-Freigabe ausgelöster kontrollierter Core-Neustart wechselt den zufälligen Loopback-Port und lädt den Renderer neu. Electron übernimmt dabei ausschließlich den aktuellen, validierten Onboarding-Schritt in die neue Bootstrap-URL; der Renderer entfernt diesen einmaligen Marker unmittelbar nach dem Wiederaufbau. Dadurch bleibt der Assistent auf „Input“, bis der neue Core die effektive Freigabe bestätigt. Sonstige lokale UI-Zustände oder Fachwerte werden nicht über die Desktop-Grenze transportiert.

### Erste Route

Countess ist empfohlen und vorausgewählt; Mephisto bleibt wählbar. Der Core projiziert Wegpunkt-, Teleport-, Town-Portal- und Pickit-Voraussetzungen mit stabilen Gründen. Die Übergabe öffnet denselben `RecordingCoordinator` der Routenbibliothek und erzeugt weder eine zweite Aufnahmeengine noch einen synthetischen Lauf.

Die Aufnahme startet nicht per F9. Der Benutzer öffnet aus dem Onboarding den Routenbereich und klickt dort beim gewünschten Run auf „Aufnahme starten“. Er bleibt am Startwegpunkt, bis der Core den Zustand `recording` beziehungsweise „Aufnahme läuft“ meldet, bewegt den Charakter anschließend manuell entlang der gewünschten Route und drückt erst an der gewählten Kampfposition F9. F9 beendet und friert ausschließlich eine bereits laufende Aufnahme ein.

Solange für den bestätigten Kontext noch kein Run eine veröffentlichte, verwendbare oder `runtime_validation_required` Route besitzt, steht „Einrichtung fortsetzen“ als erster fachlicher Dashboard-Abschnitt unmittelbar unter dem Seitenkopf und damit vor Core-Status, Charakterauswahl und Queue. Eine fehlende Route für einen weiteren optionalen Run hält die Ersteinrichtung nicht offen. Nach jeder Publish-, Replace-, Archive- oder Restore-Mutation berechnet der Core den Run-Katalog aus den neuen Lifecycle- und Assignment-Autoritäten neu; das Dashboard lädt diese Projektion auf `route_library_changed` ebenfalls neu.

Eine Übergabe aus diesem Abschnitt oder aus dem Onboarding markiert den Routenbereich als Teil der laufenden Einrichtung. Dort bleibt ein sichtbarer manueller Rückweg „Zurück zur Einrichtung“ erhalten und öffnet den Assistenten wieder bei „Erste Route“; nach seinem Abschluss wird das Dashboard sichtbar. Eine automatische Rückkehr direkt nach F9 ist absichtlich ausgeschlossen: Die Aufnahme ist zu diesem Zeitpunkt nur ein Kandidat und soll zuerst im selben Bereich isoliert getestet und veröffentlicht werden.

Eine fehlgeschlagene oder per Emergency Stop abgebrochene Aufnahme besitzt kein Resume. Der Assistent erklärt den sauberen Neustart; der unfertige Workflow wird nicht als Route behandelt. Der Abschluss ist auch ohne Route erlaubt. Solange der Katalog eine fehlende Route meldet, bleibt auf dem Dashboard der konkrete Einstieg „Erste Route aufnehmen“ sichtbar.

## Datenmodell

- `CoreProvisioningResult`: Schema 1, Veröffentlichungsstatus und Anzahl isolierter History-Diagnosen
- `RecordingPrerequisiteDTO`: `waypoint`, `teleport`, `town_portal` oder `pickit`, jeweils mit Readiness und optionalem Core-Grund
- `desktop-settings.json`: enthält nur `onboarding_completed`; Import- und Safety-Zustände bleiben Core-eigen

## Operator / CLI

Der Provisionierungsmodus ist ausschließlich für Electron bestimmt:

```text
d2rbot.exe --provision-data-root --data-root <ziel> --defaults-root <bundle>
d2rbot.exe --provision-data-root --data-root <ziel> --import-root <quelle>
```

Defaults und Import sind gegenseitig exklusiv. Der produktive Core darf vor erfolgreicher Provisionierung nicht laufen. D2R und Farming werden zu keinem Zeitpunkt automatisch gestartet.

Der Assistent kann später in den Einstellungen erneut geöffnet werden. Dies setzt einen erfolgreichen Import nicht zurück.

## Abhängigkeiten

- bestehender `DataRootManager` für Anlage, Import, Validierung und atomare Veröffentlichung
- bestehende Operator-, Status-, Katalog-, Hotkey- und Routen-APIs
- Electron-Dialog ausschließlich für die native Importordnerauswahl

## Verwandte Features

- [Installierter Datenroot und Desktop-Einstellungen](installed-data-root.md)
- [Sichere Electron-Shell und Core-Kindprozess](desktop-shell.md)
- [Tatsächliches D2R-Versionsgate](d2r-version-gate.md)
- [Geführte Farming-Routenaufnahme](guided-route-recording.md)
- [Routenbibliothek und Setup-Assistent](route-dashboard.md)

---
*Zuletzt aktualisiert: 28. Juli 2026*
