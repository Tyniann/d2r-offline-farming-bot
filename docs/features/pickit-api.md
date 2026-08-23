# Pickit-API und sichere Routengrenze

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

Profil- und Assignment-Updates verwenden `expected_revision`. Stale Writes liefern `revision_conflict` mit erwarteter und aktueller Revision. `commandMu` serialisiert parallele Writes. Während einer aktiven Route oder eines exklusiven Routen-Workflows werden Pickit-Mutationen mit `command_conflict` abgelehnt. Erfolgreiche Änderungen erzeugen ein SSE-Ereignis.

### Sprachneutrale Regel-Summary

Jede Regel in einer Profil-, Validierungs- oder Mutationsantwort enthält eine vom bestehenden Parser berechnete `summary`. `kind` unterscheidet bekannte Runen-, Trank-, Item-, Qualitäts-, Tier-, Identitäts- und Sockelfilter von `custom`. `params` transportiert ausschließlich typisierte sprachneutrale Werte wie `codes`, `types`, `qualities`, `tiers`, Identitätsschlüssel, Sockeloperator/-zahl und Ätherisch-Status. Die React-Oberfläche übersetzt diese Werte und parst den Pickit-Ausdruck nicht selbst. Nicht verlustfrei darstellbare, negierte oder manuell geschriebene Regeln werden als `custom` projiziert; ihr vollständiger Ausdruck bleibt erhalten.

### Kaskadierendes Profil-Löschen

Delete prüft Profil- und Assignment-Revision gemeinsam unter `commandMu`. Ohne `remove_assignments=true` bleibt ein verwendetes Profil mit `profile_assigned` geschützt. Nach ausdrücklicher Bestätigung entfernt der App-Vertrag alle Charakter-/Routenreferenzen in höchstens einer Assignment-Revision, löscht leere Routen- und Charaktereinträge und danach das Profil. Schlägt das Profil-Löschen fehl, wird das vorherige Assignment-Manifest wiederhergestellt. Die Erfolgsantwort enthält den autoritativen Assignment-Stand und jede entfernte Verwendung; bei einer tatsächlichen Kaskade werden Profil- und Assignment-Ereignis veröffentlicht.

### Routen-Snapshot

Vor jeder Routen-Generation einer Session löst der Core Charakter, Route, geordnete Profil-IDs, Profilrevisionen und Assignment-Revision neu auf. Erst nach vollständigem Laden, Validieren und Kompilieren wird die eine Action Policy atomar für Pickup, Stash und Town aktiviert. Ein fehlerhafter Reload stoppt die nächste Route sichtbar und lässt die aktive Policy unverändert. Session- und Queue-Preflight verlangen für jede geplante Route eine gültige Zuordnung.

### Import und Export

Import wandelt NIP-Zeilen in kanonische Entwurfsregeln um und persistiert nichts. Weil NIP keine Phase-13-Aktion transportiert, muss eine Aktion explizit gewählt werden. Export liefert kanonische Ausdrücke und weist darauf hin, dass `keep`, `sell` und `ignore` nicht im NIP-Text enthalten sind.

### Kontrollierte Item-Vorschau (Socket-Felder, Phase 19)

Der Preview-Request und die Antwort tragen zusätzlich zu den bestehenden Item-Feldern:

| Feld | Bedeutung |
|------|-----------|
| `base_tier` | Generiertes Tier des Basisitems |
| `sockets` | Gesamtsockelzahl (0 wenn unavailable) |
| `sockets_available` | Ob konsistente Socket-Evidenz vorliegt |
| `socketed` | Konsistentes Ergebnis aus Flag und Stat |

Widersprüchliche Fixtures mit `sockets_available=true` werden abgelehnt. Die Auswertung nutzt dieselbe fail-closed Semantik wie die Runtime; Details: [Sockel-Support für Pickit](socket-pickit.md).

## Grenzen

Die Vorschau arbeitet ausschließlich mit einem vom Request gelieferten Test-Item und liest weder Live-Memory noch sendet sie Input. Die React-Oberfläche ist nicht Teil dieses Abschnitts.

## Verwandte Features

- [Pickit-Profile und Assignments](pickit-profiles.md)
- [Lokale Core-API](local-core-api.md)
- [Session-Lifecycle](session-lifecycle.md)
- [Sockel-Support für Pickit](socket-pickit.md)

---
*Zuletzt aktualisiert: 23. August 2026*
