# Tatsächliches D2R-Versionsgate

## Überblick

Das Versionsgate liest die Windows-Product-/FileVersion der tatsächlich gebundenen `D2R.exe` read-only und erlaubt Input ausschließlich, wenn Core-Buildvertrag, konfigurierte Erwartung, aktiver Offsetstand und erkannte Datei exakt übereinstimmen. Es gibt keinen Override.

## Ort im Code

- **Prozessbindung:** `internal/process/process.go`, `internal/process/process_windows.go`
- **Compatibility-Matrix:** `internal/app/d2r_compatibility.go`
- **Input-Gates:** `internal/app/run_tick.go`, `internal/app/app.go`
- **API-Projektion und Locks:** `internal/api/dto.go`, `internal/api/live_backend.go`, `internal/api/live_backend_routes.go`
- **Desktop-Neustart:** `web/electron/main.ts`, `web/electron/preload.cts`
- **Maschinenvertrag:** `internal/api/schema/openapi.json`

## Prozess- und Dateibindung

Nach dem Prozessfund öffnet der Windows-Adapter ein Read-Handle und prüft, dass dessen PID noch der gefundenen PID entspricht. Der kanonische Image-Pfad wird vom selben Handle gelesen; sein Dateiname muss case-insensitive `D2R.exe` entsprechen. Erst danach werden Modulbild und Versionsresource gelesen.

Die Versionsresource wird bevorzugt aus `ProductVersion`, danach aus `FileVersion` und zuletzt aus `VS_FIXEDFILEINFO` bezogen. Windows-Darstellungen wie `3, 2, 92777, 0` werden kanonisch als `3.2.92777` verglichen. Der absolute Image-Pfad bleibt ausschließlich im Core und erscheint weder in Status noch SSE.

Eine fehlende oder kaputte Versionsresource lässt nur die read-only Prozessbindung bestehen. PID- oder Pfaddrift verwirft das Handle vollständig.

## Compatibility-Matrix

| Zustand | Bedingung | Reason-Code |
|---|---|---|
| `not_detected` | Keine lebende gebundene D2R-Instanz | `d2r_version_not_detected` |
| `compatible` | Supported, Expected, Offset und Actual sind nicht leer und exakt gleich | – |
| `incompatible` | Erwartungs-/Offset-/Builddrift oder abweichende tatsächliche Version | `offset_version_mismatch` / `d2r_version_unsupported` |
| `unreadable` | Versionsresource, PID oder Pfad nicht sicher lesbar | `d2r_version_unreadable` |
| `unreadable` | `OpenProcess` scheitert nach Prozessfund nachweislich mit Access Denied | `privilege_mismatch` |

`supported_version` stammt aus dem eingebetteten Core-Offsetvertrag. `expected_version` stammt aus der produktiven Konfiguration und fällt nur bei leerem Wert auf den geladenen Offsetvertrag zurück. `offset_version` gehört zum tatsächlich geladenen Offsetset; `actual_version` zur gebundenen Datei.

Detach, Prozessverlust und Reattach werten die Matrix neu aus. Eine zuvor kompatible PID autorisiert keine spätere Instanz.

## Input- und API-Gate

Vor `compatible` werden nicht ausgeführt:

- globale Gameplay-Hotkeys,
- D2R-Fensterbindung oder Fokus,
- Gameplay-, Maus- oder Tastaturinput,
- Task-Ticks und Supervisor-Workerstarts,
- Auswahl-Apply und Session-Start/Resume,
- Routenaufnahme, Kandidatentest, Finish oder Publish/Mutation.

Read-only Prozessdiagnose, Status, Routenbibliothek, Historie und Einstellungen bleiben erreichbar. Pause, Stop-after-run und Emergency Stop bleiben als reine Safety-Intents für einen bereits laufenden Supervisor zulässig.

Das API-Statusobjekt enthält `app_version`, `core_version` und die vollständige pfadfreie `compatibility`-Projektion. Änderungen werden als begrenztes SSE-Ereignis `compatibility_changed` publiziert.

## Administrator-Neustart

Nur wenn der Core `privilege_mismatch` nachgewiesen hat, akzeptiert Electron den argumentlosen Preload-Intent `restartAsAdministrator()`. Main validiert den IPC-Sender und den letzten Core-Status, beendet den Core, gibt den Single-Instance-Lock frei und startet dieselbe App mit demselben Datenroot über den Windows-`runas`-Dialog neu. Bei Abbruch wird der normale Core wieder gestartet. Es gibt keinen frei parametrisierbaren Prozess- oder Shell-IPC.

## Tests

Die automatisierte Matrix deckt Match, Actual-Mismatch, fehlende/kaputte Resource, Privilegienproblem, PID-/Pfaddrift, Detach/Reattach sowie Expected-/Offsetdrift ab. Laufzeittests beweisen, dass blockierte Zustände weder Hotkey-Listener noch Window-Bind, Fokus oder Gameplay-Aufrufe erreichen. API-Tests prüfen stabile Reason-Codes und SSE; Electron prüft, dass der Administrator-Intent ohne nachgewiesenen Konflikt abgelehnt wird.

## Verwandte Features

- [Process Detection](process-detection.md)
- [Input Controller](input-controller.md)
- [Lokale Core-API und eingebettete Web-Anwendung](local-core-api.md)
- [Sichere Electron-Shell und Core-Kindprozess](desktop-shell.md)

---
*Zuletzt aktualisiert: 22. Juli 2026*

