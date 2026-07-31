# Phase-13-Core-Vertrag

## Überblick

Abschnitt 13.0 friert die bestehende Loot-, Config-, Session-, API- und React-Baseline ein und legt die persistenten Pickit-Verträge fest. Dieser Abschnitt erzeugt noch keinen Item-Identitätskatalog, liest keine neue Memory-Quelle und verändert keine produktive Loot-Entscheidung.

KISS und YAGNI sind verbindlich: Phase 13 ergänzt ausschließlich wiederverwendbare Profile, die Aktionen `keep` und `sell`, exakte Set-/Unique-Identitäten sowie `ethereal + base item`. `maxquantity`, Prefix/Suffix, Affixdaten und vollständige NIP-Kompatibilität bleiben ausgeschlossen. **Sockelregeln waren in Phase 13 bewusst nicht enthalten**; sie wurden in **Phase 19** nach Live-Gate spezifiziert und implementiert — siehe [Sockel-Support für Pickit](socket-pickit.md).

## Ort im Code

- **Persistente Verträge und Ownership:** `internal/app/phase13_contract.go`
- **Aktionsvertrag:** `internal/loot/action.go`
- **Baseline-Matrix:** `internal/loot/phase13_baseline_test.go`
- **Parser-Non-Goals:** `internal/loot/pickit_test.go`
- **Bestehende Run-Config:** `internal/config/config.go`
- **Bestehender Session-Preflight:** `internal/app/session_plan.go`
- **API-Sicherheitsvertrag:** `internal/api/server.go`
- **React-Struktur:** `web/src/app/`, `web/src/features/routes/`

## Persistente Schemas

### Profil

```yaml
schema_version: 1
revision: 1
id: tal-rasha
name: Tal Rasha Set
rules:
  - id: tal-rasha-adjudication
    action: keep
    expression: "[setitem] == \"Tal Rasha's Adjudication\""
```

- Profil- und Regel-ID sind unveränderliche, kleingeschriebene Slugs.
- Anzeigename und geordnete Regeln sind editierbar.
- Ein gespeichertes Profil besitzt mindestens eine Regel.
- Eine Regel besitzt genau eine Aktion: `keep` oder `sell`.
- Die Expression bleibt die bearbeitbare kanonische Quelle. AST, Katalogauflösung und numerische IDs sind ausschließlich immutable Runtime-Artefakte.
- Doppelte Regel-IDs, leere Ausdrücke und unbekannte Aktionen machen das gesamte Dokument ungültig.

### Assignment

```yaml
schema_version: 1
revision: 12
assignments:
  MrBones:
    countess: [gems, keys, countess-standard]
    mephisto: [tal-rasha, gems, mephisto-standard]
```

- Der Character-Name wird als bestätigter Core-Kontext gespeichert und case-insensitiv eindeutig behandelt.
- Run-IDs müssen aus der autoritativen Run Registry stammen.
- Pro `(character, run)` ist die Profilliste nicht leer, geordnet und duplikatfrei.
- Die Assignment-Reihenfolge und danach die YAML-Regelreihenfolge bilden die Entscheidungskette. Das erste Match entscheidet.
- `configs/pickit/profiles/*.yaml` und `configs/pickit-assignments.local.yaml` werden ab 13.4 die einzigen persistenten Autoritäten.

## Aktionen

| Aktion | Vertrag |
|---|---|
| `keep` | Geschützten Pickup ausführen, bei identifikationspflichtiger Qualität mit demselben Policy-Snapshot identifizieren und erneut prüfen, danach ausschließlich bei erneutem `keep` stashen. |
| `sell` | Geschützten Pickup ausführen, erforderlichenfalls mit demselben Policy-Snapshot identifizieren und erneut prüfen, danach ausschließlich bei erneutem `sell` verkaufen. |
| Kein Match | Item ignorieren; kein Pickup-, Stash- oder Vendor-Input. |

Weitere Aktionen und ein stiller Fallback sind nicht erlaubt.

## Revisionen und atomare Aktivierung

- Ein neues oder dupliziertes Profil startet mit Revision `1`. Jede erfolgreiche inhaltliche Mutation erhöht ausschließlich die Revision dieses Profils um eins.
- Das Assignment besitzt eine globale positive Revision. Jede erfolgreiche atomische Ersetzung einer Zuordnung erhöht sie um eins.
- Mutationen verlangen die erwartete Revision. Eine abweichende Revision liefert einen Konflikt und schreibt nichts.
- Eine idempotent wiederholte, bereits bestätigte Mutation erzeugt keine zusätzliche Revision.
- Vor dem ersten Dateisystemwechsel werden Schema, IDs, Aktion, Ausdruck, Katalogreferenzen und Konflikte vollständig validiert.
- Commit-Reihenfolge: temporäre Datei im Zielverzeichnis, Flush, Close, atomischer Replace/Rename, Re-Read und erneute Validierung. Bei einem Fehler bleibt die vorherige gültige Version autoritativ.
- Ein Run erhält an seiner sicheren Startgrenze einen unveränderlichen Effective-Policy-Snapshot mit Profil-, Assignment- und Katalogrevision. Speichern verändert diesen Snapshot nicht.
- Vor dem nächsten Run lädt der Supervisor die aktuelle Zuordnung neu. Ein ungültiger Reload hält zwischen Runs ohne Gameplay-Input fail-closed an.

## Paketgrenzen

