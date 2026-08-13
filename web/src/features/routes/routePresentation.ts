import type { RouteCandidateDTO, RouteEntryDTO, RouteWorkflowDTO } from "../../api/generated";

export const runOrder = ["countess", "mephisto", "summoner", "nihlathak", "cows"] as const;

const runLabels: Record<string, string> = {
  countess: "Gräfin",
  mephisto: "Mephisto",
  summoner: "Beschwörer",
  nihlathak: "Nihlathak",
  cows: "Kuhlevel",
};

const roleLabels: Record<string, string> = {
  leg_acquisition: "Wirt-Route",
  cow_sweep: "Cow-Route",
  host: "Wirt-Route",
  cow: "Cow-Route",
};

const difficultyLabels: Record<string, string> = {
  normal: "Normal",
  nightmare: "Alptraum",
  hell: "Hölle",
};

const waypointLabels: Record<string, string> = {
  black_marsh: "Schwarzmoor",
  durance_of_hate_level_2: "Kerker des Hasses – Ebene 2",
  halls_of_pain: "Hallen der Schmerzen",
  arcane_sanctuary: "Geheime Zuflucht",
  stony_field: "Feld der Steine",
};

const reasonLabels: Record<string, string> = {
  input_disabled: "Aktiviere zuerst die Spielsteuerung in den Einstellungen.",
  selection_unconfirmed: "Bestätige zuerst Charakter und Schwierigkeit.",
  route_workflow_active: "Schließe zuerst den laufenden Routenvorgang ab.",
  session_active: "Beende zuerst die laufende Farming-Session.",
  recording_preflight_failed: "Stelle dich an den angegebenen Startwegpunkt.",
  recording_start_area_mismatch: "Stelle dich an den angegebenen Startort.",
  recording_terminal_area_mismatch: "Der Endpunkt liegt nicht im erwarteten Zielgebiet.",
  recording_boss_missing: "Der Boss wurde am Endpunkt nicht gefunden.",
  recording_object_missing: "Wirts Körper wurde am Endpunkt nicht gefunden.",
  recording_boss_dead: "Der Boss muss beim Aufnahmeende noch leben.",
  recording_endpoint_too_far: "Der Endpunkt liegt zu weit vom Ziel entfernt.",
  pickit_assignment_missing: "Ordne diesem Run zuerst ein Lootprofil zu.",
  route_test_start_failed: "Der Test konnte am Startort nicht vorbereitet werden.",
  route_test_playback_failed: "Die Route konnte nicht vollständig abgespielt werden.",
  route_test_terminal_mismatch: "Der Endpunkt der Route wurde nicht bestätigt.",
  route_safety_return_failed: "Die sichere Rückkehr ins Dorf ist fehlgeschlagen.",
  leg_acquisition_route_missing: "Nimm zuerst die Wirt-Route auf.",
  leg_acquisition_route_stale: "Die Wirt-Route muss neu aufgenommen werden.",
  cow_sweep_route_missing: "Nimm zuerst die Cow-Route auf.",
  cow_sweep_route_stale: "Die Cow-Route muss neu aufgenommen werden.",
  route_set_binding_mismatch: "Die beiden Kuhlevel-Routen passen nicht zum selben Charakter.",
};

const candidateStatusLabels: Record<string, string> = {
  recorded: "Bereit zum Test",
  validated: "Bereit zum Test",
  test_running: "Test läuft",
  test_passed: "Test bestanden",
  failed: "Test fehlgeschlagen",
};

export function runLabel(runID: string): string {
  return runLabels[runID] ?? "Farming-Run";
}

export function roleLabel(role?: string): string {
  return role ? (roleLabels[role] ?? "Teilroute") : "";
}

export function difficultyLabel(difficulty: string): string {
  return difficultyLabels[difficulty.toLowerCase()] ?? "Gewählte Schwierigkeit";
}

export function waypointLabel(waypoint: string, startKind?: string): string {
  if (startKind === "object_portal_arrival") return "Rotes Ankunftsportal";
  return waypointLabels[waypoint] ?? "Angegebener Startwegpunkt";
}

export function targetLabel(runID: string, role?: string): string {
  if (runID === "cows" && role === "leg_acquisition") return "Wirts Körper in Tristram";
  if (runID === "cows") return "Gewünschter Endpunkt im Kuhlevel";
  return runLabel(runID);
}

export function reasonLabel(reason?: string): string {
  if (!reason) return "Aktion derzeit nicht möglich.";
  const normalized = reason.toLowerCase();
  if (normalized.includes("teleport not configured")) {
    return "Vervollständige für diesen Charakter unter „Charaktere“ die Tastenbelegung des Kampfprofils.";
  }
  if (normalized.includes("town portal not configured")) {
    return "Hinterlege für diesen Charakter unter „Charaktere“ die Taste für das Stadtportal.";
  }
  return reasonLabels[reason] ?? "Aktion derzeit nicht möglich.";
}

