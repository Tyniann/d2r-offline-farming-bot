import { Check, ChevronRight, CircleAlert, X } from "lucide-react";
import type { HotkeyHelpDTO, RecordingOptionDTO, RouteWorkflowDTO } from "../../../api/generated";
import { prerequisiteLabel, reasonLabel, roleLabel, runLabel, runOrder, targetLabel, waypointLabel } from "../routePresentation";
import { RouteWorkflowPanel } from "./RouteWorkflowPanel";

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

function actionLabel(option: RecordingOptionDTO): string {
  if (option.route_role === "leg_acquisition") return "Wirt-Route aufnehmen";
  if (option.route_role === "cow_sweep") return "Cow-Route aufnehmen";
  return `${runLabel(option.run_id)} aufnehmen`;
}

export function RouteRecordingPanel({ options, selectedRun, selectedRole, hotkeys, workflow, locked, lockedReason, pending, onSelectRun, onSelectRole, onStart, onFinish, onOpenDrafts }: Props) {
  const runIDs = [...new Set(options.map((entry) => entry.run_id))].sort((left, right) => {
    const leftIndex = runOrder.indexOf(left as typeof runOrder[number]); const rightIndex = runOrder.indexOf(right as typeof runOrder[number]);
    return (leftIndex < 0 ? 99 : leftIndex) - (rightIndex < 0 ? 99 : rightIndex);
  });
  const runOptions = options.filter((entry) => entry.run_id === selectedRun);
  const option = runOptions.find((entry) => entry.route_role === selectedRole) ?? runOptions[0];
  const workflowActive = !!workflow && workflow.state !== "idle" && workflow.state !== "completed";
  const missingPrerequisite = option?.prerequisites?.find((entry) => !entry.ready);
  const disabledReason = locked ? (lockedReason ?? "Aktion derzeit nicht möglich.") : missingPrerequisite ? reasonLabel(missingPrerequisite.reason) : option && !option.available ? reasonLabel(option.reason) : "";

  if (options.length === 0) return <div className="route-panel"><h3>Route aufnehmen</h3><p className="route-empty">Für die aktuelle Auswahl ist keine Routenaufnahme verfügbar.</p></div>;

  return <div className="route-panel" aria-labelledby="route-recording-title">
    <div className="route-panel-heading"><div><h3 id="route-recording-title">Route aufnehmen</h3><p>Wähle einen Run und folge der Anleitung für genau diese Aufnahme.</p></div></div>
    <label className="route-run-mobile">Farming-Run
      <select value={selectedRun} onChange={(event) => onSelectRun(event.target.value)}>{runIDs.map((runID) => <option value={runID} key={runID}>{runLabel(runID)}</option>)}</select>
    </label>
    <div className="route-recording-layout">
      <nav className="route-run-rail" aria-label="Farming-Run auswählen">
        {runIDs.map((runID) => {
          const entries = options.filter((entry) => entry.run_id === runID);
          const ready = entries.some((entry) => entry.available && !(entry.prerequisites ?? []).some((prerequisite) => !prerequisite.ready));
          return <button type="button" key={runID} className={selectedRun === runID ? "active" : ""} aria-pressed={selectedRun === runID} onClick={() => onSelectRun(runID)}><span>{runLabel(runID)}</span>{ready ? <small>bereit</small> : <ChevronRight aria-hidden="true" size={17} />}</button>;
        })}
      </nav>
      <div className="route-recording-detail">
        {selectedRun === "cows" && <div className="route-cow-steps" aria-label="Kuhlevel-Aufnahmeschritt">
          {[{ role: "leg_acquisition", label: "1 Wirt-Route" }, { role: "cow_sweep", label: "2 Cow-Route" }].map((entry) => <button type="button" key={entry.role} className={option?.route_role === entry.role ? "active" : ""} aria-pressed={option?.route_role === entry.role} onClick={() => onSelectRole(entry.role)}>{entry.label}</button>)}
        </div>}
        {option && <>
          <div className="route-detail-heading"><div><h3>{runLabel(option.run_id)} aufnehmen</h3><p>{roleLabel(option.route_role) || "Farming-Route"}</p></div>{option.route_role && <span className="route-status route-status-info">{roleLabel(option.route_role)}</span>}</div>
          {option.run_id === "cows" && <div className="route-cow-notice"><strong>Vorbereitung für das Kuhlevel</strong><p>Die gewählte Schwierigkeit muss abgeschlossen sein. Halte einen geschützten Horadrimwürfel, Platz für Wirts Bein und die benötigten Stadtportalbücher bereit. Entferne alte Wirts Beine vorher manuell.</p></div>}
          <div className="route-prerequisites">
            {(option.prerequisites ?? []).map((entry) => <span key={entry.id} className={entry.ready ? "ready" : "missing"}>{entry.ready ? <Check aria-hidden="true" size={15} /> : <X aria-hidden="true" size={15} />}{prerequisiteLabel(entry.id)}</span>)}
          </div>
          {missingPrerequisite && <p className="route-inline-warning"><CircleAlert aria-hidden="true" size={18} /> {reasonLabel(missingPrerequisite.reason)}</p>}
          <div className="route-locations"><span><small>Start</small><strong>{waypointLabel(option.start_waypoint, option.start_kind)}</strong></span><ChevronRight aria-hidden="true" /><span><small>Ziel</small><strong>{targetLabel(option.run_id, option.route_role)}</strong></span></div>
          <div className="route-instructions"><p>{option.instructions_de}</p>{(option.operator_hints_de ?? []).length > 0 && <ol>{(option.operator_hints_de ?? []).map((hint, index) => <li key={hint}><span>{index + 1}</span><p>{hint}</p></li>)}</ol>}</div>
          {workflowActive && workflow?.run_id === option.run_id ? <RouteWorkflowPanel workflow={workflow} hotkeys={hotkeys} pending={pending} onFinish={onFinish} onOpenDrafts={onOpenDrafts} onNextCowStep={() => onSelectRole("cow_sweep")} /> : <div className="route-recording-actions">
            <div><div className="route-hotkeys"><span><kbd>{hotkeys?.recording_finish ?? "F9"}</kbd> Aufnahme beenden</span><span><kbd>{hotkeys?.emergency_stop ?? "F11"}</kbd> Notabbruch</span></div>{disabledReason && <p className="route-disabled-reason">{disabledReason}</p>}</div>
            <button type="button" disabled={pending || locked || !option.available || !!missingPrerequisite} onClick={() => onStart(option)}>{actionLabel(option)}</button>
          </div>}
        </>}
      </div>
    </div>
  </div>;
}
