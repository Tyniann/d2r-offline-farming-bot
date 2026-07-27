import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const mocks = vi.hoisted(() => ({
  applySelection: vi.fn(), emergencyStop: vi.fn(), connect: vi.fn(), getCatalog: vi.fn(), getStatus: vi.fn(),
  pauseAfterRun: vi.fn(), previewSelection: vi.fn(), resumeQueue: vi.fn(), startQueue: vi.fn(), stopAfterRun: vi.fn(), validateQueue: vi.fn(),
  confirmRouteMutation: vi.fn(), previewRouteMutation: vi.fn(), startRouteWorkflow: vi.fn(), finishRouteRecording: vi.fn(), getRouteLibrary: vi.fn(), getRouteCandidates: vi.fn(), getRecordingOptions: vi.fn(), getSystemRouteStatus: vi.fn(), getHotkeyHelp: vi.fn(), getRouteWorkflow: vi.fn(),
}));

vi.mock("../api/client", () => ({
  applySelection: mocks.applySelection, emergencyStop: mocks.emergencyStop, consumeBootstrapToken: vi.fn(), connectLiveEvents: mocks.connect,
  pauseAfterRun: mocks.pauseAfterRun, previewSelection: mocks.previewSelection, resumeQueue: mocks.resumeQueue,
  startQueue: mocks.startQueue, stopAfterRun: mocks.stopAfterRun, validateQueue: mocks.validateQueue,
  confirmRouteMutation: mocks.confirmRouteMutation, previewRouteMutation: mocks.previewRouteMutation, startRouteWorkflow: mocks.startRouteWorkflow, finishRouteRecording: mocks.finishRouteRecording,
}));
vi.mock("../api/generated", () => ({ getCatalog: mocks.getCatalog, getStatus: mocks.getStatus, getRouteLibrary: mocks.getRouteLibrary, getRouteCandidates: mocks.getRouteCandidates, getRecordingOptions: mocks.getRecordingOptions, getSystemRouteStatus: mocks.getSystemRouteStatus, getHotkeyHelp: mocks.getHotkeyHelp, getRouteWorkflow: mocks.getRouteWorkflow }));
vi.mock("../features/routes/RouteFeature", () => ({ RouteFeature: ({ onReturnToOnboarding }: { onReturnToOnboarding?: () => void }) => <section><h2>Routenfunktion</h2>{onReturnToOnboarding && <button onClick={onReturnToOnboarding}>Zurück zur Einrichtung</button>}</section> }));
vi.mock("../features/onboarding/OnboardingFeature", () => ({ OnboardingFeature: () => <section><h1>First-Run-Assistent</h1></section> }));
vi.mock("../features/pickit/PickitFeature", () => ({ PickitFeature: () => <section><h2>Pickit-Funktion</h2></section> }));
vi.mock("../features/history/HistoryFeature", () => ({ HistoryFeature: () => <section><h2>Historie</h2></section> }));
vi.mock("../features/settings/SettingsFeature", () => ({ SettingsFeature: () => <section><h2>Settings-Funktion</h2></section> }));

const queue = { entries: ["countess", "mephisto"], default_entries: ["countess", "mephisto"], index: 0, cycle: 0, retry: 0, started_runs: 0, consecutive_failures: 0, total_restarts: 0, budgets: { max_runs: 4, max_duration_ms: 60000, max_consecutive_failures: 2, max_total_restarts: 2 } };
const compatible = { state: "compatible", supported_version: "3.2.92777", expected_version: "3.2.92777", offset_version: "3.2.92777", actual_version: "3.2.92777", privilege_mismatch: false };
const detached = {
  schema_version: 1, app_version: "test", core_version: "test", state: "idle", lifecycle_phase: "idle", generation: 0,
  d2r: { state: "detached", window_bound: false }, input: { enabled: true, paused: false, stopped: false },
  world: { valid: false, phase: "unknown" }, compatibility: compatible, selection: {}, queue,
};
const attached = { ...detached, d2r: { state: "attached", pid: 42, window_bound: true, client_width: 1920, client_height: 1080 }, world: { valid: true, phase: "in_game", area_id: 1, area_name: "Rogue Encampment" } };

