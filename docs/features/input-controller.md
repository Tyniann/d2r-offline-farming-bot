# Input Controller

## Überblick

Phase 3.1 macht `internal/input` fensterbewusst; Phase 3.2 ergänzt mockbare Tastatur-Primitives mit YAML-konfigurierbarer Belegung; Phase 3.3 ergänzt client-relative Maus-Primitives (`MoveTo`, `Click`); Phase 3.4 ergänzt Safety-Opt-in, globale Pause/Stop-Hotkeys und einheitliches Action-Logging; Phase 3.5 ergänzt einen expliziten CLI-Testmodus zur manuellen Validierung der Input-Primitives im Offline-Spiel. Nach erfolgreichem D2R-Prozess-Attach wird das Hauptfenster per PID und Fenstertitel gefunden; HWND und Client-Geometrie in Screen-Koordinaten werden gespeichert. Der Controller kann einzelne Tasten drücken, halten/loslassen und Kombinationen senden sowie die Maus positionieren und klicken — der normale `Run()`-Loop löst noch keine automatischen Eingaben aus; der Testmodus (`--input-test`) führt konfigurierte Aktionen bewusst einmalig aus.

Ab Phase 7.2 aktiviert `Controller.Focus` das gebundene D2R-Fenster über die Windows-Fokus-APIs und bestätigt es mit `GetForegroundWindow`. Keyboard-sensitive Lifecycle-Flows wiederholen Aktivierung und Prüfung höchstens zehnmal im Abstand von 20 ms und brechen ohne Folgeinput ab, wenn D2R nicht als Foreground bestätigt wird. Falls Windows den ersten `SetForegroundWindow`-Aufruf wegen des Foreground-Locks ablehnt, verbindet der Backend-Aufruf kurzzeitig die beteiligten GUI-Input-Queues und wiederholt die Aktivierung; dabei wird kein Alt-, Maus- oder Tastaturinput synthetisiert.

Echte OS-Eingaben sind standardmäßig deaktiviert (`input.enabled: false`). Globale Hotkeys steuern Pause und Stop unabhängig vom D2R-Fokus. Window Binding, Probe und World-Updates laufen weiter, auch wenn Input deaktiviert oder pausiert ist.

## Ort im Code

- **Paket:** `internal/input/`
- **Einstieg:** [`internal/app/run_tick.go`](../../internal/app/run_tick.go) → `tryBindInput`; [`internal/app/app.go`](../../internal/app/app.go) → Hotkey-Listener im Run-Loop; [`internal/app/input_test_mode.go`](../../internal/app/input_test_mode.go) → `RunInputTest`
- **Wichtige Dateien:**
  - `input.go` — `Controller`, Window Binding (`Bind` / `Unbind` / `Bound` / `Window`)
  - `safety.go` — `SafetyConfig`, `Status`, Pause/Stop-State, Action-Guards und Logging
  - `hotkey.go`, `hotkey_windows.go`, `hotkey_stub.go` — globale Hotkey-Typen und Windows-`RegisterHotKey`-Listener
  - `keyboard.go` — `KeyboardConfig`, `KeySender`, `PressKey`, `PressCombo`
  - `skill_cast.go` — `BindingSource`, `SelectSkill`, `CastSkillAt`, `CastBelt`
  - `keyboard_windows.go` — Windows `SendInput`-Backend und Virtual-Key-Mapping
  - `keyboard_stub.go` — Nicht-Windows-Stub mit `ErrUnsupportedPlatform`
  - `mouse.go` — `MouseButton`, `MouseSender`, `MoveTo`, `Click`, Clamping
  - `mouse_windows.go` — Windows `SetCursorPos` und Mouse-`SendInput`-Backend
  - `mouse_stub.go` — Nicht-Windows-Stub mit `ErrUnsupportedPlatform`
  - `window.go` — `WindowInfo`, `windowAPI`-Interface
  - `window_windows.go` — User32 HWND-Suche und Client-Geometrie
  - `window_stub.go` — Nicht-Windows-Stub
  - `errors.go` — Sentinel-Errors, `IsBindRetryable`
  - `internal/app/input_test_spec.go` — Parser für `--input-test`-Aktionen
  - `internal/app/input_test_mode.go` — `RunInputTest`, Ready-Wait, Observation
