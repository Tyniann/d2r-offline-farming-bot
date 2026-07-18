# Phase-12-Core-Vertrag

## Überblick

Abschnitt 12.0 friert die Phase-11-Baseline und alle persistenten sowie ausführbaren Verträge der Routenverwaltung ein. Dieser Abschnitt führt noch keine Assignment-Migration, keinen neuen Egress-Player, keinen Recording-Input und keine API-Endpunkte ein. Er legt die Grenzen fest, gegen die 12.1–12.5 implementiert werden.

## Ort im Code

- **Lifecycle, Assignment-, Kandidaten- und Transaktionsverträge:** `internal/app/phase12_contract.go`
- **Run-Recording-Vertrag:** `internal/tasks/run_contract.go`, `internal/tasks/registry.go`
- **Globaler System-Egress-Vertrag:** `internal/town/system_egress_contract.go`
- **HTTP-/SSE-DTOs:** `internal/api/route_dto.go`
- **Schema-Fixtures:** `internal/app/testdata/phase12/`
- **Baseline-Charakterisierung:** `internal/app/phase12_baseline_test.go`

## Autoritäten und Ownership

| Vertrag | Einziger Owner |
|---|---|
| Automatische Difficulty-/Fingerprint-Invalidierung | `internal/app.RouteLifecycleStore` |
| Zuordnung pro `(character, run)` | `internal/app.RouteAssignmentStore` ab 12.1 |
| Kandidatenmetadaten | `internal/app.CandidateStore` ab 12.3 |
| Recording-/Test-Input | `internal/app.RecordingCoordinator` ab 12.3 |
| Punkte, Segmente, Binding und Recording-Metadaten | `internal/pathing.Route` |
| Start-, Boss- und Terminalsemantik | `internal/tasks.RunRegistry` |
| Globale Egress-Semantik | `internal/town.SystemEgressContract` |
| Transport und SSE-Projektion | `internal/api` |
| Crash-Recovery | `internal/app.RouteRecoveryJournal` ab 12.4 |

Es gibt keinen Ordner `internal/pathing/routecatalog`. Der in Phase 11 implementierte Farming-Katalog und Lifecycle bleiben in `internal/app`.

## Persistente Schemas

### Assignment

`RouteAssignmentManifest` verwendet `schema_version: 1`, eine positive monotone `revision` und genau eine Route pro Character-Slug und registrierter Run-ID. Difficulty ist kein Assignment-Schlüssel. Route-Dateien und Lifecycle-Daten werden nicht dupliziert. Nach der Migration existiert kein lesender Fallback auf `runs.definitions.*.route_id`.

### Lifecycle-Erweiterung

Der bestehende Lifecycle bleibt alleinige Invalidation-Autorität. Phase 12 ergänzt pro Route orthogonal `management_status: active|archived` und `run_id`. Eine Route kann daher beispielsweise `archived + stale` sein. Archivierung rehabilitiert keine Route und verändert ihre Datei nicht.

### Kandidat

`RouteCandidate` korreliert Kandidaten-ID, unveränderlichen relativen Dateinamen, SHA-256, Run-/Character-/Difficulty-/Versionsbindung, Zustand, Bossdistanz, Quellrevisionen und Zeitpunkte. Kandidaten liegen außerhalb des Farming-Roots. `test_passed` verlangt `tested_at`; `failed` verlangt einen stabilen Reason-Code.

### Recovery-Journal

Das Journal enthält Schema, Operation, Route-/Kandidaten-ID, dauerhaften Checkpoint und Startzeit. Ein unbekannter Checkpoint sperrt Management mit `route_transaction_recovery_required`.

## Recording-Verträge

Die Run Registry enthält den vollständigen `RecordingContract` und verwendet denselben `BossDescriptor` wie Combat:

| Run | Start | Erlaubte Areas | Terminal | Maximale Bossdistanz |
|---|---|---|---|---:|
| Countess | Black-Marsh-Wegpunkt / Black Marsh | Black Marsh, Forgotten Tower, Cellar 1–5 | Cellar 5, lebende Countess | 80 Tiles |
| Mephisto | Durance-Level-2-Wegpunkt / Durance 2 | Durance 2–3 | Durance 3, lebender Mephisto | 60 Tiles |

Beide Verträge verlangen Teleport-Navigation und Town Portal als Sicherheitsrückweg. Die deutsche Anleitung ist Produktmetadatum. Bossnähe beendet niemals automatisch; ausschließlich F9 beziehungsweise der gleichwertige Finish-Befehl beendet eine aktive Aufnahme kontrolliert.

## Globaler System-Egress

