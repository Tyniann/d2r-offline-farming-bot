import { Trash2 } from "lucide-react";
import type { RouteCandidateDTO, RouteWorkflowDTO } from "../../../api/generated";
import { candidateStatusLabel, candidateTitle, difficultyLabel, formatCandidateTime, reasonLabel, routeStatusTone, runLabel, runOrder, terminalWorkflowStates } from "../routePresentation";
import { useTranslation } from "react-i18next";

interface Props {
  candidates: RouteCandidateDTO[];
  workflow: RouteWorkflowDTO | null;
  locked: boolean;
  runFilter: string;
  onRunFilterChange(runID: string): void;
  onTest(candidate: RouteCandidateDTO): void;
  onPublish(candidate: RouteCandidateDTO): void;
  onDelete(candidate: RouteCandidateDTO): void;
}

export function RouteDraftsPanel({ candidates, workflow, locked, runFilter, onRunFilterChange, onTest, onPublish, onDelete }: Props) {
  const { t } = useTranslation();
  const runIDs = [...new Set(candidates.map((entry) => entry.run_id))].sort((left, right) => runOrder.indexOf(left as typeof runOrder[number]) - runOrder.indexOf(right as typeof runOrder[number]));
  const visible = runFilter ? candidates.filter((entry) => entry.run_id === runFilter) : candidates;
  return <div className="route-panel" aria-labelledby="route-drafts-title">
    <div className="route-panel-heading"><div><h3 id="route-drafts-title">{t("routes.draftsTab")}</h3><p>{t("routes.draftsDetail")}</p></div>
      <label>{t("routes.filterRun")}<select value={runFilter} onChange={(event) => onRunFilterChange(event.target.value)}><option value="">{t("routes.allRuns")}</option>{runIDs.map((runID) => <option key={runID} value={runID}>{runLabel(runID)}</option>)}</select></label>
    </div>
    {visible.length === 0 ? <p className="route-empty">{t("routes.noDrafts")}</p> : <div className="route-draft-list">
      {visible.map((candidate) => {
        const status = candidateStatusLabel(candidate.state);
        const testing = workflow && !terminalWorkflowStates.has(workflow.state) && workflow.run_id === candidate.run_id;
        const shownStatus = testing ? t("routes.testRunning") : status;
        const canTest = candidate.state === "validated" || candidate.state === "failed";
        return <article key={candidate.candidate_id} className={`route-draft-row${candidate.state === "failed" ? " failed" : ""}${testing ? " active" : ""}`}>
          <div><strong>{candidateTitle(candidate)}</strong><span>{formatCandidateTime(candidate)} · {difficultyLabel(candidate.difficulty)}</span></div>
          <div><span className={`route-status ${testing ? "route-status-info" : routeStatusTone(status)}`}>{shownStatus}</span><p>{candidate.reason ? reasonLabel(candidate.reason) : candidate.measured_boss_distance > 0 && candidate.run_id !== "cows" ? t("routes.tilesToRun", { count: candidate.measured_boss_distance.toFixed(0), run: runLabel(candidate.run_id) }) : candidate.state === "recorded" || candidate.state === "validated" ? t("routes.notTested") : t("routes.recordingChecked")}</p></div>
          <div className="route-draft-actions">
            {candidate.state === "test_passed" ? <button type="button" disabled={locked} onClick={() => onPublish(candidate)}>{t("routes.publish")}</button> : <button type="button" className="secondary" disabled={locked || !canTest} onClick={() => onTest(candidate)}>{t(candidate.state === "failed" ? "routes.retest" : "routes.test")}</button>}
            <button type="button" className="danger route-delete-draft" disabled={locked} onClick={() => onDelete(candidate)}><Trash2 aria-hidden="true" size={17}/> {t("routes.delete")}</button>
          </div>
        </article>;
      })}
    </div>}
  </div>;
}
