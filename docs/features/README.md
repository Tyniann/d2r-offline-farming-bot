# Feature-Dokumentation

Übersicht dokumentierter Bot-Features. Architektur-Gesamtbild: [`handoff.html`](../plans/handoff.html).
Spätere Ideen (noch nicht umgesetzt): [`docs/backlog.md`](../backlog.md).

## Fundament: Prozess, Memory, World, Input

| Feature | Beschreibung |
|---------|--------------|
| [Process Detection](process-detection.md) | Read-only D2R-Prozessbindung, Modulbasis, Lifecycle, Attach-Timeout |
| [Memory Reader](memory-reader.md) | Low-Level Byte-Reads, Primitive-Dekodierung, Pointer-Ketten |
| [State Probe](state-probe.md) | Memory-Reads, World-Update im App-Loop; `--probe` für semantisches World-State-Logging |
| [World Model](world-model.md) | Domain-Typen (Area, Player, State), eingebetteter Area-Katalog; kontinuierliches Update im App-Loop (2.3); Validierung Phase 2.4 |
| [Input Controller](input-controller.md) | D2R-Fensterbindung per PID, Client-Geometrie (3.1); Tastatur-/Maus-Primitives; YAML-Bindings für Skills, Portal und Belt; Safety-Opt-in, globale Pause/Stop-Hotkeys; manueller CLI-Input-Testmodus |
| [Read-only Game Identity](game-identity.md) | Phase 6.1: bestätigte Character Identity, kontrollierte Offline-Difficulty-Auswahl und autoritativer Layout-Fingerprint ohne persistenten Auswahl-Cache |
| [Layout-Fingerprint](layout-fingerprint.md) | Deterministischer Hash stabiler World-Anker als fail-closed Kartenprüfung vor Route Playback |
| [Read-only UI-State-Probe](ui-state-probe.md) | Phase 7.1: benannte UI-Buffer-Captures mit stabilen/volatilen Bytes, Fingerprint und lokalem JSON-Artefakt ohne Menüinput |
| [Objekt-Inspect](object-inspect.md) | Gate 23.0: read-only `--object-inspect` für Area-Objekte mit Mode und Schlüssel-Stats (Active/Base, Stat 70), ohne Produkt-Truhen-IDs |

## Runs, Tasks und Combat-Profile

| Feature | Beschreibung |
|---------|--------------|
| [Run Registry und gemeinsames Run-Schema](run-registry.md) | Typisierte Run-Definitionen (Countess, Mephisto, Summoner, Nihlathak, Cows, Lower Kurast), gemeinsames Config-Schema und fail-closed Definition Resolver |
| [Run-Verfügbarkeit und Inspect](run-availability.md) | Deterministischer read-only Availability-Resolver, Reason-Codes und `--runs-inspect`-JSON |
| [Task Runner](task-runner.md) | Gemeinsame Run-Pipeline, Lazy Run-Start und Registry-Auflösung; `--run <id>` / `runs.active` |
| [Countess-Run](countess-run.md) | Phase 5.6: vollständiger Countess-Run mit Travel, Kill, Loot-Pickup, Safety-Potion und Town-Portal-Abschluss; isolierte Testphasen bleiben verfügbar |
| [Mephisto-Run](mephisto-run.md) | Phase 10.10: gemeinsame Run-Pipeline mit Durance-Route, zwei gepinnten Boss-Aktionen, run-spezifischem Loot und Act-3-Normalisierung |
| [Summoner-Run](summoner-run.md) | Key of Hate: Arcane Sanctuary, leere Engage-Sequenz, Post-Boss-Cleanup, Act-2-Egress, Pickit `[gems, keys]`; Live-Gate bestanden |
| [Nihlathak-Run](nihlathak-run.md) | Key of Destruction: Halls of Pain → Vaught, Halls-Warps 76–78, Act-5-Egress, Pickit `[gems, keys]`; Live-Gate bestanden |
| [Character- und Encounter-Profile](character-encounter-profiles.md) | Klassenbegrenzte Lifecycle-Hooks, Resource Policy und entwicklerverwaltete Phase-16-Setup-Freigabe samt Default |
| [Character Loadouts](character-loadouts.md) | Phase 21: Skillkatalog, Strategy Registry, SkillsKnown, Schema-3-Bindings, Inventarschutz, Setup-Wizard |
| [Paladin „Hammerdin“](hammerdin.md) | CTA-/Holy-Shield-Prebuff vor der Route, gemeinsamer Blessed-Hammer-LMB-Hold für Countess, Mephisto, Nihlathak, Summoner-Route und Cow-Sweep, verpflichtender Mercenary-Preflight |
| [Mercenary Support](mercenary-support.md) | Phase 18: fail-closed Merc-State, Combat-Heal, Akara-Heal, Kashya-Revive und `waypoint-kashya` |
| [Route-Threat-Combat](route-threat-combat.md) | Gemeinsames Summoner-/Cow-Route-Hold mit stationärem Profil-Clear, Coverage, Ressourcen, Recovery-Guard und Cow-CE-Strategie |
| [Cow Level / Moo Moo Farm](cow-level-run.md) | Phase 20.0–20.6: CASC-/Leichen-Grundlagen, zwei Routenrollen, Setup, Cow-Portal-Rezept, Cow-Hold-Sweep und gemeinsamer Town-Handoff |
| [Lower-Kurast-Run](lower-kurast-run.md) | Phase 23: Supertruhen am Lagerfeuer ohne Boss, Akara-Schlüssel, Hüttengestelle, Pickit `[gems, lk-superchests]`; Live-Gate bestanden |

