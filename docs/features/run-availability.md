# Run-Verfügbarkeit und Inspect

## Überblick

Der read-only Availability-Resolver bewertet alle registrierten Farmziele deterministisch gegen Config, Character-Kontext, Profil, Route Registry, Waypoint-Unterstützung und Town-Capabilities. `--runs-inspect` gibt denselben Vertrag als stabiles JSON aus, ohne D2R-Prozess-Attach, Hotkeys, Input oder Session-Start.

## Ort im Code

- **Paket:** `internal/app/`
- **Resolver:** `run_availability.go`
- **Vertrag:** `internal/tasks/run_contract.go`
- **CLI:** `cmd/d2rbot/main.go`
- **Session-Integration:** `internal/app/session_plan.go`
- **Config:** `runs.definitions`, `combat_profiles`, `routes.directory`, `town.egress`, `session.character`, `session.difficulty`, `memory.game_version`

## Funktionalität

### Status

| Status | Bedeutung |
|---|---|
| `available` | Definition, Config, Profil, statische Route-Bindung und übergebener Live-Fingerprint passen. |
| `runtime_validation_required` | Alle statischen Prüfungen passen; der Zielgebiets-Fingerprint fehlt im read-only Kontext und wird erst bei Ankunft autoritativ. |
| `unavailable` | Mindestens ein stabiler Sperrgrund liegt vor. |

`reasons` ist dedupliziert und lexikografisch sortiert. Die Registry liefert Runs in stabiler ID-Reihenfolge. Route-spezifische Ursachen stehen zusätzlich unter `route.reason`.

### Prüfkontext

`RunAvailabilityContext` kann Character, Character-Class, Difficulty, Game-Version, Map-Seed und Layout-Fingerprint enthalten. Leere Live-Felder werden nie geraten. Die CLI übernimmt Character und Difficulty aus `session` sowie die Game-Version aus `memory`; deshalb meldet eine passende lokale Countess-Route ohne Live-Fingerprint korrekt `runtime_validation_required`.

Ein passender expliziter Map-Seed und Fingerprint machen Countess `available`. Abweichende statische Route-Metadaten liefern `route_binding_mismatch`, ein abweichender Live-Fingerprint `route_layout_mismatch`.

### Session-Preflight

`ResolveSessionPlan` verwendet denselben Resolver. `unavailable` beendet den Start vor Runtime-Erzeugung; `runtime_validation_required` ist zulässig, weil Route Playback den Fingerprint später vor dem ersten Routeninput erneut Memory-basiert prüft. Der frühere Countess-Sonderpreflight und das feste `session.run == "countess"`-Gate existieren nicht mehr.

Seit Phase 11.5 bezieht der Resolver Kandidaten aus dem rekursiven Farming-RouteCatalog statt aus einem einzelnen aktiven Verzeichnis. `route_stale` blockiert bestätigte Lifecycle-Invalidationen, `route_lifecycle_unavailable` blockiert beschädigte, doppelte, geänderte oder nicht korrelierbare Einträge. Eine statisch valide Route bleibt `runtime_validation_required`, bis der aktuelle Live-Fingerprint passt; erst dann wird der Run `available`.

## Datenmodell

Seit Phase 12.1 stammt die Route-ID ausschließlich aus dem atomischen Assignment für `(character, run)`. Fehlende Zuordnung liefert `route_assignment_missing`; ein archivierter Eintrag blockiert Playback. Config, SessionPlan und Queue besitzen keinen globalen Farming-`route_id`-Fallback mehr.

- `app.RunAvailabilityContext`: read-only Identitätsbeleg.
- `app.RunsInspectReport`: Kontext plus geordnete `runs`.
- `tasks.RunAvailability`: ID, Anzeigename, Status, Reasons und Route-Ergebnis.
- `tasks.RunReason`: stabiler maschinenlesbarer Reason-Code.

## Operator / CLI

```powershell
go run ./cmd/d2rbot --config configs/config.yaml --runs-inspect
```

Für die aktuellen lokalen Countess- und Mephisto-Bindungen wird ohne Attach jeweils `runtime_validation_required` mit Route-ID ausgegeben. Das registrierte Mephisto-Waypoint-Ziel, die Durance-Aufnahme und der globale Act-3-System-Egress sind vorhanden; Missing-Gründe werden nur noch für tatsächlich fehlende konfigurierte Assets ausgegeben. Egress-Availability prüft ausschließlich Akt, Town-Area, Game-Version und optional den Live-Layout-Fingerprint, niemals Character, Difficulty oder Map Seed.

`--runs-inspect` ist mit Session-, Run-, Probe-, Route-, Town- und Testmodi gegenseitig ausgeschlossen. `input.enabled` darf `false` sein.

## Abhängigkeiten

- `internal/tasks` für Registry und stabile Availability-Typen.
- `internal/pathing` für read-only Route Registry und Bindungsmetadaten.
- `internal/config` für identische Run-/Profiltypen.
- `internal/town` für registrierte Foreign-Town-Egress-Capabilities.

## Verwandte Features

- [Run Registry und gemeinsames Run-Schema](run-registry.md)
- [Session-Konfiguration und Inspect](session-configuration.md)
- [Route Recording und Playback](route-recording-playback.md)

---
*Zuletzt aktualisiert: 18. Juli 2026*
