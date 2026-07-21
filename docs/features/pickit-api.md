# Pickit-API und sichere Run-Grenze

## Überblick

Die lokale Core-API stellt Katalog, Profile, Validierung, kontrollierte Vorschau, Assignments sowie NIP-Import/-Export bereit. Persistierende Aufrufe laufen durch denselben Loopback-, Origin-, Control-Token-, Payload- und Exklusivitätsvertrag wie andere Core-Mutationen.

## Ort im Code

- **Paket:** `internal/api/`, `internal/app/`
- **Einstieg:** `LiveBackend`, `Runtime.prepareSessionRun`
- **Wichtige Dateien:** `internal/api/pickit_dto.go`, `internal/api/pickit_backend.go`, `internal/api/server.go`, `internal/api/schema/openapi.json`, `internal/app/session_runtime.go`
- **Generierter Client:** `web/src/api/generated.ts`

## Funktionalität

### HTTP-Vertrag

`/api/v1/pickit/catalog` liefert den patchgenauen Basis- und Identitätskatalog. Weitere Endpunkte decken Profile, Entwurfsvalidierung, kontrollierte Item-Vorschau, Assignments, Import und Export ab. JSON wird strikt dekodiert; Bodies sind auf 64 KiB begrenzt. Create, Update, Duplicate, Delete und Assignment-Update benötigen den pro Prozess zufälligen Control-Token.

### Revisionen und Exklusivität

Profil- und Assignment-Updates verwenden `expected_revision`. Stale Writes liefern `revision_conflict` mit erwarteter und aktueller Revision. `commandMu` serialisiert parallele Writes. Während eines aktiven Runs oder eines exklusiven Routen-Workflows werden Pickit-Mutationen mit `command_conflict` abgelehnt. Erfolgreiche Änderungen erzeugen ein SSE-Ereignis.

### Run-Snapshot

Vor jeder Session-Run-Generation löst der Core Character, Run, geordnete Profil-IDs, Profilrevisionen und Assignment-Revision neu auf. Erst nach vollständigem Laden, Validieren und Kompilieren wird die eine Action Policy atomar für Pickup, Stash und Town aktiviert. Ein fehlerhafter Reload stoppt den nächsten Run sichtbar und lässt die aktive Policy unverändert. Session- und Queue-Preflight verlangen für jeden geplanten Run eine gültige Zuordnung.

### Import und Export

Import wandelt NIP-Zeilen in kanonische Entwurfsregeln um und persistiert nichts. Weil NIP keine Phase-13-Aktion transportiert, muss eine Aktion explizit gewählt werden. Export liefert kanonische Ausdrücke und weist darauf hin, dass `keep`, `sell` und `ignore` nicht im NIP-Text enthalten sind.

## Grenzen

Die Vorschau arbeitet ausschließlich mit einem vom Request gelieferten Test-Item und liest weder Live-Memory noch sendet sie Input. Die React-Oberfläche ist nicht Teil dieses Abschnitts.

## Verwandte Features

- [Pickit-Profile und Assignments](pickit-profiles.md)
- [Lokale Core-API](local-core-api.md)
- [Session-Lifecycle](session-lifecycle.md)

---
*Zuletzt aktualisiert: 21. Juli 2026*