## Pathing, Routen und Town

| Feature | Beschreibung |
|---------|--------------|
| [Pathing](pathing.md) | Teleport-Navigation (Phase 4.3): Relative-Projektion + Hover-Feedback-Loop, Bearing-Explore, Stuck-Detection; `--pathing-test` |
| [Route Recording und Playback](route-recording-playback.md) | Phase 6.7: generisches Playback und live validierter Countess-Adapter über stabile Route-ID |
| [Farming-RouteCatalog und Lifecycle](route-lifecycle.md) | Abschnitt 11.5: rekursiver Farming-Katalog, atomisches Manifest, Bootstrap und präzise Difficulty-/Layout-Invalidation |
| [Farming-Route-Assignment](route-assignment.md) | Schema 2: atomische Einzelzuordnung plus feste Cow-Rollenslots, Migration, Revisionen und orthogonaler Managementstatus |
| [Geführte Farming-Routenaufnahme](guided-route-recording.md) | Exklusiver Recorder-Core mit Wegpunkt-/Portalstart, Boss-/Objekt-/Endpunktprüfung, immutable Kandidaten und TP-Sicherheitsrückweg |
| [Kandidaten-Playback und Routenverwaltung](route-management.md) | Abschnitt 12.4: isolierter Navigationstest sowie revisions- und Recovery-gesicherte Publish/Replace/Archive/Restore/Delete-Transaktionen |
| [Routenoberfläche](route-dashboard.md) | Aufgabenorientierte Bibliothek, Einzelaufnahme mit Beutezweck und Pickit-Verweis, Kuhlevel-Zweischritt, Unteres-Kurast-Fotos und sichere Entwurfsverwaltung |
| [Town Services](town-services.md) | Phase 9: fail-closed Bedarfsermittlung, zentraler Act-1-Hub und minimales Fremdakt-Egress-Format |
| [Globaler System-Egress](system-egress.md) | Abschnitt 12.2: aktgenerische globale Portal-zum-Wegpunkt-Routen für Akt 2–5 ohne Character-/Difficulty-Bindung |

## Loot, Inventar und Pickit

