# Feature-Dokumentation

Übersicht dokumentierter Bot-Features. Architektur-Gesamtbild: [`handoff.html`](../../handoff.html).

| Feature | Beschreibung |
|---------|--------------|
| [Process Detection](process-detection.md) | Read-only D2R-Prozessbindung, Modulbasis, Lifecycle, Attach-Timeout |
| [Memory Reader](memory-reader.md) | Low-Level Byte-Reads, Primitive-Dekodierung, Pointer-Ketten |
| [State Probe](state-probe.md) | Memory-Reads, World-Update im App-Loop; `--probe` für semantisches World-State-Logging |
| [World Model](world-model.md) | Domain-Typen (Area, Player, State), eingebetteter Area-Katalog; kontinuierliches Update im App-Loop (2.3); Validierung Phase 2.4 |
| [Input Controller](input-controller.md) | D2R-Fensterbindung per PID, Client-Geometrie (3.1); Tastatur-Primitives und YAML-Key-Mapping (3.2); client-relative Maus-Primitives (3.3); Safety-Opt-in, globale Pause/Stop-Hotkeys und Action-Logging (3.4); manueller CLI-Input-Testmodus (3.5) |

## Geplante Dokumentation (MVP-Phasen)

| Phase | Vorgeschlagene Datei |
|-------|----------------------|
| Countess-Run | `countess-run.md` |
