import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { App } from "./App";

afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("durchläuft Queue, Pause, Resume, Stop und Emergency Stop gegen einen Fake-Core", async () => {
  let generation = 4;
  let state = "idle_in_game";
  let pendingIntent = "none";
  let activeEntries = ["countess", "mephisto"];
  let index = 0;
  const requests: Array<{ path: string; body?: Record<string, unknown> }> = [];
  const queueStatus = () => ({ entries: activeEntries, default_entries: ["countess", "mephisto"], index, cycle: 0, retry: 0, started_runs: state === "idle_in_game" ? 0 : 1, consecutive_failures: 0, total_restarts: 0, budgets: { max_runs: 6, max_duration_ms: 7200000, max_consecutive_failures: 2, max_total_restarts: 3 } });
  const status = () => ({ schema_version: 1, core_version: "test", state, generation, pending_intent: pendingIntent, active_run_id: state === "running_run" ? activeEntries[index] : undefined, d2r: { state: "attached", window_bound: true, client_width: 1280, client_height: 720 }, input: { enabled: true, paused: false, stopped: false }, world: { valid: true, phase: "in_game", area_id: 1, area_name: "Rogue Encampment" }, selection: { character: "MrBones", difficulty: "nightmare" }, queue: queueStatus() });
  const catalog = { schema_version: 1, revision: 8, default_difficulty: "nightmare", characters: [{ name: "MrBones", slug: "mrbones", selectable: true }], difficulties: [{ id: "nightmare", display_name: "Alptraum" }], profiles: [], runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }, { run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required" }] };

  vi.stubGlobal("crypto", { randomUUID: () => `command-${requests.length}` });
  vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input);
    const body = init?.body ? JSON.parse(String(init.body)) as Record<string, unknown> : undefined;
    requests.push({ path, body });
    if (path === "/api/v1/status") return response(status());
    if (path === "/api/v1/catalog") return response(catalog);
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
  fireEvent.click(await screen.findByRole("button", { name: "Countess zur Queue hinzufügen" }));
  fireEvent.click(screen.getByRole("button", { name: "Queue prüfen und starten" }));
  await waitFor(() => expect(screen.getByText("running_run")).toBeInTheDocument());
  expect(requests.some((request) => request.path === "/api/v1/queue/validate")).toBe(true);
  expect(requests.some((request) => request.path === "/api/v1/session/start")).toBe(true);

  fireEvent.click(screen.getByRole("button", { name: "Nach aktuellem Run pausieren" }));
  expect(await screen.findByText("Vorgemerkt: pause_after_run")).toBeInTheDocument();
  state = "paused_between_runs"; pendingIntent = "none"; index = 1; generation++;
  await emitSupervisorDelta(listeners);
  fireEvent.click(await screen.findByRole("button", { name: "Queue fortsetzen" }));
  await waitFor(() => expect(screen.getByText("running_run")).toBeInTheDocument());

  fireEvent.click(screen.getByRole("button", { name: "Nach aktuellem Run stoppen" }));
  expect(await screen.findByText("Vorgemerkt: stop_after_run")).toBeInTheDocument();
  state = "idle"; pendingIntent = "none"; activeEntries = []; generation++;
  await emitSupervisorDelta(listeners);
  await waitFor(() => expect(screen.getByText("idle")).toBeInTheDocument());

  fireEvent.click(screen.getByRole("button", { name: "Queue prüfen und starten" }));
  await waitFor(() => expect(screen.getByText("running_run")).toBeInTheDocument());
  fireEvent.click(screen.getByRole("button", { name: "Emergency Stop" }));
  fireEvent.click(await screen.findByRole("button", { name: "Emergency Stop bestätigen" }));
  await waitFor(() => expect(screen.getByText("idle")).toBeInTheDocument());
  expect(requests.some((request) => request.path === "/api/v1/session/emergency-stop")).toBe(true);
});

function response(value: unknown): Response {
  return new Response(JSON.stringify(value), { status: 200, headers: { "Content-Type": "application/json" } });
}

async function emitSupervisorDelta(listeners: Map<string, EventListener>) {
  await act(async () => listeners.get("supervisor_state_changed")?.({ data: JSON.stringify({ sequence: Date.now(), timestamp: new Date().toISOString(), event: "supervisor_state_changed" }) } as unknown as Event));
}
