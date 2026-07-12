# Read-only UI-State-Probe

## Überblick

Phase 7.1 ergänzt eine strikt read-only Diagnoseoberfläche für den D2R-UI-Buffer. Der Operator bereitet einen Menü- oder Spielzustand manuell vor und versieht ihn mit einem stabilen Label. Der Bot liest zwölf Samples, trennt stabile von volatilen Bytes und veröffentlicht ein atomisches JSON-Artefakt unter `diagnostics/ui-states/`.

Die Probe löst niemals Tastatur- oder Mausaktionen aus. Einzelne Buffer-Bytes erhalten erst nach wiederholter Live-Validierung eine semantische Bedeutung. Insbesondere autorisiert ein Capture noch keinen späteren Menü-Klick.

## Ort im Code

- **Memory-Capture:** `internal/memory/ui.go` → `(*ProbeReader).CaptureUIBuffer`
- **CLI und Auswertung:** `internal/app/ui_state_probe.go` → `(*Runtime).RunUIStateProbe`
- **Wiring:** `cmd/d2rbot/main.go`, `internal/app/options.go`
- **Artefakte:** `diagnostics/ui-states/*.json` (lokal, gitignored)

## Funktionalität

### Capture-Ablauf

1. Label und Moduskonflikte werden vor Runtime-Start validiert.
2. Der normale Prozess-Attach und Signature-/Cache-Offsetpfad löst den UI-Anker auf.
3. Der normale read-only Snapshot aktualisiert `GamePhase` und die bereits bekannten UI-Flags.
4. Zwölf vollständige Buffer-Samples von `UI-0x13` bis einschließlich des bekannten Loading-Bytes werden gelesen.
5. Pro Byte wird bestimmt, ob der Wert in allen Samples stabil oder innerhalb des Capture-Fensters volatil war.
6. Ein SHA-256-Fingerprint bindet Stable-Mask und stabile Werte.
7. Das JSON wird über eine temporäre Datei geschrieben, geflusht und atomar veröffentlicht.

Der Capture besitzt eine eigene Kopie des Buffers. Spätere Memory-Reads können bereits veröffentlichte Samples nicht verändern.

### Label-Vertrag

Labels müssen `^[a-z0-9][a-z0-9-]{0,63}$` entsprechen. Empfohlene Phase-7.1-Labels:

- `gameplay-closed`
- `quit-menu`
- `character-screen`
- `difficulty-dialog`
- `loading`

Für Reproduktionsläufe darf ein Suffix verwendet werden, zum Beispiel `quit-menu-2`. Pfadtrenner, Leerzeichen und Großbuchstaben sind nicht zulässig.

### JSON-Schema v1

| Feld | Bedeutung |
|------|-----------|
| `schema_version` | Exakt `1` für den ersten Research-Vertrag. |
| `captured_at`, `label`, `game_version` | Capture-Metadaten. |
| `phase`, `world_valid`, `world_reason` | Semantischer Zustand des letzten normalen Snapshots. |
| `inventory_open`, `stash_open`, `quit_menu_open` | Semantische read-only UI-Flags; `quit_menu_open` wurde in Phase 7.1 live validiert. |
| `buffer_size`, `anchor_index`, `sample_count` | Form und Anzahl der Samples. |
| `stable_hash` | SHA-256 über Stabilitätsmaske und stabile Bytes. |
| `stable_non_zero` | Stabile, nicht-null Bytes mit absolutem Capture-Index und relativem Offset zum UI-Anker. |
| `volatile_offsets` | Bytes, die sich innerhalb der zwölf Samples verändert haben. |
| `raw_hex_samples` | Vollständige Rohsamples für spätere Offline-Vergleiche. |

Relative Offsets sind die maßgebliche Forschungsnotation. Der Capture-Index `0x13` entspricht dem signature-resolvierten UI-Anker; kleinere Indizes sind negative Offsets.

## Operator / CLI

Beispiel für einen manuell geöffneten Quit-Dialog:

```powershell
go run ./cmd/d2rbot --ui-state-probe quit-menu --verbose
```

Optionales Timeout:

```powershell
go run ./cmd/d2rbot --ui-state-probe character-screen --ui-state-probe-timeout-ms 60000 --verbose
```

Der Modus ist gegenseitig exklusiv mit Run, Run-Phase, Input-Test, Pathing-Test, Route-Modus und Offline-Difficulty-Test. Ein über `runs.active` konfigurierter Run wird für diesen expliziten Research-Modus nicht erzeugt. Gameplay-Bindings-Prechecks werden übersprungen.

Erfolgslog:

```text
read-only UI-state capture published label=quit-menu path=diagnostics/ui-states/... phase=in_game stable_hash=... stable_non_zero=... volatile_offsets=...
```

## Live-Matrix Phase 7.1

Am 11. Juli 2026 wurde die Matrix mit D2R `3.2.92777`, Charakter `MrBones` und 1280×720 durchgeführt. Der Operator stellte jeden Zustand manuell her; der Bot erzeugte keinerlei Menüinput.

