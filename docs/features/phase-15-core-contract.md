# Phase-15-Core-Vertrag

## Überblick

Abschnitt 15.0 friert die Phase-11–14-Baseline ein und legt die Grenzen der installierbaren Windows-App fest. Electron wird ausschließlich Desktop-Owner; der bestehende Go-Core bleibt alleinige Fach-, Daten-, Config-, Statistik- und Safety-Autorität. React bleibt dieselbe Anwendung und projiziert Core-Daten beziehungsweise Benutzerintents.

Dieser Abschnitt implementiert noch keinen installierten Datenroot, keinen OperatorSettingsStore, keinen Electron-Prozess und kein D2R-Versionsgate. Er schafft compile-nahe Verträge und eine reproduzierbare 10.000-Run-Performancefixture, gegen die die folgenden Abschnitte umgesetzt werden.

## Ort im Code

- **Compile-naher Vertrag:** `internal/app/phase15_contract.go`
- **Vertragstests:** `internal/app/phase15_contract_test.go`
- **Performancefixture:** `internal/telemetry/phase15_performance_test.go`
- **Bestehender History-Core:** `internal/telemetry/history_index.go`, `internal/telemetry/history_analyzer.go`
- **React-/Desktop-Workspace:** `web/`
- **Maschinenvertrag:** `internal/api/schema/openapi.json`

## Ownership

| Verantwortung | Einziger Owner | Harte Grenze |
|---|---|---|
| D2R, Memory, World, Tasks, Input und Workflows | Go-Core | Bestehender Supervisor, Availability, Preview/Confirm und Control-Token |
| Operatorwerte und Historienmutation | Go-Core | Versionierte Stores, erwartete Revision, aktive Generation und Idle-Lock |
| Fenster, Tray, Autostart und Benachrichtigungen | Electron Main | Schmale validierte Desktop-IPC ohne Gameplaysemantik |
| App-/Core-Prozessbeziehung | Electron Main | Genau ein Kindprozess, begrenzter Handshake, Shutdown- und Crashpolicy |
| Darstellung und Benutzerintents | React | Generierter API-Client plus enge Desktop-Bridge; keine Node-Primitiven |
| Historienzahlen und spätere Tages-Buckets | `internal/telemetry.AnalyzeHistory` | API, JSON, Charts und Tabellen verwenden denselben gefilterten Snapshot |
| Diagnoseinhalt und Redaktion | Go-Core | Allowlist; Electron wählt nur Ziel beziehungsweise öffnet den erzeugten Pfad |
| Einmalige Releaseabfrage | Electron Main | Fest erlaubter HTTPS-Endpunkt, Timeout und keine Credentials |

`Phase15ContractOwners` hält diese Tabelle compile-nah. Tests schützen Anzahl, Reihenfolge und äußere Paketgrenzen vor einer versehentlichen zweiten Autorität.

## Datenroot und Persistenzgrenzen

