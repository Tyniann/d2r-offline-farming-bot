# Offline-Difficulty-Auswahl

## Überblick

Phase 6.1c wählt Normal, Nightmare oder Hell kontrolliert im bereits geöffneten Offline-Difficulty-Dialog. Die Auswahl ersetzt keinen Memory-Nachweis: Der Bot bestätigt anschließend den aktiven Charakter, während der Layout-Fingerprint die Kartenkompatibilität absichert.

## Ort im Code

- **Paket:** `internal/app`
- **Einstieg:** `(*Runtime).RunOfflineDifficultyTest`
- **Wichtige Dateien:** `offline_game.go`, `offline_game_test.go`
- **CLI:** `--offline-difficulty-test normal|nightmare|hell`

## Funktionalität

Der Operator selektiert den gewünschten Offline-Charakter und öffnet den Dialog mit den drei Difficulty-Schaltflächen. Der Bot akzeptiert ausschließlich eine gebundene D2R-Clientfläche von 1280×720, klickt genau eine kalibrierte Position und wartet auf einen validen In-Game-State mit bestätigter Character Identity.

Der Modus ist isoliert und kann nicht gleichzeitig mit Run-, Phase-, Input- oder Pathing-Tests verwendet werden. Er führt keinen Kampf-Bindings-Precheck aus.

## Sicherheitsgrenzen

- Keine automatische Charakterauswahl.
- Keine allgemeine Bilderkennung des Menüs; der Dialog muss sichtbar vorbereitet sein.
- Kein persistenter Cache der letzten Auswahl.
- Difficulty bleibt flüchtiger Kontext und kann keinen Layout-Mismatch überstimmen.

## Live-Abnahme

Am 11.07.2026 wurden Hell und Nightmare bei 1280×720 kontrolliert angeklickt. Beide Starts erreichten `in_game` und bestätigten `MrBones` / Necromancer nach drei stabilen Snapshots.

## Verwandte Features

- [Read-only Game Identity](game-identity.md)
- [Layout-Fingerprint](layout-fingerprint.md)
- [Route Recording und Playback](route-recording-playback.md)

---
*Zuletzt aktualisiert: 2026-07-10*
