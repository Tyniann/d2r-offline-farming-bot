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
import { candidateTitle, roleLabel, runLabel, terminalWorkflowStates } from "./routePresentation";
import "./RouteFeature.css";
import { useTranslation } from "react-i18next";

interface Props {
  characters: string[];
  selectedCharacter: string;
  onSelectedCharacterChange?(character: string): void;
  refreshKey: number;
  liveLocked?: boolean;
  preferredRecordingRun?: string;
  onReturnToOnboarding?(): void;
}

const testWorkflowStates = new Set(["preparing_playback", "playing_candidate", "validating_terminal", "returning_after_test", "awaiting_publish_confirmation", "publishing"]);

export function RouteFeature({ characters, selectedCharacter: character, onSelectedCharacterChange, refreshKey, liveLocked = false, preferredRecordingRun = "", onReturnToOnboarding }: Props) {
  const { t } = useTranslation();
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
      if (!signal?.aborted) setError(t("routes.loadFailed"));
    }
  };

  useEffect(() => { const controller = new AbortController(); setRoutes(null); void refresh(controller.signal); return () => controller.abort(); }, [character, archive, refreshKey]);
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
    catch { setError(t("routes.prepareFailed")); if (operation === "delete_candidate") setDeletingCandidate(null); }
    finally { setPending(false); }
  };

  const confirm = async () => {
    if (!preview) return;
    setPending(true); setError("");
    try {
      await confirmRouteMutation(preview, preview.operation === "delete" ? preview.route_id : "");
      setPreview(null); setDeletingCandidate(null); await refresh();
    } catch { setError(t("routes.changedRetry")); }
    finally { setPending(false); }
  };

  const start = async (operation: string, data: { runId?: string; routeRole?: string; candidateId?: string; character?: string }) => {
    if (!workflow) return;
    setPending(true); setError("");
    try { setWorkflow(await startRouteWorkflow(operation, workflow.generation, data)); }
    catch { setError(t("routes.startFailed")); }
    finally { setPending(false); }
  };

  const finish = async () => {
    if (!workflow) return;
    setPending(true); setError("");
    try { setWorkflow(await finishRouteRecording(workflow.workflow_id, workflow.generation)); }
    catch { setError(t("routes.finishFailed")); }
    finally { setPending(false); }
  };

  const openRecording = (runID: string, role = "") => { setSelectedRun(runID); setSelectedRole(role || (runID === "cows" ? "leg_acquisition" : "")); setArea("recording"); };
  const selectRun = (runID: string) => { setSelectedRun(runID); setSelectedRole(runID === "cows" ? "leg_acquisition" : ""); };
  const deleteDraft = (candidate: RouteCandidateDTO) => { setDeletingCandidate(candidate); void prepare("delete_candidate", "", candidate.candidate_id); };

  const previewCandidate = preview?.candidate_id ? candidates.find((entry) => entry.candidate_id === preview.candidate_id) : undefined;
  const previewRoute = preview?.route_id ? routes?.find((entry) => entry.route_id === preview.route_id) : undefined;

  return <section className="route-feature" aria-labelledby="routes-title">
    <RoutePageHeader characters={characters} character={character} area={area} draftCount={visibleCandidates.length} onCharacterChange={(next) => onSelectedCharacterChange?.(next)} onAreaChange={setArea} />
    {onReturnToOnboarding && <div className="onboarding-return"><div><strong>{t("routes.onboardingTitle")}</strong><p>{t("routes.onboardingDetail")}</p></div><button type="button" className="secondary" onClick={onReturnToOnboarding}>{t("routes.returnOnboarding")}</button></div>}
    {error && <p className="route-error" role="alert">{error}</p>}
    {!character ? <p className="route-empty">{t("routes.selectCharacterFirst")}</p> : <>
      {area === "library" && <RouteLibraryPanel routes={routes} options={options} archive={archive} locked={actionsLocked} onArchiveChange={setArchive} onRecord={openRecording} onMutate={(operation, routeID) => void prepare(operation, routeID)} />}
      {area === "recording" && <RouteRecordingPanel options={options} selectedRun={selectedRun} selectedRole={selectedRole} hotkeys={hotkeys} workflow={workflow} locked={liveLocked || workflowBusy} lockedReason={workflowBusy ? t("routes.finishWorkflowFirst") : liveLocked ? t("routes.confirmCompatibilityFirst") : undefined} pending={pending} onSelectRun={selectRun} onSelectRole={setSelectedRole} onStart={(option) => void start("record", { runId: option.run_id, routeRole: option.route_role, character })} onFinish={() => void finish()} onOpenDrafts={() => setArea("drafts")} />}
      {area === "drafts" && <RouteDraftsPanel candidates={visibleCandidates} workflow={workflow} locked={actionsLocked} runFilter={draftFilter} onRunFilterChange={setDraftFilter} onTest={(candidate) => void start("test", { candidateId: candidate.candidate_id })} onPublish={(candidate) => void prepare("publish", "", candidate.candidate_id)} onDelete={deleteDraft} />}
    </>}

    {deletingCandidate && preview?.operation === "delete_candidate" && <DeleteDraftDialog candidate={deletingCandidate} pending={pending} onClose={() => { setDeletingCandidate(null); setPreview(null); }} onConfirm={() => void confirm()} />}
    {preview && preview.operation !== "delete_candidate" && <div className="modal-backdrop" onMouseDown={(event) => { if (event.currentTarget === event.target) setPreview(null); }}><div className="modal" role="dialog" aria-modal="true" aria-labelledby="route-confirm-title">
      <h3 id="route-confirm-title">{t(preview.operation === "publish" || preview.operation === "replace" ? "routes.publishConfirm" : preview.operation === "archive" ? "routes.archiveConfirm" : preview.operation === "restore" ? "routes.restoreConfirm" : "routes.deleteConfirm")}</h3>
      <p><strong>{previewCandidate ? candidateTitle(previewCandidate) : `${runLabel(previewRoute?.run_id ?? "")} ${roleLabel(previewRoute?.route_role)}`.trim()}</strong></p>
      {preview.operation === "replace" && <p>{t("routes.replaceDetail")}</p>}
      {preview.operation === "delete" && <p>{t("routes.deleteDetail")}</p>}
      <div className="modal-actions"><button type="button" className="secondary" onClick={() => setPreview(null)} disabled={pending}>{t("common.cancel")}</button><button ref={confirmRef} type="button" className={preview.operation === "delete" ? "danger" : ""} onClick={() => void confirm()} disabled={pending}>{t(pending ? "routes.checkingChange" : preview.operation === "delete" ? "routes.deleteRoute" : "routes.confirmChange")}</button></div>
    </div></div>}
  </section>;
}
