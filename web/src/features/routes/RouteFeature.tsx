import { useEffect, useMemo, useRef, useState } from "react";
import { confirmRouteMutation, finishRouteRecording, previewRouteMutation, startRouteWorkflow } from "../../api/client";
import {
  getHotkeyHelp, getRecordingOptions, getRouteCandidates, getRouteLibrary, getRouteWorkflow,
  type HotkeyHelpDTO, type RecordingOptionDTO, type RouteCandidateDTO, type RouteEntryDTO,
  type RouteMutationPreviewDTO, type RouteWorkflowDTO,
} from "../../api/generated";
import { DeleteDraftDialog } from "./components/DeleteDraftDialog";
import { RouteDraftsPanel } from "./components/RouteDraftsPanel";
import { RouteLibraryPanel } from "./components/RouteLibraryPanel";
import { RoutePageHeader, type RouteArea } from "./components/RoutePageHeader";
import { RouteRecordingPanel } from "./components/RouteRecordingPanel";
import { candidateTitle, roleLabel, runLabel } from "./routePresentation";
import "./RouteFeature.css";

interface Props {
  characters: string[];
  selectedCharacter: string;
  refreshKey: number;
  liveLocked?: boolean;
  preferredRecordingRun?: string;
  onReturnToOnboarding?(): void;
}

const terminalWorkflowStates = new Set(["idle", "completed", "failed_safe", "emergency_cancelled"]);
const testWorkflowStates = new Set(["preparing_playback", "playing_candidate", "validating_terminal", "returning_after_test", "awaiting_publish_confirmation", "publishing"]);

