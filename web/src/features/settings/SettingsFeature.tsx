import { useEffect, useMemo, useState } from "react";
import { FolderOpen, RefreshCw, RotateCcw, Save } from "lucide-react";
import { confirmHistoryDeleteAll, createDiagnosticBundle, previewHistoryDeleteAll, restoreOperatorSettings, saveOperatorSettings } from "../../api/client";
import { getOperatorSettings, previewOperatorSettings, previewResetOperatorSettings, type HistoryDeletePreviewDTO, type LiveEvent, type OperatorSettingsChangeDTO, type OperatorSettingsDTO, type RouteCombatConfigDTO } from "../../api/generated";
import { Button, Dialog, StateMessage, StatusBadge } from "../../app/ui";

export function SettingsFeature({ generation, coreState, characters, runs, events, onOpenOnboarding }: { generation: number; coreState: string; characters: string[]; runs: Array<{ id: string; label: string; routeCombat?: RouteCombatConfigDTO }>; events: LiveEvent[]; onOpenOnboarding?: () => void }) {
  const [settings, setSettings] = useState<OperatorSettingsDTO | null>(null);
  const [draft, setDraft] = useState<OperatorSettingsDTO | null>(null);
  const [desktop, setDesktop] = useState<DesktopSettingsView | null>(null);
  const [selectedCharacter, setSelectedCharacter] = useState("");
  const [preview, setPreview] = useState<{ mode: "save" | "reset"; change: OperatorSettingsChangeDTO } | null>(null);
  const [restartRequired, setRestartRequired] = useState(false);
  const [stale, setStale] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [queueCandidate, setQueueCandidate] = useState("");
  const [deletePreview, setDeletePreview] = useState<HistoryDeletePreviewDTO | null>(null);
  const [updateStatus, setUpdateStatus] = useState<DesktopUpdateStatus | null>(null);
  const [includeTelemetry, setIncludeTelemetry] = useState(false);
  const [includeRoutes, setIncludeRoutes] = useState(false);
  const mutable = coreState === "idle" || coreState === "stopped_error";

  const load = async () => {
    setBusy(true);
    setError("");
    try {
      const [operator, desktopSettings, currentUpdateStatus] = await Promise.all([
        getOperatorSettings(),
        window.d2rDesktop?.getDesktopSettings() ?? Promise.resolve(null),
        window.d2rDesktop?.getUpdateStatus?.() ?? Promise.resolve(null),
      ]);
      setSettings(operator);
      setDraft(cloneSettings(operator));
      setDesktop(desktopSettings);
      setUpdateStatus(currentUpdateStatus);
      const names = Object.keys(operator.characters).sort((left, right) => left.localeCompare(right, "de"));
      setSelectedCharacter((current) => current && operator.characters[current] ? current : names[0] ?? "");
      setPreview(null);
      setRestartRequired(false);
      setStale(false);
    } catch (reason) {
      setError(errorText(reason, "Einstellungen konnten nicht geladen werden."));
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => { void load(); }, []);
  useEffect(() => window.d2rDesktop?.onUpdateStatus?.(setUpdateStatus), []);

  const characterNames = useMemo(() => {
    const names = new Set([...characters, ...Object.keys(draft?.characters ?? {})]);
    return [...names].sort((left, right) => left.localeCompare(right, "de"));
  }, [characters, draft]);
  const characterSettings = draft?.characters[selectedCharacter];

  const changeDraft = (update: (current: OperatorSettingsDTO) => OperatorSettingsDTO) => {
    setDraft((current) => current ? update(cloneSettings(current)) : current);
    setPreview(null);
    setMessage("");
  };

  const previewSave = async () => {
    if (!draft || !settings) return;
    await run(async () => setPreview({ mode: "save", change: await previewOperatorSettings({ expected_revision: settings.revision, expected_generation: generation, settings: draft }) }));
  };

  const previewReset = async () => {
    if (!settings) return;
    await run(async () => setPreview({ mode: "reset", change: await previewResetOperatorSettings({ expected_revision: settings.revision, expected_generation: generation }) }));
  };

  const applyPreview = async () => {
    if (!preview || !settings || !draft) return;
    await run(async () => {
      const result = preview.mode === "reset"
        ? await restoreOperatorSettings({ expected_revision: settings.revision, expected_generation: generation })
        : await saveOperatorSettings({ expected_revision: settings.revision, expected_generation: generation, settings: draft });
      setSettings(result.settings);
      setDraft(cloneSettings(result.settings));
      setRestartRequired(result.restart_required);
      setPreview(null);
      setMessage(result.changed_fields.length ? "Einstellungen wurden revisionsgebunden gespeichert." : "Es waren keine Änderungen zu speichern.");
    });
  };

  const saveDesktop = async () => {
    if (!desktop || !window.d2rDesktop) return;
    await run(async () => {
      setDesktop(await window.d2rDesktop!.updateDesktopSettings({ autostart: desktop.autostart, onboarding_completed: desktop.onboarding_completed }));
      setMessage("Desktop-Einstellungen wurden atomar gespeichert.");
    });
  };

  const restartCore = async () => {
    if (!window.d2rDesktop) return;
    await run(async () => {
      await window.d2rDesktop!.restartCore();
      setMessage("Der Core wurde kontrolliert neu gestartet.");
      setRestartRequired(false);
    });
  };

  const previewDeleteHistory = async () => {
    await run(async () => setDeletePreview(await previewHistoryDeleteAll(generation)));
  };

  const deleteHistory = async () => {
    if (!deletePreview) return;
    await run(async () => {
      const result = await confirmHistoryDeleteAll({
        expected_generation: generation,
        confirmation_token: deletePreview.confirmation_token,
        index_generation: deletePreview.index_generation,
        candidate_files: deletePreview.candidate_files,
        candidate_bytes: deletePreview.candidate_bytes,
      });
      setDeletePreview(null);
      setMessage(`${result.deleted_files} Historiendatei(en) wurden dauerhaft gelöscht; ${result.protected_files} aktive Datei(en) blieben geschützt.`);
    });
  };

  const checkForUpdates = async () => {
    if (!window.d2rDesktop?.checkForUpdates) return;
    await run(async () => setUpdateStatus(await window.d2rDesktop!.checkForUpdates!()));
  };

  const buildDiagnosticBundle = async () => {
    await run(async () => {
      const result = await createDiagnosticBundle({ include_telemetry: includeTelemetry, include_routes: includeRoutes });
      setMessage(`Diagnosepaket ${result.filename} wurde lokal erstellt (${result.bytes.toLocaleString("de-DE")} Byte).`);
      await window.d2rDesktop?.revealDiagnosticBundle?.(result.filename);
    });
  };

  const run = async (action: () => void | Promise<void>) => {
    if (busy) return;
    setBusy(true);
    setError("");
    setStale(false);
    try {
      await action();
    } catch (reason) {
      const text = errorText(reason, "Einstellungsänderung fehlgeschlagen.");
      setError(text);
      if (/revision|zwischenzeitlich|zustand hat sich geändert/i.test(text)) setStale(true);
    } finally {
      setBusy(false);
    }
  };

  if (!settings || !draft) {
    return error
      ? <StateMessage kind="error" title="Einstellungen nicht verfügbar">{error}</StateMessage>
      : <StateMessage kind="loading" title="Einstellungen werden geladen">Core- und Desktopverträge werden gemeinsam gelesen.</StateMessage>;
  }

  return <>
    {!mutable && <StateMessage kind="error" title="Diese Einstellungen sind gesperrt">Die Felder und Speicheraktionen sind derzeit nicht änderbar. Speichern ist erst wieder möglich, wenn der Core vollständig inaktiv ist.</StateMessage>}
    {stale && <div className="settings-feedback"><StateMessage kind="error" title="Revision ist veraltet">Lade den aktuellen Core-Stand neu, bevor du erneut änderst.</StateMessage><Button variant="secondary" onClick={() => void load()}>Aktuellen Stand laden</Button></div>}
    {error && !stale && <StateMessage kind="error" title="Änderung fehlgeschlagen">{error}</StateMessage>}
    {message && <div className="settings-success" role="status"><strong>{message}</strong></div>}
    {restartRequired && <div className="restart-required" role="status"><div><strong>Core-Neustart erforderlich</strong><p>Input- oder Hotkeyressourcen bleiben bis zu einem kontrollierten Neustart unverändert.</p></div><Button onClick={() => void restartCore()} disabled={!mutable || busy || !window.d2rDesktop}>Core kontrolliert neu starten</Button></div>}

    <section>
      <div className="section-heading"><div><p className="eyebrow">Allgemein</p><h2>Desktop</h2></div><StatusBadge tone={desktop ? "success" : "neutral"}>{desktop ? "Electron-Store" : "Bridge nicht verfügbar"}</StatusBadge></div>
      {desktop ? <div className="settings-grid">
        <label className="check"><input type="checkbox" checked={desktop.autostart} onChange={(event) => setDesktop({ ...desktop, autostart: event.target.checked })} /> App mit Windows starten</label>
        <p className="hint">Onboarding: <strong>{desktop.onboarding_completed ? "abgeschlossen" : "noch offen"}</strong></p>
        <p className="hint">Autostart ist standardmäßig aus und startet weder D2R noch eine Farming-Session. Fensterbounds verwaltet ausschließlich Electron Main.</p>
        <div className="inline-actions"><Button onClick={() => void saveDesktop()} disabled={busy}><Save aria-hidden="true" size={16} /> Desktop-Einstellungen speichern</Button>{onOpenOnboarding && <Button variant="secondary" onClick={onOpenOnboarding}>First-Run-Assistent öffnen</Button>}</div>
      </div> : <StateMessage kind="empty" title="Desktop-Bridge nicht aktiv">Im installierten Produkt werden hier Autostart und Onboarding aus `desktop-settings.json` verwaltet.</StateMessage>}
    </section>

    <section>
      <div className="section-heading"><div><p className="eyebrow">Version</p><h2>Produktaktualisierung</h2></div><StatusBadge tone={updateStatus?.status === "available" ? "warning" : updateStatus?.status === "up_to_date" ? "success" : "neutral"}>{updateLabel(updateStatus)}</StatusBadge></div>
      <p>{updateDescription(updateStatus)}</p>
      <div className="inline-actions">
        <Button variant="secondary" onClick={() => void checkForUpdates()} disabled={busy || !window.d2rDesktop?.checkForUpdates}><RefreshCw aria-hidden="true" size={16} /> Erneut prüfen</Button>
        {updateStatus?.status === "available" && <Button onClick={() => void window.d2rDesktop?.openReleasePage?.()}>Feste Release-Seite öffnen</Button>}
      </div>
      <p className="hint">Es gibt keinen automatischen Download und keine Installation. Fehler, Offlinebetrieb und private/fehlende Releases bleiben neutral.</p>
    </section>

    <section>
      <div className="section-heading"><div><p className="eyebrow">Charakter</p><h2>Queue und Schwierigkeit</h2></div><StatusBadge tone="neutral">Revision {settings.revision}</StatusBadge></div>
      {characterNames.length === 0 || !characterSettings ? <StateMessage kind="empty" title="Keine Charakterwerte">Der Core hat noch keine charakterbezogenen Operatorwerte geliefert.</StateMessage> : <>
        <div className="settings-grid two-columns">
          <label>Charakter<select value={selectedCharacter} onChange={(event) => setSelectedCharacter(event.target.value)} disabled={!mutable}>{characterNames.map((name) => <option key={name} value={name}>{name}</option>)}</select></label>
          <label>Letzte Schwierigkeit<select value={characterSettings.last_difficulty} disabled={!mutable} onChange={(event) => changeDraft((next) => { next.characters[selectedCharacter].last_difficulty = event.target.value as "normal" | "nightmare" | "hell"; return next; })}><option value="normal">Normal</option><option value="nightmare">Alptraum</option><option value="hell">Hölle</option></select></label>
        </div>
        <ol className="queue-list settings-queue">{characterSettings.queue.map((runID, index) => <li key={runID}><span>{index + 1}</span><strong>{runs.find((run) => run.id === runID)?.label ?? runID}</strong><div className="queue-actions"><Button variant="secondary" aria-label={`${runID} nach oben`} disabled={!mutable || index === 0} onClick={() => changeDraft((next) => { next.characters[selectedCharacter].queue = move(next.characters[selectedCharacter].queue, index, index - 1); return next; })}>↑</Button><Button variant="secondary" aria-label={`${runID} nach unten`} disabled={!mutable || index === characterSettings.queue.length - 1} onClick={() => changeDraft((next) => { next.characters[selectedCharacter].queue = move(next.characters[selectedCharacter].queue, index, index + 1); return next; })}>↓</Button><Button variant="secondary" disabled={!mutable} onClick={() => changeDraft((next) => { next.characters[selectedCharacter].queue = next.characters[selectedCharacter].queue.filter((entry) => entry !== runID); return next; })}>Entfernen</Button></div></li>)}</ol>
        <div className="inline-actions"><label>Run hinzufügen<select value={queueCandidate} disabled={!mutable} onChange={(event) => setQueueCandidate(event.target.value)}><option value="">Bitte wählen</option>{runs.filter((run) => !characterSettings.queue.includes(run.id)).map((run) => <option key={run.id} value={run.id}>{run.label}</option>)}</select></label><Button variant="secondary" disabled={!mutable || !queueCandidate} onClick={() => { changeDraft((next) => { if (!next.characters[selectedCharacter].queue.includes(queueCandidate)) next.characters[selectedCharacter].queue.push(queueCandidate); return next; }); setQueueCandidate(""); }}>Hinzufügen</Button></div>
      </>}
    </section>

    <section>
      <div className="section-heading"><div><p className="eyebrow">Sicherheit</p><h2>Budgets, Input und Hotkeys</h2></div></div>
      <div className="settings-grid four-columns">
        <NumberField label="Maximale Runs" value={draft.budgets.max_runs} disabled={!mutable} onChange={(value) => changeDraft((next) => { next.budgets.max_runs = value; return next; })} />
        <NumberField label="Maximale Dauer (ms)" value={draft.budgets.max_duration_ms} disabled={!mutable} onChange={(value) => changeDraft((next) => { next.budgets.max_duration_ms = value; return next; })} />
        <NumberField label="Fehler in Folge" value={draft.budgets.max_consecutive_failures} disabled={!mutable} onChange={(value) => changeDraft((next) => { next.budgets.max_consecutive_failures = value; return next; })} />
        <NumberField label="Gesamte Restarts" value={draft.budgets.max_total_restarts} disabled={!mutable} onChange={(value) => changeDraft((next) => { next.budgets.max_total_restarts = value; return next; })} />
      </div>
      <label className="check"><input type="checkbox" checked={draft.input.enabled} disabled={!mutable} onChange={(event) => changeDraft((next) => { next.input.enabled = event.target.checked; return next; })} /> Gameplay-Input ausdrücklich freigeben</label>
      <div className="settings-grid four-columns">
        <TextField label="Pause" value={draft.input.pause_hotkey} disabled={!mutable} onChange={(value) => changeDraft((next) => { next.input.pause_hotkey = value; return next; })} />
        <TextField label="Stop nach Run" value={draft.input.stop_after_run_hotkey} disabled={!mutable} onChange={(value) => changeDraft((next) => { next.input.stop_after_run_hotkey = value; return next; })} />
        <TextField label="Aufnahme beenden" value={draft.input.recording_finish_hotkey} disabled={!mutable} onChange={(value) => changeDraft((next) => { next.input.recording_finish_hotkey = value; return next; })} />
        <TextField label="Emergency Stop" value={draft.input.emergency_stop_hotkey} disabled={!mutable} onChange={(value) => changeDraft((next) => { next.input.emergency_stop_hotkey = value; return next; })} />
      </div>
    </section>

    <section>
      <div className="section-heading"><div><p className="eyebrow">Historie</p><h2>Aufbewahrung</h2></div></div>
      <div className="settings-grid two-columns"><label className="check"><input type="checkbox" checked={draft.history.retention_enabled} disabled={!mutable} onChange={(event) => changeDraft((next) => { next.history.retention_enabled = event.target.checked; return next; })} /> Automatische Retention aktiv</label><NumberField label="Retention in Tagen" value={draft.history.retention_days} disabled={!mutable} onChange={(value) => changeDraft((next) => { next.history.retention_days = value; return next; })} /></div>
      <div className="danger-zone"><h3>Gesamte Historie löschen</h3><p>Die Vorschau umfasst alle direkten JSONL-Kategorien. Aktive Writer bleiben auch nach der zweiten Bestätigung geschützt. Es gibt weder Papierkorb noch Telemetriebackup.</p><Button variant="danger" onClick={() => void previewDeleteHistory()} disabled={!mutable || busy}>Löschvorschau erstellen</Button></div>
    </section>

    <section>
      <div className="section-heading"><div><p className="eyebrow">Revisionierter Vertrag</p><h2>Prüfen und speichern</h2></div></div>
      <p>Jede Mutation verwendet die erwartete Store-Revision {settings.revision} und Coregeneration {generation}. Der Core validiert den Gesamtstand und bleibt die einzige Autorität.</p>
      <div className="inline-actions"><Button onClick={() => void previewSave()} disabled={!mutable || busy}><Save aria-hidden="true" size={16} /> Änderungen prüfen</Button><Button variant="danger" onClick={() => void previewReset()} disabled={!mutable || busy}><RotateCcw aria-hidden="true" size={16} /> Sichere Defaults vorschauen</Button></div>
      <details className="effective-settings"><summary>Effektive erweiterte Werte (read-only)</summary><p>Datei: <code>configs/operator-settings.local.yaml</code></p><pre>{JSON.stringify(settings, null, 2)}</pre></details>
      <details className="effective-settings"><summary>Effektive Route-Combat-Werte (read-only)</summary><p>Vom Core nach Defaults und Validierung projiziert; <code>enabled: false</code> deaktiviert das Interleave ohne Änderung der Route.</p><pre>{JSON.stringify(Object.fromEntries(runs.map((run) => [run.id, run.routeCombat ?? null])), null, 2)}</pre></details>
    </section>

    <section>
      <div className="section-heading"><div><p className="eyebrow">Diagnose</p><h2>Lokales Diagnosepaket</h2></div><StatusBadge tone="neutral">kein Upload</StatusBadge></div>
      <p>Der Go-Core wählt feste Inhalte aus und redigiert Tokens sowie absolute Benutzerpfade. Spielstände, Speicherabbilder und Screenshots sind immer ausgeschlossen.</p>
      <div className="settings-grid two-columns">
        <label className="check"><input type="checkbox" checked={includeTelemetry} onChange={(event) => setIncludeTelemetry(event.target.checked)} /> Vollständige Telemetrie ausdrücklich beilegen</label>
        <label className="check"><input type="checkbox" checked={includeRoutes} onChange={(event) => setIncludeRoutes(event.target.checked)} /> Routenkoordinaten ausdrücklich beilegen</label>
      </div>
      <Button onClick={() => void buildDiagnosticBundle()} disabled={busy}><FolderOpen aria-hidden="true" size={16} /> Redigiertes ZIP lokal erstellen</Button>
    </section>

    <section>
      <div className="section-heading"><div><p className="eyebrow">Diagnose</p><h2>Live-Ereignisse</h2></div><StatusBadge tone="neutral">maximal 40</StatusBadge></div>
      {events.length === 0 ? <StateMessage kind="empty" title="Noch keine Zustandsänderung">Der Rohfeed bleibt eine begrenzte Diagnoseprojektion.</StateMessage> : <ol className="event-feed">{events.map((event) => <li key={event.sequence}><time>{new Date(event.timestamp).toLocaleTimeString("de-DE")}</time><strong>{event.event}</strong><span>{event.area || event.step || event.reason || "Core-Aktualisierung"}</span></li>)}</ol>}
    </section>

    {deletePreview && <Dialog title="Gesamte Historie dauerhaft löschen?" onClose={() => !busy && setDeletePreview(null)}><p><strong>{deletePreview.candidate_files}</strong> direkte Datei(en) mit {deletePreview.candidate_bytes.toLocaleString("de-DE")} Byte sind vorgesehen. {deletePreview.protected_files} aktive Datei(en) sind geschützt.</p><p>Kategorien: {Object.entries(deletePreview.categories).map(([name, count]) => `${name}: ${count}`).join(" · ") || "keine"}</p><StateMessage kind="error" title="Nicht rückgängig zu machen">Die Metadaten und Indexgeneration werden unmittelbar erneut geprüft. Veraltete Vorschauen werden vollständig abgelehnt.</StateMessage><div className="modal-actions"><Button variant="secondary" onClick={() => setDeletePreview(null)} disabled={busy}>Abbrechen</Button><Button variant="danger" onClick={() => void deleteHistory()} disabled={busy || deletePreview.candidate_files === 0}>Alles endgültig löschen</Button></div></Dialog>}

    {preview && <Dialog title={preview.mode === "reset" ? "Sichere Defaults anwenden?" : "Änderungen speichern?"} onClose={() => !busy && setPreview(null)}><p>Geänderte Felder: <strong>{preview.change.changed_fields.join(", ") || "keine"}</strong></p>{preview.change.restart_required && <StateMessage kind="error" title="Core-Neustart erforderlich">Input und Hotkeys werden erst nach dem kontrollierten Neustart wirksam.</StateMessage>}<div className="modal-actions"><Button variant="secondary" onClick={() => setPreview(null)} disabled={busy}>Abbrechen</Button><Button variant={preview.mode === "reset" ? "danger" : "primary"} onClick={() => void applyPreview()} disabled={busy}>{preview.mode === "reset" ? "Defaults anwenden" : "Revisionsgebunden speichern"}</Button></div></Dialog>}
  </>;
}

function NumberField({ label, value, onChange, disabled }: { label: string; value: number; onChange: (value: number) => void; disabled?: boolean }) {
  return <label>{label}<input type="number" min={1} step={1} value={value} disabled={disabled} onChange={(event) => onChange(Number(event.target.value))} /></label>;
}

function TextField({ label, value, onChange, disabled }: { label: string; value: string; onChange: (value: string) => void; disabled?: boolean }) {
  return <label>{label}<input value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)} /></label>;
}

