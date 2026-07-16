import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const mocks = vi.hoisted(() => ({
  applySelection: vi.fn(), emergencyStop: vi.fn(), connect: vi.fn(), getCatalog: vi.fn(), getStatus: vi.fn(),
  pauseAfterRun: vi.fn(), previewSelection: vi.fn(), resumeQueue: vi.fn(), startQueue: vi.fn(), stopAfterRun: vi.fn(), validateQueue: vi.fn(),
}));

vi.mock("../api/client", () => ({
  applySelection: mocks.applySelection, emergencyStop: mocks.emergencyStop, consumeBootstrapToken: vi.fn(), connectLiveEvents: mocks.connect,
  pauseAfterRun: mocks.pauseAfterRun, previewSelection: mocks.previewSelection, resumeQueue: mocks.resumeQueue,
  startQueue: mocks.startQueue, stopAfterRun: mocks.stopAfterRun, validateQueue: mocks.validateQueue,
}));
vi.mock("../api/generated", () => ({ getCatalog: mocks.getCatalog, getStatus: mocks.getStatus }));

const queue = { entries: ["countess", "mephisto"], default_entries: ["countess", "mephisto"], index: 0, cycle: 0, retry: 0, started_runs: 0, consecutive_failures: 0, total_restarts: 0, budgets: { max_runs: 4, max_duration_ms: 60000, max_consecutive_failures: 2, max_total_restarts: 2 } };
const detached = {
  schema_version: 1, core_version: "test", state: "idle", generation: 0,
  d2r: { state: "detached", window_bound: false }, input: { enabled: true, paused: false, stopped: false },
  world: { valid: false, phase: "unknown" }, selection: {}, queue,
};
const attached = { ...detached, d2r: { state: "attached", pid: 42, window_bound: true, client_width: 1920, client_height: 1080 }, world: { valid: true, phase: "in_game", area_id: 1, area_name: "Rogue Encampment" } };

