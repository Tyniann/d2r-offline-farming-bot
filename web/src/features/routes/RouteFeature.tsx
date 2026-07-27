import { useEffect, useRef, useState } from "react";
import { confirmRouteMutation, finishRouteRecording, previewRouteMutation, startRouteWorkflow } from "../../api/client";
import {
  getHotkeyHelp, getRecordingOptions, getRouteCandidates, getRouteLibrary, getRouteWorkflow, getSystemRouteStatus,
  type HotkeyHelpDTO, type RecordingOptionDTO, type RouteCandidateDTO, type RouteEntryDTO,
  type RouteMutationPreviewDTO, type RouteWorkflowDTO, type SystemRouteStatusDTO,
} from "../../api/generated";

interface Props {
  characters: string[];
  selectedCharacter: string;
  refreshKey: number;
  liveLocked?: boolean;
  preferredRecordingRun?: string;
  onReturnToOnboarding?(): void;
}
const terminalWorkflowStates = new Set(["idle", "completed", "failed_safe", "emergency_cancelled"]);

const workflowLabels: Record<string, string> = {
  idle: "Bereit", preflight: "Start wird geprüft", recording: "Aufnahme läuft", freezing: "Aufnahme wird eingefroren",
  validating: "Kandidat wird geprüft", returning_via_portal: "Rückkehr per Town Portal", candidate_ready: "Kandidat ist bereit",
  preparing_playback: "Test wird vorbereitet", playing_candidate: "Kandidat wird abgespielt", validating_terminal: "Ziel wird geprüft",
  returning_after_test: "Rückkehr nach dem Test", awaiting_publish_confirmation: "Freigabe steht aus", publishing: "Route wird veröffentlicht",
  completed: "Abgeschlossen", failed_safe: "Sicher abgebrochen", emergency_cancelled: "Notabbruch",
};

const candidateLabels: Record<string, string> = {
  recorded: "Aufgenommen", validated: "Bereit zum Test", test_running: "Test läuft", test_passed: "Test bestanden", failed: "Abgelehnt",
};

const reasonLabels: Record<string, string> = {
  input_disabled: "Gameplay-Input ist in der Konfiguration deaktiviert.",
  selection_unconfirmed: "Charakter und Schwierigkeit müssen zuerst im Core bestätigt sein.",
  route_workflow_active: "Ein anderer Routen-Workflow ist aktiv.",
  session_active: "Eine Farming-Session ist aktiv.",
  recording_preflight_failed: "Der Core wartet auf den bestätigten Startwegpunkt.",
  recording_start_area_mismatch: "Startgebiet oder Startwegpunkt stimmen nicht.",
  recording_terminal_area_mismatch: "Das registrierte Zielgebiet wurde nicht bestätigt.",
  recording_boss_missing: "Der registrierte lebende Boss wurde nicht bestätigt.",
  recording_boss_dead: "Der Boss muss beim Aufnahmeende noch leben.",
  recording_endpoint_too_far: "Die gewählte Endposition ist zu weit vom Boss entfernt.",
  pickit_assignment_missing: "Für diesen Charakter und Run ist noch kein Lootprofil zugeordnet.",
  route_test_playback_failed: "Die isolierte Wiedergabe ist fehlgeschlagen.",
  route_test_terminal_mismatch: "Die Zielprüfung nach der Wiedergabe ist fehlgeschlagen.",
  route_safety_return_failed: "Die sichere Rückkehr per Town Portal ist fehlgeschlagen.",
};

function reasonLabel(reason?: string): string {
  return reason ? (reasonLabels[reason] ?? reason) : "";
}

function operationLabel(operation: string): string {
  return ({ publish: "Veröffentlichen", replace: "Ersetzen", archive: "Archivieren", restore: "Wiederherstellen", delete: "Endgültig löschen" } as Record<string, string>)[operation] ?? operation;
}