describe("App", () => {
  afterEach(() => { cleanup(); delete window.d2rDesktop; });
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getCatalog.mockReset();
    mocks.getStatus.mockReset();
    mocks.connect.mockReset();
    window.history.replaceState(null, "", "#dashboard");
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 1, default_difficulty: "nightmare", characters: [], difficulties: [], profiles: [], runs: [] });
    mocks.getStatus.mockResolvedValueOnce(detached);
    mocks.connect.mockReturnValue(vi.fn());
    mocks.getRouteLibrary.mockResolvedValue({ schema_version: 1, revision: 1, character: "", routes: [] });
    mocks.getRouteCandidates.mockResolvedValue([]); mocks.getRecordingOptions.mockResolvedValue([]); mocks.getSystemRouteStatus.mockResolvedValue([]);
    mocks.getHotkeyHelp.mockResolvedValue({ recording_finish: "f9", stop_after_run: "f10", emergency_stop: "f11", pause: "pause" });
    mocks.getRouteWorkflow.mockResolvedValue({ workflow_id: "", generation: 1, state: "idle", run_id: "", character: "" });
  });

  it("bindet die Core-autoritäre Historie über die Hauptnavigation ein", async () => {
    render(<App />);
    expect(await screen.findByRole("heading", { name: "Lokales Dashboard" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Historie" })).toHaveAttribute("href", "#history");
    await act(async () => {
      window.location.hash = "#history";
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(screen.getByRole("heading", { level: 1, name: "Historie" })).toBeInTheDocument();
  });

  it("ordnet die offene Einrichtung zuerst an und führt aus dem Routenbereich zurück", async () => {
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 1, default_difficulty: "nightmare", characters: [], difficulties: [], profiles: [],
      runs: [{ run_id: "countess", display_name: "Countess", status: "unavailable", reasons: ["route_assignment_missing"] }],
    });
    render(<App />);
    const setup = await screen.findByText("Einrichtung fortsetzen");
    const coreStatus = screen.getByRole("heading", { name: "Core-Status" });
    expect(setup.closest("section")!.compareDocumentPosition(coreStatus.closest("section")!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Erste Route aufnehmen" }));
    expect(await screen.findByRole("button", { name: "Zurück zur Einrichtung" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Zurück zur Einrichtung" }));
    expect(await screen.findByRole("heading", { name: "First-Run-Assistent" })).toBeInTheDocument();
  });

  it("beendet die Erste-Route-Einrichtung sobald ein Run runtime-validierbar ist", async () => {
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 3, default_difficulty: "hell", characters: [], difficulties: [], profiles: [],
      runs: [
        { run_id: "countess", display_name: "Countess", status: "runtime_validation_required", reasons: ["route_runtime_validation_required"] },
        { run_id: "mephisto", display_name: "Mephisto", status: "unavailable", reasons: ["route_assignment_missing"] },
      ],
    });
    render(<App />);
    await screen.findByRole("heading", { name: "Lokales Dashboard" });
    expect(screen.queryByText("Einrichtung fortsetzen")).not.toBeInTheDocument();
  });

  it("lädt den Katalog nach einer veröffentlichten Routenmutation live neu", async () => {
    const missing = {
      schema_version: 1, revision: 2, default_difficulty: "hell", characters: [], difficulties: [], profiles: [],
      runs: [{ run_id: "countess", display_name: "Countess", status: "unavailable", reasons: ["route_assignment_missing"] }],
    };
    const published = {
      ...missing, revision: 3,
      runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required", reasons: ["route_runtime_validation_required"] }],
    };
    mocks.getCatalog.mockReset().mockResolvedValueOnce(missing).mockResolvedValue(published);
    render(<App />);
    expect(await screen.findByText("Einrichtung fortsetzen")).toBeInTheDocument();
    await waitFor(() => expect(mocks.connect).toHaveBeenCalledOnce());
    const onEvent = mocks.connect.mock.calls[0][1] as (event: unknown) => void;
    await act(async () => onEvent({ sequence: 3, timestamp: new Date().toISOString(), event: "route_library_changed" }));
    await waitFor(() => expect(screen.queryByText("Einrichtung fortsetzen")).not.toBeInTheDocument());
  });

  it("stellt alle fünf iconbeschrifteten Hash-Ziele tastaturzugänglich bereit", async () => {
    render(<App />);
    await screen.findByRole("heading", { level: 1, name: "Lokales Dashboard" });
    const targets = [
      ["Dashboard", "dashboard", "Lokales Dashboard"],
      ["Routen", "routes", "Routen"],
      ["Pickit", "pickit", "Pickit"],
      ["Historie", "history", "Historie"],
      ["Einstellungen", "settings", "Einstellungen"],
    ] as const;
    for (const [label, hash, heading] of targets) {
      const link = screen.getByRole("link", { name: label });
      expect(link).toHaveAttribute("href", `#${hash}`);
      expect(link.querySelector("svg")).not.toBeNull();
      link.focus();
      expect(link).toHaveFocus();
      await act(async () => {
        window.location.hash = `#${hash}`;
        window.dispatchEvent(new HashChangeEvent("hashchange"));
      });
      expect(screen.getByRole("heading", { level: 1, name: heading })).toBeInTheDocument();
      expect(link).toHaveAttribute("aria-current", "page");
    }
  });

  it("öffnet native Notification-Ziele im vorhandenen Fenster am stabilen Hash", async () => {
    let navigate: ((target: string) => void) | undefined;
    window.d2rDesktop = {
      getProvisioningState: vi.fn().mockResolvedValue({ required: false, import_selected: false, import_label: "" }), chooseImportRoot: vi.fn(), provision: vi.fn(),
      getAppInfo: vi.fn(), getDesktopSettings: vi.fn().mockResolvedValue({ schema_version: 1, autostart: false, onboarding_completed: true }), updateDesktopSettings: vi.fn(), showWindow: vi.fn(), restartCore: vi.fn(), restartAsAdministrator: vi.fn(),
      onNavigate: vi.fn((callback) => { navigate = callback; return vi.fn(); }),
    };
    render(<App />);
    await screen.findByRole("heading", { level: 1, name: "Lokales Dashboard" });
    await act(async () => navigate?.("settings"));
    expect(window.location.hash).toBe("#settings");
    expect(await screen.findByRole("heading", { level: 1, name: "Einstellungen" })).toBeInTheDocument();
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
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 3, default_difficulty: "nightmare", profiles: [{ id: "necro_bone_spear", character_class: "necromancer" }], runs: [], difficulties: [{ id: "nightmare", display_name: "Alptraum" }], characters: [{ name: "MrBones", slug: "mrbones", selectable: true }, { name: "MrHammer", slug: "mrhammer", selectable: false, reasons: ["character_unconfigured", "character_anchor_missing"] }] });
    mocks.applySelection.mockResolvedValue(undefined);
    mocks.previewSelection.mockResolvedValue({ schema_version: 1, character: "MrBones", new_difficulty: "nightmare", affected_routes: [], requires_confirmation: false, confirmation_token: "safe-preview", catalog_revision: 3, lifecycle_revision: 1 });
    mocks.getStatus.mockResolvedValue(detached);
    render(<App />);
    expect(await screen.findByText(/Kein unterstütztes Kampfprofil zugeordnet.*Totenbeschwörer.*Automatische Auswahl/)).toBeInTheDocument();
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
    await waitFor(() => expect(mocks.previewSelection).toHaveBeenCalledOnce());
    expect(await screen.findByText("Character-Screen nicht bestätigt", {}, { timeout: 10_000 })).toBeInTheDocument();
    const onEvent = mocks.connect.mock.calls[0][1] as (event: unknown) => void;
    await act(async () => onEvent({ sequence: 2, timestamp: new Date().toISOString(), event: "selection_failed" }));
    expect(screen.getByText("Character-Screen nicht bestätigt")).toBeInTheDocument();
  });

  it("startet die persistente Charakter-Queue nach vollständigem Preflight genau einmal", async () => {
    const ready = { ...detached, state: "idle_in_game", generation: 5, selection: { character: "MrBones", difficulty: "nightmare" } };
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 9, default_difficulty: "nightmare", profiles: [], characters: [], difficulties: [], runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }, { run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required" }] });
    mocks.getStatus.mockReset().mockResolvedValue(ready);
    mocks.validateQueue.mockResolvedValue({ entries: [], budgets: {} });
    mocks.startQueue.mockResolvedValue({ state: "starting_run", generation: 6 });
    render(<App />);
    expect(await screen.findByRole("heading", { name: "Konfigurierte Queue" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Queue in Einstellungen ändern" })).toHaveAttribute("href", "#settings");
    const start = screen.getByRole("button", { name: "Queue prüfen und starten" });
    fireEvent.click(start);
    fireEvent.click(start);
    await waitFor(() => expect(mocks.validateQueue).toHaveBeenCalledOnce());
    expect(mocks.validateQueue).toHaveBeenCalledWith(["countess", "mephisto"], "MrBones", "nightmare", 9);
    await waitFor(() => expect(mocks.startQueue).toHaveBeenCalledOnce());
  });

  it("zeigt die persistente Queue read-only in ihrer gespeicherten Reihenfolge", async () => {
    const ready = { ...detached, state: "idle_in_game", selection: { character: "MrBones", difficulty: "nightmare" } };
    mocks.getStatus.mockReset().mockResolvedValue(ready);
    render(<App />);
    const entries = await screen.findAllByRole("listitem");
    expect(entries.some((entry) => entry.textContent?.includes("1countess"))).toBe(true);
    expect(entries.some((entry) => entry.textContent?.includes("2mephisto"))).toBe(true);
    expect(screen.queryByRole("button", { name: /entfernen/ })).not.toBeInTheDocument();
  });

  it("sperrt den Queue-Start und bestätigt Emergency Stop per Tastatur und Dialog", async () => {
    const running = { ...detached, state: "running_run", generation: 12, pending_intent: "", active_run_id: "countess", selection: { character: "MrBones", difficulty: "nightmare" } };
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 2, default_difficulty: "nightmare", profiles: [], characters: [], difficulties: [], runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }] });
    mocks.getStatus.mockReset().mockResolvedValue(running);
    mocks.emergencyStop.mockResolvedValue({ state: "cancelling", generation: 13 });
    render(<App />);
    expect(await screen.findByRole("button", { name: "Queue prüfen und starten" })).toBeDisabled();
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