| Feature | Beschreibung |
|---------|--------------|
| [Item Enumeration Read-Only](item-enumeration.md) | Phase 5.1: positionierte Ground-Drops read-only aus Memory ins World Model und Probe-Log |
| [Inventory Model und Lock Grid](inventory-lock-grid.md) | Phase 5.2: persönliche Inventar-Items, 4x10 Lock-Grid und fail-closed Pickup-Kapazität |
| [Pickit Engine](pickit-engine.md) | Phase 5.3: kleiner NIP-Subset gegen `world.Item`, Default-Countess-Regeln und read-only Match-Ergebnisse |
| [Loot Decision Pipeline](loot-decision-pipeline.md) | Phase 5.4 und Abschnitt 13.7: Action-gesteuerte Pickup-/Keep-/Stash-/Sell-Entscheidungen mit fail-closed Recheck |
| [Hover-Confirmed Item Pickup](hover-confirmed-item-pickup.md) | Phase 5.5: Hover-bestätigter Ground-Item-Pickup mit Retry-, Distanz-, Verify- und Monster-Abbruchregeln |
| [Loot- und Recovery-Loop](loot-recovery-loop.md) | Phase 5.6: Ground-Loot, Pickit, Inventory-Lock, hover-bestätigter Pickup und Countess-Loot-Integration; spätere Recovery-Slices bleiben geplant |
| [Inventory-Full-Recovery](inventory-full-recovery.md) | Phase 5.7: explizites `inventory_full`, hover-bestätigter Town-Portal-Eintritt und verifizierte Rückkehr ins Rogue Encampment |
| [Personal-Stash MVP](personal-stash-mvp.md) | Phase 5.8: Memory-bestätigte Town-Navigation, geschützte Ctrl+LMB-Transfers und sauberer UI-Abschluss |
| [Identification-Strategie](identification-strategy.md) | Phase 5.9: Statregeln nur für identifizierte Items und `identify_required` vor Keep/Stash |
| [Pickit-Profile und Assignments](pickit-profiles.md) | Atomare revisionierte YAML-Profile, geordnete Charakter-/Run-Zuordnung und idempotente Ergänzung vollständig fehlender Setup-Defaults |
| [Pickit-API und sichere Run-Grenze](pickit-api.md) | Abschnitt 13.5: vollständiger HTTP-Vertrag, sichere revisionierte Mutationen und unveränderlicher Policy-Snapshot pro Run |
| [Pickit-Profilbibliothek und Editor](pickit-editor.md) | Abschnitte 13.6–13.7: geführte Katalogregeln, Profil-CRUD, Assignments und Core-basierte Testitem-Vorschau mit vollständigem Trace |
| [Sockel-Support für Pickit](socket-pickit.md) | Phase 19 abgeschlossen (Gates 19.0–19.5): fail-closed `[sockets]`/`socketed`, kombinierter Builder; Freigabe `v0.17.0` |

## Session, Queue und Game-Lifecycle

| Feature | Beschreibung |
|---------|--------------|
| [Session-Lifecycle](session-lifecycle.md) | Zentraler Phase-11-Game-Lifecycle, frischer Run-Zustand pro Queue-Eintrag, Same-game-Folge, Pause/Stop und verifizierte Spielgrenzen |
| [FarmQueue-Scheduler](farm-queue-scheduler.md) | Abschnitt 11.7/15.13: vollständiger Queue-Preflight, persistente Charakter-Queue, zyklischer Scheduler, Retry-same-index und Core-autoritative Safety-Budgets |
| [Session-Konfiguration und Inspect](session-configuration.md) | Phase 7.5: explizites Opt-in, endliche Budgets und read-only Planauflösung mit Route-/Character-/Difficulty-Preflight vor Attach/Input |
| [Session-Recovery und Lifecycle-Telemetrie](session-recovery-telemetry.md) | Phase 7.7: exakte Retry-Klassifikation, harte Fehler-/Restart-Budgets und synchron korrelierte Session-/Game-/Run-JSONL-Ereignisse |
| [Charaktereinrichtung](character-setup.md) | Phase 16: Core-validierte Profil- und Pickit-Einrichtung, sichere Auswahlbilderfassung und erneute Selection-/Queue-/Run-Gates |
| [Read-only Charakterkatalog und Screenshot-gated Selector](character-selection.md) | Abschnitt 11.4: Save-Dateinamen-Katalog, fail-closed Anker-Availability und begrenzte Home/Down-Auswahl mit Memory-Bestätigung |
| [Verifizierter Offline-Game-Start](offline-difficulty-selection.md) | Phase 7.3: Screen- und Memory-gated Charakter-, Play- und Difficulty-Auswahl mit bestätigter Ankunft in Rogue Encampment |
| [Offline-Spieleranzahl](offline-players.md) | Pro Charakter `/players 1–8` nach bestätigtem Spielstart, Default 1, Chat nur nach zertifiziertem In-Game |
| [Verifiziertes Offline Save & Exit](offline-game-exit.md) | Phase 7.2: isolierter Memory-gated Exit mit einmaligem Esc/Klick, 1280×720-Gate und bestätigter Menü-Ankunft |