export function RouteFeature({ characters, selectedCharacter, refreshKey, liveLocked = false, preferredRecordingRun = "", onReturnToOnboarding }: Props) {
  const [character, setCharacter] = useState(selectedCharacter);
  const [area, setArea] = useState<RouteArea>(preferredRecordingRun ? "recording" : "library");
  const [archive, setArchive] = useState(false);
  const [routes, setRoutes] = useState<RouteEntryDTO[] | null>(null);
  const [candidates, setCandidates] = useState<RouteCandidateDTO[]>([]);
  const [options, setOptions] = useState<RecordingOptionDTO[]>([]);
  const [hotkeys, setHotkeys] = useState<HotkeyHelpDTO | null>(null);
  const [workflow, setWorkflow] = useState<RouteWorkflowDTO | null>(null);
  const [selectedRun, setSelectedRun] = useState(preferredRecordingRun);
  const [selectedRole, setSelectedRole] = useState("");
  const [draftFilter, setDraftFilter] = useState("");
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<RouteMutationPreviewDTO | null>(null);
  const [deletingCandidate, setDeletingCandidate] = useState<RouteCandidateDTO | null>(null);
  const [pending, setPending] = useState(false);
  const confirmRef = useRef<HTMLButtonElement>(null);

  const visibleCandidates = useMemo(() => candidates.filter((candidate) => candidate.character.localeCompare(character, undefined, { sensitivity: "accent" }) === 0), [candidates, character]);
  const workflowBusy = !!workflow && !terminalWorkflowStates.has(workflow.state);
  const actionsLocked = liveLocked || pending || workflowBusy;

  const refresh = async (signal?: AbortSignal) => {
    try {
      const [library, nextCandidates, nextOptions, nextHotkeys, nextWorkflow] = await Promise.all([
        getRouteLibrary(character, archive, signal), getRouteCandidates(signal), getRecordingOptions(character, signal), getHotkeyHelp(signal), getRouteWorkflow(signal),
      ]);
      setRoutes(library.routes.filter((entry) => archive ? entry.management_status === "archived" : entry.management_status !== "archived"));
      setCandidates(nextCandidates);
      setOptions(nextOptions);
      setHotkeys(nextHotkeys);
      setWorkflow(nextWorkflow);
      setSelectedRun((current) => {
        const availableRuns = new Set(nextOptions.map((entry) => entry.run_id));
        const wanted = nextWorkflow.run_id || current || preferredRecordingRun;
        return availableRuns.has(wanted) ? wanted : nextOptions[0]?.run_id ?? "";
      });
      if (!terminalWorkflowStates.has(nextWorkflow.state)) setArea(testWorkflowStates.has(nextWorkflow.state) ? "drafts" : "recording");
      setError("");
    } catch {
      if (!signal?.aborted) setError("Routen konnten nicht geladen werden.");
    }
  };

  useEffect(() => { const controller = new AbortController(); setRoutes(null); void refresh(controller.signal); return () => controller.abort(); }, [character, archive, refreshKey]);
  useEffect(() => { if (!character && selectedCharacter) setCharacter(selectedCharacter); }, [character, selectedCharacter]);
  useEffect(() => {
    if (!preview || preview.operation === "delete_candidate") return;
    confirmRef.current?.focus();
    const close = (event: KeyboardEvent) => { if (event.key === "Escape") setPreview(null); };
    window.addEventListener("keydown", close); return () => window.removeEventListener("keydown", close);
  }, [preview]);
  useEffect(() => {
    if (!deletingCandidate) return;
    const close = (event: KeyboardEvent) => { if (event.key === "Escape") { setDeletingCandidate(null); setPreview(null); } };
    window.addEventListener("keydown", close); return () => window.removeEventListener("keydown", close);
  }, [deletingCandidate]);

  const prepare = async (operation: string, routeID = "", candidateID = "") => {
    setPending(true); setError("");
    try { setPreview(await previewRouteMutation(operation, routeID, candidateID)); }
    catch { setError("Die Aktion kann gerade nicht vorbereitet werden."); if (operation === "delete_candidate") setDeletingCandidate(null); }
    finally { setPending(false); }
  };

  const confirm = async () => {
    if (!preview) return;
    setPending(true); setError("");
    try {
      await confirmRouteMutation(preview, preview.operation === "delete" ? preview.route_id : "");
      setPreview(null); setDeletingCandidate(null); await refresh();
    } catch { setError("Der Stand hat sich geändert. Bitte erneut versuchen."); }
    finally { setPending(false); }
  };

  const start = async (operation: string, data: { runId?: string; routeRole?: string; candidateId?: string; character?: string }) => {
    if (!workflow) return;
    setPending(true); setError("");
    try { setWorkflow(await startRouteWorkflow(operation, workflow.generation, data)); }
    catch { setError("Der Vorgang konnte nicht gestartet werden."); }
    finally { setPending(false); }
  };

  const finish = async () => {
    if (!workflow) return;
    setPending(true); setError("");
    try { setWorkflow(await finishRouteRecording(workflow.workflow_id, workflow.generation)); }
    catch { setError("Die Aufnahme konnte nicht beendet werden."); }
    finally { setPending(false); }
  };

  const openRecording = (runID: string, role = "") => { setSelectedRun(runID); setSelectedRole(role || (runID === "cows" ? "leg_acquisition" : "")); setArea("recording"); };
  const selectRun = (runID: string) => { setSelectedRun(runID); setSelectedRole(runID === "cows" ? "leg_acquisition" : ""); };
  const deleteDraft = (candidate: RouteCandidateDTO) => { setDeletingCandidate(candidate); void prepare("delete_candidate", "", candidate.candidate_id); };

  const previewCandidate = preview?.candidate_id ? candidates.find((entry) => entry.candidate_id === preview.candidate_id) : undefined;
  const previewRoute = preview?.route_id ? routes?.find((entry) => entry.route_id === preview.route_id) : undefined;

  return <section className="route-feature" aria-labelledby="routes-title">
    <RoutePageHeader characters={characters} character={character} area={area} draftCount={visibleCandidates.length} onCharacterChange={setCharacter} onAreaChange={setArea} />
    {onReturnToOnboarding && <div className="onboarding-return"><div><strong>Aus der Einrichtung geöffnet</strong><p>Schließe die Aufnahme hier ab und kehre danach zur Einrichtung zurück.</p></div><button type="button" className="secondary" onClick={onReturnToOnboarding}>Zurück zur Einrichtung</button></div>}
    {error && <p className="route-error" role="alert">{error}</p>}
    {!character ? <p className="route-empty">Wähle zuerst einen Charakter.</p> : <>
      {area === "library" && <RouteLibraryPanel routes={routes} options={options} archive={archive} locked={actionsLocked} onArchiveChange={setArchive} onRecord={openRecording} onMutate={(operation, routeID) => void prepare(operation, routeID)} />}
      {area === "recording" && <RouteRecordingPanel options={options} selectedRun={selectedRun} selectedRole={selectedRole} hotkeys={hotkeys} workflow={workflow} locked={liveLocked || workflowBusy} lockedReason={workflowBusy ? "Schließe zuerst den laufenden Routenvorgang ab." : liveLocked ? "Bestätige zuerst eine kompatible D2R-Version." : undefined} pending={pending} onSelectRun={selectRun} onSelectRole={setSelectedRole} onStart={(option) => void start("record", { runId: option.run_id, routeRole: option.route_role, character })} onFinish={() => void finish()} onOpenDrafts={() => setArea("drafts")} />}
      {area === "drafts" && <RouteDraftsPanel candidates={visibleCandidates} workflow={workflow} locked={actionsLocked} runFilter={draftFilter} onRunFilterChange={setDraftFilter} onTest={(candidate) => void start("test", { candidateId: candidate.candidate_id })} onPublish={(candidate) => void prepare("publish", "", candidate.candidate_id)} onDelete={deleteDraft} />}
    </>}

    {deletingCandidate && preview?.operation === "delete_candidate" && <DeleteDraftDialog candidate={deletingCandidate} pending={pending} onClose={() => { setDeletingCandidate(null); setPreview(null); }} onConfirm={() => void confirm()} />}
    {preview && preview.operation !== "delete_candidate" && <div className="modal-backdrop" onMouseDown={(event) => { if (event.currentTarget === event.target) setPreview(null); }}><div className="modal" role="dialog" aria-modal="true" aria-labelledby="route-confirm-title">
      <h3 id="route-confirm-title">{preview.operation === "publish" || preview.operation === "replace" ? "Route veröffentlichen?" : preview.operation === "archive" ? "Route archivieren?" : preview.operation === "restore" ? "Route wiederherstellen?" : "Route endgültig löschen?"}</h3>
      <p><strong>{previewCandidate ? candidateTitle(previewCandidate) : `${runLabel(previewRoute?.run_id ?? "")} ${roleLabel(previewRoute?.route_role)}`.trim()}</strong></p>
      {preview.operation === "replace" && <p>Die bisher aktive Route wird unverändert archiviert und bleibt wiederherstellbar.</p>}
      {preview.operation === "delete" && <p>Die archivierte Route wird dauerhaft entfernt. Diese Aktion kann nicht rückgängig gemacht werden.</p>}
      <div className="modal-actions"><button type="button" className="secondary" onClick={() => setPreview(null)} disabled={pending}>Abbrechen</button><button ref={confirmRef} type="button" className={preview.operation === "delete" ? "danger" : ""} onClick={() => void confirm()} disabled={pending}>{pending ? "Änderung wird geprüft …" : preview.operation === "delete" ? "Route löschen" : "Änderung bestätigen"}</button></div>
    </div></div>}
  </section>;
}