| Verantwortung | Owner |
|---|---|
| Rohe Set-/Unique-Referenz transportieren | `internal/memory` |
| Qualität, Basiscode und Identität gegen den generierten Katalog auflösen | `internal/world` |
| Parser, Aktionen, First-Match, Re-Evaluation und Trace | `internal/loot` |
| Profile, Assignments, Revisionen, Migration und Run-Snapshot | `internal/app` |
| HTTP-/JSON-/SSE-Projektion und Security Envelope | `internal/api` |
| Bedienoberfläche ohne eigene Entscheidungsengine | `web/src/features/pickit` |

Runtime-Code liest weder `.tmp/` noch D2R-Dateien. React liest oder schreibt keine YAML-Datei und entscheidet kein Item selbst.

## Reason-Codes

Vorhandene Codes werden ohne Synonyme wiederverwendet: `pickit_match`, `pickit_no_match`, `identify_required`, `inventory_full`, `stash_candidate`, `town_item_classification_invalid`, `town_item_verify_timeout`, `request_unauthorized`, `origin_rejected`, `request_invalid`, `payload_too_large`, `command_conflict` und `state_changed`.

Neue Phase-13-Codes sind abschließend:

| Bereich | Codes |
|---|---|
| Katalog/Identität | `item_identity_unavailable`, `item_identity_unknown`, `item_identity_quality_mismatch`, `item_identity_base_mismatch`, `pickit_catalog_version_mismatch` |
| Profil | `pickit_profile_missing`, `pickit_profile_invalid`, `pickit_profile_id_conflict`, `pickit_profile_assigned`, `pickit_profile_revision_conflict`, `pickit_profile_write_failed` |
| Regel | `pickit_rule_invalid`, `pickit_rule_reference_unknown`, `pickit_rule_conflict`, `pickit_rule_no_match`, `pickit_rule_matched` |
| Assignment | `pickit_assignment_missing`, `pickit_assignment_empty`, `pickit_assignment_duplicate_profile`, `pickit_assignment_context_unknown`, `pickit_assignment_revision_conflict` |
| Import/Runtime | `pickit_import_unsupported`, `pickit_import_invalid`, `pickit_reload_pending`, `pickit_reload_failed`, `pickit_snapshot_activated` |

## Migrationsmatrix

| Alte Autorität | Ziel | Entfernungsgate |
|---|---|---|
| Countess `pickup_file` | `countess-standard` mit `keep`-Regeln | Vollständige Countess-Matchmatrix bleibt identisch. |
| Mephisto `pickup_file` | `mephisto-standard` mit `keep`- und `sell`-Regeln | Pickup-/Keep-/Sell-Matrix bleibt identisch. |
| Mephisto `sell_file` | `sell`-Aktionen im `mephisto-standard`-Profil | Identifizierte und unidentifizierte Servicekandidaten bleiben identisch. |

Die Migration wurde in 13.4 einmalig nach bestandener Vorher-/Nachher-Matrix ausgeführt. `runs.definitions.*.loot.pickup_file`, `sell_file`, ihre Runtime-/Telemetrie-Wiring-Felder und die drei alten Policy-Dateien sind entfernt. Das alte Schema liefert ausschließlich einen klaren Migrationsfehler; es gibt keinen lesenden Legacy-Fallback.

## Characterization-Baseline

Die festgeschriebene Matrix umfasst:

- Countess: Runen, Key of Terror, Rejuvenation Potions sowie Flawless/Perfect Gems und Skulls werden aufgenommen; Chipped Gems und Exceptional-/Elite-Set-/Unique-Basen nicht.
- Mephisto: Flawless/Perfect Gems und Skulls werden aufgenommen und nie verkauft. Exceptional-/Elite-Set-/Unique-Basen werden unabhängig vom Identifikationsstatus aufgenommen und als Sell-Kandidaten markiert. Normale Basen und andere Qualitäten matchen nicht.
- Bei mehreren Treffern bleibt ausschließlich der erste Regelmatch autoritativ.
- `maxquantity`, Prefix, Suffix und unbekannte NIP-Sektionen schlagen mit Datei- und Zeilenkontext fehl. `[sockets]` und `socketed` waren in der Phase-13-Baseline Parser-Fehler; Phase 19 hebt das für Gesamtsockel auf ([Sockel-Support für Pickit](socket-pickit.md)).
- Config- und Session-Tests sichern die bestehenden Run-Policy-Pfade und den read-only Preflight. API-Tests sichern Host, Origin, Token, Methode, Content-Type, Body-Limit und strikte JSON-Dekodierung. React-Tests sichern die bestehende App-/Routenstruktur und Core-autoritative Mutationen.

## Grenzen

- Kein neuer Memory-Read und keine Live-Identitätsprobe in 13.0.
- Kein Kataloggenerator, kein `setitem`-/`uniqueitem`-Parser und keine neue Loot-Runtime.
- Keine Profilpersistenz, API oder Pickit-UI vor den dafür vorgesehenen Abschnitten.
- Keine Mengen-, Prefix-/Suffix- oder Affixlogik. Gesamtsockel-Prädikate: Phase 19 ([Sockel-Support für Pickit](socket-pickit.md)).

## Verwandte Features

- [Pickit Engine](pickit-engine.md)
- [Loot Decision Pipeline](loot-decision-pipeline.md)
- [Session-Lifecycle](session-lifecycle.md)
- [Lokale Core-API](local-core-api.md)
- [Phase-12-Core-Vertrag](phase-12-core-contract.md)
- [Sockel-Support für Pickit](socket-pickit.md) — Phase-19-Nachtrag zu Socket-Non-Goals

---
*Zuletzt aktualisiert: 2026-07-31*
