import { Check, ChevronRight, CircleAlert, Sparkles, X } from "lucide-react";
import { useState } from "react";
import type { HotkeyHelpDTO, RecordingOptionDTO, RouteWorkflowDTO } from "../../../api/generated";
import { Dialog } from "../../../app/ui";
import campfireGuide from "../../../assets/route-guides/lower-kurast/campfire.png";
import hutGuide from "../../../assets/route-guides/lower-kurast/huts.png";
import { prerequisiteLabel, reasonLabel, roleLabel, runLabel, runOrder, runPurposeLabels, targetLabel, terminalWorkflowStates, waypointLabel } from "../routePresentation";
import { RouteWorkflowPanel } from "./RouteWorkflowPanel";
import { useTranslation } from "react-i18next";
import { presentRecordingHint, presentRecordingInstruction, type AppTranslator } from "../../../i18n/presenters";

interface Props {
  options: RecordingOptionDTO[];
  selectedRun: string;
  selectedRole: string;
  hotkeys: HotkeyHelpDTO | null;
  workflow: RouteWorkflowDTO | null;
  locked: boolean;
  lockedReason?: string;
  pending: boolean;
  onSelectRun(runID: string): void;
  onSelectRole(role: string): void;
  onStart(option: RecordingOptionDTO): void;
  onFinish(): void;
  onOpenDrafts(): void;
}

function actionLabel(option: RecordingOptionDTO, t: AppTranslator): string {
  if (option.route_role === "leg_acquisition") return t("routes.recordWirt");
  if (option.route_role === "cow_sweep") return t("routes.recordCow");
  return t("routes.recordRun", { run: runLabel(option.run_id) });
}

const lowerKurastGuides = [
  { id: "campfire", titleKey: "routes.campfire", src: campfireGuide, altKey: "routes.campfireAlt" },
  { id: "huts", titleKey: "routes.huts", src: hutGuide, altKey: "routes.hutsAlt" },
] as const;