export function prerequisiteLabel(id: string): string {
  return ({ waypoint: "Wegpunkt verfügbar", teleport: "Teleport verfügbar", town_portal: "Stadtportal verfügbar", pickit: "Pickit bereit" } as Record<string, string>)[id] ?? "Voraussetzung geprüft";
}

export function candidateStatusLabel(state: string): string {
  return candidateStatusLabels[state] ?? "Prüfung ausstehend";
}

export function routeStatus(route?: RouteEntryDTO): string {
  if (!route) return "Noch nicht aufgenommen";
  if (route.management_status === "archived") return "Archiviert";
  if (route.lifecycle_status === "unavailable" || route.lifecycle_status === "stale") return "Unvollständig";
  return route.assigned ? "Aktiv" : "Nicht verwendet";
}

export function routeStatusTone(status: string): string {
  if (status === "Aktiv" || status === "Vollständig" || status === "Test bestanden") return "route-status-good";
  if (status === "Unvollständig" || status === "Test fehlgeschlagen") return "route-status-bad";
  if (status === "Bereit zum Test" || status === "Test läuft") return "route-status-info";
  return "route-status-muted";
}

export function formatCandidateTime(candidate: RouteCandidateDTO): string {
  if (!candidate.created_at) return "Aufnahmezeit nicht verfügbar";
  const date = new Date(candidate.created_at);
  if (Number.isNaN(date.getTime())) return "Aufnahmezeit nicht verfügbar";
  const now = new Date();
  const sameDay = date.toDateString() === now.toDateString();
  const yesterday = new Date(now); yesterday.setDate(now.getDate() - 1);
  const prefix = sameDay ? "Heute" : date.toDateString() === yesterday.toDateString() ? "Gestern" : date.toLocaleDateString("de-DE", { day: "2-digit", month: "2-digit", year: "numeric" });
  return `${prefix}, ${date.toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" })}`;
}

export type WorkflowPresentation = { title: string; instruction: string; tone: "active" | "success" | "danger" };

export function workflowPresentation(workflow: RouteWorkflowDTO): WorkflowPresentation | null {
  switch (workflow.state) {
    case "idle": return null;
    case "preflight": case "preparing_playback": return { title: "Start wird vorbereitet", instruction: workflow.state === "preflight" ? "Bleibe am angegebenen Startort stehen, bis der Vorgang beginnt." : "Der Charakter wird zum Start der Route gebracht.", tone: "active" };
    case "recording": return { title: "Aufnahme läuft", instruction: "Folge jetzt dem gewünschten Laufweg und beende die Aufnahme am Ziel.", tone: "active" };
    case "freezing": case "validating": return { title: "Aufnahme wird geprüft", instruction: "Keine Eingabe nötig. Die Aufnahme wird gespeichert und geprüft.", tone: "active" };
    case "playing_candidate": case "validating_terminal": return { title: "Test läuft", instruction: "Keine Eingabe nötig. Die Aufnahme wird im Spiel geprüft.", tone: "active" };
    case "returning_via_portal": case "returning_after_test": return { title: "Sichere Rückkehr", instruction: "Der Charakter kehrt sicher ins Dorf zurück.", tone: "active" };
    case "candidate_ready": return { title: "Aufnahme gespeichert", instruction: "Der Entwurf ist bereit für seinen Test.", tone: "success" };
    case "awaiting_publish_confirmation": return { title: "Veröffentlichung bestätigen", instruction: "Der Test ist bestanden. Bestätige jetzt die Veröffentlichung.", tone: "active" };
    case "publishing": return { title: "Route wird veröffentlicht", instruction: "Die neue Route wird sicher zugeordnet.", tone: "active" };
    case "completed": return { title: "Vorgang abgeschlossen", instruction: "Die Änderung wurde erfolgreich übernommen.", tone: "success" };
    case "failed_safe": return { title: "Vorgang abgebrochen", instruction: reasonLabel(workflow.reason), tone: "danger" };
    case "emergency_cancelled": return { title: "Notabbruch ausgeführt", instruction: "Der Vorgang wurde sofort beendet. Du kannst ihn neu starten, sobald das Spiel bereit ist.", tone: "danger" };
    default: return { title: "Vorgang läuft", instruction: "Bitte warte, bis der aktuelle Vorgang abgeschlossen ist.", tone: "active" };
  }
}

export function candidateTitle(candidate: RouteCandidateDTO): string {
  const role = roleLabel(candidate.route_role);
  return role ? `${runLabel(candidate.run_id)} · ${role}` : runLabel(candidate.run_id);
}
