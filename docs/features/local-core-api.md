# Lokale Core-API und eingebettete Web-Anwendung

## Überblick

Abschnitt 11.2 stellt den Go-Core über eine versionierte lokale HTTP-/JSON-Grenze bereit und liefert einen reproduzierbar gebauten React-/TypeScript-Client aus demselben Prozess aus. Abschnitt 11.3 ergänzt den read-only Runtime-Status und einen nicht blockierenden SSE-Stream. Seit Abschluss von Phase 15 ist diese Grenze ausschließlich der interne Transport zwischen dem gebündelten Core und dem Electron-Renderer; ein öffentlicher Browsermodus existiert nicht mehr.

## Ort im Code

- **Paket:** `internal/api/`
- **Server und Security:** `internal/api/server.go`
- **DTOs:** `internal/api/dto.go`
- **Maschinenvertrag:** `internal/api/schema/openapi.json`
- **Bootstrap-Projektion:** `internal/api/bootstrap_backend.go`
- **Live-Projektion:** `internal/api/live_backend.go`
- **Read-only Monitor:** `internal/app/ui_monitor.go`
- **Embed-Paket:** `internal/api/ui/embed.go`
- **Produktionsassets:** `internal/api/ui/dist/`
- **Frontend-Quelle:** `web/`
- **Desktop-Wiring:** `cmd/d2rbot/main.go` → private `--desktop-handshake-pipe`

## Funktionalität

### Prozessmodell

Der private Desktopmodus wird ausschließlich durch eine Electron-eigene Handshake-Pipe aktiviert. Er erzeugt die normale Core-Runtime, startet aber weder deren Task-Loop noch `RunSession`. Idle-Start und Routenaufnahme verlangen kein Countess-Pickit und keine Countess-Strategy für den Dummy-Träger. Ein eigener passiver Monitor verwendet den bestehenden Snapshot-Tick für Prozess-Attach, Fensterbindung und World-Update. Er ruft keine Farming-Tasks auf. Gameplay-Input ist ausschließlich über den expliziten, screenshot- und Memory-bestätigten Selection-Apply erlaubt. Das Live-Backend projiziert D2R, Input-Safety-Gates, World Model, aktive Auswahl und Supervisor.

Der Server wählt über `net.Listen("tcp4", "127.0.0.1:0")` einen freien Loopback-Port. URL und einmaliger Bootstrap-Token gelangen nur über die PID-gebundene Named Pipe an Electron. Der Control-Token steht bei der ersten Renderer-Navigation im URL-Fragment, wird von React in Memory übernommen und sofort per `history.replaceState` entfernt. Nach einem Reload erneuert `GET /api/v1/control/bootstrap` denselben Prozess-Token ausschließlich für einen Request mit dem nicht einfachen Header `X-D2RBot-Bootstrap: 1`. Ein fremder Origin scheitert am exakten Origin-Gate. Der Token wird weder in Web Storage noch in Cookie, Datei, URL-History, Standardausgabe oder Log persistiert.

### Endpunkte

