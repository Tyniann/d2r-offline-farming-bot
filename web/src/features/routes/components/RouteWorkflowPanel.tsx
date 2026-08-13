import { CheckCircle2, LoaderCircle, TriangleAlert } from "lucide-react";
import type { HotkeyHelpDTO, RouteWorkflowDTO } from "../../../api/generated";
import { workflowPresentation } from "../routePresentation";

interface Props {
  workflow: RouteWorkflowDTO | null;
  hotkeys: HotkeyHelpDTO | null;
  pending: boolean;
  onFinish(): void;
  onOpenDrafts(): void;
  onNextCowStep?(): void;
}

export function RouteWorkflowPanel({ workflow, hotkeys, pending, onFinish, onOpenDrafts, onNextCowStep }: Props) {
  if (!workflow) return null;
  const presentation = workflowPresentation(workflow);
  if (!presentation) return null;
  const Icon = presentation.tone === "success" ? CheckCircle2 : presentation.tone === "danger" ? TriangleAlert : LoaderCircle;
  return <div className={`route-workflow route-workflow-${presentation.tone}`} aria-live={presentation.tone === "danger" ? "assertive" : "polite"}>
    <Icon aria-hidden="true" className={presentation.tone === "active" ? "route-spin" : ""} />
    <div><strong>{presentation.title}</strong><p>{presentation.instruction}</p>
      {(workflow.state === "recording" || workflow.state === "preflight") && <div className="route-hotkeys"><span><kbd>{hotkeys?.recording_finish ?? "F9"}</kbd> Aufnahme beenden</span><span><kbd>{hotkeys?.emergency_stop ?? "F11"}</kbd> Notabbruch</span></div>}
      <div className="route-workflow-actions">
        {workflow.state === "recording" && <button type="button" disabled={pending} onClick={onFinish}>Aufnahme beenden</button>}
        {workflow.state === "candidate_ready" && <button type="button" onClick={onOpenDrafts}>Entwurf öffnen</button>}
        {workflow.state === "candidate_ready" && workflow.run_id === "cows" && workflow.route_role === "leg_acquisition" && onNextCowStep && <button type="button" className="secondary" onClick={onNextCowStep}>Weiter zur Cow-Route</button>}
      </div>
    </div>
  </div>;
}