function cloneSettings(settings: OperatorSettingsDTO): OperatorSettingsDTO {
  return JSON.parse(JSON.stringify(settings)) as OperatorSettingsDTO;
}

function move<T>(entries: T[], from: number, to: number): T[] {
  const result = [...entries];
  const [entry] = result.splice(from, 1);
  result.splice(to, 0, entry);
  return result;
}

function errorText(reason: unknown, fallback: string): string {
  return reason instanceof Error ? reason.message : fallback;
}

function updateLabel(status: DesktopUpdateStatus | null): string {
  if (!status) return "Desktop-Bridge fehlt";
  if (status.status === "checking") return "Prüfung läuft";
  if (status.status === "available") return "Neue Version";
  if (status.status === "up_to_date") return "Aktuell";
  return "Nicht verfügbar";
}

function updateDescription(status: DesktopUpdateStatus | null): string {
  if (!status) return "Der Versionshinweis ist nur in der Desktop-App verfügbar.";
  if (status.status === "checking") return `Version ${status.current_version} wird einmalig geprüft.`;
  if (status.status === "available") return `Installiert: ${status.current_version}. Veröffentlicht: ${status.latest_version}.`;
  if (status.status === "up_to_date") return `Installiert: ${status.current_version}. Kein neueres stabiles Release gefunden.`;
  return `Installiert: ${status.current_version}. Die Prüfung ist derzeit neutral nicht verfügbar.`;
}
