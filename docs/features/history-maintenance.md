# Historien-Retention und Komplettlöschung

## Überblick

Abschnitt 15.9 hält die lokale JSONL-Historie begrenzt, ohne eine zweite Datenautorität einzuführen. Der Go-Core besitzt den kanonischen Telemetrie-Root, löscht automatisch nur vollständig geschlossene alte Session-Bundles und bietet getrennt eine ausdrücklich bestätigte Komplettlöschung aller nicht aktiven direkten JSONL-Kategorien.

## Ort im Code

- **Paket:** `internal/telemetry/`
- **Service:** `internal/telemetry/history_maintenance.go`
- **API/Backend:** `internal/api/history_maintenance_server.go`, `internal/api/history_maintenance_backend.go`
- **UI:** `web/src/features/settings/SettingsFeature.tsx`
- **Config:** `history.retention_enabled`, `history.retention_days` in `configs/operator-settings.local.yaml`

## Funktionalität

### Automatische Retention

Der Service wacht beim stabilen UI-Start und danach stündlich auf, führt selbst jedoch höchstens alle 24 Stunden einen Lauf aus. Retention ist standardmäßig mit 60 Tagen aktiv und nur bei inaktivem Supervisor sowie inaktivem Routenworkflow zulässig.

Ein Kandidat ist ausschließlich ein vollständiges Schema-4-Bundle: eine valide terminale Sessiondatei, exakt alle von ihr gestarteten validen Run-Dateien und je genau ein Run-Terminal. Zusätzliche oder fehlende Gegenstreams, aktive Namen, Legacy, beschädigte, unvollständige und zeitlich gemischte Bundles bleiben vollständig erhalten. Sessionterminal und neuestes Run-Terminal müssen strikt älter als den Stichtag sein; exakt 60 Tage werden bei 60 Tagen Retention nicht gelöscht.

### Bestätigte Komplettlöschung

Die Vorschau zählt alle regulären direkten `*.jsonl`-Dateien nach `schema3_session`, `schema3_run`, `legacy` und `corrupt`, weist aktive Dateien getrennt als geschützt aus und liefert nur Anzahl, Bytes, Kategorien, Indexgeneration und einen zufälligen Einmaltoken. Absolute Pfade verlassen den Core nicht.

Die zweite Bestätigung muss Token, Indexgeneration, Anzahl und Bytes exakt wiederholen. Der Core refreshed den Index, bestimmt das Active-Set neu und revalidiert vor der ersten sowie vor jeder einzelnen Löschung Dateiname, regulären Direktdateistatus, Größe, Änderungszeit und Reparse-Status. Eine veraltete Vorschau wird ohne Mutation abgelehnt. Neu aktive Dateien bleiben selbst bei gültiger alter Vorschau geschützt. Terminale Routenworkflow-Zustände (`completed`, `failed_safe`, `emergency_cancelled`) gelten dabei wie im restlichen Backend als inaktiv und blockieren die Wartung nicht. Eine erneute Vorschau nach vollständiger Löschung bleibt zulässig und weist `0` Dateien aus.

## Fehlerverhalten

- `history_retention_blocked`: Supervisor/Workflow aktiv oder Bundle unklar.
- `history_retention_partial`: ein automatischer Kandidat blieb wegen Dateifehler erhalten.
- `history_delete_preview_stale`: Token, Generation oder Metadaten stimmen nicht mehr.
- `history_delete_active_protected`: unmittelbar aktiver Writer blieb erhalten.
- `history_delete_failed`: mindestens ein bestätigter Kandidat konnte nicht entfernt werden.

Diagnosen verwenden nur eine neutrale Hash-ID. Es gibt keinen Papierkorb, kein Telemetriebackup, keine Reparatur, keine Globs und keine freie Pfad- oder Stichtagseingabe. Nach jeder vollständigen oder teilweisen Mutation wird der rebuildbare Index gegen den verbleibenden JSONL-Bestand aktualisiert.

## Getestete Grenzen

Die Temp-Root-Matrix deckt 59/60/61 Tage, mehrere Runs, gemischtes Alter, fehlende und zusätzliche Gegenstreams, aktive Namen, Legacy, beschädigte Dateien, Unterverzeichnisse, Reparse/Symlink, stale Metadaten und injizierte Locked-/Teilfehlerschritte ab.

## Verwandte Features

- [History-Reader und In-Memory-Index](history-reader-index.md)
- [Historien-API und Export](history-api-export.md)
- [Persistente Operator-Einstellungen](operator-settings.md)

---
*Zuletzt aktualisiert: 13. August 2026*