## Telemetrie und Historie

| Feature | Beschreibung |
|---------|--------------|
| [Runtime-Trace und Headless Replay](runtime-trace-replay.md) | Phase 22: opt-in Runtime-Entscheidungstraces, zeitliche Fehlerklasse, Simulationsgrenze und Regression-Promotion ohne D2R |
| [Run-Telemetrie](run-telemetry.md) | Phase 5.10: fail-closed JSONL pro Run für Drop-, Pickit-, Pickup-, Inventory- und Stash-Events |
| [History-Reader und In-Memory-Index](history-reader-index.md) | Strikter Schema-4-Reader mit kompaktem Run-Kontext, Cross-Stream-Korrelation, Dateidiagnosen und vollständig rebuildbarem flüchtigem Index |
| [Historienanalyse und Boss-/Routenvergleich](history-analysis.md) | Abschnitte 14.4/15.8: kanonische Filter, IANA-lokale Tages-Buckets, DST, terminale Kennzahlen, Stages, Funnel und Vergleich |
| [Historien-API und Export](history-api-export.md) | Abschnitt 14.5: read-only DTOs, kanonische Filter, stabile Cursor-Pagination, JSON-/CSV-Parität und begrenztes Änderungssignal |
| [Run-Historie im Dashboard](run-history.md) | Abschnitte 14.6/15.8: Filter, Kennzahlen, vier tabellengestützte Charts, Core-sortierte Vergleiche, Drill-down und Exporte |
| [Historien-Retention und Komplettlöschung](history-maintenance.md) | Abschnitt 15.9: vollständige alte Session-Bundles, tägliches Idle-Gate und metadatengebundene Komplettlöschung mit Active-Set-Schutz |

## Desktop-App und Operator-UI

