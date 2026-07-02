# Feature-Dokumentation

Übersicht dokumentierter Bot-Features. Architektur-Gesamtbild: [`handoff.html`](../../handoff.html).  
Spätere Ideen (noch nicht umgesetzt): [`docs/backlog.md`](../backlog.md).

| Feature | Beschreibung |
|---------|--------------|
| [Process Detection](process-detection.md) | Read-only D2R-Prozessbindung, Modulbasis, Lifecycle, Attach-Timeout |
| [Memory Reader](memory-reader.md) | Low-Level Byte-Reads, Primitive-Dekodierung, Pointer-Ketten |
| [State Probe](state-probe.md) | Memory-Reads, World-Update im App-Loop; `--probe` für semantisches World-State-Logging |
| [World Model](world-model.md) | Domain-Typen (Area, Player, State), eingebetteter Area-Katalog; kontinuierliches Update im App-Loop (2.3); Validierung Phase 2.4 |
| [Input Controller](input-controller.md) | D2R-Fensterbindung per PID, Client-Geometrie (3.1); Tastatur-/Maus-Primitives; YAML-Bindings für Skills, Portal und Belt; Safety-Opt-in, globale Pause/Stop-Hotkeys; manueller CLI-Input-Testmodus |
| [Task Runner](task-runner.md) | Task-Framework, Lazy Run-Start, Countess-Stub (Phase 4.1); `--run countess` / `runs.active` |
| [Pathing](pathing.md) | Teleport-Navigation (Phase 4.3): Relative-Projektion + Hover-Feedback-Loop, Bearing-Explore, Stuck-Detection; `--pathing-test` |

## Geplante Dokumentation (MVP-Phasen)

| Phase | Vorgeschlagene Datei |
|-------|----------------------|
| Countess-Run (Navigation) | `countess-run.md` |
