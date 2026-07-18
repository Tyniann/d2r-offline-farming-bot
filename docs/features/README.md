# Feature-Dokumentation

Übersicht dokumentierter Bot-Features. Architektur-Gesamtbild: [`handoff.html`](../../handoff.html).
Spätere Ideen (noch nicht umgesetzt): [`docs/backlog.md`](../backlog.md).

| Feature | Beschreibung |
|---------|--------------|
| [Process Detection](process-detection.md) | Read-only D2R-Prozessbindung, Modulbasis, Lifecycle, Attach-Timeout |
| [Memory Reader](memory-reader.md) | Low-Level Byte-Reads, Primitive-Dekodierung, Pointer-Ketten |
| [State Probe](state-probe.md) | Memory-Reads, World-Update im App-Loop; `--probe` für semantisches World-State-Logging |
| [World Model](world-model.md) | Domain-Typen (Area, Player, State), eingebetteter Area-Katalog; kontinuierliches Update im App-Loop (2.3); Validierung Phase 2.4 |
| [Input Controller](input-controller.md) | D2R-Fensterbindung per PID, Client-Geometrie (3.1); Tastatur-/Maus-Primitives; YAML-Bindings für Skills, Portal und Belt; Safety-Opt-in, globale Pause/Stop-Hotkeys; manueller CLI-Input-Testmodus |
| [Character- und Encounter-Profile](character-encounter-profiles.md) | Phase 8: klassenbegrenzte Lifecycle-Hooks, Once-/Reset-Semantik und profilabhängige HP-/Mana-/Rejuvenation-Policy |
| [Town Services](town-services.md) | Phase 9: fail-closed Bedarfsermittlung, zentraler Act-1-Hub und minimales Fremdakt-Egress-Format |
| [Globaler System-Egress](system-egress.md) | Abschnitt 12.2: aktgenerische globale Portal-zum-Wegpunkt-Routen für Akt 2–5 ohne Character-/Difficulty-Bindung |
| [Run Registry und gemeinsames Run-Schema](run-registry.md) | Phase 10.1: typisierte Countess-/Mephisto-Definitionen, gemeinsames Config-Schema und fail-closed Definition Resolver |
| [Run-Verfügbarkeit und Inspect](run-availability.md) | Deterministischer read-only Availability-Resolver, Reason-Codes und `--runs-inspect`-JSON |
| [Task Runner](task-runner.md) | Gemeinsame Run-Pipeline, Lazy Run-Start und Registry-Auflösung; `--run <id>` / `runs.active` |
| [Pathing](pathing.md) | Teleport-Navigation (Phase 4.3): Relative-Projektion + Hover-Feedback-Loop, Bearing-Explore, Stuck-Detection; `--pathing-test` |
| [Countess-Run](countess-run.md) | Phase 5.6: vollständiger Countess-Run mit Travel, Kill, Loot-Pickup, Safety-Potion und Town-Portal-Abschluss; isolierte Testphasen bleiben verfügbar |
| [Mephisto-Run](mephisto-run.md) | Phase 10.10: gemeinsame Run-Pipeline mit Durance-Route, zwei gepinnten Boss-Aktionen, run-spezifischem Loot und Act-3-Normalisierung |
| [Loot- und Recovery-Loop](loot-recovery-loop.md) | Phase 5.6: Ground-Loot, Pickit, Inventory-Lock, hover-bestätigter Pickup und Countess-Loot-Integration; spätere Recovery-Slices bleiben geplant |
| [Item Enumeration Read-Only](item-enumeration.md) | Phase 5.1: positionierte Ground-Drops read-only aus Memory ins World Model und Probe-Log |
| [Inventory Model und Lock Grid](inventory-lock-grid.md) | Phase 5.2: persönliche Inventar-Items, 4x10 Lock-Grid und fail-closed Pickup-Kapazität |
| [Pickit Engine](pickit-engine.md) | Phase 5.3: kleiner NIP-Subset gegen `world.Item`, Default-Countess-Regeln und read-only Match-Ergebnisse |
| [Loot Decision Pipeline](loot-decision-pipeline.md) | Phase 5.4: read-only Stage-Liste für Pickit-Match, Pickup-Kandidaten, Keep/Stash und Fail-Gründe |
| [Hover-Confirmed Item Pickup](hover-confirmed-item-pickup.md) | Phase 5.5: Hover-bestätigter Ground-Item-Pickup mit Retry-, Distanz-, Verify- und Monster-Abbruchregeln |
| [Inventory-Full-Recovery](inventory-full-recovery.md) | Phase 5.7: explizites `inventory_full`, hover-bestätigter Town-Portal-Eintritt und verifizierte Rückkehr ins Rogue Encampment |
| [Personal-Stash MVP](personal-stash-mvp.md) | Phase 5.8: Memory-bestätigte Town-Navigation, geschützte Ctrl+LMB-Transfers und sauberer UI-Abschluss |
| [Identification-Strategie](identification-strategy.md) | Phase 5.9: Statregeln nur für identifizierte Items und `identify_required` vor Keep/Stash |
| [Run-Telemetrie](run-telemetry.md) | Phase 5.10: fail-closed JSONL pro Run für Drop-, Pickit-, Pickup-, Inventory- und Stash-Events |
| [Route Recording und Playback](route-recording-playback.md) | Phase 6.7: generisches Playback und live validierter Countess-Adapter über stabile Route-ID |
| [Farming-RouteCatalog und Lifecycle](route-lifecycle.md) | Abschnitt 11.5: rekursiver Farming-Katalog, atomisches Manifest, Bootstrap und präzise Difficulty-/Layout-Invalidation |
| [Farming-Route-Assignment](route-assignment.md) | Abschnitt 12.1: atomische Zuordnung pro Character und Run, Legacy-Migration, Revisionen und orthogonaler Managementstatus |
| [Geführte Farming-Routenaufnahme](guided-route-recording.md) | Abschnitt 12.3: exklusiver Recorder-Core, F9-Freeze, immutable Kandidaten, Boss-/Distanzprüfung und TP-Sicherheitsrückweg |
| [Kandidaten-Playback und Routenverwaltung](route-management.md) | Abschnitt 12.4: isolierter Navigationstest sowie revisions- und Recovery-gesicherte Publish/Replace/Archive/Restore/Delete-Transaktionen |
| [Routenbibliothek und Setup-Assistent](route-dashboard.md) | Abschnitt 12.5: pfadfreie Core-API, React-Routenfeature, System-Egress-Setup und Core-geladene Hotkey-Hilfe |
| [Session-Lifecycle](session-lifecycle.md) | Zentraler Phase-11-Game-Lifecycle, frischer Run-Zustand pro Queue-Eintrag, Same-game-Folge, Pause/Stop und verifizierte Spielgrenzen |
| [FarmQueue-Scheduler](farm-queue-scheduler.md) | Abschnitt 11.7: vollständiger Queue-Preflight, zyklischer Scheduler, Retry-same-index und YAML-authoritative Safety-Budgets |
| [Session-Konfiguration und Inspect](session-configuration.md) | Phase 7.5: explizites Opt-in, endliche Budgets und read-only Planauflösung mit Route-/Character-/Difficulty-Preflight vor Attach/Input |
| [Session-Recovery und Lifecycle-Telemetrie](session-recovery-telemetry.md) | Phase 7.7: exakte Retry-Klassifikation, harte Fehler-/Restart-Budgets und synchron korrelierte Session-/Game-/Run-JSONL-Ereignisse |
| [Phase-11-Core-Vertrag](phase-11-core-contract.md) | Supervisor-Zustände, Commands, Queue-/Lifecycle-Semantik, DTO-Grundformen, Reason-Codes und finale Ownership-Matrix |
| [Phase-12-Core-Vertrag](phase-12-core-contract.md) | Baseline, Assignment-/Kandidaten-Schemas, Recording-/System-Egress-Verträge, Workflow, Locks, Crash-Matrix und Ownership |
| [Lokale Core-API und eingebettete Web-Anwendung](local-core-api.md) | Abschnitt 11.2: Loopback-only HTTP/JSON, Security Envelope, OpenAPI-generierter TypeScript-Client und eingebetteter React-Build |
| [Live-Dashboard und Session-Steuerung](live-dashboard.md) | Live-Projektion, Queue Builder, sichere Supervisor-Controls, Reconnect und deutsche Statuskarten |
| [Read-only Charakterkatalog und Screenshot-gated Selector](character-selection.md) | Abschnitt 11.4: Save-Dateinamen-Katalog, fail-closed Anker-Availability und begrenzte Home/Down-Auswahl mit Memory-Bestätigung |
| [Read-only UI-State-Probe](ui-state-probe.md) | Phase 7.1: benannte UI-Buffer-Captures mit stabilen/volatilen Bytes, Fingerprint und lokalem JSON-Artefakt ohne Menüinput |
| [Verifiziertes Offline Save & Exit](offline-game-exit.md) | Phase 7.2: isolierter Memory-gated Exit mit einmaligem Esc/Klick, 1280×720-Gate und bestätigter Menü-Ankunft |
| [Read-only Game Identity](game-identity.md) | Phase 6.1: bestätigte Character Identity, kontrollierte Offline-Difficulty-Auswahl und autoritativer Layout-Fingerprint ohne persistenten Auswahl-Cache |
| [Verifizierter Offline-Game-Start](offline-difficulty-selection.md) | Phase 7.3: Screen- und Memory-gated Charakter-, Play- und Difficulty-Auswahl mit bestätigter Ankunft in Rogue Encampment |
| [Layout-Fingerprint](layout-fingerprint.md) | Deterministischer Hash stabiler World-Anker als fail-closed Kartenprüfung vor Route Playback |