export function RouteRecordingPanel({ options, selectedRun, selectedRole, hotkeys, workflow, locked, lockedReason, pending, onSelectRun, onSelectRole, onStart, onFinish, onOpenDrafts }: Props) {
  const { t } = useTranslation();
  const [guideID, setGuideID] = useState<(typeof lowerKurastGuides)[number]["id"] | null>(null);
  const runIDs = [...new Set(options.map((entry) => entry.run_id))].sort((left, right) => {
    const leftIndex = runOrder.indexOf(left as typeof runOrder[number]); const rightIndex = runOrder.indexOf(right as typeof runOrder[number]);
    return (leftIndex < 0 ? 99 : leftIndex) - (rightIndex < 0 ? 99 : rightIndex);
  });
  const runOptions = options.filter((entry) => entry.run_id === selectedRun);
  const option = runOptions.find((entry) => entry.route_role === selectedRole) ?? runOptions[0];
  const workflowBusy = !!workflow && !terminalWorkflowStates.has(workflow.state);
  const showTerminalFailure = !!workflow && (workflow.state === "failed_safe" || workflow.state === "emergency_cancelled") && workflow.run_id === option?.run_id;
  const missingPrerequisite = option?.prerequisites?.find((entry) => !entry.ready);
  const disabledReason = locked ? (lockedReason ?? reasonLabel("")) : missingPrerequisite ? reasonLabel(missingPrerequisite.reason) : option && !option.available ? reasonLabel(option.reason) : "";
  const openGuide = lowerKurastGuides.find((entry) => entry.id === guideID);
  const purposeLabels = option ? runPurposeLabels(option.run_id) : [];

  if (options.length === 0) return <div className="route-panel"><h3>{t("routes.recordingTab")}</h3><p className="route-empty">{t("routes.recordUnavailable")}</p></div>;

  return <div className="route-panel" aria-labelledby="route-recording-title">
    <div className="route-panel-heading"><div><h3 id="route-recording-title">{t("routes.recordingTab")}</h3><p>{t("routes.recordDetail")}</p></div></div>
    <label className="route-run-mobile">{t("routes.farmingRun")}
      <select value={selectedRun} onChange={(event) => onSelectRun(event.target.value)}>{runIDs.map((runID) => <option value={runID} key={runID}>{runLabel(runID)}</option>)}</select>
    </label>
    <div className="route-recording-layout">
      <nav className="route-run-rail" aria-label={t("routes.chooseRunAria")}>
        {runIDs.map((runID) => {
          const entries = options.filter((entry) => entry.run_id === runID);
          const ready = entries.some((entry) => entry.available && !(entry.prerequisites ?? []).some((prerequisite) => !prerequisite.ready));
          return <button type="button" key={runID} className={selectedRun === runID ? "active" : ""} aria-pressed={selectedRun === runID} onClick={() => onSelectRun(runID)}><span>{runLabel(runID)}</span>{ready ? <small>{t("routes.ready")}</small> : <ChevronRight aria-hidden="true" size={17} />}</button>;
        })}
      </nav>
      <div className="route-recording-detail">
        {selectedRun === "cows" && <div className="route-cow-steps" aria-label={t("routes.cowStepAria")}>
          {[{ role: "leg_acquisition", label: t("routes.wirtRouteStep") }, { role: "cow_sweep", label: t("routes.cowRouteStep") }].map((entry) => <button type="button" key={entry.role} className={option?.route_role === entry.role ? "active" : ""} aria-pressed={option?.route_role === entry.role} onClick={() => onSelectRole(entry.role)}>{entry.label}</button>)}
        </div>}
        {option && <>
          <div className="route-detail-heading"><div><h3>{t("routes.recordRun", { run: runLabel(option.run_id) })}</h3><p>{roleLabel(option.route_role) || t("routes.farmingRoute")}</p></div>{option.route_role && <span className="route-status route-status-info">{roleLabel(option.route_role)}</span>}</div>
          {purposeLabels.length > 0 && <aside className="route-purpose" role="note" aria-label={t("routes.suitableForAria", { run: runLabel(option.run_id) })}>
            <Sparkles aria-hidden="true" size={20} />
            <div><strong>{t("routes.suitableFor")}</strong><ul>{purposeLabels.map((label) => <li key={label}>{label}</li>)}</ul></div>
            <a href="#pickit">{t("routes.configurePickit")}</a>
          </aside>}
          {option.run_id === "cows" && <div className="route-cow-notice"><strong>{t("routes.cowPreparation")}</strong><p>{t("routes.cowPreparationDetail")}</p></div>}
          <div className="route-prerequisites">
            {(option.prerequisites ?? []).map((entry) => <span key={entry.id} className={entry.ready ? "ready" : "missing"}>{entry.ready ? <Check aria-hidden="true" size={15} /> : <X aria-hidden="true" size={15} />}{prerequisiteLabel(entry.id)}</span>)}
          </div>
          {missingPrerequisite && <p className="route-inline-warning"><CircleAlert aria-hidden="true" size={18} /> {reasonLabel(missingPrerequisite.reason)}</p>}
          <div className="route-locations"><span><small>{t("routes.start")}</small><strong>{waypointLabel(option.start_waypoint, option.start_kind)}</strong></span><ChevronRight aria-hidden="true" /><span><small>{t("routes.target")}</small><strong>{targetLabel(option.run_id, option.route_role)}</strong></span></div>
          <div className="route-instructions"><p>{presentRecordingInstruction(option.instruction_code, hotkeys?.recording_finish ?? "F9", t)}</p>{(option.operator_hint_codes ?? []).length > 0 && <ol>{(option.operator_hint_codes ?? []).map((hint, index) => <li key={hint}><span>{index + 1}</span><p>{presentRecordingHint(hint, t)}</p></li>)}</ol>}</div>
          {option.run_id === "lower-kurast" && <div className="route-guide-thumbs" aria-label={t("routes.guideAria")}>
            {lowerKurastGuides.map((guide) => <button type="button" key={guide.id} onClick={() => setGuideID(guide.id)} aria-label={t("routes.enlargeGuide", { guide: t(guide.titleKey) })}>
              <img src={guide.src} alt={t(guide.altKey)} />
              <span>{t(guide.titleKey)}</span>
            </button>)}
          </div>}
          {(workflowBusy && workflow?.run_id === option.run_id || showTerminalFailure) && <RouteWorkflowPanel workflow={workflow} hotkeys={hotkeys} pending={pending} onFinish={onFinish} onOpenDrafts={onOpenDrafts} onNextCowStep={() => onSelectRole("cow_sweep")} />}
          {!workflowBusy && <div className="route-recording-actions">
            <div><div className="route-hotkeys"><span><kbd>{hotkeys?.recording_finish ?? "F9"}</kbd> {t("routes.finishRecording")}</span><span><kbd>{hotkeys?.emergency_stop ?? "F11"}</kbd> {t("routes.emergencyStop")}</span></div>{disabledReason && <p className="route-disabled-reason">{disabledReason}</p>}</div>
            <button type="button" disabled={pending || locked || !option.available || !!missingPrerequisite} onClick={() => onStart(option)}>{actionLabel(option, t)}</button>
          </div>}
        </>}
      </div>
    </div>
    {openGuide && <Dialog title={t(openGuide.titleKey)} className="route-guide-modal" onClose={() => setGuideID(null)}>
      <img className="route-guide-full" src={openGuide.src} alt={t(openGuide.altKey)} />
      <div className="modal-actions"><button type="button" onClick={() => setGuideID(null)}>{t("routes.close")}</button></div>
    </Dialog>}
  </div>;
}
