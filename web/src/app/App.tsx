import { useEffect, useRef, useState } from "react";
import {
  applySelection, connectLiveEvents, consumeBootstrapToken, emergencyStop, pauseAfterRun,
  previewSelection, resumeQueue, startQueue, stopAfterRun, validateQueue, type LiveConnectionState,
} from "../api/client";
import { getCatalog, getStatus, type CatalogDTO, type LiveEvent, type SelectionPreviewDTO, type StatusDTO } from "../api/generated";
import "./app.css";
import { RouteFeature } from "../features/routes/RouteFeature";
import { PickitFeature } from "../features/pickit/PickitFeature";
import { HistoryFeature } from "../features/history/HistoryFeature";

const editableStates = new Set(["idle", "idle_in_game", "stopped_error"]);
const emergencyStates = new Set(["starting_game", "starting_run", "running_run", "paused_between_runs", "exiting_game"]);

export function App() {
  const [status, setStatus] = useState<StatusDTO | null>(null);
  const [catalog, setCatalog] = useState<CatalogDTO | null>(null);
  const [events, setEvents] = useState<LiveEvent[]>([]);
  const [connection, setConnection] = useState<LiveConnectionState>("wird verbunden");
  const [character, setCharacter] = useState("");
  const [difficulty, setDifficulty] = useState("");
  const [queue, setQueue] = useState<string[]>([]);
  const [applying, setApplying] = useState(false);
  const [commandPending, setCommandPending] = useState(false);
  const commandLock = useRef(false);
  const [error, setError] = useState("");
  const [selectionError, setSelectionError] = useState("");
  const [queueError, setQueueError] = useState("");
  const [preview, setPreview] = useState<SelectionPreviewDTO | null>(null);
  const [confirmEmergency, setConfirmEmergency] = useState(false);
  const [routeRefreshKey, setRouteRefreshKey] = useState(0);
  const [pickitRefreshKey, setPickitRefreshKey] = useState(0);
  const [historyRefreshKey, setHistoryRefreshKey] = useState(0);
  const emergencyConfirmRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    consumeBootstrapToken();
    const controller = new AbortController();
    let disconnect = () => {};
    let refreshing = false;
    let refreshPending = false;

    const reportError = (reason: unknown) => {
      if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : "Statusabfrage fehlgeschlagen");
    };
    const refreshStatus = async () => {
      if (refreshing) {
        refreshPending = true;
        return;
      }
      refreshing = true;
      try {
        do {
          refreshPending = false;
          setStatus(await getStatus(controller.signal));
          setError("");
        } while (refreshPending && !controller.signal.aborted);
      } catch (reason: unknown) {
        reportError(reason);
      } finally {
        refreshing = false;
      }
    };

    void Promise.all([getStatus(controller.signal), getCatalog(controller.signal)]).then(([nextStatus, nextCatalog]) => {
      if (controller.signal.aborted) return;
      setStatus(nextStatus);
      setCatalog(nextCatalog);
      setQueue(nextStatus.queue?.entries ?? []);
      setCharacter(nextCatalog.characters.find((entry) => entry.selectable)?.name ?? "");
      setDifficulty(nextCatalog.default_difficulty);
      disconnect = connectLiveEvents(
        (data) => setStatus(data as StatusDTO),
        (data) => {
          setEvents((current) => [data as LiveEvent, ...current].slice(0, 40));
          if ((data as LiveEvent).event.startsWith("route_")) setRouteRefreshKey((value) => value + 1);
          if ((data as LiveEvent).event.startsWith("pickit_")) setPickitRefreshKey((value) => value + 1);
          if ((data as LiveEvent).event === "history_changed") setHistoryRefreshKey((value) => value + 1);
          void refreshStatus();
        },
        setConnection,
      );
    }).catch(reportError);
    return () => { controller.abort(); disconnect(); };
  }, []);

  useEffect(() => {
    if (!confirmEmergency) return;
    emergencyConfirmRef.current?.focus();
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setConfirmEmergency(false);
    };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [confirmEmergency]);

  const refreshAfterCommand = async () => {
    const [nextStatus, nextCatalog] = await Promise.all([getStatus(), getCatalog()]);
    setStatus(nextStatus);
    setCatalog(nextCatalog);
    setQueue(nextStatus.queue.entries);
  };

  const applyPreview = async (selectionPreview: SelectionPreviewDTO) => {
    if (!catalog || !status) return;
    setApplying(true);
    setSelectionError("");
    try {
      await applySelection(selectionPreview.character, selectionPreview.new_difficulty, catalog.revision, status.generation, selectionPreview.confirmation_token);
      await refreshAfterCommand();
      setPreview(null);
    } catch (reason: unknown) {
      setSelectionError(reason instanceof Error ? reason.message : "Auswahl fehlgeschlagen");
    } finally {
      setApplying(false);
    }
  };

  const submitSelection = async () => {
    if (!catalog || !status || !character || !difficulty) return;
    setApplying(true);
    setSelectionError("");
    try {
      const nextPreview = await previewSelection(character, difficulty, catalog.revision);
      if (nextPreview.requires_confirmation) setPreview(nextPreview);
      else {
        setApplying(false);
        await applyPreview(nextPreview);
        return;
      }
    } catch (reason: unknown) {
      setSelectionError(reason instanceof Error ? reason.message : "Vorschau fehlgeschlagen");
    } finally {
      setApplying(false);
    }
  };

  const runCommand = async (action: () => Promise<unknown>) => {
    if (commandLock.current) return;
    commandLock.current = true;
    setCommandPending(true);
    setQueueError("");
    try {
      await action();
      await refreshAfterCommand();
    } catch (reason: unknown) {
      setQueueError(reason instanceof Error ? reason.message : "Session-Befehl fehlgeschlagen");
    } finally {
      commandLock.current = false;
      setCommandPending(false);
    }
  };

  const submitQueue = async () => {
    if (!status || !catalog || !status.selection.character || !status.selection.difficulty) return;
    await runCommand(async () => {
      await validateQueue(queue, status.selection.character!, status.selection.difficulty!, catalog.revision);
      await startQueue(queue, status.selection.character!, status.selection.difficulty!, catalog.revision, status.generation);
    });
  };

  const editorLocked = !status || !editableStates.has(status.state) || commandPending;
  const selectionLocked = applying || commandPending || (!!status && !editableStates.has(status.state));
  const yamlDefault = status?.queue.default_entries ?? [];
  const hasPendingIntent = !!status?.pending_intent && status.pending_intent !== "none";

  return (
    <main>
      <header>
        <p className="eyebrow">D2R Offline Farming Bot</p>
        <h1>Lokales Dashboard</h1>
        <p>Live-Sicht auf Core, D2R und World Model. Gameplay-Input wird nur nach einer explizit bestätigten Aktion gesendet.</p>
        <span className={`connection ${connection === "verbunden" ? "online" : ""}`}>Live: {connection}</span>
      </header>

      <nav className="main-navigation" aria-label="Dashboard-Bereiche"><a href="#betrieb">Betrieb</a><a href="#routes">Routen</a><a href="#pickit">Pickit</a><a href="#history">Historie</a></nav>

      <section id="betrieb" aria-live="polite">
        <h2>Core-Status</h2>
        {error && <p role="alert">{error}</p>}
        {!error && !status && <p>Verbindung wird hergestellt …</p>}
        {status && <div className="cards">
          <article><span>Core</span><strong>{status.state}</strong><small>Generation {status.generation}</small></article>
          <article><span>D2R</span><strong>{status.d2r.state}</strong><small>{status.d2r.window_bound ? `${status.d2r.client_width ?? 0} × ${status.d2r.client_height ?? 0}` : "Kein Fenster gebunden"}</small></article>
          <article><span>Input</span><strong>{status.input.stopped ? "gestoppt" : status.input.paused ? "pausiert" : status.input.enabled ? "freigegeben" : "deaktiviert"}</strong><small>Safety-Gates aus dem Core</small></article>
          <article><span>Gebiet</span><strong>{status.world.area_name || "Unbekannt"}</strong><small>{status.world.valid ? status.world.phase : "World Model noch ungültig"}</small></article>
        </div>}
      </section>

      <div id="routes"><RouteFeature characters={catalog?.characters.map((entry) => entry.name) ?? []} selectedCharacter={status?.selection.character ?? character} refreshKey={routeRefreshKey} /></div>

      <div id="pickit"><PickitFeature characters={catalog?.characters.map((entry) => entry.name) ?? []} selectedCharacter={status?.selection.character ?? character} runs={catalog?.runs.map((entry) => entry.run_id) ?? []} locked={!!status && !editableStates.has(status.state)} refreshKey={pickitRefreshKey} /></div>

      <HistoryFeature characters={catalog?.characters.map((entry) => entry.name) ?? []} runs={catalog?.runs.map((entry) => entry.run_id) ?? []} refreshKey={historyRefreshKey} />

      <section>
        <h2>Charakter und Schwierigkeit</h2>
        <p>D2R muss auf dem Offline-Charakterbildschirm bei 1280 × 720 stehen. Die Auswahl wird vor jedem Klick visuell und anschließend im Spiel über Memory bestätigt.</p>
        {selectionError && <p role="alert">{selectionError}</p>}
        <p><strong>Aktiv bestätigt:</strong> {status?.selection.character ? `${status.selection.character} / ${status.selection.difficulty}` : "Noch kein Kontext bestätigt"}<br /><strong>Entwurf:</strong> {character || "–"} / {difficulty || "–"}</p>
        <div className="selection-grid">
          <label>Charakter<select value={character} onChange={(event) => setCharacter(event.target.value)} disabled={selectionLocked}>{catalog?.characters.map((entry) => <option key={entry.slug} value={entry.name} disabled={!entry.selectable}>{entry.name}{entry.selectable ? "" : ` – ${entry.reasons?.join(", ")}`}</option>)}</select></label>
          <label>Schwierigkeit<select value={difficulty} onChange={(event) => setDifficulty(event.target.value)} disabled={selectionLocked}>{catalog?.difficulties.map((entry) => <option key={entry.id} value={entry.id}>{entry.display_name}</option>)}</select></label>
          <button type="button" disabled={selectionLocked || !character || (status?.state !== "idle" && status?.state !== "idle_in_game" && status?.state !== "stopped_error")} onClick={() => void submitSelection()}>{applying ? "Auswahl wird geprüft …" : "Auswahl in D2R anwenden"}</button>
        </div>
        {preview && <div className="modal-backdrop"><div role="dialog" aria-modal="true" aria-labelledby="selection-confirm-title" className="modal"><h3 id="selection-confirm-title">Routen werden unbrauchbar</h3><p>Der Wechsel von <strong>{preview.old_difficulty || "unbestätigt"}</strong> auf <strong>{preview.new_difficulty}</strong> markiert folgende Farming-Routen als <code>stale</code>:</p><ul>{preview.affected_routes.map((route) => <li key={route}>{route}</li>)}</ul><p>Die Dateien werden nicht gelöscht oder verändert. Neue Aufnahmen sind vor Farming erforderlich.</p><div className="modal-actions"><button type="button" className="secondary" onClick={() => setPreview(null)} disabled={applying}>Abbrechen</button><button type="button" onClick={() => void applyPreview(preview)} disabled={applying}>{applying ? "Wird angewendet …" : "Auswirkungen bestätigen und anwenden"}</button></div></div></div>}
        <ul className="character-list">{catalog?.characters.filter((entry) => !entry.selectable).map((entry) => <li key={entry.slug}><strong>{entry.name}</strong><span>{entry.reasons?.join(", ")}</span></li>)}</ul>
      </section>

      <section>
        <h2>Run-Reihenfolge pro Spiel</h2>
        <p>Jeder verfügbare Run kann genau einmal enthalten sein. Die Reihenfolge läuft innerhalb desselben Spiels; erst nach der vollständigen Folge beginnt bei freien Budgets ein neues Spiel. Änderungen sind während einer aktiven oder pausierten Session gesperrt.</p>
        {queueError && <p role="alert">{queueError}</p>}
        <div className="run-grid">{catalog?.runs.map((run) => {
          const available = run.status === "available" || run.status === "runtime_validation_required";
          const alreadyQueued = queue.includes(run.run_id);
          return <article key={run.run_id}><strong>{run.display_name}</strong><span>{run.status}</span>{run.reasons?.map((reason) => <small key={reason}>{reason}</small>)}<button type="button" aria-label={`${run.display_name} zur Queue hinzufügen`} disabled={editorLocked || !available || alreadyQueued} title={alreadyQueued ? "Dieser Run ist bereits in der Reihenfolge enthalten." : undefined} onClick={() => setQueue((current) => current.includes(run.run_id) ? current : [...current, run.run_id])}>{alreadyQueued ? "Bereits enthalten" : "Zur Queue hinzufügen"}</button></article>;
        }) ?? <p>Katalog wird geladen …</p>}</div>
        <h3>Queue-Entwurf</h3>
        {queue.length === 0 ? <p>Die Queue ist leer und kann nicht gestartet werden.</p> : <ol className="queue-list">{queue.map((runID, index) => <li key={`${runID}-${index}`}><span>{index + 1}</span><strong>{runID}</strong><div className="queue-actions"><button type="button" className="secondary" aria-label={`${runID} an Position ${index + 1} nach oben`} disabled={editorLocked || index === 0} onClick={() => setQueue((current) => moveEntry(current, index, index - 1))}>↑</button><button type="button" className="secondary" aria-label={`${runID} an Position ${index + 1} nach unten`} disabled={editorLocked || index === queue.length - 1} onClick={() => setQueue((current) => moveEntry(current, index, index + 1))}>↓</button><button type="button" className="secondary" aria-label={`${runID} an Position ${index + 1} entfernen`} disabled={editorLocked} onClick={() => setQueue((current) => current.filter((_, itemIndex) => itemIndex !== index))}>Entfernen</button></div></li>)}</ol>}
        <div className="queue-toolbar">
          <button type="button" className="secondary" disabled={editorLocked} onClick={() => setQueue([...yamlDefault])}>Auf YAML-Default zurücksetzen</button>
          <button type="button" disabled={editorLocked || queue.length === 0 || !status?.selection.character} onClick={() => void submitQueue()}>{commandPending ? "Core bestätigt …" : "Queue prüfen und starten"}</button>
        </div>
        {status && <div className="queue-status" aria-live="polite"><strong>Core-Queue:</strong> {status.queue.entries.length ? status.queue.entries.join(" → ") : "keine aktive Queue"}<span>Spiel {status.game_id || "–"} · Spielzyklus {status.queue.cycle + 1} · Lifecycle {status.lifecycle_phase}</span><span>Index {status.queue.index + 1} · Retry {status.queue.retry} · Run-ID {status.run_id || "–"}</span><span>Gestartet {status.queue.started_runs}/{status.queue.budgets.max_runs} · Restarts {status.queue.total_restarts}/{status.queue.budgets.max_total_restarts}</span>{status.active_run_id && <span>Aktiv: {status.active_run_id}{status.step ? ` · ${status.step}` : ""}</span>}{hasPendingIntent && <span>Vorgemerkt: {status.pending_intent}</span>}{status.last_result && <span>Letztes Ergebnis: {status.last_result.disposition}{status.last_result.reason ? ` · ${status.last_result.reason}` : ""}</span>}</div>}
        <div className="session-controls">
          <button type="button" disabled={commandPending || status?.state !== "running_run" || hasPendingIntent} onClick={() => status && void runCommand(() => pauseAfterRun(status.generation))}>Nach aktuellem Run pausieren</button>
          <button type="button" disabled={commandPending || status?.state !== "paused_between_runs"} onClick={() => status && void runCommand(() => resumeQueue(status.generation))}>Queue fortsetzen</button>
          <button type="button" disabled={commandPending || status?.state !== "running_run" || hasPendingIntent} onClick={() => status && void runCommand(() => stopAfterRun(status.generation))}>Nach aktuellem Run stoppen</button>
          <button type="button" className="danger" disabled={commandPending || !status || !emergencyStates.has(status.state)} onClick={() => setConfirmEmergency(true)}>Emergency Stop</button>
        </div>
        <p className="hint">Die globale Pause-Taste merkt „nach aktuellem Run pausieren“ vor, ohne D2R den Fokus zu nehmen. Pause wartet auf Loot und den sicheren Town-Handoff und lässt das aktuelle Spiel geöffnet. Fortsetzen revalidiert dasselbe Spiel. „Nach aktuellem Run stoppen“ verlässt das Spiel danach genau einmal. Emergency Stop und F11 brechen sofort ab und garantieren kein Save &amp; Exit.</p>
        {confirmEmergency && <div className="modal-backdrop"><div role="dialog" aria-modal="true" aria-labelledby="emergency-title" className="modal danger-modal"><h3 id="emergency-title">Session sofort abbrechen?</h3><p>Der aktuelle Input wird sofort gesperrt. Save &amp; Exit ist nicht garantiert. Dies entspricht F11 im Spiel.</p><div className="modal-actions"><button type="button" className="secondary" onClick={() => setConfirmEmergency(false)}>Abbrechen</button><button ref={emergencyConfirmRef} type="button" className="danger" onClick={() => { setConfirmEmergency(false); if (status) void runCommand(() => emergencyStop(status.generation)); }}>Emergency Stop bestätigen</button></div></div></div>}
      </section>

      <section>
        <h2>Live-Ereignisse</h2>
        {events.length === 0 ? <p>Noch keine Zustandsänderung.</p> : <ol>{events.map((event) => <li key={event.sequence}><time>{new Date(event.timestamp).toLocaleTimeString("de-DE")}</time><strong>{event.event}</strong><span>{event.area || event.step || event.reason || "Core-Aktualisierung"}</span></li>)}</ol>}
      </section>
    </main>
  );
}

function moveEntry(entries: string[], from: number, to: number): string[] {
  const next = [...entries];
  const [entry] = next.splice(from, 1);
  next.splice(to, 0, entry);
  return next;
}