- **Config:** `input`-Sektion in YAML (Safety, Delays, Skill-/Portal-/Belt-Hotkeys)

## Funktionalität

### Window Binding (Phase 3.1)

- Nach `process attached` ruft der App-Loop `Input.Bind(pid)` auf.
- Windows: `EnumWindows` durchsucht sichtbare Top-Level-Fenster mit passender PID, Titel und ohne Owner-Fenster.
- Client-Geometrie via `GetClientRect` + `ClientToScreen` (Screen-Koordinaten der spielbaren Fläche ohne Fensterrahmen).
- Erfolg: Log `input window bound` mit PID, Titel, HWND und Client-Maßen.

### Keyboard Primitives (Phase 3.2)

- **Low-Level:** `KeyDown`, `KeyUp`, `PressKey`, `PressCombo` — serialisiert über `keyMu`.
- **Skill-Cast:** `SelectSkill`, `CastSkillAt`, `CastBelt` — Hotkeys aus YAML-Config über `BindingSource`.
- `PressKey`: Down → zufälliger Delay (`key_delay_ms_min`–`key_delay_ms_max`, Default 10–40 ms) → Up.
- `PressCombo`: Down in Reihenfolge → Hold (`combo_hold_ms`, Default 200 ms) → Up in umgekehrter Reihenfolge.
- Erfolgreiche Aktionen: strukturiertes Log `input action` mit `kind`, `action`, `reason`, `allowed=true`.
- Windows-Backend: `SendInput` über User32/LazyDLL, ohne CGO.

**Unterstützte Keys:** `0`–`9`, `a`–`z`, `f1`–`f12`, `shift`/`ctrl`/`alt` (linke VKs), `esc`, `enter`, `home`, `down`, `space`, `tab`, `pause`, `,`, `.`, `-`, `]`. Aliase wie `control` oder `lctrl` sind ungültig.

### Mouse Primitives (Phase 3.3)

- **Low-Level:** `MoveTo(clientX, clientY)`, `Click(left|right)` — serialisiert über `mouseMu`.
- Koordinaten sind **client-relativ**: `(0,0)` = obere linke Ecke des D2R-Clientbereichs aus `WindowInfo`.
- `MoveTo` klemmt auf eine sichere Client-Fläche (Default-Rand 10 px), konvertiert dann zu Screen-Koordinaten (`ClientLeft/Top + geklemmte Werte`).
- `Click` sendet Button Down/Up an der **aktuellen** Cursorposition; bewegt die Maus nicht.
- `ClickWithModifier` hält für genau einen Maus-Down/Up-Zyklus einen Modifier wie `ctrl` und gibt ihn auf jedem Fehlerpfad best-effort wieder frei. Phase 5.8 nutzt dies für atomare Personal-Stash-Transfers.
- Beide Methoden verlangen ein gebundenes Fenster (`ErrWindowNotBound` ohne `Bind`).
- Erfolgreiche Aktionen: Log `input action` mit `kind=mouse`, `action=move|click`, `allowed=true`.
- Windows-Backend: `SetCursorPos` + `SendInput` über User32/LazyDLL, ohne CGO. Separate `mouseInputRecord`-Structs (nicht Keyboard-`inputRecord` wiederverwenden).

### Safety & Logging (Phase 3.4)

