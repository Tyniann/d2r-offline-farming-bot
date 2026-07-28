# Layout-Fingerprint

## Überblick

Phase 6.1d erzeugt einen deterministischen Fingerabdruck des aktuell beobachtbaren lokalen Kartenlayouts. Eine Route darf später erst Input ausführen, wenn ihr gespeicherter Fingerprint reproduziert wurde.

## Ort im Code

- **Paket:** `internal/pathing`
- **Einstieg:** `pathing.BuildLayoutFingerprint`
- **Wichtige Dateien:** `layout_fingerprint.go`, `layout_fingerprint_test.go`
- **CLI-Diagnose:** `--pathing-test inspect:layout`

## Datenmodell

`LayoutFingerprint` enthält Schemaversion, Area-ID, Spielerposition als Diagnose, Anzahl stabiler Anker und SHA-256-Hash. Der kanonische Hash verwendet Area-ID sowie sortierte semantische IDs und World-Koordinaten stabiler Waypoints, Truhen, Stash-Objekte und Entrances.

UnitIDs, Pointer, Zeitstempel, Monster, Items und Spielerposition werden nicht gehasht. Damit bleibt derselbe Layoutanker trotz neuer Laufzeit-IDs und kleiner manueller Bewegung stabil.

## Fehlerverhalten

- Ungültiger oder nicht In-Game befindlicher World-State: `ErrLayoutStateInvalid`.
- Keine sichtbaren stabilen Anker: `ErrLayoutAnchorsUnavailable`.
- Playback-Vertrag: fehlender Fingerprint führt zu `route_layout_unverified`, Abweichung zu `route_layout_mismatch`; beides vor dem ersten Input.

## Operator / CLI

`--pathing-test inspect:layout` ist read-only. Der Modus loggt den Hash nur bei Änderung sowie Area, Spielerposition, Ankerzahl und die kanonischen Ankerzeilen (`o:<txtFileNo>:<x>,<y>` bzw. `e:<txtFileNo>:<kind>:<x>,<y>`). Für vergleichbare Messungen positioniert der Operator den Charakter an demselben stabilen Startanker.

## Live-Abnahme

Am 11.07.2026 reproduzierten zwei getrennte Nightmare-Spiele mit `MrBones` am Black-Marsh-Waypoint exakt denselben Fingerprint `c233f9b137a09e07e3b8d0d2fc02c74103bbc54e42ff89e57d9592a6024fb51c`. Das Hell-Layout ergab `c8b942fbe3c30b921caa6fdd1d9da3a2207f7934325788685ac154dc432e3b8c` und wurde damit eindeutig getrennt.

## Verwandte Features

- [Read-only Game Identity](game-identity.md)
- [Offline-Difficulty-Auswahl](offline-difficulty-selection.md)
- [Route Recording und Playback](route-recording-playback.md)

---
*Zuletzt aktualisiert: 2026-07-10*
