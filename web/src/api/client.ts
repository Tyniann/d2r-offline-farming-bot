import { captureCharacterSelection as captureCharacterSelectionGenerated, confirmCharacterSetup as confirmCharacterSetupGenerated, confirmHistoryDeleteAll as confirmHistoryDeleteAllGenerated, createDiagnosticBundle as createDiagnosticBundleGenerated, previewHistoryDeleteAll as previewHistoryDeleteAllGenerated, resetOperatorSettings as resetOperatorSettingsGenerated, updateOperatorSettings as updateOperatorSettingsGenerated, type CharacterSelectionCaptureRequest, type CharacterSetupConfirmRequest, type CharacterSetupPreviewDTO, type CommandResponse, type DiagnosticBundleDTO, type DiagnosticBundleRequest, type HistoryDeleteConfirmRequest, type HistoryDeletePreviewDTO, type HistoryDeleteResultDTO, type OperatorSettingsChangeDTO, type OperatorSettingsMutationRequest, type OperatorSettingsResetRequest, type PickitAssignmentUpdateRequest, type PickitAssignmentsDTO, type PickitCreateRequest, type PickitDeleteRequest, type PickitDuplicateRequest, type PickitProfileDTO, type PickitUpdateRequest, type QueueValidationDTO, type RouteMutationPreviewDTO, type RouteWorkflowDTO, type SelectionPreviewDTO } from "./generated";

let controlToken = "";

export function consumeBootstrapToken(location: Location = window.location): string {
  const params = new URLSearchParams(location.hash.slice(1));
  const token = params.get("control_token");
  if (token) controlToken = token;
  if (location.hash) history.replaceState(null, "", `${location.pathname}${location.search}`);
  return controlToken;
}

async function ensureControlToken(): Promise<string> {
  if (controlToken) return controlToken;
  const response = await fetch("/api/v1/control/bootstrap", {
    headers: { Accept: "application/json", "X-D2RBot-Bootstrap": "1" },
  });
  if (!response.ok) throw new Error("Control-Token konnte nicht sicher erneuert werden.");
  const body = await response.json() as { control_token?: string };
  if (!body.control_token) throw new Error("Control-Token fehlt in der Bootstrap-Antwort.");
  controlToken = body.control_token;
  return controlToken;
}

export function controlHeaders(): HeadersInit {
  return { "Content-Type": "application/json", "X-D2RBot-Control-Token": controlToken };
}

export async function previewSelection(character: string, difficulty: string, catalogRevision: number): Promise<SelectionPreviewDTO> {
  const response = await fetch("/api/v1/selection/preview", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ character, difficulty, catalog_revision: catalogRevision }),
  });
  if (!response.ok) {
    const error = await response.json().catch(() => null) as { message?: string } | null;
    throw new Error(error?.message ?? `Vorschau fehlgeschlagen (${response.status})`);
  }
  return response.json() as Promise<SelectionPreviewDTO>;
}

export async function applySelection(character: string, difficulty: string, catalogRevision: number, expectedGeneration: number, confirmationToken: string): Promise<void> {
  await ensureControlToken();
  const response = await fetch("/api/v1/selection/apply", {
    method: "POST",
    headers: controlHeaders(),
    body: JSON.stringify({
      command_id: crypto.randomUUID(),
      expected_generation: expectedGeneration,
      payload: { character, difficulty, catalog_revision: catalogRevision, confirmation_token: confirmationToken },
    }),
  });
  if (!response.ok) {
    const error = await response.json().catch(() => null) as { message?: string } | null;
    throw new Error(error?.message ?? `Auswahl fehlgeschlagen (${response.status})`);
  }
}

async function errorMessage(response: Response, fallback: string): Promise<Error> {
  const error = await response.json().catch(() => null) as { message?: string } | null;
  return new Error(error?.message ?? `${fallback} (${response.status})`);
}

export async function validateQueue(entries: string[], character: string, difficulty: string, catalogRevision: number): Promise<QueueValidationDTO> {
  const response = await fetch("/api/v1/queue/validate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ entries, character, difficulty, catalog_revision: catalogRevision }),
  });
  if (!response.ok) throw await errorMessage(response, "Queue-Prüfung fehlgeschlagen");
  return response.json() as Promise<QueueValidationDTO>;
}