- **`input.enabled`:** Default `false`. Echte Tastatur-/Mausaktionen werden abgelehnt (`ErrInputDisabled`), bis in der lokalen YAML explizit `enabled: true` gesetzt wird.
- **Pause/Stop-State:** `Pause`, `Resume`, `TogglePause`, `Stop` am Controller; `Status()` liefert `Enabled`, `Paused`, `Stopped`.
- **Action-Guards:** Alle Sender-Methoden prüfen vor OS-Aufrufen: `stopped` → `ErrInputStopped`, `!enabled` → `ErrInputDisabled`, `paused` → `ErrInputPaused`. Argumentvalidierung und `ErrWindowNotBound` haben Vorrang.
- **Action-Logging:** Einheitliches `input action`-Log mit `allowed=true|false`; bei Blockierung zusätzlich `blocked_by` (`disabled|paused|stopped`).
- **Globale Hotkeys:** `pause_hotkey` (Default `pause`) toggelt Pause; `stop_hotkey` (Default `f12`) stoppt Input und beendet den Bot. Hotkeys funktionieren unabhängig vom D2R-Fokus, sind aber Windows-global und können mit anderer Software kollidieren — bei Registrierungsfehler bricht der Start hart ab (`ErrHotkeyUnavailable`). Registrierung, Message-Polling und Deregistrierung bleiben wegen der Windows-Threadbindung auf demselben OS-Thread; aufeinanderfolgende Session-Phasen warten synchron auf die Freigabe. Alternative Stop-Taste z. B. `stop_hotkey: f11`, falls `f12` belegt ist.
- **Signal-Shutdown:** `SIGINT`/`SIGTERM` ruft `Stop("signal")` auf, analog zum Stop-Hotkey.
- **Cleanup-Ausnahme:** Best-effort Key-/Button-Release nach begonnenem `PressCombo`/`Click` umgeht den Guard, damit keine Taste hängen bleibt.

Startup-Log: `input safety configured enabled=… pause_hotkey=… stop_hotkey=…`.

### Manual Input Test (Phase 3.5)

Expliziter CLI-Testmodus (`--input-test`) zur Validierung der Phase-3-Primitives im Offline-Spiel. Der normale passive `Run()`-Loop bleibt unverändert.

**Voraussetzungen:**

- `input.enabled: true` in der lokalen `config.yaml` (bewusstes Opt-in)
- D2R offline mit geladenem Charakter (gültiger In-Game-World-State, nicht Menü/Loading)
- D2R-Fenster sollte im Fokus sein (kein Fokus-Management in 3.5)

**CLI:**

```powershell
.\d2rbot.exe --config configs\config.yaml --input-test "belt:1"
.\d2rbot.exe --config configs\config.yaml --input-test "potion:1"
.\d2rbot.exe --config configs\config.yaml --input-test "portal"
.\d2rbot.exe --config configs\config.yaml --input-test "skill:teleport"
.\d2rbot.exe --config configs\config.yaml --input-test "skill:teleport,click:640,360"
.\d2rbot.exe --config configs\config.yaml --input-test "belt:1" --input-test-observe-ms 3000
```

**Aktionen:**

| Spec | Verhalten |
|------|-----------|
| `belt:N` / `potion:N` | `CastBelt(N)` aus YAML, N = 1..4 |
| `portal` | Town-Portal-Skill-Hotkey aus YAML (`SelectSkill`) |
| `skill:teleport` / `skill:town_portal` | `SelectSkill` per Skill-ID; folgender `click` nutzt LMB/RMB der Leiste |
| `center-click` | `MoveTo(width/2, height/2)` + `Click(left)` |
| `click:X,Y` | `MoveTo(X,Y)` + `Click(left)`, client-relativ |

Komma trennt kurze Sequenzen. Bei `click:X,Y` in Sequenzen wird die Koordinate intern zusammengehalten (`belt:1,click:10,20,portal`).

**Skill-Actions und Bindings:** Normale Runs, Pathing und `--input-test` nutzen ausschließlich `input.bindings` aus YAML. Es gibt keinen Memory-Fallback und keine Hotkey-Kalibrierung.

**Ablauf:**

