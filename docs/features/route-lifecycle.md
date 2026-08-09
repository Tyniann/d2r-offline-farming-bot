# Farming-RouteCatalog und Lifecycle

## Überblick

Abschnitt 11.5 katalogisiert alle Farming-Routen unter einem gemeinsamen Root und verwaltet ihre Verwendbarkeit in einem atomischen lokalen Lifecycle-Manifest. Route-Dateien bleiben die autoritative Quelle für Navigation, Bindung, Aufnahmezeit und Layout-Fingerprint; das Manifest enthält ausschließlich Bestätigung, beobachteten Datei-Fingerprint und Invalidation.

## Ort im Code

- **Katalog und Lifecycle:** `internal/app/route_lifecycle.go`
- **Atomisches Windows-Replace:** `internal/app/route_lifecycle_replace_windows.go`
- **Availability:** `internal/app/run_availability.go`
- **Playback-Gate:** `internal/app/route_adapter.go`
- **Recorder-Integration:** `internal/app/route_record.go`
- **Config:** `routes.farming_root`, `routes.lifecycle_file`

## Funktionalität

Die erneute Memory-bestätigte Auswahl desselben bereits persistierten Charakters und derselben Schwierigkeit ist revisions-idempotent: Sie bestätigt den Runtime-Kontext, verändert aber keine Route-Autorität. Dadurch bleibt ein hashgeschützter Kandidat über einen Core-Neustart verwendbar. Difficulty-, Layout-, Datei- und Managementänderungen erhöhen die Revision weiterhin und machen darauf gebundene Vorschauen beziehungsweise Kandidaten fail-closed ungültig.

### Katalog und Kontextpfade

Der Katalog scannt rekursiv ausschließlich `<farming_root>/<character>/<difficulty>/*.yaml`. Pfadkontext und Route-Binding müssen übereinstimmen. Ungültige Dateien, Symlinks, unerwartete Pfadtiefen, globale doppelte Route-IDs und geänderte Dateien werden als `unavailable` gesperrt. Town-Routen liegen außerhalb des Farming-Roots und werden weder katalogisiert noch invalidiert.

Resolver und Recorder besitzen keinen konfigurierbaren Active-Directory-Zeiger mehr. Der Kontextpfad wird aus dem bestätigten Charakter und der Difficulty abgeleitet. Das entfernte `routes.directory` wird beim Laden mit einem eindeutigen Migrationsfehler abgewiesen.

### Manifest und Bootstrap

`configs/route-lifecycle.local.yaml` ist standardmäßig gitignoriert. Beim ersten Phase-11-Lauf wird der konfigurierte YAML-Kontext als `bootstrap_expected` gespeichert und jede statisch valide vorhandene Farming-Route mit SHA-256-Dateifingerprint importiert. Das autorisiert noch kein Playback. Ein erfolgreich bestätigtes Apply desselben Kontexts entfernt den Bootstrap-Marker ohne pauschale Invalidation; Live-Layoutprüfung bleibt erforderlich.

Das Manifest wird unter einem Prozess-Mutex vollständig neu kodiert, in eine temporäre Datei im selben Verzeichnis geschrieben, geflusht und über Windows `MoveFileEx` mit `REPLACE_EXISTING` und `WRITE_THROUGH` ersetzt. Beschädigte Manifeste, Revisionskonflikte und Write-Fehler brechen fail-closed ab.

### Status und Invalidation

- `runtime_validation_required`: Datei, Binding, Lifecycle und beobachteter Fingerprint sind statisch gültig; der Live-Layoutnachweis fehlt.
- `valid`: Availability besitzt zusätzlich den passenden Live-Fingerprint.
- `stale`: Aufnahme ist nicht jünger als ihre bestätigte Invalidation.
- `unavailable`: Datei oder Lifecycle-Korrelation ist nicht verlässlich.

Ein bestätigter Difficulty-Wechsel invalidiert atomisch alle Farming-Routen genau dieses Charakters über alle Difficulty-Unterordner. Gleiche Difficulty aktualisiert nur `confirmed_at`; ein reiner Charakterwechsel invalidiert nichts. Ein autoritativer Layout-Mismatch stoppt Playback vor Erstellung des Players und invalidiert ebenfalls nur den betroffenen Charakter. Eine spätere Aufnahme rehabilitiert ausschließlich die neu veröffentlichte Route.

Dropdown-Änderung, Preview, Abbruch, Game-Start-Fehler und falsche Identität schreiben nichts. Route-Dateien werden niemals automatisch gelöscht, verschoben oder umgeschrieben.

## Operator / CLI

### Managementstatus und Run-Zuordnung (Phase 12.1)

Das Lifecycle-Manifest enthält zusätzlich `management_status: active|archived` und die registrierte `run_id`. Diese Metadaten sind orthogonal zur automatischen Invalidation: Eine Route kann gleichzeitig `archived + stale` sein. Bestehende Phase-11-Manifeste werden deterministisch anhand ihrer typisierten Start-/Terminal-Areas erweitert; die Route-Datei bleibt unverändert.

Cow-Routen werden über ihre typisierte `RouteRole` und den zugehörigen Recording-Contract dem Run `cows` zugeordnet, da die einzelnen Rollen bewusst nicht dieselben Gesamt-Endpunkte besitzen. Lifecycle und Management bleiben pro Route getrennt. Cow-Availability verlangt beide aktiven Rollen; eine archivierte, stale oder unavailable Rolle sperrt den gemeinsamen Run mit einem gezielten Rollenreason.

```yaml
routes:
  farming_root: routes/farming
  lifecycle_file: route-lifecycle.local.yaml
```

Das Manifest ist lokale Runtime-Metadaten. Es soll nicht manuell als Ersatz für Route-Dateien oder bestätigte D2R-Auswahl editiert werden.

## Abhängigkeiten

- `internal/pathing` für Route-Contract-Validierung und Live-Fingerprint.
- Windows `MoveFileEx` für atomisches Ersetzen bestehender Manifestdateien.
- YAML dient nur der lokalen Persistenz; React greift nie direkt darauf zu.

## Verwandte Features

- [Route Recording und Playback](route-recording-playback.md)
- [Run-Availability](run-availability.md)
- [Charakterauswahl](character-selection.md)
- [Session-Lifecycle](session-lifecycle.md)

---
*Zuletzt aktualisiert: 1. August 2026*
