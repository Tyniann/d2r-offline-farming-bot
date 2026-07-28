# Kandidaten-Playback und Routenverwaltung

## Überblick

Abschnitt 12.4 testet unveröffentlichte Kandidaten isoliert und veröffentlicht sie ausschließlich über revisionsgebundene Preview-/Confirm-Transaktionen. Pro `(Character, Run)` existiert höchstens eine aktive Assignment-Zuordnung; ersetzte Routen bleiben unverändert archiviert.

## Ort im Code

- **Candidate Playback:** `internal/app/candidate_playback.go`
- **Management:** `internal/app/route_management.go`
- **Recovery:** `RouteRecoveryJournal` und `routes.recovery_file`
- **Autoritäten:** `CandidateStore`, `RouteLifecycleStore`, `RouteAssignmentStore`

## Kandidatentest

`CandidateTestOrchestrator` lädt ausschließlich die SHA-256-geprüfte Kandidatendatei. Der produktive Runtime-Driver komponiert die bereits vorhandenen TP-, Town-Graph-, globalen Egress-, Waypoint- und `RoutePlayer`-Komponenten. Sein Driver-Vertrag enthält nur Town-Rückkehr, aktlokale Waypoint-Reise, kandidatengenaues Playback, Terminalevidenz und den zweiten TP-Rückweg. Combat, Loot, Town Services und Save & Exit sind nicht Teil der aufrufbaren Oberfläche. Der Test startet ausschließlich am Memory-bestätigten `portal_arrival`: Im erwarteten Town muss ein Town Portal innerhalb der konfigurierten Interaktionsdistanz sichtbar sein. Diese Entity-Nähe ist auch für die erste Graphkante der autoritative Startbeweis, weil D2R die Figur nicht an einer exakt identischen Koordinate neben dem Portal absetzt; der Walker schließt anschließend den kleinen Abstand zum ersten aufgenommenen Punkt. Ein New-Game-Spawn oder anderer Town-Anker wird vor dem ersten Town-Graph-Input abgewiesen und lässt den Kandidaten für einen korrekten Retry auf `validated`.

Der Weg zum Kandidatenstart bleibt im Rückkehrakt des jeweiligen Runs: Countess verwendet vom Akt-1-`portal_arrival` den bestehenden Town-Graph zum lokalen Wegpunkt; Mephisto verwendet vom Kurast-`portal_arrival` den globalen Akt-3-Egress zum lokalen Wegpunkt und wählt dort direkt Durance Level 2. Eine Zwischenreise nach Rogue Encampment ist kein Bestandteil des Candidate-Tests. Damit nutzt der isolierte Test denselben registrierten Zielwegpunkt wie der Run-Vertrag und vermeidet einen abweichenden Cross-Act-Vorlauf.

Nach Playback werden Terminalgebiet, exakter lebender Boss, Super-Unique-Flag und Maximaldistanz erneut aus Memory-Evidenz geprüft. Nur der vollständige Erfolg inklusive zweitem Rückweg setzt `test_passed` und `tested_at`. Fehler bleiben diagnostisch am Kandidaten gespeichert. Der Runtime-Driver meldet `preparing_playback`, `playing_candidate`, `validating_terminal` und `returning_after_test` mit grobem Fortschritt an die SSE-Projektion.

## Preview und Confirm

Jede Vorschau bindet Candidate-Hash, Catalog-, Lifecycle- und Assignment-Revision, Operation, Character, Run sowie aktuelle und neu generierte Route-ID an ein zufälliges Einmal-Token. Confirm verbraucht das Token; geänderte Revisionen oder Inhalte stoppen fail-closed.

Publish erzeugt die Route-ID kollisionsfest im Core. Replace veröffentlicht zuerst die neue Datei, archiviert den bisherigen aktiven Eintrag und schaltet erst danach das Assignment um. Der einzelne Freigabedialog zeigt den bisherigen aktiven Eintrag ausdrücklich als unverändert zu archivierenden Vorgänger. Archive entfernt gegebenenfalls das Assignment. Restore archiviert den aktuellen Slot vor Aktivierung des kompatiblen Vorgängers. Delete ist nur für unzugewiesene archivierte Routen möglich und verlangt, dass der Benutzer die exakte Route-ID im Dialog selbst eingibt; die UI übernimmt sie nicht automatisch.

## Recovery

`route-recovery.local.yaml` protokolliert dauerhafte Checkpoints. Fehler rollen logische Assignment-/Lifecycle-Zustände zurück und behalten Kandidaten. Delete verwendet ein Quarantäne-Rename. Beim Start werden bekannte Checkpoints deterministisch abgeschlossen oder zurückgerollt; unbekannte Checkpoints blockieren Verwaltung mit `route_transaction_recovery_required`.

## Grenzen

Automatische Difficulty-/Layout-Invalidierung bleibt allein beim Lifecycle Store. Managementstatus ist orthogonal. API und Dashboard greifen in 12.5 ausschließlich über diese Services zu und schreiben keine YAML-Datei selbst.

## Live-Abnahme Phase 12

Der isolierte Test von `candidate-5af9deda83bfdd91` führte ohne Operatorinput über `portal-cain`, die rückwärts abgespielte Kante `akara-cain` und `akara-waypoint` zum Wegpunkt, wählte Black Marsh und spielte exakt die sieben Kandidatensegmente bis zur lebenden Countess. Nach erneuter Terminal- und Distanzprüfung kehrte er über ein UnitID-bestätigtes Town Portal zurück und setzte den Kandidaten auf `test_passed`. Das Inputlog enthält ausschließlich Town-Walk, Waypoint, Route-Pathing und Town-Portal-Aktionen; Combat, Loot, Town Services und Save & Exit fehlen.

Der anschließende einzige Replace-Dialog zeigte die neue Route und `black-marsh-cellar5-nightmare-mrbones` als unverändert zu archivierenden Vorgänger. Nach genau einer Bestätigung weist Assignment-Revision 2 `countess` für `mrbones` der neuen Route `countess-mrbones-fd1756c208` zu. Der Vorgänger bleibt mit unverändertem SHA-256 `8d3dbd…e0d1f` und `management_status: archived` wiederherstellbar.

## Erneute Live-Abnahme Phase 16

Der während Phase 16 neu aufgezeichnete Countess-Kandidat bestätigte eine wichtige Act-1-Grenze: Die Playback-Vorbereitung muss den Run-Ursprung über den gemeinsamen Resolver direkt auf Rogue Encampment abbilden. `TownAreaForAct` ist ausschließlich die Registry für fremde Acts 2–5 und darf für `act1` nicht aufgerufen werden. Nach dieser Korrektur bestand der neue Kandidat den isolierten Test, wurde als `countess-mrbones-b801e63e3c` veröffentlicht und `MrBones/countess` zugewiesen. Die unveränderliche Kandidatendatei musste nicht manuell editiert oder neu aufgenommen werden.

## Verwandte Features

- [Geführte Farming-Routenaufnahme](guided-route-recording.md)
- [Farming-Route-Assignment](route-assignment.md)
- [Farming-RouteCatalog und Lifecycle](route-lifecycle.md)

---
*Zuletzt aktualisiert: 28. Juli 2026*