`SystemEgressContract` gilt ausschließlich für Acts 2–5. Er bindet Akt, Town-Area, Game-Version, Layout-Fingerprint, `portal_arrival`, `waypoint`, Walk-Bewegung und positive Ankunftstoleranz. Character, Class, Difficulty und Map Seed sind absichtlich nicht Teil des Typs. Act 1 verwendet weiterhin den Town-Graph.

## Workflow-State-Machine

```text
idle → preflight → recording → freezing → validating
→ returning_via_portal → candidate_ready → preparing_playback
→ playing_candidate → validating_terminal → returning_after_test
→ awaiting_publish_confirmation → publishing → completed

jeder aktive Zustand → failed_safe
recording + F11 → emergency_cancelled
```

F9 ist nur in `recording` gültig. Wiederholte Finish-Befehle dürfen keinen zweiten Kandidaten und kein zweites TP erzeugen. Nach `freezing` bleibt der Kandidat bei Cancellation erhalten. Session, Selection, Queue, Egress-Setup und Route-Mutationen sind während eines Workflows gesperrt.

## Lock-Reihenfolge und Revisionen

Die einzige zulässige äußere nach innere Reihenfolge lautet:

```text
workflow → catalog → lifecycle → assignment → candidate → journal
```

Preview/Confirm bindet Katalog-, Lifecycle- und Assignment-Revision sowie Kandidaten-Hash. Eine veraltete Revision oder geänderte Datei erzeugt keine Mutation.

## Crash-Matrix

| Operation | Dauerhafter Checkpoint | Recovery |
|---|---|---|
| Publish | vor Route-Publish | Kandidat behalten |
| Publish | nach Route-Publish, vor Assignment | unzugeordnete neue Route entfernen |
| Replace | nach neuer Route | alte Route aktiv und Kandidat retrybar halten |
| Replace | nach vorbereitetem Archivstatus | alten Aktivstatus wiederherstellen |
| Archive | vor Assignment-Entfernung | unverändert aktiv lassen |
| Archive | nach Assignment-Entfernung | Assignment wiederherstellen |
| Restore | nach vorbereitetem Archivieren des Vorgängers | bisherigen Aktivstatus wiederherstellen |
| Delete | nach Quarantäne-Rename | Datei aus Quarantäne zurückstellen |
| Delete | nach Manifest-Commit | Quarantäne-Löschung abschließen |

Jeder Write-/Rename-Fehler muss in späteren Abschnitten an jedem Checkpoint injizierbar sein. Die alte aktive Route gewinnt bei jedem unvollständigen Replace.

## API-/SSE-Vertrag

Die DTOs für Bibliothek, Recording-Option, Workflow, Kandidat, Mutation-Preview, System-Readiness und Route-Event enthalten keine lokalen Dateipfade. Mutationen verwenden den bestehenden Security-Envelope und zusätzlich Kandidaten-Hash, Confirmation Token und aktuelle Revisionen. SSE trägt Workflow-ID, Zustand, Run, Area, Segment, Fortschritt und stabilen Reason-Code; Snapshot-first und Reconnect bleiben unverändert.

## Baseline-Evidenz 12.0

Die produktiven Farming-Dateien werden bytegenau charakterisiert:

- Countess: `8d3dbd3bfc693689739aab56dbb9086d606d9eb9691629dfef5abad701ee0d1f`, sieben Segmente.
- Mephisto: `58752d884dfdf3db06aa546c9371de8326cd053994dd8e5a92393a130085b1f4`, zwei Segmente.
- Act-1-Graph: `806d48adabd46c29493228fe44adbbf34e70ed844674545e8a01f21d35ae079f`.
- Bestehender Act-3-Egress: `dd8f74d2ca6a2739cfe24cfdf8a9254267bbb105540eb738370ac6b8240871f5`.

Die Tests bestätigen außerdem beide Runs als `runtime_validation_required`, die Queue-Reihenfolge Countess → Mephisto, Route Contract v1, den bestehenden Act-3-Sondervertrag sowie die unveränderten Phase-11-Tests für Lifecycle, SessionPlan, Queue, Recorder, TP/Waypoint und Act-1-Town-Graph.

## Verwandte Features

- [Farming-RouteCatalog und Lifecycle](route-lifecycle.md)
- [Route Recording und Playback](route-recording-playback.md)
- [Run Registry und gemeinsames Run-Schema](run-registry.md)
- [FarmQueue-Scheduler](farm-queue-scheduler.md)
- [Lokale Core-API](local-core-api.md)

---
*Zuletzt aktualisiert: 18. Juli 2026*
