# Offline-Spieleranzahl

## Überblick

Jeder Charakter speichert eine Offline-Spieleranzahl von 1 bis 8. Nach einem bestätigten Spielstart sendet der Bot genau einmal `/players N` im Chat, damit eine in D2R gespeicherte Players-Einstellung den Farm-Wunsch nicht überstimmt. Same-Game-Folgeruns senden den Befehl nicht erneut.

## Ort im Code

- **Paket:** `internal/app/`, `internal/input/`
- **Einstieg:** `(*runtimeQueueUnit).finishVerifiedQueueGame` nach `verifyActiveQueueGame`
- **Wichtige Dateien:** `internal/app/offline_players.go`, `internal/app/operator_settings.go`, `internal/input/chat.go`, `web/src/features/characters/CharactersTab.tsx`
- **Config:** `characters.<name>.players` in `operator-settings.local.yaml` (Schema 3)

## Funktionalität

### Persistenz

`players` gehört zum Charakter, nicht zur Queue oder zum Kampfprofil. Fehlt das Feld, gilt 1. Werte außerhalb 1–8 werden abgelehnt. Ein Schema-Sprung entfällt.

Die Queue friert denselben Wert im Loadout ein. Eine Änderung während einer laufenden Session ist weiterhin gesperrt.

### Zeitpunkt

Der Befehl läuft erst, nachdem `verifyActiveQueueGame` ein In-Game im Rogue Encampment ohne blockierende UI bestätigt hat. Das gilt für ein neu erzeugtes Spiel und für ein übernommenes Town-Spiel. Loading, Menü und Same-Game-Fortsetzung lösen keinen Chat aus. Routenaufnahme, Kandidatentests und Town-Tests bleiben unberührt.

### Chat-Eingabe

Enter öffnet den Chat, Unicode tippt `/players N` layoutunabhängig, Enter sendet. Der erlaubte Text ist genau `/players 1` bis `/players 8`. Schlägt der Versand fehl, bricht der Spielstart ab. Es gibt kein Esc-Raten und keine Chat-Open-Memoryprüfung: ein frisch bestätigtes Spiel hat keinen offenen Chat.

## Datenmodell

- `OperatorCharacterSettings.Players`: 1–8, fehlend wird als 1 gelesen
- `CharacterLoadoutSnapshot.Players`: eingefrorene Kopie für die Queue-Session

## Operator / CLI

Einstellungen → Charaktere zeigt ein Dropdown 1–8. Höhere Werte machen Gegner und Erfahrung härter. Schlägt der Befehl fehl, bleibt die Queue stehen mit dem Hinweis, dass die Spieleranzahl nicht gesetzt werden konnte.

## Abhängigkeiten

Windows `SendInput` mit `KEYEVENTF_UNICODE` für den Befehlstext. Enter bleibt ein virtueller Key.

## Verwandte Features

- [Persistente Operator-Einstellungen](operator-settings.md)
- [Session-Lifecycle](session-lifecycle.md)
- [FarmQueue-Scheduler](farm-queue-scheduler.md)
- [Input Controller](input-controller.md)

---
*Zuletzt aktualisiert: 2026-08-19*
