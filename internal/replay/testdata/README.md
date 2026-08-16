# Runtime-Replay-Fixtures

`mephisto-live-hard-stuck.trace.gz` stammt aus einem echten, explizit aufgezeichneten Offline-D2R-Mephisto-Lauf vom 14. August 2026. Der produktive Lauf und das Headless-Replay endeten identisch mit:

```text
ticks=328 step=play_bound_route outcome=failed reason=hard_stuck
```

Für das permanente Fixture wurden lokale Charakter- und Routenidentitäten ersetzt. Checkpoints, Verträge, World-Entities und Runtime-Felder wurden nur entfernt, wenn der unveränderte produktive `tasks.Runner` danach weiterhin denselben Dependency-, Intent- und Terminalverlauf replayte. Dadurch schrumpfte das Diagnosebundle von rund 1,8 MiB auf weniger als 32 KiB.

Das Fixture enthält ausschließlich interpretierte Daten und keine Prozesspointer, Raw-Memory-, Savegame- oder lokale Pfaddaten.
