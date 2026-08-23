import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { App } from "./App";

afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("startet die Queue und spiegelt Core-Fortschritt sowie Hotkey-Vormerkungen", async () => {
  let generation = 4;
  let state = "idle_in_game";
  let pendingIntent = "none";
  let activeEntries = ["countess", "mephisto"];
  let index = 0;
  const requests: Array<{ path: string; body?: Record<string, unknown> }> = [];
  const queueStatus = () => ({ entries: activeEntries, default_entries: ["countess", "mephisto"], index, cycle: 0, retry: 0, started_runs: state === "idle_in_game" ? 0 : 1, consecutive_failures: 0, total_restarts: 0, budgets: { max_runs: 6, max_duration_ms: 7200000, max_consecutive_failures: 2, max_total_restarts: 3 } });
  const status = () => ({ schema_version: 1, app_version: "test", core_version: "test", state, lifecycle_phase: state, generation, pending_intent: pendingIntent, active_run_id: state === "running_run" ? activeEntries[index] : undefined, run_id: state === "running_run" ? `run-${index + 1}` : undefined, game_id: state === "running_run" || state === "paused_between_runs" ? "game-001" : undefined, run_progress: state === "running_run" ? { stage_code: "cellar_floor", params: { floor: 3, floors: 5 }, current: 6, total: 13 } : undefined, d2r: { state: "attached", window_bound: true, client_width: 1280, client_height: 720 }, compatibility: { state: "compatible", supported_version: "3.2.92777", expected_version: "3.2.92777", offset_version: "3.2.92777", actual_version: "3.2.92777", privilege_mismatch: false }, input: { enabled: true, paused: false, stopped: false }, world: { valid: true, phase: "in_game", area_id: 1, area_name: "Rogue Encampment" }, selection: { character: "MrBones", difficulty: "nightmare" }, queue: queueStatus() });
  const catalog = { schema_version: 1, revision: 8, default_difficulty: "nightmare", characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }], difficulties: [{ id: "nightmare", display_name: "Alptraum" }], profiles: [], runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }, { run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required" }, { run_id: "nihlathak", display_name: "Nihlathak", status: "unavailable" }, { run_id: "summoner", display_name: "Summoner", status: "unavailable" }] };

  vi.stubGlobal("crypto", { randomUUID: () => `command-${requests.length}` });
  vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input);
    const body = init?.body ? JSON.parse(String(init.body)) as Record<string, unknown> : undefined;
    requests.push({ path, body });
    if (path === "/api/v1/status") return response(status());
    if (path === "/api/v1/catalog") return response(catalog);
    if (path.includes("/api/v1/runs")) return response({ schema_version: 1, character: "MrBones", difficulty: "nightmare", runs: catalog.runs });
    if (path === "/api/v1/routes/workflow") return response({ workflow_id: "", generation: 1, state: "idle", run_id: "", character: "" });
    if (path === "/api/v1/control/bootstrap") return response({ control_token: "fake-token" });
    if (path === "/api/v1/queue/validate") return response({ ...body, schema_version: 1, budgets: queueStatus().budgets });
    if (path === "/api/v1/session/start") {
      activeEntries = ((body?.payload as { entries: string[] }).entries);
      state = "running_run"; pendingIntent = "none"; generation++;
      return response({ schema_version: 1, command_id: body?.command_id, generation, state });
    }
    if (path === "/api/v1/session/pause-after-run") { pendingIntent = "pause_after_run"; generation++; return response({ generation, state }); }
    if (path === "/api/v1/session/resume") { state = "running_run"; pendingIntent = "none"; generation++; return response({ generation, state }); }
    if (path === "/api/v1/session/stop-after-run") { pendingIntent = "stop_after_run"; generation++; return response({ generation, state }); }
    if (path === "/api/v1/session/emergency-stop") { state = "idle"; pendingIntent = "none"; activeEntries = []; generation++; return response({ generation, state }); }
    return new Response(null, { status: 404 });
  }));

  const listeners = new Map<string, EventListener>();
  class FakeEventSource {
    onopen: (() => void) | null = null;
    onerror: (() => void) | null = null;
    addEventListener(name: string, listener: EventListener) { listeners.set(name, listener); }
    close() {}
  }
  vi.stubGlobal("EventSource", FakeEventSource);

  render(<App />);
  expect(await screen.findByRole("heading", { name: "Deine Routenreihenfolge" })).toBeInTheDocument();
  const start = await screen.findByRole("button", { name: "Jetzt farmen" });
  await waitFor(() => expect(start).toBeEnabled());
  fireEvent.click(start);
  expect(await screen.findByLabelText("Etappe 6 von 13")).toBeInTheDocument();
  expect(requests.some((request) => request.path === "/api/v1/queue/validate")).toBe(true);
  expect(requests.some((request) => request.path === "/api/v1/session/start")).toBe(true);

  pendingIntent = "pause_after_run"; generation++;
  await emitSupervisorDelta(listeners);
  expect(await screen.findByText("Pause nach dieser Route vorgemerkt")).toBeInTheDocument();
  pendingIntent = "stop_after_run"; generation++;
  await emitSupervisorDelta(listeners);
  expect(await screen.findByText("Stopp nach dieser Route vorgemerkt")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /pausieren|stoppen|Emergency Stop/i })).not.toBeInTheDocument();
  expect(requests.some((request) => ["/api/v1/session/pause-after-run", "/api/v1/session/resume", "/api/v1/session/stop-after-run", "/api/v1/session/emergency-stop"].includes(request.path))).toBe(false);
});

function response(value: unknown): Response {
  return new Response(JSON.stringify(value), { status: 200, headers: { "Content-Type": "application/json" } });
}

async function emitSupervisorDelta(listeners: Map<string, EventListener>) {
  await act(async () => listeners.get("supervisor_state_changed")?.({ data: JSON.stringify({ sequence: Date.now(), timestamp: new Date().toISOString(), event: "supervisor_state_changed" }) } as unknown as Event));
}
