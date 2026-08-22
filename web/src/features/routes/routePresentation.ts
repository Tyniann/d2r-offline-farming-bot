import type { RouteCandidateDTO, RouteEntryDTO, RouteWorkflowDTO } from "../../api/generated";
import { formatDate } from "../../i18n/format";
import { i18n } from "../../i18n";
import { presentDifficultyName, presentRouteReason, presentRunName } from "../../i18n/presenters";

export const runOrder = ["countess", "mephisto", "lower-kurast", "summoner", "nihlathak", "cows"] as const;

const runPurposes = {
  countess: ["routes.purposes.terrorKey", "routes.purposes.runes"],
  summoner: ["routes.purposes.hateKey"],
  "lower-kurast": ["routes.purposes.highRunes", "routes.purposes.gems"],
  mephisto: ["routes.purposes.setItems", "routes.purposes.uniqueItems"],
  nihlathak: ["routes.purposes.destructionKey"],
  cows: ["routes.purposes.whiteBases", "routes.purposes.socketedBases", "routes.purposes.gems", "routes.purposes.experience"],
} as const;

const roleLabels = {
  leg_acquisition: "routes.roles.legAcquisition",
  cow_sweep: "routes.roles.cowSweep",
  host: "routes.roles.legAcquisition",
  cow: "routes.roles.cowSweep",
} as const;

const waypointLabels = {
  black_marsh: "routes.waypoints.blackMarsh",
  durance_of_hate_level_2: "routes.waypoints.durance2",
  lower_kurast: "routes.waypoints.lowerKurast",
  halls_of_pain: "routes.waypoints.hallsPain",
  arcane_sanctuary: "routes.waypoints.arcaneSanctuary",
  stony_field: "routes.waypoints.stonyField",
} as const;

const candidateStatusLabels = {
  recorded: "routes.candidateStatus.readyTest",
  validated: "routes.candidateStatus.readyTest",
  test_running: "routes.candidateStatus.testRunning",
  test_passed: "routes.candidateStatus.testPassed",
  failed: "routes.candidateStatus.testFailed",
} as const;

export function runLabel(runID: string): string {
  return presentRunName(runID, i18n.t);
}

export function runPurposeLabels(runID: string): readonly string[] {
  const purposes = runPurposes[runID as keyof typeof runPurposes] ?? [];
  return purposes.map((purpose) => i18n.t(purpose));
}

export function roleLabel(role?: string): string {
  const key = roleLabels[role as keyof typeof roleLabels] ?? "routes.roles.partial";
  return role ? i18n.t(key) : "";
}

export function difficultyLabel(difficulty: string): string {
  return presentDifficultyName(difficulty.toLowerCase(), i18n.t);
}

export function waypointLabel(waypoint: string, startKind?: string): string {
  if (startKind === "object_portal_arrival") return i18n.t("routes.waypoints.redArrivalPortal");
  const key = waypointLabels[waypoint as keyof typeof waypointLabels] ?? "routes.waypoints.specifiedStart";
  return i18n.t(key);
}

export function targetLabel(runID: string, role?: string): string {
  if (runID === "cows" && role === "leg_acquisition") return i18n.t("routes.targets.wirtBody");
  if (runID === "cows") return i18n.t("routes.targets.cowEndpoint");
  if (runID === "lower-kurast") return i18n.t("routes.targets.campfireHuts");
  return runLabel(runID);
}

export function reasonLabel(reason?: string): string {
  return presentRouteReason(reason ?? "", i18n.t);
}

export function prerequisiteLabel(id: string): string {
  const keys = { waypoint: "routes.prerequisites.waypoint", teleport: "routes.prerequisites.teleport", town_portal: "routes.prerequisites.townPortal", pickit: "routes.prerequisites.pickit" } as const;
  const key = keys[id as keyof typeof keys] ?? "routes.prerequisites.checked";
  return i18n.t(key);
}

export function candidateStatusLabel(state: string): string {
  const key = candidateStatusLabels[state as keyof typeof candidateStatusLabels] ?? "routes.candidateStatus.pending";
  return i18n.t(key);
}

export function routeStatus(route?: RouteEntryDTO): string {
  if (!route) return i18n.t("routes.routeStatus.notRecorded");
  if (route.management_status === "archived") return i18n.t("routes.routeStatus.archived");
  if (route.lifecycle_status === "unavailable" || route.lifecycle_status === "stale") return i18n.t("routes.routeStatus.incomplete");
  return i18n.t(route.assigned ? "routes.routeStatus.active" : "routes.routeStatus.unused");
}