| Endpunkt | Verhalten |
|---|---|
| `GET /api/v1/status` | Aktueller App-/Core-, D2R-Compatibility-, Input-, World- und Queue-Snapshot einschließlich tatsächlicher/erwarteter/Offsetversion, Game-ID, Run-ID, Lifecycle-Phase, Index, Spielzyklus, Retry und Safety-Budgets. |
| `GET /api/v1/catalog` | Read-only Run-Katalog aus dem bestehenden Availability-Resolver. |
| `GET /api/v1/events` | SSE: vollständiger Snapshot, Replay ab `Last-Event-ID`, danach Live-Deltas und Heartbeats. |
| `GET /api/v1/history/summary` | Gefilterte Historienpopulation mit Ergebnis-, Dauer-, Stage-, Funnel-, Fehler- und Dateidiagnosewerten. |
| `GET /api/v1/history/comparisons` | Core-sortierte Charakter-/Difficulty-/Definition-/Routenvergleiche mit Samplewarnung. |
| `GET /api/v1/history/items` | Cursor-paginierte Itemerträge und getrennte Keep-, Sell- und Verlustpfade. |
| `GET /api/v1/history/runs` | Cursor-paginierte, stabil nach UTC-Start und Run-ID sortierte Runliste. |
| `GET /api/v1/history/runs/{runID}` | Nutzerorientierter Run-Drill-down mit optionalen Rohereignissen. |
| `GET /api/v1/history/export` | Gefilterter JSON-Gesamtreport oder sichere CSV-Run-/Itemtabelle. |
| `GET /api/v1/control/bootstrap` | Same-origin Wiederherstellung des rein im Memory gehaltenen Prozess-Tokens nach Refresh; Custom Header und Security Envelope sind Pflicht. |
| `POST /api/v1/characters/reload` | Liest die begrenzten lokalen Saveheader neu; unveränderter Fachzustand bleibt revisions- und ereignisneutral. |
| `POST /api/v1/characters/setup/preview` | Liefert Core-validierte Klasse, kompatible Profile, Entwickler-Default, Setupstatus, Pickit-Zuordnungen und alle gebundenen Revisionen. |
| `POST /api/v1/characters/setup/confirm` | Speichert das klassenkompatible Profil und ergänzt ausschließlich vollständig fehlende Standard-Pickit-Zuordnungen. |
| `POST /api/v1/characters/selection/capture` | Erfasst nach expliziter Nutzerbestätigung den markierten 210×60-Charakterbereich ohne Navigationsinput und veröffentlicht ihn atomar. |
| `POST /api/v1/selection/preview` | Seiteneffektfreie, an Katalog- und Lifecycle-Revision gebundene Vorschau einschließlich betroffener Route-IDs und kurzlebigem Confirmation-Token. |
| `POST /api/v1/selection/apply` | Wendet exakt die unveränderte Vorschau screenshot- und Memory-verifiziert an; Lifecycle-Commit erfolgt erst nach bestätigtem Spieleintritt. |
| `POST /api/v1/queue/validate` | Seiteneffektfreier Gesamt-Preflight gegen bestätigte Auswahl, Katalogrevision, Availability, Lifecycle und Safety-Budgets. |
| `POST /api/v1/session/start` | Validiert den unveränderten Queue-Kontext erneut und startet genau eine Core-eigene Supervisor-Queue. |
| `POST /api/v1/session/pause-after-run` | Merkt Pause nach Run, Loot und Town-Handoff vor; das Spiel bleibt geöffnet. |
| `POST /api/v1/session/resume` | Revalidiert dasselbe offene Spiel und startet den nächsten Queue-Eintrag mit frischem Run-Zustand. |
| `POST /api/v1/session/stop-after-run` | Merkt nach Town genau einen supervisor-eigenen Save-&-Exit vor. |
| `POST /api/v1/session/emergency-stop` | Bricht sofort mit demselben Grund wie F11 ab; Save & Exit ist nicht garantiert. |

Die Routenoberfläche projiziert für den Cow-Run seit Phase 20.2 zwei getrennte Aufnahmeverträge: `leg_acquisition` als Wirt-Route ab Stony-Field-Wegpunkt und `cow_sweep` als Cow-Route ab bestätigter Ankunft am permanenten Cow-Portal. Kandidat, Workflow, Bibliothek und Mutationsvorschau tragen die Rolle explizit. Veröffentlichung ersetzt atomisch nur dieselbe Rolle und erhält die kompatible Schwesterroute.

Unbekannte `/api/`-Versionen oder Endpunkte liefern `api_version_unsupported` und niemals die SPA-Fallbackseite.

### Security Envelope

- Ausschließliche IPv4-Loopback-Bindung; keine konfigurierbare Remote-Adresse.
- Exakte `Host`-Prüfung gegen den tatsächlich gewählten Listener.
- Fehlender `Origin` ist für lokale Nicht-Browser-Clients zulässig; ein vorhandener Origin muss exakt der lokalen Base-URL entsprechen.
- Kryptografisch zufälliger 256-Bit-Control-Token pro Prozess; keine Persistenz und kein Logging.
- Mutationen verlangen `X-D2RBot-Control-Token`, `POST` und `Content-Type: application/json`.
- JSON wird mit unbekannten Feldern fail-closed dekodiert; Body-Limit ist 64 KiB.
- Jede Antwort trägt eine zufällige Request-ID, `nosniff`, eine restriktive Content Security Policy und bei JSON `Cache-Control: no-store`.
- Fehler verwenden stabilen englischen `code`, ordentliche deutsche `message`, optionale `details` und `request_id`.
- `command_id` ist verpflichtend; `expected_generation` gehört zum maschinenlesbaren DTO. Fachliche Idempotenz und Generation bleiben beim `SessionSupervisor`.

