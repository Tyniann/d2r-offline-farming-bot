# Memory Reader

## Überblick

Low-Level Read-only Memory Reader für Phase 1 Schritt 2: rohe Bytes aus dem angebundenen D2R-Prozess lesen, `uint32`/`uint64` little-endian decodieren und Pointer-Ketten auflösen. Spiel-Semantik (State Probe) in Schritt 3: [State Probe](state-probe.md).

## Ort im Code

- **Paket:** `internal/memory/`
- **Prozess-Zugriff:** `internal/process/` → [`Service.ReadAt`](../../internal/process/process.go)
- **Einstieg:** [`internal/app/app.go`](../../internal/app/app.go) → `Memory.Bind(Process)` beim Start
- **Wichtige Dateien:**
  - `memory.go` — `Reader`, `ProcessAccess`, Primitive, Pointer-Ketten
  - `process.go` / `process_windows.go` — `ReadAt` und `ReadProcessMemory`
- **Config:** `memory.game_version`, `memory.offsets_file` (siehe [State Probe](state-probe.md)); keine eigenen Keys für Primitive-Reads

## Funktionalität

### ProcessAccess

Schmales Interface, das `process.Service` erfüllt:

```go
type ProcessAccess interface {
    ReadAt(addr uintptr, buf []byte) error
    ModuleBase() uintptr
}
```

Kein exportiertes `windows.Handle` — `memory` bleibt plattformneutral testbar.

### Primitive Reads

| Methode | Verhalten |
|---------|-----------|
| `ReadBytes(addr, size)` | Liest `size` Bytes, gibt Kopie zurück; max. 64 KiB |
| `ReadUint8(addr)` | 1 Byte |
| `ReadUint16(addr)` | 2 Bytes little-endian |
| `ReadUint32(addr)` | 4 Bytes little-endian |
| `ReadUint64(addr)` | 8 Bytes little-endian |
| `ReadInt32(addr)` | 4 Bytes little-endian (signed) |

### Pointer-Ketten

`ResolvePointerChain(base, offsets...)`:

1. Start bei `base` (typisch `moduleBase + staticOffset`)
2. Pro Offset: `uint64`-Pointer lesen → Null prüfen → Offset addieren
3. Leere Offset-Liste → `base` unverändert
4. Rückgabe: finale Zieladresse für einen Folge-Read, nicht der Feldwert

### Retry-Policy

- Bis zu 3 Versuche, 2 ms Backoff (nur im `memory.Reader`, nicht in `process`)
- Retry nur bei `process.IsReadRetryable(err)` → aktuell nur `ErrReadFailed`
- Kein Retry bei: `ErrNotBound`, `ErrNotAttached`, `ErrInvalidAddress`, `ErrInvalidPointer`, `ErrPartialRead`, `ErrInvalidRead`

## Datenmodell

### Memory-Fehler (`internal/memory`)

| Fehler | Bedeutung |
|--------|-----------|
| `ErrNotBound` | Reader ohne `Bind` |
| `ErrInvalidAddress` | Adresse `0` oder ungültige Größe |
| `ErrInvalidPointer` | Null-Pointer in Kette |

### Process-Read-Fehler (`internal/process`)

| Fehler | Bedeutung |
|--------|-----------|
| `ErrNotAttached` | Read in `detached`/`lost` |
| `ErrInvalidRead` | Adresse `0`, leerer Buffer, ungültiger Zeiger (OS) |
| `ErrPartialRead` | Weniger Bytes gelesen als angefordert |
| `ErrReadFailed` | Vermutlich transiente OS-Fehler (retryable) |

`ReadAt` hält den Service-Mutex für die gesamte Operation — blockiert `Poll`/`Detach` während eines Reads (bewusst, um Use-after-close zu vermeiden).

## Operator / CLI

Schritt 2 liefert Primitive; Spiel-Semantik, Snapshot-Modell und Probe-Loop: [State Probe](state-probe.md).

```powershell
go run ./cmd/d2rbot
go run ./cmd/d2rbot --probe
```

Erwartung ohne `--probe`: Wait-/Attach-/Lost-Verhalten wie in [Process Detection](process-detection.md); Memory-Snapshots werden im attached-Zustand gelesen und in `world.State` gemappt, aber nicht geloggt.

## Abhängigkeiten

- `process.Service` — Handle und `ReadProcessMemory` (Windows)
- `encoding/binary` — Little-Endian-Dekodierung
- `golang.org/x/sys/windows` — nur in `process_windows.go`

## Grenzen

- Kein vollständiges World Model; State Probe liefert nur Main-Player-Minimalsnapshot (siehe [State Probe](state-probe.md))
- `ModuleBase()` wird für modulrelative Offsets in der State Probe genutzt
- Retry-Backoff fest (2 ms), nicht per Config

## Verwandte Features

- [Process Detection](process-detection.md) — Phase 1 Schritt 1, liefert Handle und Modulbasis
- [State Probe](state-probe.md) — Phase 1 Schritt 3, Main-Player-Minimalsnapshot

---
*Zuletzt aktualisiert: 2026-06-25*