| Zustand | Captures | Ergebnis |
|---------|----------|----------|
| Gameplay, alle Menüs geschlossen | 3 vor dem Exit, 3 nach neuem Offline-Start | Reproduzierbare Negativ-Baseline; `UI-0xB=0`. Die drei Post-Start-Captures waren identisch. |
| Escape-/Quit-Menü geöffnet | 3 Research-Captures plus 1 semantischer Abschluss | `UI-0xB=1` in jedem Capture; `quit_menu_open=true` im Abschlussartefakt. |
| Offline-Charakterbildschirm | 3 | Stabil vom In-Game-Zustand unterscheidbar, unter anderem `UI+0x10E=2`; kein exklusiver Unterschied zum Difficulty-Dialog. |
| Difficulty-Dialog | 3 | Alle über Wiederholungen stabilen Kandidaten entsprechen dem Charakterbildschirm. Der untersuchte Buffer kann beide Screens nicht sicher trennen. |
| Loading beim Offline-Start | kontrollierter Start zwischen den Capture-Gruppen | Bestehendes Loading-Signal bleibt der Übergangsnachweis; danach bestätigten drei Captures erneut `in_game` und Character Identity. |

Ein Kandidat wird erst semantisches Flag, wenn er innerhalb eines Zustands stabil, über dessen Wiederholungen reproduzierbar und in allen relevanten Negativzuständen abweichend ist. Zufällige, zeitabhängige oder zwischen Starts driftende Bytes werden verworfen.

### Validierte Kandidaten und Entscheidung

| Relativer Offset | Gameplay | Quit-Menü | Charakterbildschirm | Difficulty-Dialog | Bewertung |
|------------------|----------|-----------|---------------------|-------------------|-----------|
| `UI-0xB` | `0` | `1` | `0` | `0` | Autoritatives `QuitMenuOpen`-Flag für D2R `3.2.92777`. |
| `UI-0x2` | `1` | `0` | `0` | `0` | Ergänzender In-Game-/Quit-Kontext, nicht als einzelnes Gate verwendet. |
| `UI+0x0` | `1` | `0` | `1` | `1` | Kein eindeutiges Menüsignal. |
| `UI+0x9` | `1` | `0` | `1` | `1` | Kein eindeutiges Menüsignal. |
| `UI+0x10E` | `1` | `1` | `2` | `2` | Frontend-/In-Game-Kandidat, trennt Character und Difficulty nicht. |

Die Phase-7.1-Entscheidung lautet daher **Memory plus enge Screen-Anker**:

- Save & Exit kann mit `InGame`, `QuitMenuOpen` und Ergebnisbestätigung vollständig Memory-gated werden.
- Der Offline-Charakterbildschirm und der Difficulty-Dialog benötigen jeweils einen eng begrenzten visuellen Zustandsnachweis.
- Die Screen-Nachweise dürfen ausschließlich diese Frontend-Zustände bestätigen. Gameplay-Entscheidungen bleiben Memory-basiert.
- Ein zeitbasierter Fallback oder ein Klick allein aufgrund von `GamePhaseMenu` ist verboten.

## Sicherheitsgrenzen

- Kein Input und keine automatische Zustandsherstellung.
- Kein Schreiben in D2R-Memory, Installations- oder Savegame-Dateien.
- Capture-Dateien enthalten Rohdaten und bleiben lokal/gitignored.
- Der Buffer ist patch-sensitiv; `game_version` und der signature-resolvierter UI-Anker gehören zur Auswertung.
- `stable_hash` identifiziert einen Capture, ist aber kein alleiniger Menüzustandsnachweis.
- Phase 7.2 und 7.3 dürfen erst nach abgeschlossener Live-Matrix semantische Flags als Input-Gates verwenden.
- Wenn Character-Screen oder Difficulty-Dialog nicht eindeutig aus Memory erkennbar sind, wird der unterstützte Startpunkt enger gefasst oder ein einzelner visueller Anker separat geplant. Es gibt keinen Fallback auf Blindklicks.

## Tests

- Memory-Tests bestätigen vollständige Länge, Anchor-Relation, unveränderliche Kopie und Fehler ohne Attach.
- App-Tests bestätigen Label-Validierung, stabile/volatile Klassifikation, Fingerprint, atomare Veröffentlichung und Deaktivierung eines konfigurierten Runs.
- `go test ./...` deckt den read-only Modus ohne D2R-Liveprozess ab.

## Implementierungsstand Phase 7.1

Phase 7.1 ist abgeschlossen. Capture- und Artefaktpfad sind implementiert, die Live-Matrix ist ausgewertet, `QuitMenuOpen` wurde in `memory.UIState` und `world.UIState` übernommen und live bestätigt. Für Charakterbildschirm und Difficulty-Dialog ist dokumentiert, dass Memory nicht genügt; Phase 7.3 muss die zwei engen Screen-Nachweise implementieren und negativ testen.

## Verwandte Features

- [Session-Lifecycle](session-lifecycle.md)
- [State Probe](state-probe.md)
- [Offline-Difficulty-Auswahl](offline-difficulty-selection.md)
- [Input Controller](input-controller.md)

---
*Zuletzt aktualisiert: 2026-07-11*