async function sessionCommand(path: string, expectedGeneration: number, payload?: Record<string, unknown>): Promise<CommandResponse> {
  await ensureControlToken();
  const body: Record<string, unknown> = { command_id: crypto.randomUUID(), expected_generation: expectedGeneration };
  if (payload) body.payload = payload;
  const response = await fetch(path, { method: "POST", headers: controlHeaders(), body: JSON.stringify(body) });
  if (!response.ok) throw await errorMessage(response, "Session-Befehl fehlgeschlagen");
  return response.json() as Promise<CommandResponse>;
}

export function startQueue(entries: string[], character: string, difficulty: string, catalogRevision: number, expectedGeneration: number): Promise<CommandResponse> {
  return sessionCommand("/api/v1/session/start", expectedGeneration, { entries, character, difficulty, catalog_revision: catalogRevision });
}

export function pauseAfterRun(expectedGeneration: number): Promise<CommandResponse> {
  return sessionCommand("/api/v1/session/pause-after-run", expectedGeneration);
}

export function resumeQueue(expectedGeneration: number): Promise<CommandResponse> {
  return sessionCommand("/api/v1/session/resume", expectedGeneration);
}

export function stopAfterRun(expectedGeneration: number): Promise<CommandResponse> {
  return sessionCommand("/api/v1/session/stop-after-run", expectedGeneration);
}

export function emergencyStop(expectedGeneration: number): Promise<CommandResponse> {
  return sessionCommand("/api/v1/session/emergency-stop", expectedGeneration);
}

export async function saveOperatorSettings(request: OperatorSettingsMutationRequest): Promise<OperatorSettingsChangeDTO> {
  return updateOperatorSettingsGenerated(request, await ensureControlToken());
}

export async function confirmCharacterSetup(request: CharacterSetupConfirmRequest): Promise<CharacterSetupPreviewDTO> {
  return confirmCharacterSetupGenerated(request, await ensureControlToken());
}

export async function captureCharacterSelection(request: CharacterSelectionCaptureRequest): Promise<CharacterSetupPreviewDTO> {
  return captureCharacterSelectionGenerated(request, await ensureControlToken());
}

export async function restoreOperatorSettings(request: OperatorSettingsResetRequest): Promise<OperatorSettingsChangeDTO> {
  return resetOperatorSettingsGenerated(request, await ensureControlToken());
}

export async function previewHistoryDeleteAll(expectedGeneration: number): Promise<HistoryDeletePreviewDTO> {
  return previewHistoryDeleteAllGenerated({ expected_generation: expectedGeneration }, await ensureControlToken());
}

export async function confirmHistoryDeleteAll(request: HistoryDeleteConfirmRequest): Promise<HistoryDeleteResultDTO> {
  return confirmHistoryDeleteAllGenerated(request, await ensureControlToken());
}

export async function createDiagnosticBundle(request: DiagnosticBundleRequest): Promise<DiagnosticBundleDTO> {
  return createDiagnosticBundleGenerated(request, await ensureControlToken());
}