Ein Fremd-Origin, falscher Host, fehlender Token, falsche Methode, falscher Content-Type, malformed JSON, unbekanntes Feld oder übergroßer Body erreicht niemals das Command-Backend.

### Live-Stream

Der Core vergibt pro Live-Ereignis eine streng monotone Sequenz und hält nur einen begrenzten Ring im Speicher. Jede SSE-Verbindung erhält zunächst einen vollständigen Status-Snapshot. Mit `Last-Event-ID` werden anschließend noch verfügbare Deltas nachgeliefert; ohne Header beginnt der Client am aktuellen Rand. Pro Client existiert eine begrenzte Queue. Ein langsamer oder abgebrochener Renderer wird getrennt, statt Core, JSONL oder andere Clients zu blockieren. Area- und Step-Ereignisse werden bei unveränderter Identität dedupliziert.

Nach einem terminalen Run-Wechsel aktualisiert das Live-Backend den flüchtigen History-Index. Eine geänderte Generation erzeugt ausschließlich `history_changed` mit der Generation; persistente Telemetriezeilen und lokale Pfade werden nicht in SSE kopiert.

`compatibility_changed` enthält nur Zustand, stabilen Reason-Code und die vier pfadfreien Versionswerte. Auswahl-Apply, Queue-Start/Resume sowie Live-Routenworkflows und deren Publish-Mutationen werden zusätzlich im Backend abgewiesen, solange der Zustand nicht `compatible` ist.

`catalog_changed` enthält ausschließlich die neue Katalogrevision. Reload, Setup und Capture veröffentlichen es genau einmal, wenn sich die fachliche Katalogprojektion tatsächlich geändert hat; Pfade, Saveinhalte und Bilddaten werden nie über SSE übertragen. Der Renderer lädt daraufhin die abhängigen read-only Projektionen neu.

## Frontend-Build

`web/package.json` pinnt React, TypeScript, Vite, Vitest und Testabhängigkeiten exakt. `pnpm-lock.yaml` hält den vollständigen aufgelösten Dependency-Graph. Das OpenAPI-Dokument ist die einzige DTO-Quelle; `web/scripts/generate-api.mjs` erzeugt `web/src/api/generated.ts` einschließlich Query-Client. `pretest` und `prebuild` brechen ab, wenn das generierte Ergebnis veraltet ist.

```powershell
cd web
pnpm install --frozen-lockfile
pnpm test
pnpm typecheck
pnpm build
```

Vite schreibt den Produktionsbuild direkt nach `internal/api/ui/dist`. Das ausgelieferte Go-Binary benötigt deshalb weder Node.js noch einen separaten Webserver.

## Operator / Desktop

Die API wird nicht direkt vom Operator gestartet. Electron übergibt absoluten Datenroot und private Handshake-Pipe an den gebündelten Core. Desktopmodus und Session-, Run-, Inspect-, Probe-, Route-, Town- oder Testmodi sind gegenseitig exklusiv. Der Prozess endet über den kontrollierten Electron-Shutdown oder `SIGTERM` mit einem begrenzten HTTP-Shutdown.

## Abhängigkeiten und Grenzen

- Go-Standardbibliothek für HTTP, JSON, Crypto-Random, Embed und Loopback-Listener.
- React/TypeScript/Vite nur als Build-Abhängigkeiten unter `web/`.
- Keine Business-Logik in Handlern und keine HTTP-Typen in `internal/app`.
- Live-Ereignisse sind flüchtig; persistente Diagnose bleibt Aufgabe der JSONL-Telemetrie.
- Produktive Mutationen laufen ausschließlich über den gemeinsamen `SessionSupervisor`; React besitzt weder Runtime noch Worker.
- Keine YAML-, Route-, Memory- oder Input-Zugriffe aus React.

## Verwandte Features

- [Phase-11-Core-Vertrag](phase-11-core-contract.md)
- [Session-Lifecycle](session-lifecycle.md)
- [Run-Verfügbarkeit und Inspect](run-availability.md)
- [Charaktereinrichtung](character-setup.md)
- [Historien-API und Export](history-api-export.md)

---
*Zuletzt aktualisiert: 16. August 2026*