1. Parse Spec, prüfe `input.enabled`
2. Hotkeys und Signal-Handler wie in `Run()`
3. Ready-Wait: Prozess attached, Fenster gebunden, `World.Current().Valid == true` (Timeout: `process.attach_timeout_ms` oder 60 s)
4. Log `input test ready` mit World-State vor der Aktion
5. Aktionen in Reihenfolge; zwischen Actions Stop/Pause-Check
6. Observation für `--input-test-observe-ms` (Default 3000 ms) mit `input test observation`-Deltas
7. Sauberes Cleanup (`Unbind`, `Detach`)

**Hotkeys während des Tests:** Pause blockiert Aktionen (`ErrInputPaused`); Stop beendet den Test jederzeit sauber. Während einer einzelnen synchronen Action greift Stop erst nach Rückkehr (akzeptiert in 3.5).

**Logging:** Dedizierte `input test`-Logs unabhängig von `--probe`. `--verbose` bleibt optional für zusätzliche Debug-Ausgaben.

**Grenzen:** Kein interaktiver Prompt, keine Wiederholungsschleife, keine UI-/Waypoint-Klicks, keine automatische Kampf-/Pathing-Logik. World-State-Verifikation ist beobachtend, nicht beweisend (Portal/Skill ändern HP/Mana nicht zwingend).

### Weiches Retry

- Fehlendes oder noch nicht messbares Fenster bricht `--probe` und den Snapshot-Loop **nicht** ab.
- `ErrWindowNotFound` und `ErrInvalidClientArea` sind retry-fähig; Bind wird maximal einmal pro Sekunde erneut versucht.
- Gedrosseltes Log `waiting for input window` (Heartbeat 5 s).
- Harte Fehler (`ErrInvalidPID`, `ErrUnsupportedPlatform`) beenden den Run-Loop.

### Lifecycle

| Ereignis | Aktion |
|----------|--------|
| App-Start | Hotkey-Listener registrieren (unabhängig von `input.enabled`) |
| Prozess-Attach | `Bind(pid)` (soft retry) |
| Attached, noch nicht bound | erneutes `Bind` vor jedem Snapshot (gedrosselt) |
| Pause-Hotkey | `TogglePause("hotkey")` |
| Stop-Hotkey / Signal | `Stop(…)`, Context-Cancel, sauberes Shutdown |
| Prozess lost | `Unbind()` **vor** World-Reset |
| Shutdown | idempotentes `Unbind()` vor `Process.Detach()` |

## Datenmodell

```go
type WindowInfo struct {
    PID          uint32
    Title        string
    Handle       uintptr // HWND
    ClientLeft   int
    ClientTop    int
    ClientWidth  int
    ClientHeight int
}

type KeyboardConfig struct {
    KeyDelayMsMin int
    KeyDelayMsMax int
    ComboHoldMs   int
}

type SafetyConfig struct {
    Enabled     bool
    PauseHotkey string
    StopHotkey  string
}

type Status struct {
    Enabled bool
    Paused  bool
    Stopped bool
}

type MouseButton string // "left" | "right"
```

- `Ready()` = Controller initialisiert; `Bound()` = Fenster gebunden — getrennte Zustände. `Ready()` bedeutet **nicht**, dass echte Eingaben erlaubt sind (`enabled` separat prüfen).
- `Window()` liefert eine Kopie, analog zu `process.Status()` und `world.Current()`.
- `SelectSkill` / `CastSkillAt` / `CastBelt` nutzen `input.bindings` aus YAML.
- `MoveTo`/`Click` verlangen `Bound()`; Mausfehler als `ErrWindowNotBound`, `ErrInvalidMouseButton`, `ErrMouseSendFailed`.

### YAML-Config (`input`)

```yaml
input:
  enabled: false
  pause_hotkey: pause
  stop_hotkey: f12
  key_delay_ms_min: 10
  key_delay_ms_max: 40
  combo_hold_ms: 200
  bindings:
    skills:
      teleport:
        key: f7
        button: right
      town_portal:
        key: f6
        button: right
      bone_spear:
        key: f8
        button: right
    belt:
      slot_1: ","
      slot_2: "."
      slot_3: "-"
      slot_4: "]"
```