export async function previewRouteMutation(operation: string, routeId = "", candidateId = ""): Promise<RouteMutationPreviewDTO> {
  const candidateOperation = operation === "delete_candidate" ? "delete" : "publish";
  const path = candidateId ? `/api/v1/route-candidates/${encodeURIComponent(candidateId)}/${candidateOperation}/preview` : `/api/v1/routes/${encodeURIComponent(routeId)}/${encodeURIComponent(operation)}/preview`;
  const response = await fetch(path, { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" });
  if (!response.ok) throw await errorMessage(response, "Routenvorschau fehlgeschlagen");
  return response.json() as Promise<RouteMutationPreviewDTO>;
}

export async function confirmRouteMutation(preview: RouteMutationPreviewDTO, confirmRouteId = ""): Promise<void> {
  await ensureControlToken();
  const candidateOperation = preview.operation === "delete_candidate" ? "delete" : "publish";
  const path = preview.candidate_id ? `/api/v1/route-candidates/${encodeURIComponent(preview.candidate_id)}/${candidateOperation}/confirm` : `/api/v1/routes/${encodeURIComponent(preview.route_id)}/${encodeURIComponent(preview.operation)}/confirm`;
  const response = await fetch(path, { method: "POST", headers: controlHeaders(), body: JSON.stringify({ confirmation_token: preview.confirmation_token, confirm_route_id: preview.operation === "delete" ? confirmRouteId : undefined }) });
  if (!response.ok) throw await errorMessage(response, "Routenänderung fehlgeschlagen");
}

export async function startRouteWorkflow(operation: string, expectedGeneration: number, options: { runId?: string; routeRole?: string; candidateId?: string; act?: string }): Promise<RouteWorkflowDTO> {
  await ensureControlToken();
  let path = "/api/v1/routes/workflow/start";
  let body: Record<string, unknown> = { expected_generation: expectedGeneration, operation, run_id: options.runId, route_role: options.routeRole, candidate_id: options.candidateId, act: options.act };
  if (operation === "record") { path = "/api/v1/route-recordings"; body = { expected_generation: expectedGeneration, run_id: options.runId, route_role: options.routeRole }; }
  if (operation === "test" && options.candidateId) { path = `/api/v1/route-candidates/${encodeURIComponent(options.candidateId)}/test`; body = { expected_generation: expectedGeneration }; }
  const response = await fetch(path, { method: "POST", headers: controlHeaders(), body: JSON.stringify(body) });
  if (!response.ok) throw await errorMessage(response, "Routen-Workflow konnte nicht gestartet werden");
  return response.json() as Promise<RouteWorkflowDTO>;
}

export async function finishRouteRecording(workflowId: string, expectedGeneration: number): Promise<RouteWorkflowDTO> {
  await ensureControlToken();
  const response = await fetch(`/api/v1/route-recordings/${encodeURIComponent(workflowId)}/finish`, { method: "POST", headers: controlHeaders(), body: JSON.stringify({ expected_generation: expectedGeneration }) });
  if (!response.ok) throw await errorMessage(response, "Aufnahme konnte nicht beendet werden");
  return response.json() as Promise<RouteWorkflowDTO>;
}

async function pickitMutation<T>(path: string, method: string, body: unknown): Promise<T> {
  await ensureControlToken();
  const response = await fetch(path, { method, headers: controlHeaders(), body: JSON.stringify(body) });
  if (!response.ok) throw await errorMessage(response, "Pickit-Änderung fehlgeschlagen");
  return response.status === 204 ? (undefined as T) : response.json() as Promise<T>;
}

export function createPickitProfile(request: PickitCreateRequest): Promise<PickitProfileDTO> { return pickitMutation("/api/v1/pickit/profiles", "POST", request); }
export function updatePickitProfile(id: string, request: PickitUpdateRequest): Promise<PickitProfileDTO> { return pickitMutation(`/api/v1/pickit/profiles/${encodeURIComponent(id)}`, "PUT", request); }
export function duplicatePickitProfile(id: string, request: PickitDuplicateRequest): Promise<PickitProfileDTO> { return pickitMutation(`/api/v1/pickit/profiles/${encodeURIComponent(id)}/duplicate`, "POST", request); }
export function deletePickitProfile(id: string, request: PickitDeleteRequest): Promise<void> { return pickitMutation(`/api/v1/pickit/profiles/${encodeURIComponent(id)}`, "DELETE", request); }
export function updatePickitAssignment(request: PickitAssignmentUpdateRequest): Promise<PickitAssignmentsDTO> { return pickitMutation("/api/v1/pickit/assignments", "PUT", request); }

export type LiveConnectionState = "wird verbunden" | "verbunden" | "getrennt";

export function connectLiveEvents(
  onSnapshot: (data: unknown) => void,
  onEvent: (data: unknown) => void,
  onState: (state: LiveConnectionState) => void,
): () => void {
  onState("wird verbunden");
  const source = new EventSource("/api/v1/events");
  source.onopen = () => onState("verbunden");
  source.onerror = () => onState("getrennt");
  source.addEventListener("snapshot", (event) => onSnapshot(JSON.parse((event as MessageEvent<string>).data)));
  for (const name of ["supervisor_state_changed", "session_result", "selection_completed", "selection_failed", "d2r_state_changed", "input_state_changed", "world_state_changed", "area_changed", "runtime_error", "runtime_error_cleared", "step_changed", "route_workflow_changed", "route_library_changed", "catalog_changed", "pickit_profile_changed", "pickit_assignment_changed", "operator_settings_changed", "history_changed", "history_maintenance"]) {
    source.addEventListener(name, (event) => onEvent(JSON.parse((event as MessageEvent<string>).data)));
  }
  return () => source.close();
}
