# Geführte Farming-Routenaufnahme

## Überblick

Der `RecordingCoordinator` kapselt den bestehenden World-Koordinaten-Recorder in einem exklusiven, fail-closed Workflow. Eine Aufnahme wird niemals direkt zur Farming-Route: F9 friert zuerst einen unveränderlichen Kandidaten außerhalb des Farming-Roots ein, danach folgen semantische Prüfung und ein separater TP-Sicherheitsrückweg.

## Ort im Code

- **Coordinator:** `internal/app/recording_coordinator.go`
- **Candidate Store:** `internal/app/candidate_store.go`
- **Sampling:** `internal/pathing/route_recorder.go`
- **Semantik:** `internal/tasks.RunDefinition.Recording`
- **Config:** `routes.candidate_root`, `input.recording_finish_hotkey`

## Preflight und Aufnahme

Der Start verlangt einen registrierten Run, vollständige Character-/Class-/Difficulty-/Versions- und Revisionsbindung, bestätigten Zielwegpunktkontext, geschlossene tatsächlich blockierende UIs, D2R-Fokus sowie einen freien exklusiven Input-Owner. Der aktuelle World-State muss den zuvor im Core bestätigten Charakternamen und die konfigurierte Klasse aus Memory, das Startgebiet, einen stabilen Layout-Fingerprint und einen sichtbaren Startwegpunkt innerhalb der konfigurierten `pathing.waypoint.max_click_distance` liefern. Das D2R-UI-Bit `WaypointOpen` bleibt nach einer abgeschlossenen Wegpunktreise auf diesem Build nachweislich gesetzt, obwohl das Panel sichtbar geschlossen ist. Es ist deshalb für diesen read-only Aufnahmestart kein Blocker; Inventory, NPC-Dialog, Shop, Stash und Quit-Menü bleiben fail-closed. Solange die übrige Memory-Evidenz fehlt, bleibt der Workflow sichtbar in `preflight`, protokolliert im Zwei-Sekunden-Takt Gebiet, Sichtbarkeit sowie gemessene und erlaubte Wegpunktdistanz und akzeptiert F9 nicht als Aufnahmeende. Eine Aufnahme endet nach 30 Minuten fail-closed mit `recording_timeout`.

Der CLI-Befehl `--route record:<run-id> --route-difficulty <difficulty>` ist nur ein Runtime-Adapter auf diesen Coordinator. Der frühere direkte Publish aus der CLI existiert nicht mehr. API-Finish und F9 laufen über denselben idempotenten Finish-Kanal; API und Dashboard verwenden denselben Core.

Während `recording` akzeptiert der bestehende `pathing.RouteRecorder` nur konsistente In-Game-Snapshots und die im `RecordingContract` erlaubten Gebiete. Sampling-Distanz und Area-Übergänge bleiben generische Pathing-Verantwortung.

## F9, Freeze und Terminalprüfung

F9 beziehungsweise `Finish` ist ausschließlich in `recording` autoritativ und idempotent. Der Candidate Store erzeugt eine kollisionsfeste ID, speichert `route.yaml` unter `routes/candidates/<candidate-id>/`, berechnet SHA-256 und publiziert danach atomisch `candidate.yaml`. Jeder spätere Load prüft den Hash erneut.

Erst nach diesem Freeze prüft der Coordinator Terminalgebiet, erlaubte Segmente und Bewegung, exakte Boss-ID, gegebenenfalls Super-Unique-Flag, Alive-Evidenz und die frei gewählte Endposition gegen die run-spezifische Maximaldistanz. Abgelehnte Kandidaten bleiben mit stabilem Diagnosegrund erhalten.

## Safety Return und F11

Nach Freeze wird genau ein TP-Sicherheitsrückweg angefordert. Erfolg führt bei validem Kandidaten zu `candidate_ready`; ein TP-Fehler endet `failed_safe`, ohne den Kandidaten zu löschen, markiert ihn jedoch persistent als `failed` und verhindert dadurch Test oder Publish ohne erfolgreichen Safety Return. Weil der Boss und sein Pack während der Prüfung absichtlich leben, verwendet der gemeinsame Town-Portal-Clicker für Portale einen dreifach erweiterten, weiterhin strikt begrenzten Hover-Suchraum. Auch dort erfolgt niemals ein Blindklick: Nur die exakte Memory-bestätigte Portal-UnitID erlaubt den Klick. F11 vor Freeze beendet sofort als `emergency_cancelled`, erzeugt keinen Kandidaten und verspricht keinen Portalrückweg. F11 nach Freeze erhält den bereits unveränderlichen Kandidaten, veröffentlicht ihn aber nie.

Die Hotkey-Semantik ist getrennt: F9 Aufnahme beenden, F10 Stop-after-run, F11 Emergency Stop und `Pause` für Pause/Pause-after-run. Der bisherige CLI-Recorder beendet nicht mehr über den Emergency-Stop-Hotkey.

## Grenzen

Candidate Root und Farming Root müssen disjunkt sein. Weder Recording noch Validierung schreiben Assignment-, Lifecycle- oder produktive Farming-Dateien. Isoliertes Test-Playback und bestätigungsgebundene Veröffentlichung sind in [Kandidaten-Playback und Routenverwaltung](route-management.md) beschrieben.

## Live-Abnahme Phase 12

Die Countess-Aufnahme für `MrBones` auf Nightmare wurde vollständig über das Dashboard beendet. F9 fror `candidate-5af9deda83bfdd91` mit sieben Segmenten ein; Terminalgebiet, lebende Countess und 15,65 Tiles Enddistanz wurden bestätigt. Der erweiterte Portal-Hover fand das vom lebenden Pack überlagerte Portal begrenzt und UnitID-bestätigt. Nach dem erfolgreichen Safety Return blieb der Kandidat unveränderlich und testbar gespeichert.

## Verwandte Features

- [Route Recording und Playback](route-recording-playback.md)
- [Run Registry](run-registry.md)
- [Farming-Route-Assignment](route-assignment.md)

---
*Zuletzt aktualisiert: 18. Juli 2026*
