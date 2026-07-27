# Process Detection

## Überblick

Read-only Bindung an den lokalen `D2R.exe`-Prozess: Prozess finden, Read-Handle öffnen, Handle-PID, kanonischen Image-Pfad, Windows-Dateiversion und Modul-Basisadresse ermitteln sowie den Lifecycle (attached / lost / detached) verfolgen. Kein Memory-Snapshot und keine Spielsteuerung.

## Ort im Code

- **Paket:** `internal/process/`
- **Einstieg:** [`internal/app/app.go`](../../internal/app/app.go) → `Runtime.Run()`
- **Wichtige Dateien:**
  - `process.go` — Service-API und State-Machine
  - `process_windows.go` — Windows-API-Adapter
  - `errors.go` — retryable/fatal Attach-Fehler
- **Config:** `configs/config.example.yaml` → `process.process_name`, `process.attach_timeout_ms`, `runtime.poll_interval_ms`

## Funktionalität

### Prozess finden

- Enumeriert Prozesse per `CreateToolhelp32Snapshot` / `Process32First` / `Process32Next`
- Vergleicht `process_name` case-insensitive (Default: `D2R.exe`)
- Bei mehreren Treffern: Fehler `multiple instances` (kein zufälliger Pick)

### Handle und Modulbasis

- `OpenProcess` mit `PROCESS_QUERY_INFORMATION | PROCESS_VM_READ`
- Handle-PID via `GetProcessId`; Abweichung zur gefundenen PID verwirft das Handle
- kanonischer Image-Pfad via `QueryFullProcessImageName`; der Dateiname muss weiterhin `D2R.exe` sein
- Windows-Product-/FileVersion aus derselben gebundenen Datei; Pfad bleibt Core-intern
- Modulbasis via `TH32CS_SNAPMODULE` (64-bit), Modulname case-insensitive
- Alive-Check: `GetExitCodeProcess` + `STILL_ACTIVE`

### Lifecycle

| State | Bedeutung |
|-------|-----------|
| `detached` | Kein Handle |
| `attached` | Handle offen, Prozess lebt |
| `lost` | Prozess beendet, Handle geschlossen |

- `Attach()` — erlaubt aus `detached` und `lost`; Fehler wenn bereits `attached`
- `Poll()` — prüft Alive-Status, wechselt zu `lost` bei Exit; danach liest `app` optional die State Probe (nur mit `--probe`)
- `Status()` — read-only Snapshot, keine Seiteneffekte
- `Detach()` — idempotent, schließt Handle

### App-Loop (`app.Run`)

- Wait-Loop: wenn D2R noch nicht läuft, wird im `poll_interval_ms`-Takt erneut `Attach()` versucht
- `attach_timeout_ms > 0`: begrenzt nur die **erste** Wartezeit bis zum ersten erfolgreichen Attach; `0` = unbegrenzt warten
- Nach Timeout: Fehler `attach timeout after …ms` und sauberer Exit (kein endloses Warten)
- Nach `lost`: Re-Attach ohne erneuten Start-Timeout
- Ctrl+C → Context-Cancel → `Detach()`
- Logging: Service auf Debug, operator-relevante State-Wechsel (`waiting`, `attached`, `lost`) auf Info/Error in `app`

### Attach-Fehler

| Fehler | Retry |
|--------|-------|
| Prozess nicht gefunden | ja |
| Mehrere Instanzen | nein (einmalig Error-Log) |
| Access denied (UAC) | nein |
| Modulbasis nicht lesbar | nein |

## Datenmodell

```yaml
process:
  process_name: D2R.exe
  attach_timeout_ms: 30000  # 0 = unbegrenzt
```

```go
type Status struct {
    State      State   // detached | attached | lost
    PID        uint32
    Process    string
    ModuleBase uintptr
    ModuleSize uint32
    ImagePath string       // ausschließlich Core-intern
    FileVersion string
    VersionError string
    PrivilegeMismatch bool
    LastError  string
}
```

## Operator / CLI

```powershell
go run ./cmd/d2rbot
go run ./cmd/d2rbot --probe
```

- Bot kann **vor** D2R gestartet werden → wartet mit `waiting for target process` (oder bricht nach `attach_timeout_ms` ab)
- Nach Erfolg: `process attached` mit PID und `module_base`
- Mit `--probe` im Spiel (attached): sparsame `world state`-Logs — siehe [State Probe](state-probe.md)
- Ohne `--probe`: Prozess-Lifecycle-Logs; Memory-Snapshots und World-Update laufen intern ohne Operator-Log (Default)
- D2R schließen → einmalig `process lost`, danach wieder warten
- D2R neu starten → automatischer Re-Attach
- Im Desktopprodukt wird ein Neustart als Administrator ausschließlich angeboten, wenn `OpenProcess` nach Prozessfund tatsächlich `access denied` meldet.

## Abhängigkeiten

- `golang.org/x/sys/windows` — Prozess-/Modul-Enumeration, Handles
- Windows 64-bit (Bot und D2R)

## Grenzen

- Memory-Snapshots und World-Update laufen im App-Loop nach jedem attached `Poll()`; `--probe` steuert nur Operator-Logging (siehe [State Probe](state-probe.md))
- Re-Attach nur im App-Loop, nicht in `Poll()`
- Zweite `D2R.exe` während bestehender Bindung wird nicht erkannt; Mehrfach-Prüfung greift erst beim nächsten Attach
- `memory.Reader` erhält Zugriff über `process.Service` via `ProcessAccess`-Interface — kein exportiertes `windows.Handle` (siehe [Memory Reader](memory-reader.md))

## Verwandte Features

- [Memory Reader](memory-reader.md) — Phase 1 Schritt 2, Primitive und Pointer-Ketten
- [State Probe](state-probe.md) — Phase 1 Schritt 3, Main-Player-Minimalsnapshot

---
*Zuletzt aktualisiert: 22. Juli 2026*