export function routeStatusTone(status: string): string {
  if ([i18n.t("routes.routeStatus.active"), i18n.t("routes.complete"), i18n.t("routes.candidateStatus.testPassed")].includes(status)) return "route-status-good";
  if ([i18n.t("routes.routeStatus.incomplete"), i18n.t("routes.candidateStatus.testFailed")].includes(status)) return "route-status-bad";
  if ([i18n.t("routes.candidateStatus.readyTest"), i18n.t("routes.candidateStatus.testRunning")].includes(status)) return "route-status-info";
  return "route-status-muted";
}

export function formatCandidateTime(candidate: RouteCandidateDTO): string {
  if (!candidate.created_at) return i18n.t("routes.timeUnavailable");
  const date = new Date(candidate.created_at);
  if (Number.isNaN(date.getTime())) return i18n.t("routes.timeUnavailable");
  const now = new Date();
  const sameDay = date.toDateString() === now.toDateString();
  const yesterday = new Date(now); yesterday.setDate(now.getDate() - 1);
  const prefix = sameDay ? i18n.t("routes.today") : date.toDateString() === yesterday.toDateString() ? i18n.t("routes.yesterday") : formatDate(date, { day: "2-digit", month: "2-digit", year: "numeric" });
  return `${prefix}, ${formatDate(date, { hour: "2-digit", minute: "2-digit" })}`;
}

export const terminalWorkflowStates = new Set(["idle", "completed", "failed_safe", "emergency_cancelled"]);

export type WorkflowPresentation = { title: string; instruction: string; tone: "active" | "success" | "danger" };

export function workflowPresentation(workflow: RouteWorkflowDTO): WorkflowPresentation | null {
  switch (workflow.state) {
    case "idle": return null;
    case "preflight": case "preparing_playback": return { title: i18n.t("routes.workflow.prepareStartTitle"), instruction: i18n.t(workflow.state === "preflight" ? "routes.workflow.prepareStartInstruction" : "routes.workflow.preparePlaybackInstruction"), tone: "active" };
    case "recording": return { title: i18n.t("routes.workflow.recordingTitle"), instruction: i18n.t("routes.workflow.recordingInstruction"), tone: "active" };
    case "freezing": case "validating": return { title: i18n.t("routes.workflow.validatingTitle"), instruction: i18n.t("routes.workflow.validatingInstruction"), tone: "active" };
    case "playing_candidate": case "validating_terminal": return { title: i18n.t("routes.workflow.testTitle"), instruction: i18n.t("routes.workflow.testInstruction"), tone: "active" };
    case "returning_via_portal": case "returning_after_test": return { title: i18n.t("routes.workflow.returningTitle"), instruction: i18n.t("routes.workflow.returningInstruction"), tone: "active" };
    case "candidate_ready": return { title: i18n.t("routes.workflow.candidateReadyTitle"), instruction: i18n.t("routes.workflow.candidateReadyInstruction"), tone: "success" };
    case "awaiting_publish_confirmation": return { title: i18n.t("routes.workflow.publishConfirmTitle"), instruction: i18n.t("routes.workflow.publishConfirmInstruction"), tone: "active" };
    case "publishing": return { title: i18n.t("routes.workflow.publishingTitle"), instruction: i18n.t("routes.workflow.publishingInstruction"), tone: "active" };
    case "completed": return { title: i18n.t("routes.workflow.completedTitle"), instruction: i18n.t("routes.workflow.completedInstruction"), tone: "success" };
    case "failed_safe": return { title: i18n.t("routes.workflow.failedTitle"), instruction: i18n.t("routes.workflow.failedInstruction", { reason: reasonLabel(workflow.reason) }), tone: "danger" };
    case "emergency_cancelled": return { title: i18n.t("routes.workflow.emergencyTitle"), instruction: i18n.t("routes.workflow.emergencyInstruction"), tone: "danger" };
    default: return { title: i18n.t("routes.workflow.activeTitle"), instruction: i18n.t("routes.workflow.activeInstruction"), tone: "active" };
  }
}

export function candidateTitle(candidate: RouteCandidateDTO): string {
  const role = roleLabel(candidate.route_role);
  return role ? `${runLabel(candidate.run_id)} · ${role}` : runLabel(candidate.run_id);
}
