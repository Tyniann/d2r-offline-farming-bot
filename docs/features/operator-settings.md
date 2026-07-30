# Persistente Operator-Einstellungen

## Überblick

Abschnitt 15.2 macht den Go-Core zur einzigen Autorität für alle über die GUI editierbaren Fach- und Safetywerte. Abschnitt 16.2 hebt `operator-settings.local.yaml` auf einen strikten, revisionierten Schema-2-Vertrag und ergänzt das charakterbezogene Setup-Paar. Fehlt die Datei in einem frischen Datenroot, wird Revision 1 aus der validierten Basiskonfiguration initialisiert. Schema 1 wird weder gelesen noch migriert.

## Ort im Code

- **Paket:** `internal/app/`
- **Store:** `OperatorSettingsStore` in `internal/app/operator_settings.go`
- **HTTP-Vertrag:** `internal/api/operator_settings_dto.go`, `internal/api/operator_settings_backend.go`, `internal/api/server.go`
- **Schema:** `internal/api/schema/openapi.json`
- **Generierter Client:** `web/src/api/generated.ts`
- **Persistenz:** `<Datenroot>/configs/operator-settings.local.yaml`
- **Backups:** `<Datenroot>/backups/operator-settings-*.yaml`
- **UI:** `web/src/features/settings/` (`SettingsFeature.tsx`, Tabs, `QueueEditor`, `SettingsActionBar`, `settingsDiff.ts`)
- **Shell-Guard:** `web/src/app/App.tsx` (`onDirtyChange`, Navigationsdialog)
- **Styles:** `web/src/app/app.css` (`.settings-tabs`, `.settings-actionbar`, `.settings-queue-editor`)

## Funktionalität

### Schema und Werte

Der Store enthält eine positive Revision und:

- `last_character` als zuletzt erfolgreich vom Core bestätigten Bedienkontext;
- pro kanonischem Charakternamen eine nicht leere, geordnete und duplikatfreie Queue sowie `normal`, `nightmare` oder `hell` als letzte Difficulty;
- `character_class` und `combat_profile` als gemeinsam leeres oder gemeinsam gesetztes Setup-Paar; ein gesetztes Profil muss bekannt, für Setup freigegeben und mit der Klasse kompatibel sein;
- globale Grenzen für maximale Runs, Dauer, aufeinanderfolgende Fehler und Restarts;
- explizite Input-Freigabe sowie paarweise verschiedene, vom Input-Core unterstützte Pause-, Stop-after-run-, Recording-Finish- und Emergency-Hotkeys;
- History-Retention mit sicheren Defaults `retention_enabled: true` und `retention_days: 60`.

Unbekannte YAML-Felder, weitere YAML-Dokumente, Schemaabweichungen einschließlich Schema 1, Revision `0`, halbe oder ungültige Setup-Paare, leere oder duplizierte Queues, unbekannte Run-IDs, unendliche beziehungsweise außerhalb der Grenzen liegende Budgets und ungültige Hotkeys werden vollständig abgelehnt.

### Initialisierung, Schreiben und Backups

Beim ersten Öffnen erzeugt der Store Revision 1 aus den bereits validierten `config.yaml`-Werten und den bekannten Charakteren. Jede echte Änderung erzeugt vor dem Replace ein vollständiges Backup des alten Standes. Backups sind nach Revision stabil sortierbar und werden auf exakt zehn Dateien begrenzt.

Eine erfolgreiche Charakterauswahl aktualisiert `last_character` und dessen `last_difficulty` gemeinsam über denselben Store, ohne Queue, Setup-Paar, Budgets, Input oder History zu ersetzen. Beim nächsten Core-Start wird dieser Kontext vor dem Aufbau der Runtime angewendet. Es gibt keinen Lifecycle-basierten Legacy-Fallback für fehlende Auswahl- oder Setupwerte.

Nur `AssignCharacterProfile` darf `character_class` und `combat_profile` gemeinsam setzen. Ein während des Prozesses neu gefundener Charakter erhält dabei die sicheren Difficulty- und Queue-Defaults und wird im selben atomaren Write angelegt. Ein identischer Aufruf bleibt revisionsneutral.

Ein Update validiert zuerst den Gesamtvertrag, schreibt eine Temp-Datei im gleichen Verzeichnis über den vorhandenen atomaren Windows-Replace, flusht und schließt sie und liest den neuen Stand erneut. Erst der identische Re-Read wird effektiv. Bei Write- oder Re-Read-Fehler bleibt der bisherige effektive Stand erhalten; nach einem fehlgeschlagenen Re-Read wird die alte Datei best-effort atomar zurückgeschrieben.

### Vorschau, Update und Reset

Die API stellt Read, Änderungsvorschau, Resetvorschau, Update und Reset bereit. Vorschauen sind seiteneffektfrei. Update und Reset benötigen zusätzlich zum Control-Token:

- `expected_revision` des Store-Snapshots;
- `expected_generation` des Supervisors;
- einen inaktiven Corezustand ohne aktiven Routen-Workflow.

