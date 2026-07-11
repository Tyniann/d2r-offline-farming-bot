# Read-only Game Identity

## Überblick

Phase 6.1 führt eine Memory-bestätigte Identität der aktiven Offline-Spielsession ein. Character Name und Class ID sind belastbare Quellen und werden über drei Snapshots stabilisiert. Der untersuchte Map-Seed ist dagegen über Charaktere und Difficulties konstant und nur Diagnose. Difficulty wird vom Bot kontrolliert ausgewählt; die Playback-Freigabe erfolgt autoritativ über einen Layout-Fingerprint.

Eine operatorseitige Profil-ID bleibt ein optionales Organisationsmerkmal für CLI und spätere GUI. Sie ist kein Sicherheitsnachweis und ersetzt niemals die gelesene Game Identity.

## Ort im Code

- **Paket:** `internal/memory` für Rohdaten und Read-Logik
- **World Model:** `internal/world` für die semantische, immutable `GameIdentity`
- **Orchestrierung:** `internal/app` für Probe-Logging und spätere Route-Prechecks
- **Config:** Difficulty darf als Label konfiguriert werden, aber niemals Character-/Map-Seed-Memory überschreiben

## Zielmodell

```go
// GameIdentity is the Memory-confirmed active offline character identity.
type GameIdentity struct {
    Valid         bool
    CharacterName string
    Class         CharacterClass
    MapSeed       uint32
}
```

`memory.Snapshot.Identity` trägt Rohwert, Bestätigungsstatus und Stabilitätszähler; `world.GameIdentity` erhält nur bestätigte Werte. `GameVersion` stammt weiterhin aus der geladenen Memory-Konfiguration; Charaktername und Charakterklasse bilden die bestätigte Charakteridentität. `MapSeed` bleibt ein nicht sicherheitsrelevanter Diagnosewert.

## Umsetzungsslices

### 6.1a Quellenanalyse und read-only Probe

- belastbare Memory-Quellen für Charaktername, Charakterklasse und einen Difficulty-/Layout-Wechsel identifizieren;
- keine Savegame- oder D2R-Installationsdateien lesen oder verändern;
- Pointer-Ketten und Plausibilitätsgrenzen dokumentieren;
- Rohwerte zunächst ausschließlich im Probe-Modus sichtbar machen.

**Gate:** Wiederholte Reads im selben Spiel liefern stabile Werte und Loading erzeugt keine falsche Identität.

#### Forschungsstand 6.1a

Am 10.07.2026 wurden drei read-only Quellen implementiert und mit D2R `3.2.92777` live geprüft:

| Wert | Quelle | Ergebnis |
|------|--------|----------|
| Charaktername | `D2UnitStrc.pUnitData` bei Player Unit `+0x10`, nullterminierter Name am PlayerData-Anfang | `MrBones` stabil gelesen |
| Charakterklasse | `D2UnitStrc.dwClassId` bei Player Unit `+0x04` | `2` / Necromancer stabil gelesen |
| Map-Seed | Player Unit `+0x20` → Act `+0x78` → DRLG-Seed-Hashes `+0x840`/`+0x868`, MapAssist-Rekonstruktion nach Koolo | `466817790` gelesen, aber nicht als Character-/Difficulty-Fingerprint geeignet |

Der ältere d2go-Reader verwendet für die Klasse `playerUnit+0x174`. Live lieferte dieser Wert für `MrBones` fälschlich `0`; 6.1a verwirft ihn deshalb ausdrücklich und nutzt den grundlegenden Unit-Class-Wert bei `+0x04`.

Weder der untersuchte d2go-Stand noch Koolo lesen die tatsächliche Difficulty aus D2R-Memory; Koolo verwendet dafür seine Konfiguration. 6.1a erfindet daher keinen Difficulty-Offset.

Die Live-Matrix widerlegte den Map-Seed als Identitätsschlüssel:

| Session | Ergebnis |
|---------|----------|
| `MrBones` / Hell | Name `MrBones`, Klasse `2`, Seed `466817790` |
| `MrBones` / Nightmare | Name `MrBones`, Klasse `2`, Seed `466817790` |
| `MrHammer` | Name `MrHammer`, Klasse `3`, Seed `466817790` |

Zusätzlich wurde der lesbare DRLG-Bereich zwischen Hell und Nightmare byteweise auf einen Wechsel `2 → 1` untersucht. Es existierte kein Kandidat. Der Seed bleibt deshalb reine Diagnose; er darf weder Charakter noch Difficulty im Route-Precheck ersetzen.