function workflowInstruction(workflow: RouteWorkflowDTO, hotkeys: HotkeyHelpDTO | null): string {
  const finish = hotkeys?.recording_finish ?? "F9";
  const emergency = hotkeys?.emergency_stop ?? "F11";
  switch (workflow.state) {
    case "preflight": return workflow.act
      ? `Am Portal-Ankunftspunkt stehen bleiben. Der Core startet die Aufnahme erst nach der Memory-Bestätigung.`
      : `Am angezeigten Startwegpunkt stehen bleiben. Erst bei „recording“ loslegen.`;
    case "recording": return workflow.act
      ? `Jetzt ohne Teleport zum lokalen Wegpunkt laufen und dort ${finish} drücken. ${emergency} bricht sofort ab.`
      : `Jetzt die angezeigte Farming-Route bis zur selbst gewählten Kampfposition aufzeichnen. Den Boss nicht angreifen: Er muss für die Prüfung leben. Dort ${finish} drücken; ${emergency} bricht sofort ab.`;
    case "freezing": return "Keine Eingabe: Der Core friert den unveränderlichen Kandidaten ein.";
    case "validating": return "Keine Eingabe: Terminalgebiet, lebender Boss und Enddistanz werden geprüft.";
    case "returning_via_portal": return "Keine Eingabe: Der Core öffnet das Town Portal und kehrt sicher ins Dorf zurück.";
    case "candidate_ready": return "Der Kandidat ist gespeichert. Als Nächstes im Kandidatenreview „Isoliert testen“ wählen.";
    case "preparing_playback": return "Keine Eingabe: Der Core normalisiert Town/Egress und reist zum registrierten Startwegpunkt.";
    case "playing_candidate": return "Keine Eingabe: Der Kandidat wird ohne Combat, Loot oder Town Services abgespielt.";
    case "validating_terminal": return "Keine Eingabe: Zielgebiet, lebender Boss und Enddistanz werden erneut geprüft.";
    case "returning_after_test": return "Keine Eingabe: Der Core kehrt nach dem Test per Town Portal ins Dorf zurück.";
    case "awaiting_publish_confirmation": return "Der Test ist bestanden. Die Veröffentlichung benötigt genau eine bewusste Bestätigung.";
    case "publishing": return "Die neue Route wird atomisch zugeordnet; der Vorgänger wird unverändert archiviert.";
    case "failed_safe": case "emergency_cancelled": return reasonLabel(workflow.reason) || "Der Workflow wurde ohne Veröffentlichung beendet.";
    default: return "";
  }
}

