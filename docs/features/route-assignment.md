# Farming-Route-Assignment

## Überblick

Phase 12.1 ersetzt globale Run-`route_id`-Felder durch eine atomische Zuordnung pro `(character, run)`. Difficulty bleibt Route- und Lifecycle-Binding, ist aber kein paralleler Assignment-Slot.

## Ort im Code

- **Store:** `internal/app/route_assignment.go`
- **Schema:** `internal/app/phase12_contract.go`
- **Config:** `routes.assignments_file`
- **Standarddatei:** `configs/route-assignments.local.yaml`

## Funktionalität

`RouteAssignmentStore` lädt ein Manifest mit Schema-Version und monotoner Revision. Availability, SessionPlan, Queue-Preflight und Runtime-Wiring verwenden ausschließlich diese Auflösung. Ein Assignment autorisiert weder stale noch archivierte, geänderte oder statisch inkompatible Routen.

Beim ersten Start ohne Manifest werden vorhandene `runs.definitions.*.route_id`-Werte einmalig übernommen. Danach entfernt die Migration ausschließlich diese Farming-Felder atomisch aus der geladenen Config; Town-/Egress-`route_id` bleibt unangetastet. Ein vorhandenes Manifest verhindert jede erneute Migration und es gibt keinen lesenden Legacy-Fallback.

`Commit` verlangt die aktuelle Revision. Parallele Bestätigungen können daher höchstens einen Commit gewinnen. Schreiben erfolgt über Temp-Datei, Flush und atomisches Replace; Korruption und Write-Fehler enden fail-closed.

Phase 20.2 migriert das Manifest atomisch auf Schema 2 und ergänzt `route_sets`. Bestehende Einzelzuweisungen bleiben unverändert. Der einzige feste Route-Satz gehört zu `cows` und enthält die typisierten Slots `leg_acquisition` und `cow_sweep`. `CommitRouteSetRole` ersetzt oder entfernt genau einen Slot und erhält den anderen; ein partieller Satz ist diagnostisch sichtbar, autorisiert aber keinen Cow-Run. Beide Rollen müssen dieselbe Charakter-, Klassen-, Difficulty-, Versions- und Profilidentität tragen. Map Seed und Layout-Fingerprint bleiben rollenlokale Runtime-Bindings.

## Lifecycle

`RouteLifecycleRoute` enthält zusätzlich den registrierten `run_id` und den orthogonalen `management_status` `active|archived`. Bestehende Phase-11-Manifeste werden deterministisch anhand der Route-Endpunkte erweitert. Difficulty-/Layout-Invalidierung bleibt unabhängig; `archived + stale` ist zulässig.

Die Phase-12-Live-Abnahme endete mit Assignment-Revision 2: `mrbones/countess` verweist auf `countess-mrbones-fd1756c208`, `mrbones/mephisto` unverändert auf `durance-2-mephisto-nightmare-mrbones`. Lifecycle-Revision 8 markiert die neue Countess-Route aktiv und den unveränderten Vorgänger archiviert.

## Verwandte Features

- [Farming-RouteCatalog und Lifecycle](route-lifecycle.md)
- [Run-Verfügbarkeit und Inspect](run-availability.md)
- [Phase-12-Core-Vertrag](phase-12-core-contract.md)

---
*Zuletzt aktualisiert: 1. August 2026*