6.1a ist damit als Quellenanalyse abgeschlossen: Charaktername und Klasse sind validiert, während die tatsächliche Difficulty in den untersuchten Quellen nicht verfügbar ist. Die Architekturentscheidung lautet deshalb: kein weiteres Memory-RE. Der Bot kontrolliert die Difficulty-Auswahl im Offline-Menü und verifiziert das resultierende Layout separat.

### 6.1b Snapshot und Konsistenz

- rohe Identity-Felder in `memory.Snapshot` aufnehmen;
- Name auf Länge, Encoding und erlaubte Zeichen plausibilisieren;
- Klasse nur aus bekannten Enum-Werten akzeptieren und Map-Seed-Rekonstruktion explizit validieren;
- Identity erst nach mindestens drei identischen validen In-Game-Snapshots bestätigen;
- Änderung innerhalb derselben Session invalidiert die Identity, statt sie still zu übernehmen.

**Gate:** Unit-Tests decken gültige Werte, unbekannte Enums, leere/kaputte Namen, Loading und flapping Reads ab.

**Status:** Implementiert. Snapshot-Stabilisierung und World-Mapping bestätigen erst den dritten identischen In-Game-Read und setzen bei Wechsel oder ungültigem Zustand zurück.

### 6.1c Kontrollierte Difficulty-Auswahl

Koolo liest Difficulty ebenfalls nicht aus Memory. Es verwendet die konfigurierte Difficulty, klickt die zugehörige Offline-Menüposition und verwendet denselben Wert danach für seinen Ablauf. Phase 6 übernimmt dieses Prinzip in einem engeren, fail-closed Baustein:

1. Operator beziehungsweise spätere GUI wählt Character-Profil und Difficulty.
2. Der gewünschte Character ist im Offline-Menü bereits selektiert; Character-Selection-Automation ist nicht Teil dieses Slices.
3. Bot bestätigt bekannten Screen, Fensterfokus und unterstützte Client-Geometrie.
4. Bot klickt genau eine kalibrierte Difficulty-Position.
5. Bot wartet Loading und stabiles `in_game` ab.
6. Memory-bestätigter Character muss zum gewählten Profil passen.
7. Die angeforderte Difficulty bleibt nur als flüchtiger Auswahlkontext für Logs und Erwartungsprüfung im aktuellen Prozess.

Es wird keine lokale „zuletzt gewählt“-Datei persistiert. Sie könnte nach manueller Bedienung veralten und würde keine zusätzliche Sicherheit liefern. Eine spätere GUI darf die letzte Auswahl als Komfortpräferenz speichern, aber der Core ignoriert sie bei der Playback-Freigabe.

**Gate:** Der kontrollierte Start bestätigt In-Game und Character; Playback bleibt bis zum Layout-Fingerprint gesperrt.

**Status:** Implementiert und mit `MrBones` für Hell und Nightmare live abgenommen. Der isolierte CLI-Test `--offline-difficulty-test normal|nightmare|hell` verlangt den manuell vorbereiteten Difficulty-Dialog und exakt 1280×720 Clientfläche; eine breitere Screen-Erkennung und Character-Menüautomation gehören nicht zu 6.1.

### 6.1d Layout-Fingerprint

Der Layout-Fingerprint ist die autoritative Prüfung der Kartengültigkeit. Er wird beim Recording an einem stabilen Startanker erzeugt und mit der Route gespeichert. V1 verwendet ausschließlich bereits beobachtbare World-Daten:

- Area-ID;
- exakte oder quantisierte Spielerposition am Anchor;
- Position und semantische ID des Black-Marsh-Waypoints;
- sortierte stabile Object-/Entrance-Anker mit World-Koordinaten;
- keine UnitIDs, Pointer, Timestamps oder volatile Monster-/Item-Daten.

Die kanonische Repräsentation wird deterministisch sortiert und gehasht. Vor dem ersten Route-Input bildet Playback denselben Fingerprint. Ein Mismatch endet mit `route_layout_mismatch`; ein fehlender ausreichend starker Fingerprint mit `route_layout_unverified`.

Ein manuell gestartetes Spiel kann ebenfalls verwendet werden, wenn Character Identity und Layout-Fingerprint vollständig passen. Die zuletzt gewählte Difficulty muss dafür nicht lokal bekannt sein.

