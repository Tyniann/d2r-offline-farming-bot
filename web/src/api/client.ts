import type { CommandResponse, QueueValidationDTO, SelectionPreviewDTO } from "./generated";

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
  for (const name of ["supervisor_state_changed", "session_result", "selection_completed", "selection_failed", "d2r_state_changed", "input_state_changed", "world_state_changed", "area_changed", "runtime_error", "runtime_error_cleared", "step_changed"]) {
    source.addEventListener(name, (event) => onEvent(JSON.parse((event as MessageEvent<string>).data)));
  }
  return () => source.close();
}