Eine veraltete Revision liefert `config_revision_conflict`; eine veraltete Generation liefert den bestehenden `state_changed`-Vertrag. Während einer Session wird mit `command_conflict` abgelehnt. Änderungen an Input-Freigabe oder Hotkeys liefern `restart_required: true` und `config_restart_required`. Der bereits aufgebaute Input-Controller wird niemals halb live verändert. Beim nächsten kontrollierten Core-Start werden die persistenten Settings vor Konstruktion von Runtime, Hotkeys und Input auf die Core-Konfiguration angewendet.

Die allgemeine Settings-Preview-/Update-API projiziert Klasse und Profil nur lesbar und verlangt beide Werte einschließlich der Charakterschlüssel unverändert zum aktuellen Stand. React führt sie beim Draft-Klonen und DTO-Round-trip mit, bietet dafür aber kein Settings-Feld an. PreviewReset und Reset übernehmen die Setup-Paare bytegleich aus dem aktuellen Stand und setzen nur die bisherigen allgemeinen Werte zurück.

Nur der dedizierte Charakter-Setup-Command darf `character_class` und `combat_profile` gemeinsam setzen. Der `CharacterCatalogStore` projiziert dieses Paar gegen einen frisch gelesenen Saveheader und die aktuelle Profilfreigabe; ein leeres, klassenfremdes oder nicht mehr freigegebenes Paar macht den Charakter nicht auswählbar. Ein bereits bestätigter Auswahlkontext wird beim Core-Start verworfen, falls diese Prüfung nicht mehr besteht.

Nach einer erfolgreichen allgemeinen Mutation übernimmt der Core Queue und Budgets bei inaktiver Session gemeinsam in die nächste Runtime-Konfiguration und die Statusprojektion. Danach veröffentlicht er `operator_settings_changed`; der Renderer abonniert dieses SSE-Ereignis in `connectLiveEvents` und lädt den autoritativen Status neu. Zusätzlich ruft die Settings-Seite nach erfolgreichem Speichern oder Reset `onSettingsApplied` auf, damit das Dashboard die Queue ohne App-Neustart übernimmt und keine vor dem Speichern gecachte Queue an den Startpfad senden kann. Input- und Hotkeyänderungen bleiben hiervon ausgenommen und werden weiterhin erst durch den ausdrücklich angezeigten kontrollierten Core-Neustart wirksam.

## Datenmodell

- `OperatorSettings`: Schema, Revision, letzter Charakter, Setup-Paare, Charakterwerte, Budgets, Input und History.
- `OperatorSettingsChange`: validierter Ergebnisstand, geänderte Bereiche und Neustartpflicht.
- `OperatorSettingsMutationRequest`: erwartete Revision, Supervisorgeneration und vollständiger Ersatzvertrag.
- `OperatorSettingsResetRequest`: erwartete Revision und Supervisorgeneration.

## Operator / CLI

Die Datei ist Core-eigene Persistenz und nicht für paralleles manuelles Editieren während des Betriebs gedacht. Konflikte werden nicht gemergt. Der Repositorybetrieb ohne expliziten Datenroot bleibt weiterhin allein durch `config.yaml` bestimmt.

Seit Abschnitt 15.6 bildet die Settings-Seite Read, Preview, Update, Resetvorschau und Reset direkt ab. Die Bedienoberfläche trennt die Speicherziele in die Tabs **Farming** (revisioniertes Core-Dokument), **App** (Desktop-Store) und **Wartung** (Sofortaktionen). Farming speichert über eine sticky Action-Bar: `Speichern` führt intern Preview und Bestätigungsdialog aus; `Verwerfen` setzt nur den lokalen Draft zurück. Ungespeicherte Farming-Änderungen blockieren die Hash-Navigation und `beforeunload`. Autostart im App-Tab speichert sofort beim Umschalten. Die Run-Reihenfolge ist ein Zwei-Spalten-Editor mit Katalogbereitschaft, ↑↓ sowie Drag & Drop zum Umsortieren und zum Ziehen von Katalog-Runs in die aktive Queue. Die Seite hält Revisionkonflikte bis zum expliziten Neuladen sichtbar, sperrt Mutationen während aktiver Sessions und bietet bei `restart_required` ausschließlich den kontrollierten Electron-Core-Neustart an. Effektive Werte und Speicherort werden unter Wartung nur lesbar projiziert.

## Abhängigkeiten

- `internal/tasks.DefaultRunRegistry` für erlaubte Queueeinträge.
- `internal/input.ValidateKeyStrings` für dieselbe Hotkeyvalidierung wie produktive Controller.
- Bestehender atomarer YAML-Writer und Windows-`MoveFileEx`-Replace.
- Bestehender API-Control-Token und Supervisorgeneration.

## Verwandte Features

- [Installierter Datenroot und Desktop-Einstellungen](installed-data-root.md)
- [FarmQueue-Scheduler](farm-queue-scheduler.md)
- [Lokale Core-API](local-core-api.md)
- [Phase-15-Core-Vertrag](phase-15-core-contract.md)
- [Desktop-Betrieb und Einstellungen](desktop-operation.md)
- [Charaktereinrichtung](character-setup.md)

---
*Zuletzt aktualisiert: 30. Juli 2026*