Der installierte kanonische Root lautet `%LOCALAPPDATA%\D2ROfflineFarmingBot\`. Sein minimales direktes Layout ist:

```text
configs\                 Go-Core-Konfiguration, Routen und Pickit
logs\telemetry\          JSONL-Autorität
backups\                 höchstens zehn Config-/Migrationsbackups
diagnostics\             lokal erzeugte Diagnosepakete
desktop-settings.json    ausschließlich Fenster, Autostart und Onboarding
```

`desktop-settings.json` darf niemals Queue, Difficulty, Budgets, Input-Freigabe, Hotkeys, Retention oder andere Fachwerte enthalten. Diese Werte gehören in den ab 15.2 Core-eigenen OperatorSettingsStore. Installationsdateien sind read-only; Repository-/CLI-Entwicklung behält ihre relativen Workspace-Pfade.

Import und Migration folgen ab 15.1 immer `Quelle lesen → geschlossenen Stagingstand anlegen → alle produktiven Loader anwenden → atomar veröffentlichen`. Es gibt keinen Merge, keinen Reparse-/Traversal-Fallback und keinen teilweise sichtbaren Zielroot.

## Desktop- und Core-Lifecycle

Electron erwirbt zuerst den Single-Instance-Lock, löst danach den Datenroot und startet anschließend genau einen Core. Ein zufälliger Loopback-Port und der Control-Token werden einmal über einen privaten Parent-/Child-Kanal übertragen; Token gehören weder in Argumente, Environment, Datei noch Log.

Die Supervisorzustände `starting_game`, `starting_run`, `running_run`, `paused_between_runs`, `exiting_game` und `cancelling` gelten für X als aktiv und führen ausschließlich ins Tray. Der compile-nahe Vertrag `Phase15DesktopActiveStates` hält diese Menge fest. Ein vollständiges Beenden während Aktivität verlangt zuerst den bestehenden Stop-/Emergency-Vertrag; Electron killt den Core niemals blind.

Ein Core darf nur dann einmal begrenzt automatisch neu starten, wenn der letzte autoritative Zustand sicher inaktiv war und kein mutierender Workflow lief. Ein aktiver oder unbekannter Exit führt zu `core_recovery_required`.

## D2R-Version und Input-Gate

Der Go-Core liest seit 15.4 den kanonischen Image-Pfad und die Windows-Dateiversion der gebundenen `D2R.exe` über ein mockbares read-only Interface. Unterstützte Core-Version, konfigurierte Erwartung, Offsetversion und tatsächlich erkannte Dateiversion müssen exakt übereinstimmen.

Vor `compatible` entstehen kein Input-Controller-Opt-in, keine Gameplay-Hotkeys, kein RecordingCoordinator und kein Supervisor-Worker. `not_detected`, `incompatible` und `unreadable` bleiben read-only Diagnosezustände ohne Override. Absolute Installationspfade verlassen den Core nicht.

## OperatorSettings-Vertrag

Das geplante Schema 1 besitzt eine positive Revision und speichert:

- pro Charakter genau eine geordnete, nicht leere und duplikatfreie Queue sowie die letzte Difficulty;
- globale endliche Sessionbudgets;
- explizite Input-Freigabe und paarweise verschiedene Hotkeys;
- `history.retention_enabled=true` und `history.retention_days=60` als sichere Defaults.

Jede Mutation benötigt Control-Token, erwartete Revision, aktuelle Supervisorgeneration und einen inaktiven Core. Schreiben bedeutet vollständige Validierung, Temp-Datei im Zielverzeichnis, Flush, Close, atomischer Replace und Re-Read. Input-/Hotkeyänderungen werden nicht halb live angewendet, sondern liefern `config_restart_required`.

## Historie und Retention

JSONL bleibt einzige persistente Autorität. SQLite, ein persistenter Cache und eine zweite Indexform sind ausgeschlossen. Automatische Retention darf ab 15.9 nur vollständige, terminale Schema-3-Session-Bundles samt sämtlicher referenzierter Run-Dateien löschen. Default sind 60 Tage; aktive, unvollständige, beschädigte, Legacy- oder unklare Daten werden automatisch niemals entfernt.

Eine getrennte Komplettlöschung arbeitet ausschließlich über Core-Vorschau, Confirmation-Token, Indexgeneration, Metadaten-Recheck und unmittelbar neu bestimmtes Active-Set. Sie erhält aktive direkte JSONL-Dateien als einzige Ausnahme und folgt keinen Unterverzeichnissen oder Reparse Points.

## Reason-Codes

`Phase15ReasonGroups` reserviert die folgenden eindeutigen Codefamilien. Bestehende semantisch identische Codes bleiben unverändert; Renderer und Electron erfinden keine Synonyme.

- **Desktop/Core:** `desktop_instance_running`, `core_start_failed`, `core_handshake_timeout`, `core_handshake_invalid`, `core_exited`, `core_recovery_required`, `core_shutdown_failed`
- **Daten/Migration:** `data_root_unavailable`, `data_root_locked`, `data_import_invalid`, `data_import_conflict`, `data_import_failed`, `config_schema_unsupported`, `config_revision_conflict`, `config_restart_required`
- **Kompatibilität:** `d2r_version_not_detected`, `d2r_version_unreadable`, `d2r_version_unsupported`, `offset_version_mismatch`, `privilege_mismatch`
- **First Run:** `onboarding_input_disabled`, `onboarding_route_prerequisite_missing`, `onboarding_route_interrupted`
- **Historie:** `history_timezone_invalid`, `history_retention_blocked`, `history_retention_partial`, `history_delete_preview_stale`, `history_delete_active_protected`, `history_delete_failed`
- **Desktopdienste:** `update_check_unavailable`, `update_response_invalid`, `external_link_rejected`, `diagnostic_bundle_failed`, `diagnostic_content_rejected`

## Messbare Baseline

Die Fixture erzeugt exakt 10.000 terminale produktive Runs über 100 UTC-Tage mit 100 Runs pro Tag. Jeder fünfte Run schlägt deterministisch fehl. Daraus folgen für das halboffene 30-Tage-Intervall exakt 3.000 terminale Runs, 2.400 Erfolge und 2.400 Keep-Returns; für 60 Tage exakt 6.000, 4.800 und 4.800.

Reproduktionsbefehl:

```powershell
go test ./internal/telemetry -run '^$' -bench '^BenchmarkPhase15HistoryTenThousandRuns$' -benchtime=1x -benchmem -count=1
```

Baseline vom 22. Juli 2026 auf Windows/amd64, Go 1.26.4 und Intel i7-8700K:

| Messpunkt | Zeit | Heap | Allokationen |
|---|---:|---:|---:|
| Kalter Start inklusive erstem Reader-Scan von 10.001 Dateien | 57,708 s | 265.436.344 B | 1.082.593 |
| Vollständiger warmer `HistoryIndex.Rebuild` | 1,604 s | 265.383.864 B | 1.082.445 |
| 30-Tage-Analyse mit 3.000 Runs | 2,727 ms | 8.058.136 B | 10.289 |
| 60-Tage-Analyse mit 6.000 Runs | 6,011 ms | 18.867.480 B | 20.496 |

Der kalte Windows-Dateiscan ist die dominante Kostenstelle; Analyse und warmer Rebuild bleiben klar begrenzt. Phase 15 führt deshalb kein SQLite-Scaffold ein. Zuerst gelten die bestehende inkrementelle Fingerprint-Wiederverwendung und die 60-Tage-Produktgrenze. Eine spätere Neubewertung braucht reproduzierbares Verfehlen konkreter Produktziele unter realem Retentionsbestand.

## Gepinnte Abhängigkeiten

Der bestehende React-/Vite-Stack bleibt exakt gepinnt. Phase 15 ergänzt ausschließlich:

- `electron@39.8.10` – neueste geprüfte Linie, deren Entwicklungs-Engine mit dem vorhandenen Node 20 kompatibel ist;
- `electron-builder@26.15.3`;
- `@playwright/test@1.61.1`;
- `lucide-react@1.25.0`;
- `recharts@3.10.0`.

`web/pnpm-workspace.yaml` erlaubt ausschließlich das erforderliche Electron-Installskript und lehnt das für den NSIS-Pfad nicht benötigte `electron-winstaller`-Buildskript ab. Das Lockfile bleibt die vollständige reproduzierbare Dependency-Autorität.

## Verbotene Pfade

`Phase15NonGoals` friert insbesondere Node-Integration, Renderer-Dateisystem/Prozess/D2R-Zugriff, Electron-eigene Queue-/Config-/Statistiklogik, zweite UI, Overlay, Browserprodukt, Remote-Content, SQLite, Auto-Update, FFI, neue Gameplay-Pfade, Router-/State-/Komponentenframeworks, fremde Plattformartefakte sowie D2R-/Farming-Autostart ein.

## Verwandte Features

- [Phase-11-Core-Vertrag](phase-11-core-contract.md)
- [Phase-12-Core-Vertrag](phase-12-core-contract.md)
- [Phase-13-Core-Vertrag](phase-13-core-contract.md)
- [Phase-14-Core-Vertrag](phase-14-core-contract.md)
- [Lokale Core-API](local-core-api.md)
- [History-Reader und In-Memory-Index](history-reader-index.md)

---
*Zuletzt aktualisiert: 22. Juli 2026*
