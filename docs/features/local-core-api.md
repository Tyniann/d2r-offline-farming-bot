# Lokale Core-API und eingebettete Web-Anwendung

## Überblick

Abschnitt 11.2 stellt den Go-Core über eine versionierte lokale HTTP-/JSON-Grenze bereit und liefert einen reproduzierbar gebauten React-/TypeScript-Client aus demselben Prozess aus. Abschnitt 11.3 ergänzt den read-only Runtime-Status und einen nicht blockierenden SSE-Stream. `d2rbot.exe --ui` bindet ausschließlich einen zufälligen Port auf `127.0.0.1`, öffnet den Standardbrowser und startet unabhängig von YAML-Session-Defaults niemals automatisch einen Bot-Run.

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
- **CLI:** `cmd/d2rbot/main.go` → `--ui`

## Funktionalität

### Prozessmodell

`--ui` erzeugt die normale Core-Runtime, startet aber weder deren Task-Loop noch `RunSession`. Ein eigener passiver Monitor verwendet den bestehenden Snapshot-Tick für Prozess-Attach, Fensterbindung und World-Update. Er ruft keine Farming-Tasks auf. Gameplay-Input ist ausschließlich über den expliziten, screenshot- und Memory-bestätigten Selection-Apply erlaubt. Das Live-Backend projiziert D2R, Input-Safety-Gates, World Model, aktive Auswahl und Supervisor.

Der Server wählt über `net.Listen("tcp4", "127.0.0.1:0")` einen freien Loopback-Port. Die sichere URL ohne Secret wird einmal an der Konsole ausgegeben. Der Control-Token steht bei der ersten Navigation in der Browser-URL als Fragment, wird von React in Memory übernommen und sofort per `history.replaceState` aus der sichtbaren URL entfernt. Nach einem Refresh erneuert `GET /api/v1/control/bootstrap` denselben Prozess-Token ausschließlich für einen Request mit dem nicht einfachen Header `X-D2RBot-Bootstrap: 1`. Eine fremde Webseite müsste dafür einen CORS-Preflight ausführen, der bereits am exakten Origin-Gate scheitert. Der Token wird weiterhin weder in Web Storage noch in Cookie, Datei, URL-History oder Log persistiert.

### Endpunkte

| Endpunkt | Stand 11.3 |
|---|---|
| `GET /api/v1/status` | Aktueller Core-, D2R-, Input-, World- und Queue-Snapshot einschließlich Index, Zyklus, Retry und Safety-Budgets. |
| `GET /api/v1/catalog` | Read-only Run-Katalog aus dem bestehenden Availability-Resolver. |
| `GET /api/v1/events` | SSE: vollständiger Snapshot, Replay ab `Last-Event-ID`, danach Live-Deltas und Heartbeats. |
| `GET /api/v1/control/bootstrap` | Same-origin Wiederherstellung des rein im Memory gehaltenen Prozess-Tokens nach Refresh; Custom Header und Security Envelope sind Pflicht. |
| `POST /api/v1/selection/preview` | Seiteneffektfreie, an Katalog- und Lifecycle-Revision gebundene Vorschau einschließlich betroffener Route-IDs und kurzlebigem Confirmation-Token. |
| `POST /api/v1/selection/apply` | Wendet exakt die unveränderte Vorschau screenshot- und Memory-verifiziert an; Lifecycle-Commit erfolgt erst nach bestätigtem Spieleintritt. |
| `POST /api/v1/queue/validate` | Seiteneffektfreier Gesamt-Preflight gegen bestätigte Auswahl, Katalogrevision, Availability, Lifecycle und Safety-Budgets. |
| `POST /api/v1/session/start` | Validiert den unveränderten Queue-Kontext erneut und startet genau eine Core-eigene Supervisor-Queue. |
| `POST /api/v1/session/pause-after-run` | Merkt Pause erst nach aktuellem Run, Town und Save & Exit vor. |
| `POST /api/v1/session/resume` | Startet den bereits bestimmten nächsten Queue-Eintrag frisch. |
| `POST /api/v1/session/stop-after-run` | Merkt geordneten Stopp nach aktuellem vollständigem Run vor. |
| `POST /api/v1/session/emergency-stop` | Bricht sofort mit demselben Grund wie F11 ab; Save & Exit ist nicht garantiert. |

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

Der Core vergibt pro Live-Ereignis eine streng monotone Sequenz und hält nur einen begrenzten Ring im Speicher. Jede SSE-Verbindung erhält zunächst einen vollständigen Status-Snapshot. Mit `Last-Event-ID` werden anschließend noch verfügbare Deltas nachgeliefert; ohne Header beginnt der Client am aktuellen Rand. Pro Client existiert eine begrenzte Queue. Ein langsamer oder abgebrochener Browser wird getrennt, statt Core, JSONL oder andere Clients zu blockieren. Area- und Step-Ereignisse werden bei unveränderter Identität dedupliziert.

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

## Operator / CLI

```powershell
go run ./cmd/d2rbot --config configs/config.yaml --ui
```

`--ui` ist mit Session-, Run-, Inspect-, Probe-, Route-, Town- und Testmodi gegenseitig exklusiv. `--verbose` bleibt als reine Logging-Option zulässig. Browser-Öffnen ist Komfort: Schlägt es fehl, bleibt der Server aktiv und die sichere tokenfreie Base-URL steht an der Konsole. Der Prozess endet über `Ctrl+C`/`SIGTERM` mit einem begrenzten HTTP-Shutdown.

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

---
*Zuletzt aktualisiert: 16. Juli 2026*