| Feature | Beschreibung |
|---------|--------------|
| [Internationalisierung Deutsch und Englisch](internationalization.md) | Dynamischer DE-/EN-Wechsel für React, Electron, Recovery und Installer mit sprachneutralem Core-Vertrag und CASC-generierten D2R-Namen |
| [Lokale Core-API und eingebettete Web-Anwendung](local-core-api.md) | Abschnitt 11.2: Loopback-only HTTP/JSON, Security Envelope, OpenAPI-generierter TypeScript-Client und eingebetteter React-Build |
| [Live-Dashboard](live-dashboard.md) | App-weite Auswahl, read-only Farm-Queue, Historienkennzahlen und Core-projizierter Run-Fortschritt mit globalen Safety-Hotkeys |
| [Installierter Datenroot und Desktop-Einstellungen](installed-data-root.md) | Abschnitt 15.1: expliziter Core-Root, hashgebundene Defaults, stagingbasierter Import und getrennter atomarer Desktop-Settings-Store |
| [Persistente Operator-Einstellungen](operator-settings.md) | Schema 2 mit geschütztem Charakterprofil-Paar, Queues, Budgets, Input, Hotkeys, Retention, Preview/Reset und zehn Backups |
| [Sichere Electron-Shell und Core-Kindprozess](desktop-shell.md) | Abschnitt 15.3: gehärtetes Einzelfenster, private Handshake-Pipe, Datenroot-Lock, minimale IPC-Bridge und fail-closed Crash-Recovery |
| [Tatsächliches D2R-Versionsgate](d2r-version-gate.md) | Abschnitt 15.4: PID-/pfadgebundene Windows-Dateiversion, exakte Compatibility-Matrix und Input-/Workflow-Sperre vor `compatible` |
| [Desktop-App-Shell und Designsystem](desktop-app-shell.md) | Abschnitt 15.5: eine responsive Shell mit fünf stabilen Hash-Zielen, gemeinsamer Zustandsbasis und unveränderten Core-getriebenen Featureflüssen |
| [Desktop-Betrieb und Einstellungen](desktop-operation.md) | Abschnitt 15.6: Core-revisionierte Settings, achtstufige native Zustandsmatrix, sichtbare Fensterbounds, Opt-in-Autostart, Tray, begrenzte Notifications und fail-closed Quit-Ordering |
| [First Run, Provisionierung und erste Route](first-run-onboarding.md) | Abschnitt 15.7: Pre-Core-Provisionierung derselben React-App, einmaliger Go-Import, neun Core-getriebene Schritte und Übergabe an denselben RecordingCoordinator |
| [Lokales Diagnosepaket und Versionshinweis](diagnostics-and-update-check.md) | Abschnitt 15.10: Core-Allowlist und Redaktion, getrennte sensitive Opt-ins sowie einmaliger stabiler GitHub-SemVer-Hinweis ohne Download |
| [Windows-Installer und lokale Releasepipeline](windows-packaging.md) | Abschnitt 15.11: per-user NSIS x64, eine Releaseversion, minimale Ressourcen, Inhaltsaudit, temporärer Install-/Upgrade-/Uninstall-Smoke und SHA-256 |

## Core-Verträge (Phasen-Baseline)

| Feature | Beschreibung |
|---------|--------------|
| [Phase-11-Core-Vertrag](phase-11-core-contract.md) | Supervisor-Zustände, Commands, Queue-/Lifecycle-Semantik, DTO-Grundformen, Reason-Codes und finale Ownership-Matrix |
| [Phase-12-Core-Vertrag](phase-12-core-contract.md) | Baseline, Assignment-/Kandidaten-Schemas, Recording-/System-Egress-Verträge, Workflow, Locks, Crash-Matrix und Ownership |
| [Phase-13-Core-Vertrag](phase-13-core-contract.md) | Abschnitt 13.0: Pickit-Baseline, Profil-/Assignment-Schemas, Aktionen, Revisionen, Reason-Codes, Paketgrenzen und fallback-freie Migrationsmatrix |
| [Phase-14-Core-Vertrag](phase-14-core-contract.md) | Abschnitt 14.0: Historienpopulation, Metriken, Denominatoren, Funnel, Stages, Filter, Pagination, Export und Reason-Codes |
| [Phase-15-Core-Vertrag](phase-15-core-contract.md) | Abschnitt 15.0: Desktop-/Core-Ownership, Datenroot, Lifecycle, Version-Gate, Operatorwerte, Retention, Reason-Codes und 10.000-Run-Performancebaseline |
| [Phase-16-Core-Vertrag](phase-16-core-contract.md) | Abschnitt 16.0: D2S-v105-Präfix, Klassenmapping einschließlich Warlock, Charakter-Setup-Ownership, Reason-Codes, Defaults und Nicht-Ziele |
| [Phase-17-Core-Vertrag](phase-17-core-contract.md) | Abschnitt 17.0: Summoner-Baseline, RouteProgress, Threat-/Coverage-Zustände, Ressourcenkontext, Watchdogs, Reasons, Ownership und Nicht-Ziele |
| [Phase-18-Core-Vertrag](phase-18-core-contract.md) | Abschnitt 18.0: Hireling-IDs aus lokaler `hireling.txt`, read-only Merc-Diagnose, Evidenzregeln und manuelles Gate vor jedem Merc-Input |
| [Phase-21-Core-Vertrag](phase-21-core-contract.md) | Abschnitte 21.0–21.6: Skillkatalog, Strategy Registry, SkillsKnown, Schema-3-Loadouts, Wizard und Config-Abschluss |
