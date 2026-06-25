# Memory Reader

## Überblick

Low-Level Read-only Memory Reader für Phase 1 Schritt 2: rohe Bytes aus dem angebundenen D2R-Prozess lesen, `uint32`/`uint64` little-endian decodieren und Pointer-Ketten auflösen. Keine Spiel-Semantik (HP, Mana, Area), keine Offsets und kein Snapshot.

## Ort im Code

- **Paket:** `internal/memory/`
- **Prozess-Zugriff:** `internal/process/` → [`Service.ReadAt`](../../internal/process/process.go)
- **Einstieg:** [`internal/app/app.go`](../../internal/app/app.go) → `Memory.Bind(Process)` beim Start
- **Wichtige Dateien:**
  - `memory.go` — `Reader`, `ProcessAccess`, Primitive, Pointer-Ketten
  - `process.go` / `process_windows.go` — `ReadAt` und `ReadProcessMemory`
- **Config:** keine eigenen Keys in Schritt 2

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
| `ReadUint32(addr)` | 4 Bytes little-endian |
| `ReadUint64(addr)` | 8 Bytes little-endian |

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

Schritt 2 ändert das CLI-Verhalten nicht: `app.Run()` führt weiterhin nur Prozess-Attach/Poll aus, keine aktiven Memory-Reads im Loop.

```powershell
go run ./cmd/d2rbot
```

Erwartung: unverändertes Wait-/Attach-/Lost-Verhalten wie in [Process Detection](process-detection.md).

## Abhängigkeiten

- `process.Service` — Handle und `ReadProcessMemory` (Windows)
- `encoding/binary` — Little-Endian-Dekodierung
- `golang.org/x/sys/windows` — nur in `process_windows.go`

## Grenzen

- Keine Game-State-Semantik, keine D2R-Offsets
- Kein Snapshot, keine World-Model-Integration
- `ModuleBase()` im Interface für Schritt 3 vorbereitet, in Schritt 2 noch nicht aktiv genutzt
- Retry-Backoff fest (2 ms), nicht per Config

## Verwandte Features

- [Process Detection](process-detection.md) — Phase 1 Schritt 1, liefert Handle und Modulbasis
- Geplant: World Model / Snapshot (Phase 1 Schritt 3)

---
*Zuletzt aktualisiert: 2026-06-25*
