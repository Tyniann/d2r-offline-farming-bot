# Verifiziertes Offline Save & Exit

## Überblick

Phase 7.2 ergänzt einen isolierten CLI-Test für genau einen kontrollierten Offline-Game-Austritt. Der Flow startet ausschließlich im stabil bestätigten Rogue Encampment, öffnet das Quit-Menü mit genau einem `Esc`, wartet auf drei Memory-bestätigte `QuitMenuOpen`-Ticks, klickt Save & Exit bei exakt 1280×720 genau einmal und bestätigt danach drei stabile `menu`-Ticks ohne lesbaren In-Game-Player.

Der Slice ist noch keine Multi-Run-Schleife. Er beweist den später benötigten Game-Exit unabhängig vom Session-Orchestrator.

## Ort im Code

- **Paket:** `internal/app/`
- **Einstieg:** `(*Runtime).RunOfflineExitTest`
- **Wichtige Dateien:** `offline_exit.go`, `offline_exit_test.go`
- **Memory-Gate:** `world.UIState.QuitMenuOpen` aus Phase 7.1
- **CLI:** `--offline-exit-test`

## Funktionalität

```text
await_safe_town
  -- Esc --> await_quit_menu
  -- MoveTo + LMB --> await_completion
  --> complete
```

### `await_safe_town`

Vor `Esc` verlangt die State-Machine drei aufeinanderfolgende Ticks mit:

- `Valid=true` und `Phase=in_game`;
- `Area=Rogue Encampment`;
- bestätigter Character Identity;
- geschlossenem Inventory und Stash;
- `QuitMenuOpen=false`.

Loading, `unknown` und vorübergehend ungültige In-Game-Snapshots setzen den Stabilitätszähler zurück. Ein bestätigter Menüscreen, eine falsche Area, offene kritische UI oder ein bereits geöffnetes Quit-Menü endet fail-closed vor Input.

### `await_quit_menu`

Nach genau einem `Esc` wartet der Flow maximal fünf Sekunden auf drei stabile `QuitMenuOpen=true`-Ticks. Area, In-Game-Phase und geschlossene Inventory-/Stash-UI müssen erhalten bleiben. Ohne Bestätigung erfolgt kein Save-&-Exit-Klick und kein zweites `Esc`.

### `await_completion`

Nach Quit-Bestätigung wird die Maus client-relativ zu `(640,327)` bewegt und genau ein Linksklick ausgeführt. Die Koordinate entspricht Koolos 1280×720-Referenz und wird in der isolierten Live-Abnahme validiert.

Erfolg benötigt innerhalb von 15 Sekunden drei aufeinanderfolgende Snapshots mit:

- `Phase=menu`;
- `Valid=false`, also kein lesbarer In-Game-Player;
- `QuitMenuOpen=false`.

Loading ist ein erlaubter beobachteter Zwischenzustand und erzeugt keinen Input.

## Operator / CLI

Voraussetzungen:

- D2R Offline/Singleplayer;
- Charakter vollständig im Rogue Encampment geladen;
- Inventory, Stash und Quit-Menü geschlossen;
- D2R-Client exakt 1280×720;
- `input.enabled: true`;
- Pause-/Stop-Hotkeys funktionsfähig.

```powershell
go run ./cmd/d2rbot --offline-exit-test --verbose
```

Der Modus ist gegenseitig exklusiv mit Run, Phase, Input-Test, Pathing-Test, Route-Modus, UI-State-Probe und Offline-Difficulty-Test. Ein über `runs.active` konfigurierter Run wird nicht erzeugt; Gameplay-Bindings-Prechecks werden übersprungen.

## Timeouts und Invarianten

| Grenze | Wert | Reaktion |
|--------|------|----------|
| Gesamtflow | 30 s | Abbruch ohne weitere Inputs. |
| Quit-Menü-Bestätigung | 5 s | Abbruch ohne Save-&-Exit-Klick. |
| Menü-Ankunft nach Klick | 15 s | Abbruch; kein zweiter Klick. |
| Stabilität je Gate | 3 Ticks | Jeder abweichende Tick setzt nur das aktuelle Gate zurück. |

