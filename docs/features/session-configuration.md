# Session-Konfiguration und Inspect

## Überblick

Phase 7.5 ergänzt den sicheren Operator-Vertrag für autonome Offline-Sessions. Das YAML enthält eine explizite Aktivierung, Character-/Run-/Difficulty-Auswahl, endliche Lauf- und Zeitbudgets sowie begrenzte Fehler-Restarts. `--session-inspect` löst den Plan read-only auf, bevor eine Runtime erzeugt, D2R attached, ein Hotkey registriert oder Input gesendet wird.

Phase 7.5 führt noch keine produktive Session aus. `session.enabled: true` ohne `--session-inspect` wird ausdrücklich abgewiesen, bis die zyklische Game-/Run-Verifikation in den folgenden Phasen verdrahtet ist.

## Ort im Code

- **Config:** `internal/config/session.go`
- **Planauflösung:** `internal/app/session_plan.go`
- **CLI:** `cmd/d2rbot/main.go`
- **Beispiel:** `configs/config.example.yaml`

## Konfiguration

```yaml
session:
  enabled: false
  run: countess
  character: MrBones
  difficulty: nightmare
  max_runs: 3
  max_duration_ms: 7200000
  cooldown_ms: 3000
  max_consecutive_failures: 2
  max_total_restarts: 3
  state_timeout_ms: 30000
  exit_timeout_ms: 30000
  start_timeout_ms: 45000
  retry_classes:
    - hard_stuck
    - route_drift_exceeded
    - route_segment_timeout
    - route_transition_failed
```

`max_runs`, `max_duration_ms` und alle State-Timeouts müssen positiv sein. Es gibt keinen Wert für unbegrenzten Betrieb. `cooldown_ms`, `max_consecutive_failures` und `max_total_restarts` dürfen explizit `0` sein; diese restriktiven Nullwerte werden nicht durch Defaults ersetzt.

Retry-Klassen werden exakt validiert. Unbekannte oder doppelte Einträge sind ungültig; Präfix- und Substring-Matching sind ausgeschlossen.

## Inspect-Preflight

```powershell
go run ./cmd/d2rbot --config configs/config.yaml --session-inspect
```

Bei deaktivierter Session zeigt JSON `status: disabled` und die endlichen Defaults. Bei `enabled: true` wird zusätzlich statisch geprüft:

- `input.enabled: true` und leeres `runs.active`;
- registrierter Full Run ohne Diagnosephase;
- gültiger Character-Name und vorhandene Character-/Play-/Dialog-Anker;
- vorhandene, eindeutige Route-ID;
- Character, Difficulty und Game-Version stimmen mit dem Route Contract überein;
- Telemetriepfad und feste 1280×720-Clientanforderung sind aufgelöst.

Ein erfolgreicher Preflight liefert `status: ready`, Route-Pfad und erwarteten Layout-Fingerprint. Widersprüche enden mit einem Fehler, ohne D2R-Prozesszugriff oder Input. `--session-inspect` ist gegenseitig exklusiv zu Probe-, Run-, Route- und Testmodi.

## Abhängigkeiten und Grenzen

- Die Planauflösung verwendet die vorhandene Route Registry und Run Registry read-only.
- Ein `ready`-Plan ist noch keine Input-Autorisierung und startet keine Session.
- Phase 7.6 verifiziert Identity, Version und Rogue Encampment über drei frische Snapshots pro Game; der Layout-Fingerprint wird am Black-Marsh-Routenstart erneut aufgebaut.
- Recovery-Klassifikation und korrelierte Session-Telemetrie folgen in Phase 7.7.

## Abnahme

Die Phase-7.5-Abnahme umfasst drei Operatorfälle: einen deaktivierten Plan mit endlichen Defaults, einen vollständig aufgelösten `MrBones`-/Nightmare-Plan gegen die vorhandene Route sowie einen absichtlichen Difficulty-/Route-Widerspruch. Alle Fälle laufen vor Runtime/Attach/Input.

## Verwandte Features

- [Session-Lifecycle](session-lifecycle.md)
- [Verifizierter Offline-Game-Start](offline-difficulty-selection.md)
- [Route Recording und Playback](route-recording-playback.md)

---
*Zuletzt aktualisiert: 2026-07-11*