export function RouteFeature({ characters, selectedCharacter, refreshKey, liveLocked = false, preferredRecordingRun = "", onReturnToOnboarding }: Props) {
  const [character, setCharacter] = useState(selectedCharacter);
  const [archive, setArchive] = useState(false);
  const [routes, setRoutes] = useState<RouteEntryDTO[] | null>(null);
  const [candidates, setCandidates] = useState<RouteCandidateDTO[]>([]);
  const [options, setOptions] = useState<RecordingOptionDTO[]>([]);
  const [system, setSystem] = useState<SystemRouteStatusDTO[]>([]);
  const [hotkeys, setHotkeys] = useState<HotkeyHelpDTO | null>(null);
  const [workflow, setWorkflow] = useState<RouteWorkflowDTO | null>(null);
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<RouteMutationPreviewDTO | null>(null);
  const [deleteConfirmation, setDeleteConfirmation] = useState("");
  const [pending, setPending] = useState(false);
  const confirmRef = useRef<HTMLButtonElement>(null);
  const deleteRef = useRef<HTMLInputElement>(null);
  const workflowBusy = !!workflow && !terminalWorkflowStates.has(workflow.state);
  const liveActionLocked = liveLocked || pending || workflowBusy;
  const visibleCandidates = candidates.filter((candidate) => candidate.character.toLocaleLowerCase() === character.toLocaleLowerCase());
  const activeWorkflowInstruction = workflow ? workflowInstruction(workflow, hotkeys) : "";
  const orderedOptions = [...options].sort((left, right) => Number(right.run_id === preferredRecordingRun) - Number(left.run_id === preferredRecordingRun));

  const refresh = async (signal?: AbortSignal) => {
    try {
      const [library, nextCandidates, nextOptions, nextSystem, nextHotkeys, nextWorkflow] = await Promise.all([
        getRouteLibrary(character, archive, signal), getRouteCandidates(signal), getRecordingOptions(signal),
        getSystemRouteStatus(signal), getHotkeyHelp(signal), getRouteWorkflow(signal),
      ]);
      setRoutes(library.routes.filter((entry) => archive ? entry.management_status === "archived" : entry.management_status !== "archived")); setCandidates(nextCandidates); setOptions(nextOptions); setSystem(nextSystem); setHotkeys(nextHotkeys); setWorkflow(nextWorkflow); setError("");
    } catch (reason) { if (!signal?.aborted) setError(reason instanceof Error ? reason.message : "Routen konnten nicht geladen werden"); }
  };

  useEffect(() => { const controller = new AbortController(); setRoutes(null); void refresh(controller.signal); return () => controller.abort(); }, [character, archive, refreshKey]);
  useEffect(() => { if (!character && selectedCharacter) setCharacter(selectedCharacter); }, [character, selectedCharacter]);
  useEffect(() => { if (!preview) return; setDeleteConfirmation(""); if (preview.operation === "delete") deleteRef.current?.focus(); else confirmRef.current?.focus(); const close = (event: KeyboardEvent) => { if (event.key === "Escape") setPreview(null); }; window.addEventListener("keydown", close); return () => window.removeEventListener("keydown", close); }, [preview]);

  const prepare = async (operation: string, routeId = "", candidateId = "") => { setPending(true); setError(""); try { setPreview(await previewRouteMutation(operation, routeId, candidateId)); } catch (reason) { setError(reason instanceof Error ? reason.message : "Vorschau fehlgeschlagen"); } finally { setPending(false); } };
  const confirm = async () => { if (!preview) return; setPending(true); try { await confirmRouteMutation(preview, deleteConfirmation); setPreview(null); await refresh(); } catch (reason) { setError(reason instanceof Error ? reason.message : "Bestätigung ist veraltet"); } finally { setPending(false); } };
  const start = async (operation: string, data: { runId?: string; candidateId?: string; act?: string }) => { if (!workflow) return; setPending(true); setError(""); try { setWorkflow(await startRouteWorkflow(operation, workflow.generation, data)); } catch (reason) { setError(reason instanceof Error ? reason.message : "Workflowstart fehlgeschlagen"); } finally { setPending(false); } };

  return <section aria-labelledby="routes-title">
    <h2 id="routes-title">Farming-Routen</h2>
    <p>Aufnahmen, Tests und Veröffentlichung verwenden denselben Core wie die CLI. Town- und Egress-Dateien erscheinen nie in dieser Bibliothek.</p>
    {onReturnToOnboarding && <div className="onboarding-return">
      <div><strong>Aus der Einrichtung geöffnet</strong><p>Du kannst Aufnahme, Test und Veröffentlichung hier abschließen und anschließend zum First-Run-Assistenten zurückkehren.</p></div>
      <button type="button" className="secondary" onClick={onReturnToOnboarding}>Zurück zur Einrichtung</button>
    </div>}
    <div className="route-toolbar">
      <label>Charakter<select value={character} onChange={(event) => setCharacter(event.target.value)}>{characters.map((name) => <option key={name}>{name}</option>)}</select></label>
      <button type="button" className="secondary" aria-pressed={archive} onClick={() => setArchive((value) => !value)}>{archive ? "Aktive Routen" : "Archiv anzeigen"}</button>
    </div>
    {error && <p role="alert">{error}</p>}
    {routes === null && !error && <p>Routen werden geladen …</p>}
    {routes?.length === 0 && <p>{archive ? "Das Archiv ist leer." : "Für diesen Charakter gibt es noch keine Farming-Route."}</p>}
    {!!routes?.length && <div className="run-grid">{routes.map((route) => <article key={route.route_id}><strong>{route.display_name}</strong><span>{route.run_id} · {route.difficulty}</span><small>{route.lifecycle_status} · {route.management_status}{route.assigned ? " · zugewiesen" : ""}</small><div className="route-actions">{archive ? <><button disabled={liveActionLocked} onClick={() => void prepare("restore", route.route_id)}>Wiederherstellen</button><button className="danger" disabled={liveActionLocked} onClick={() => void prepare("delete", route.route_id)}>Endgültig löschen</button></> : <button disabled={liveActionLocked} onClick={() => void prepare("archive", route.route_id)}>Archivieren</button>}</div></article>)}</div>}

    <h3>Geführte Aufnahme</h3>
    <div className="run-grid">{orderedOptions.map((option) => <article key={option.run_id} className={option.run_id === preferredRecordingRun ? "preferred-route" : undefined}><strong>{option.display_name}{option.run_id === preferredRecordingRun ? " · ausgewählt" : ""}</strong><p>{option.instructions_de}</p><small>Start: {option.start_waypoint} · Zielgebiet {option.terminal_area_id} · maximale Bossdistanz {option.terminal_max_distance_tiles.toFixed(0)} Tiles</small>{(option.prerequisites ?? []).map((entry) => <small key={entry.id}>{entry.id}: {entry.ready ? "bereit" : reasonLabel(entry.reason)}</small>)}{!option.available && option.reason && <small>{reasonLabel(option.reason)}</small>}<button disabled={liveActionLocked || !option.available || (option.prerequisites ?? []).some((entry) => !entry.ready)} onClick={() => void start("record", { runId: option.run_id })}>Aufnahme starten</button></article>)}</div>
    {workflow && <div className="queue-status" aria-live="polite"><strong>Routen-Workflow: {workflow.state}</strong><span>{workflowLabels[workflow.state] ?? workflow.state} · Generation {workflow.generation}{workflow.run_id ? ` · ${workflow.run_id}` : ""}{workflow.act ? ` · ${workflow.act.toUpperCase()}` : ""}</span>{activeWorkflowInstruction && <span>{activeWorkflowInstruction}</span>}{workflow.state === "recording" && <button disabled={liveLocked || pending} onClick={() => void finishRouteRecording(workflow.workflow_id, workflow.generation).then(setWorkflow).catch((reason: unknown) => setError(reason instanceof Error ? reason.message : "Finish fehlgeschlagen"))}>Aufnahme beenden</button>}{workflow.reason && workflow.state !== "failed_safe" && workflow.state !== "emergency_cancelled" && <span>{reasonLabel(workflow.reason)}</span>}</div>}

    <h3>Kandidatenreview</h3>
    {visibleCandidates.length === 0 ? <p>Für diesen Charakter gibt es noch keinen aufgenommenen Kandidaten.</p> : <div className="run-grid">{visibleCandidates.map((candidate) => <article key={candidate.candidate_id}><strong>{candidate.run_id}</strong><code>{candidate.candidate_id}</code><span>{candidate.character} · {candidate.difficulty}</span><small>{candidateLabels[candidate.state] ?? candidate.state} · Bossdistanz {candidate.measured_boss_distance.toFixed(1)} Tiles</small>{candidate.reason && <small>{reasonLabel(candidate.reason)}</small>}<div className="route-actions"><button disabled={liveActionLocked || candidate.state !== "validated"} onClick={() => void start("test", { candidateId: candidate.candidate_id })}>{candidate.state === "test_passed" ? "Test bestanden" : "Isoliert testen"}</button><button disabled={liveActionLocked || candidate.state !== "test_passed"} onClick={() => void prepare("publish", "", candidate.candidate_id)}>Veröffentlichen</button></div></article>)}</div>}

    <h3>System-Egress-Setup</h3>
    <p>Nur fehlende globale Portal→Wegpunkt-Routen werden als Setup-Bedarf gezeigt; sie gehören nicht zur Farming-Bibliothek.</p>
    <div className="run-grid">{system.map((entry) => <article key={entry.act}><strong>{entry.act.toUpperCase()}</strong><span>{entry.ready ? "bereit" : "Setup erforderlich"}</span>{workflow?.act === entry.act && <p aria-live="polite"><strong>Core-Status: {workflow.state}</strong>{workflow.state === "preflight" ? " – am Portal stehen bleiben; Aufnahme noch nicht gestartet." : workflow.state === "recording" ? " – Aufnahme läuft; jetzt zum Wegpunkt gehen." : workflow.reason ? ` – ${workflow.reason}` : ""}</p>}{entry.ready ? <><p>Stelle dich für den isolierten Test wieder direkt an den Portal-Ankunftspunkt dieses Akts. Der Core prüft die Startnähe vor dem ersten Walk-Input.</p><button disabled={liveActionLocked} onClick={() => void start("system_test", { act: entry.act })}>Playback prüfen</button></> : <><p>Öffne in einem bestehenden Spiel in diesem Akt ein Town Portal, betrete es und starte direkt am Portal-Ankunftspunkt. Laufe ohne Teleport zum lokalen Wegpunkt und beende dort mit {hotkeys?.recording_finish ?? "F9"}.</p><small>{entry.reason}</small><button disabled={liveActionLocked} onClick={() => void start("system_record", { act: entry.act })}>Egress aufnehmen</button></>}</article>)}</div>

    <details className="hotkey-help"><summary>Hotkey-Hilfe</summary>{hotkeys ? <dl><div><dt>{hotkeys.recording_finish}</dt><dd>Aufnahme einfrieren und sicher abschließen</dd></div><div><dt>{hotkeys.stop_after_run}</dt><dd>Nach dem aktuellen Run stoppen</dd></div><div><dt>{hotkeys.emergency_stop}</dt><dd>Sofortiger Emergency Stop; kein Save &amp; Exit garantiert</dd></div><div><dt>{hotkeys.pause}</dt><dd>Nach dem aktuellen Run pausieren</dd></div></dl> : <p>Hotkeys werden geladen …</p>}</details>

    {preview && <div className="modal-backdrop"><div className="modal" role="dialog" aria-modal="true" aria-labelledby="route-confirm-title"><h3 id="route-confirm-title">Routenänderung bestätigen</h3><p><strong>{operationLabel(preview.operation)}</strong>: {preview.candidate_id ? "Neue Route" : "Route"} <code>{preview.route_id}</code>.</p>{preview.replaced_route_id && <p>Die bisher aktive Route <code>{preview.replaced_route_id}</code> wird unverändert archiviert und bleibt wiederherstellbar.</p>}<p>Die Bestätigung gilt nur für die angezeigten Katalog-, Lifecycle- und Assignment-Revisionen.</p>{preview.operation === "delete" && <label>Route-ID zur endgültigen Löschung eingeben<input ref={deleteRef} value={deleteConfirmation} onChange={(event) => setDeleteConfirmation(event.target.value)} autoComplete="off"/><small>Erforderlich: <code>{preview.route_id}</code></small></label>}<div className="modal-actions"><button className="secondary" onClick={() => setPreview(null)} disabled={pending}>Abbrechen</button><button ref={confirmRef} className={preview.operation === "delete" ? "danger" : ""} onClick={() => void confirm()} disabled={pending || (preview.operation === "delete" && deleteConfirmation !== preview.route_id)}>{pending ? "Core prüft …" : "Änderung bestätigen"}</button></div></div></div>}
  </section>;
}