Verbindliche Invarianten:

1. höchstens ein `Esc`;
2. kein Save-&-Exit-Klick ohne drei `QuitMenuOpen`-Ticks;
3. höchstens ein Save-&-Exit-Klick;
4. keine Aktion bei Pause oder Stop;
5. vor `Esc` und vor dem Klick muss D2R per Windows-API aktiviert und als Foreground bestätigt sein;
6. kein Klick außerhalb exakt 1280×720;
7. kein Erfolg allein durch Zeitablauf;
8. kein Explorer-, Run- oder sonstiger Gameplay-Fallback.

## Fehlerverhalten

| Situation | Verhalten |
|-----------|-----------|
| Falsche Area | Sofortiger Fehler vor `Esc`. |
| Inventory/Stash offen | Sofortiger Fehler vor dem nächsten Input. |
| Quit-Menü bereits offen | Fehler; der isolierte Test verlangt die vollständige eigene Sequenz. |
| Quit-Flag bleibt aus | Timeout; kein Save-&-Exit-Klick. |
| Fenstergeometrie falsch | Fehler vor `Esc` beziehungsweise vor dem Klick. |
| D2R-Fokus nicht bestätigt | Fehler vor dem nächsten Keyboard-/Mausinput; die Foreground-Prüfung wartet höchstens 200 ms. |
| Input pausiert/gestoppt | Der Input Controller blockiert die Aktion; Flow endet mit Kontextfehler. |
| Prozessverlust | Bestehender Runtime-Reset; kein weiterer Lifecycle-Input. |
| Menü-Ankunft nicht bestätigt | Timeout; kein Retry-Klick. |

## Tests

- Erfolgspfad mit je drei stabilen Town-, Quit- und Menü-Ticks;
- falsche Area, offene Inventory-/Stash-UI, bereits geöffnetes Quit-Menü und anfänglicher Menüscreen;
- Quit-Menü-Timeout ohne Klickfreigabe;
- 1280×720-Gate und abweichende Geometrie;
- CLI-Opt-in, Input-Pflicht, Moduskonflikte und Deaktivierung von `runs.active`.

## Live-Abnahme Phase 7.2

Am 11. Juli 2026 wurden drei vollständige Exits mit D2R `3.2.92777`, `MrBones` und 1280×720 erfolgreich ausgeführt. Jeder Lauf bestätigte:

- drei stabile Town-/Identity-Ticks;
- genau ein `Esc`;
- drei stabile `QuitMenuOpen`-Ticks;
- genau einen Save-&-Exit-Klick auf `(640,327)`;
- drei stabile Menü-Ticks ohne In-Game-Player;
- `escape_presses=1` und `save_exit_clicks=1` im Abschlusslog.

Zwischen den Erfolgsfällen trat zweimal ein nicht bestätigter Foreground-Fokus auf. Im ersten Fall erreichte das bereits gesendete `Esc` D2R nicht; `QuitMenuOpen` blieb aus und der Flow brach ohne Mausmove/Klick ab. Danach wurde `input.Controller.Focus` ergänzt. Im zweiten Negativlauf schlug dessen unmittelbare Foreground-Bestätigung fehl und blockierte bereits vor `Esc`. Eine begrenzte Settle-Prüfung bestätigte im abschließenden Erfolgsfall den Fokus im zweiten 20-ms-Versuch. Kein Negativlauf erzeugte einen Save-&-Exit-Klick.

Phase 7.2 ist abgeschlossen. Längere Serien bleiben freiwillige Diagnose, keine Abnahmevoraussetzung.

## Verwandte Features

- [Session-Lifecycle](session-lifecycle.md)
- [Read-only UI-State-Probe](ui-state-probe.md)
- [Input Controller](input-controller.md)
- [Offline-Difficulty-Auswahl](offline-difficulty-selection.md)

---
*Zuletzt aktualisiert: 2026-07-11*