Skill- und Belt-Hotkeys müssen zu den D2R-Optionen passen. `button` ist `left` oder `right` und bestimmt, welchen Mausbutton ein folgender `click` für diese Skill-Auswahl verwendet.

Fehlt die gesamte `input`-Sektion, werden sichere Defaults angewendet (`enabled=false`, Hotkeys `pause`/`f12`, Timing-Defaults).

## Operator / CLI

```powershell
.\d2rbot.exe --config configs\config.yaml --probe --verbose
```

Erwartung nach Attach:

- `input safety configured` mit `enabled=false` (oder `true` nach explizitem Opt-in)
- `input window bound` mit plausibler Client-Größe, oder
- gedrosseltes `waiting for input window`, während Snapshots weiterlaufen.

Manuelle Validierung Phase 3.5 (Release-Kriterium Phase 3):

```powershell
.\d2rbot.exe --config configs\config.yaml --input-test "belt:1"
.\d2rbot.exe --config configs\config.yaml --input-test "portal"
.\d2rbot.exe --config configs\config.yaml --input-test "skill:teleport"
.\d2rbot.exe --config configs\config.yaml --input-test "center-click"
```

Erwartung: Fenster gebunden, Aktionen in `input action`-Logs sichtbar, `input test ready` / `input test observation` mit World-State, Pause blockiert, Stop beendet sauber.

## Grenzen (Phase 3.5)

- **Automatische Nutzung nur in aktiven Runs:** Der passive Modus sendet keine Eingaben; konfigurierte Run-Phasen verwenden die Primitives hinter World-, Safety- und UI-Guards.
- **Fokus nur für Lifecycle-Flows:** Phase-7-Menüaktionen aktivieren und bestätigen D2R explizit; ältere allgemeine Run-/Test-Primitives besitzen nicht automatisch denselben semantischen Fokus-Guard.
- **Input-Test sendet echte Eingaben:** nur mit explizitem `--input-test` und `input.enabled: true`.
- **Keine Pathing-/UI-Klicks:** nur Low-Level-Primitives, kein semantisches D2R-UI-Modell.
- **Globale Hotkeys:** können von anderer Software belegt sein; Start schlägt dann fehl statt ohne Safety zu laufen.
- **Statische Geometrie:** Client-Rect wird nur bei `Bind` aktualisiert; Move/Resize erfordert später `Refresh()` oder Re-Bind.
- **Teleport-Bewegung:** `SetCursorPos` setzt den Cursor sofort; keine schrittweise Bewegung in 3.3.
- **DPI/Multi-Monitor:** Screen-Koordinaten hängen von `ClientToScreen` und der DPI-Awareness des Prozesses ab; kein eigener DPI-Fix in 3.3.
- **Rand-Clamp only:** 10-px-Margin vermeidet Fensterränder, keine HUD-Erkennung.
- **Fenstertitel:** hardcoded englischer Titel; Lokalisierung/Config später.
- **Globaler OS-Input:** `SendInput`/`SetCursorPos` sind nicht an `Bound()` gekoppelt; `MoveTo`/`Click` prüfen Binding, Tasks prüfen Fokus in späteren Phasen.

## Abhängigkeiten

- Windows User32 (`EnumWindows`, `GetClientRect`, `ClientToScreen`, `SetCursorPos`, `SendInput`, `RegisterHotKey`, `PeekMessageW`, …)
- `golang.org/x/sys/windows` (LazyDLL für User32-Procs)
- `github.com/kbinani/screenshot` für read-only Client-Screenshots der Phase-7-Frontend-Anker
- App-Loop in `internal/app` für Lifecycle-Wiring und Config-Mapping

## Verwandte Features

- [Process Detection](process-detection.md) — liefert PID für Bind
- [State Probe](state-probe.md) — läuft parallel weiter, auch ohne erfolgreiches Bind

---
*Zuletzt aktualisiert: 2026-06-26*