describe("App", () => {
  afterEach(cleanup);
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 1, default_difficulty: "nightmare", characters: [], difficulties: [], profiles: [], runs: [] });
    mocks.getStatus.mockResolvedValueOnce(detached);
    mocks.connect.mockReturnValue(vi.fn());
  });

  it("lädt nach einem Live-Delta die aktuelle Statusprojektion neu", async () => {
    mocks.getStatus.mockResolvedValueOnce(attached);
    render(<App />);
    expect(await screen.findByText("detached")).toBeInTheDocument();
    await waitFor(() => expect(mocks.connect).toHaveBeenCalledOnce());
    const onEvent = mocks.connect.mock.calls[0][1] as (event: unknown) => void;
    await act(async () => onEvent({ sequence: 1, timestamp: new Date().toISOString(), event: "d2r_state_changed" }));
    await waitFor(() => expect(screen.getByText("attached")).toBeInTheDocument());
    expect(screen.getByText("1920 × 1080")).toBeInTheDocument();
    expect(screen.getByText("Rogue Encampment")).toBeInTheDocument();
  });

  it("zeigt gesperrte Charaktere und sendet nur eine freigegebene Auswahl", async () => {
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 3, default_difficulty: "nightmare", profiles: [], runs: [], difficulties: [{ id: "nightmare", display_name: "Alptraum" }], characters: [{ name: "MrBones", slug: "mrbones", selectable: true }, { name: "MrHammer", slug: "mrhammer", selectable: false, reasons: ["character_unconfigured", "character_anchor_missing"] }] });
    mocks.applySelection.mockResolvedValue(undefined);
    mocks.previewSelection.mockResolvedValue({ schema_version: 1, character: "MrBones", new_difficulty: "nightmare", affected_routes: [], requires_confirmation: false, confirmation_token: "safe-preview", catalog_revision: 3, lifecycle_revision: 1 });
    mocks.getStatus.mockResolvedValue(detached);
    render(<App />);
    expect(await screen.findByText("character_unconfigured, character_anchor_missing")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Auswahl in D2R anwenden" }));
    await waitFor(() => expect(mocks.applySelection).toHaveBeenCalledWith("MrBones", "nightmare", 3, 0, "safe-preview"));
  });

  it("fordert vor einer Routen-Invalidierung eine explizite Bestätigung", async () => {
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 4, default_difficulty: "nightmare", profiles: [], runs: [], difficulties: [{ id: "nightmare", display_name: "Alptraum" }, { id: "hell", display_name: "Hölle" }], characters: [{ name: "MrBones", slug: "mrbones", selectable: true }] });
    mocks.getStatus.mockResolvedValue({ ...detached, selection: { character: "MrBones", difficulty: "nightmare" } });
    mocks.previewSelection.mockResolvedValue({ schema_version: 1, character: "MrBones", old_difficulty: "nightmare", new_difficulty: "hell", affected_routes: ["countess.yaml", "mephisto.yaml"], invalidation_reason: "difficulty_changed", requires_confirmation: true, confirmation_token: "impact-preview", catalog_revision: 4, lifecycle_revision: 7 });
    mocks.applySelection.mockResolvedValue(undefined);
    render(<App />);
    fireEvent.change(await screen.findByLabelText("Schwierigkeit"), { target: { value: "hell" } });
    fireEvent.click(screen.getByRole("button", { name: "Auswahl in D2R anwenden" }));
    expect(await screen.findByRole("dialog", { name: "Routen werden unbrauchbar" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(mocks.applySelection).not.toHaveBeenCalled();
  });

  it("behält einen Selection-Fehler trotz folgendem Live-Refresh sichtbar", async () => {
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 1, default_difficulty: "nightmare", profiles: [], runs: [], difficulties: [{ id: "nightmare", display_name: "Alptraum" }], characters: [{ name: "MrBones", slug: "mrbones", selectable: true }] });
    mocks.previewSelection.mockRejectedValue(new Error("Character-Screen nicht bestätigt"));
    mocks.getStatus.mockResolvedValue(detached);
    render(<App />);
    fireEvent.click(await screen.findByRole("button", { name: "Auswahl in D2R anwenden" }));
    expect(await screen.findByText("Character-Screen nicht bestätigt")).toBeInTheDocument();
    const onEvent = mocks.connect.mock.calls[0][1] as (event: unknown) => void;
    await act(async () => onEvent({ sequence: 2, timestamp: new Date().toISOString(), event: "selection_failed" }));
    expect(screen.getByText("Character-Screen nicht bestätigt")).toBeInTheDocument();
  });

  it("baut eine Queue mit Duplikaten und startet nach vollständigem Preflight genau einmal", async () => {
    const ready = { ...detached, state: "idle_in_game", generation: 5, selection: { character: "MrBones", difficulty: "nightmare" } };
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 9, default_difficulty: "nightmare", profiles: [], characters: [], difficulties: [], runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }, { run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required" }] });
    mocks.getStatus.mockReset().mockResolvedValue(ready);
    mocks.validateQueue.mockResolvedValue({ entries: [], budgets: {} });
    mocks.startQueue.mockResolvedValue({ state: "starting_run", generation: 6 });
    render(<App />);
    fireEvent.click(await screen.findByRole("button", { name: "Countess zur Queue hinzufügen" }));
    expect(screen.getAllByText("countess")).toHaveLength(2);
    const start = screen.getByRole("button", { name: "Queue prüfen und starten" });
    fireEvent.click(start);
    fireEvent.click(start);
    await waitFor(() => expect(mocks.validateQueue).toHaveBeenCalledOnce());
    expect(mocks.validateQueue).toHaveBeenCalledWith(["countess", "mephisto", "countess"], "MrBones", "nightmare", 9);
    await waitFor(() => expect(mocks.startQueue).toHaveBeenCalledOnce());
  });

  it("ordnet, entfernt und rekonstruiert den YAML-Default tastaturzugänglich", async () => {
    const ready = { ...detached, state: "idle_in_game", selection: { character: "MrBones", difficulty: "nightmare" } };
    mocks.getStatus.mockReset().mockResolvedValue(ready);
    render(<App />);
    fireEvent.click(await screen.findByRole("button", { name: "mephisto an Position 2 nach oben" }));
    expect(screen.getByRole("button", { name: "mephisto an Position 1 nach unten" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "countess an Position 2 entfernen" }));
    expect(screen.queryByRole("button", { name: "countess an Position 2 entfernen" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Auf YAML-Default zurücksetzen" }));
    expect(screen.getByRole("button", { name: "countess an Position 1 entfernen" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "mephisto an Position 2 entfernen" })).toBeInTheDocument();
  });

  it("sperrt Queue-Mutationen und bestätigt Emergency Stop per Tastatur und Dialog", async () => {
    const running = { ...detached, state: "running_run", generation: 12, pending_intent: "", active_run_id: "countess", selection: { character: "MrBones", difficulty: "nightmare" } };
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 2, default_difficulty: "nightmare", profiles: [], characters: [], difficulties: [], runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }] });
    mocks.getStatus.mockReset().mockResolvedValue(running);
    mocks.emergencyStop.mockResolvedValue({ state: "cancelling", generation: 13 });
    render(<App />);
    expect(await screen.findByRole("button", { name: "Countess zur Queue hinzufügen" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "countess an Position 1 entfernen" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Emergency Stop" }));
    const confirm = await screen.findByRole("button", { name: "Emergency Stop bestätigen" });
    await waitFor(() => expect(confirm).toHaveFocus());
    fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: "Session sofort abbrechen?" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Emergency Stop" }));
    fireEvent.click(await screen.findByRole("button", { name: "Emergency Stop bestätigen" }));
    await waitFor(() => expect(mocks.emergencyStop).toHaveBeenCalledWith(12));
  });
});
