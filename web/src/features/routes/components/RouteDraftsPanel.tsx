import { Trash2 } from "lucide-react";
import type { RouteCandidateDTO, RouteWorkflowDTO } from "../../../api/generated";
import { candidateStatusLabel, candidateTitle, difficultyLabel, formatCandidateTime, reasonLabel, routeStatusTone, runLabel, runOrder } from "../routePresentation";

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
  const runIDs = [...new Set(candidates.map((entry) => entry.run_id))].sort((left, right) => runOrder.indexOf(left as typeof runOrder[number]) - runOrder.indexOf(right as typeof runOrder[number]));
  const visible = runFilter ? candidates.filter((entry) => entry.run_id === runFilter) : candidates;
  return <div className="route-panel" aria-labelledby="route-drafts-title">
    <div className="route-panel-heading"><div><h3 id="route-drafts-title">Entwürfe</h3><p>Teste Aufnahmen und veröffentliche die passende Route.</p></div>
      <label>Run filtern<select value={runFilter} onChange={(event) => onRunFilterChange(event.target.value)}><option value="">Alle Runs</option>{runIDs.map((runID) => <option key={runID} value={runID}>{runLabel(runID)}</option>)}</select></label>
    </div>
    {visible.length === 0 ? <p className="route-empty">Für diesen Charakter gibt es noch keine unveröffentlichte Aufnahme.</p> : <div className="route-draft-list">
      {visible.map((candidate) => {
        const status = candidateStatusLabel(candidate.state);
        const testing = workflow && !["idle", "completed", "failed_safe", "emergency_cancelled"].includes(workflow.state) && workflow.run_id === candidate.run_id;
        const shownStatus = testing ? "Test läuft" : status;
        const canTest = candidate.state === "validated" || candidate.state === "failed";
        return <article key={candidate.candidate_id} className={`route-draft-row${candidate.state === "failed" ? " failed" : ""}${testing ? " active" : ""}`}>
          <div><strong>{candidateTitle(candidate)}</strong><span>{formatCandidateTime(candidate)} · {difficultyLabel(candidate.difficulty)}</span></div>
          <div><span className={`route-status ${routeStatusTone(shownStatus)}`}>{shownStatus}</span><p>{candidate.reason ? reasonLabel(candidate.reason) : candidate.measured_boss_distance > 0 && candidate.run_id !== "cows" ? `${candidate.measured_boss_distance.toFixed(0)} Tiles bis ${runLabel(candidate.run_id)}` : shownStatus === "Bereit zum Test" ? "Noch nicht im Spiel geprüft" : "Aufnahme geprüft"}</p></div>
          <div className="route-draft-actions">
            {candidate.state === "test_passed" ? <button type="button" disabled={locked} onClick={() => onPublish(candidate)}>Veröffentlichen</button> : <button type="button" className="secondary" disabled={locked || !canTest} onClick={() => onTest(candidate)}>{candidate.state === "failed" ? "Erneut testen" : "Testen"}</button>}
            <button type="button" className="danger route-delete-draft" disabled={locked} onClick={() => onDelete(candidate)}><Trash2 aria-hidden="true" size={17}/> Löschen</button>
          </div>
        </article>;
      })}
    </div>}
  </div>;
}