**Gate:** Fingerprint ist über neue Spiele desselben Character-/Difficulty-Layouts stabil und unterscheidet mindestens die live getesteten Hell-/Nightmare-Layouts.

**Status:** Implementiert und live abgenommen als kanonischer SHA-256 über Area-ID und sortierte stabile Object-/Entrance-Anker. Flüchtige UnitIDs und die Spielerposition gehen nicht in den Hash ein; letztere bleibt Diagnosemetadatum. `--pathing-test inspect:layout` macht den Fingerprint read-only sichtbar. Zwei neue Nightmare-Spiele reproduzierten exakt `c233f9b137a09e07e3b8d0d2fc02c74103bbc54e42ff89e57d9592a6024fb51c`; Hell lieferte abweichend `c8b942fbe3c30b921caa6fdd1d9da3a2207f7934325788685ac154dc432e3b8c`.

### World Model und Probe-Logging

- semantische `world.GameIdentity` immutable pro Tick mappen;
- `Valid=false` mit internem Invaliditätsgrund erhalten;
- Probe loggt Identity nur bei Änderung und im Heartbeat, niemals Memory-Adressen;
- Attach/Detach und neue Session setzen die bestätigte Identity zurück.

**Gate:** App-/Mapper-Tests beweisen Reset, Stabilisierung und unveränderte Snapshot-Semantik.

### Live-Validierung und Route-Precondition

Mindestens folgende Matrix manuell prüfen:

| Fall | Erwartung |
|------|-----------|
| Charakter A / Normal | Name A stabil; kontrollierte Auswahl und Normal-Fingerprint |
| Charakter A / Nightmare oder Hell | gleicher Name; anderer Layout-Fingerprint |
| Charakter B / gleiche Difficulty | Name B; keine Wiederverwendung von A |
| Loading / Menü / Prozessverlust | Identity ungültig; kein Recording/Playback |
| Rückkehr zu Charakter A / ursprüngliche Difficulty | Route darf nach vollständigem Precheck wieder auswählbar sein |

**Gate:** Eine Route für Charakter A/Layout A kann bei Charakter B oder Layout B nicht bis zum ersten Input gelangen.

## Route-Bindung

Route Contract v1 kann nach 6.1a sicher verpflichtend binden an:

- `character_name` aus `world.GameIdentity`;
- `game_version` aus der aktiven Memory-Konfiguration;
- plausible Start-Area und Startposition.

`character_class` wird mitgespeichert und geprüft, ist aber kein Ersatz für den Namen. `map_seed` darf als Diagnosewert gespeichert werden, besitzt keine Sicherheitswirkung. `difficulty` ist die vom Bot kontrolliert ausgeführte Auswahl. Der Layout-Fingerprint bestätigt, dass die aktive Karte zur Route passt. `profile_id` dient nur Gruppierung und Anzeige.

Ein späterer Map-Seed oder Layout-Fingerprint kann die Bindung erweitern. Ohne eine validierte Quelle ist er keine Voraussetzung für Phase 6.1.

## Fehlerverhalten

| Code | Bedeutung |
|------|-----------|
| `game_identity_unavailable` | Charaktername oder Klasse konnte nicht konsistent bestimmt werden. |
| `game_identity_unstable` | Werte wechseln innerhalb des Bestätigungsfensters oder der Session. |
| `route_character_mismatch` | Aktiver Charaktername passt nicht zur Route. |
| `route_layout_unverified` | Es konnten nicht genügend stabile Layoutanker beobachtet werden. |
| `route_layout_mismatch` | Aktueller Fingerprint passt nicht zur Route. |
| `route_game_version_mismatch` | Konfigurierte/unterstützte Spielversion passt nicht zur Route. |

Alle Fehler führen vor Recording oder Playback zu einem fail-closed Abbruch. Ein Difficulty-Label oder flüchtiger Auswahlkontext kann einen Layout-Mismatch nicht übersteuern.

## Grenzen

- Keine Charakterauswahl im D2R-Menü; Phase 6.1 erkennt nur den bereits aktiven Charakter.
- Keine Savegame-Inspektion.
- Der gelesene Map-Seed ist nur Diagnose und keine Playback-Freigabe.
- Keine GUI; die Identity-Daten werden jedoch GUI-neutral modelliert.

## Verwandte Features

- [Route Recording und Playback](route-recording-playback.md)
- [State Probe](state-probe.md)
- [World Model](world-model.md)

---
*Zuletzt aktualisiert: 2026-07-10*
