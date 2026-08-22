import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { changeAppLanguage } from "../i18n";
import { apiError } from "../test/apiError";
import { App } from "./App";

const mocks = vi.hoisted(() => ({
  applySelection: vi.fn(), emergencyStop: vi.fn(), connect: vi.fn(), getCatalog: vi.fn(), getStatus: vi.fn(), getRunAvailabilities: vi.fn(), getOperatorSettings: vi.fn(),
  pauseAfterRun: vi.fn(), previewSelection: vi.fn(), resumeQueue: vi.fn(), startQueue: vi.fn(), stopAfterRun: vi.fn(), validateQueue: vi.fn(),
  confirmRouteMutation: vi.fn(), previewRouteMutation: vi.fn(), startRouteWorkflow: vi.fn(), finishRouteRecording: vi.fn(), getRouteLibrary: vi.fn(), getRouteCandidates: vi.fn(), getRecordingOptions: vi.fn(), getSystemRouteStatus: vi.fn(), getHotkeyHelp: vi.fn(), getRouteWorkflow: vi.fn(),
  getHistorySummary: vi.fn(), getHistoryComparisons: vi.fn(), getHistoryRuns: vi.fn(),
}));

vi.mock("../api/client", () => ({
  applySelection: mocks.applySelection, emergencyStop: mocks.emergencyStop, consumeBootstrapToken: vi.fn(), connectLiveEvents: mocks.connect,
  pauseAfterRun: mocks.pauseAfterRun, previewSelection: mocks.previewSelection, resumeQueue: mocks.resumeQueue,
  startQueue: mocks.startQueue, stopAfterRun: mocks.stopAfterRun, validateQueue: mocks.validateQueue,
  confirmRouteMutation: mocks.confirmRouteMutation, previewRouteMutation: mocks.previewRouteMutation, startRouteWorkflow: mocks.startRouteWorkflow, finishRouteRecording: mocks.finishRouteRecording,
}));
vi.mock("../api/generated", () => ({ getCatalog: mocks.getCatalog, getStatus: mocks.getStatus, getRunAvailabilities: mocks.getRunAvailabilities, getOperatorSettings: mocks.getOperatorSettings, getRouteLibrary: mocks.getRouteLibrary, getRouteCandidates: mocks.getRouteCandidates, getRecordingOptions: mocks.getRecordingOptions, getSystemRouteStatus: mocks.getSystemRouteStatus, getHotkeyHelp: mocks.getHotkeyHelp, getRouteWorkflow: mocks.getRouteWorkflow, getHistorySummary: mocks.getHistorySummary, getHistoryComparisons: mocks.getHistoryComparisons, getHistoryRuns: mocks.getHistoryRuns }));
vi.mock("../features/routes/RouteFeature", () => ({ RouteFeature: ({ selectedCharacter, onReturnToOnboarding }: { selectedCharacter: string; onReturnToOnboarding?: () => void }) => <section><h1>Routen</h1><p>Routen-Kontext {selectedCharacter}</p>{onReturnToOnboarding && <button onClick={onReturnToOnboarding}>Zurück zur Einrichtung</button>}</section> }));
vi.mock("../features/onboarding/OnboardingFeature", () => ({ OnboardingFeature: () => <section><h1>First-Run-Assistent</h1></section> }));
vi.mock("../features/pickit/PickitFeature", () => ({ PickitFeature: ({ selectedCharacter }: { selectedCharacter: string }) => <section><h2>Pickit-Funktion</h2><p>Pickit-Kontext {selectedCharacter}</p></section> }));
vi.mock("../features/history/HistoryFeature", () => ({ HistoryFeature: ({ selectedCharacter, selectedDifficulty }: { selectedCharacter: string; selectedDifficulty: string }) => <section><h2>Historie</h2><p>Historien-Kontext {selectedCharacter} / {selectedDifficulty}</p></section> }));
vi.mock("../features/settings/SettingsFeature", () => ({ SettingsFeature: ({ selectedCharacter }: { selectedCharacter: string }) => <section><h2>Settings-Funktion</h2><p>Einstellungs-Kontext {selectedCharacter}</p></section> }));
vi.mock("../features/characters/CharacterSetupWizard", () => ({ CharacterSetupWizard: () => <section><h2>Charakter-Setup</h2></section> }));

const operatorSettings = {
  schema_version: 3, revision: 1, characters: {} as Record<string, { last_difficulty: "normal" | "nightmare" | "hell"; queue: string[] }>,
  budgets: { max_runs: 4, max_duration_ms: 60000, max_consecutive_failures: 2, max_total_restarts: 2 },
  input: { enabled: true, pause_hotkey: "pause", stop_after_run_hotkey: "f10", recording_finish_hotkey: "f9", emergency_stop_hotkey: "f11" },
  history: { retention_enabled: false, retention_days: 14 },
};
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
  beforeEach(async () => {
    await changeAppLanguage("de");
    vi.clearAllMocks();
    delete window.d2rDesktop;
    mocks.getCatalog.mockReset();
    mocks.getStatus.mockReset();
    mocks.connect.mockReset();
    mocks.getRunAvailabilities.mockReset();
    mocks.getOperatorSettings.mockReset();
    window.history.replaceState(null, "", "#dashboard");
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 1, default_difficulty: "nightmare", characters: [], difficulties: [], profiles: [], runs: [] });
    mocks.getRunAvailabilities.mockResolvedValue({ schema_version: 1, character: "", difficulty: "", runs: [] });
    mocks.getOperatorSettings.mockResolvedValue(operatorSettings);
    mocks.getStatus.mockResolvedValueOnce(detached);
    mocks.connect.mockReturnValue(vi.fn());
    mocks.getRouteLibrary.mockResolvedValue({ schema_version: 1, revision: 1, character: "", routes: [] });
    mocks.getRouteCandidates.mockResolvedValue([]); mocks.getRecordingOptions.mockResolvedValue([]); mocks.getSystemRouteStatus.mockResolvedValue([]);
    mocks.getHotkeyHelp.mockResolvedValue({ recording_finish: "f9", stop_after_run: "f10", emergency_stop: "f11", pause: "pause" });
    mocks.getRouteWorkflow.mockResolvedValue({ workflow_id: "", generation: 1, state: "idle", run_id: "", character: "" });
    mocks.getHistorySummary.mockResolvedValue({ summary: { runs: 0, terminal_runs: 0, successful: 0, failed: 0, aborted: 0, incomplete: 0, running: 0, boss_kills: 0, durations: { count: 0, total_ms: 0, average_ms: 0, median_ms: 0, minimum_ms: 0, maximum_ms: 0 }, stages: { travel_ms: 0, combat_ms: 0, loot_ms: 0, return_town_ms: 0, other_ms: 0 }, funnel: { seen: 0, matched: 0, picked_up: 0, stashed: 0, sold: 0, keep_return: 0, pickup_lost: 0, post_pickup_lost: 0 } } });
    mocks.getHistoryComparisons.mockResolvedValue({ comparisons: [] });
    mocks.getHistoryRuns.mockResolvedValue({ runs: [] });
  });

  it("rendert die repräsentative Shell auf Englisch", async () => {
    await changeAppLanguage("en");
    render(<App />);

    expect(await screen.findByRole("heading", { name: "Prepare farming" })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Main navigation" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "History" })).toHaveAttribute("href", "#history");
    expect(screen.getByRole("link", { name: "Settings" })).toHaveAttribute("href", "#settings");
  });

  it("bindet die Core-autoritäre Historie über die Hauptnavigation ein", async () => {
    render(<App />);
    expect(await screen.findByRole("heading", { name: "Farming vorbereiten" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Historie" })).toHaveAttribute("href", "#history");
    await act(async () => {
      window.location.hash = "#history";
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(screen.getByRole("heading", { level: 1, name: "Historie" })).toBeInTheDocument();
  });

  it("rendert nach Emergency-Stop mit null Queue-Entries ohne Absturz", async () => {
    mocks.getStatus.mockReset();
    mocks.getStatus.mockResolvedValue({
      ...detached,
      queue: { ...queue, entries: null as unknown as string[], default_entries: ["countess"] },
      last_result: { disposition: "stop", reason: "emergency_stop_requested" },
    });
    render(<App />);
    expect(await screen.findByRole("heading", { name: "Farming vorbereiten" })).toBeInTheDocument();
  });

  it("ordnet die offene Einrichtung zuerst an und führt aus dem Routenbereich zurück", async () => {
    mocks.getStatus.mockReset().mockResolvedValue({ ...detached, selection: { character: "MrBones", difficulty: "nightmare" } });
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 1, default_difficulty: "nightmare", characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }], difficulties: [{ id: "nightmare", display_name: "Alptraum" }], profiles: [],
      runs: [{ run_id: "countess", display_name: "Countess", status: "unavailable", reasons: ["route_assignment_missing"] }],
    });
    mocks.getRunAvailabilities.mockResolvedValue({
      schema_version: 1, character: "MrBones", difficulty: "nightmare",
      runs: [{ run_id: "countess", display_name: "Countess", status: "unavailable", reasons: ["route_assignment_missing"] }],
    });
    render(<App />);
    const setup = await screen.findByText("Einrichtung fortsetzen");
    const farming = screen.getByRole("heading", { name: "Deine Run-Reihenfolge" });
    expect(setup.closest("section")!.compareDocumentPosition(farming.closest("section")!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Erste Route aufnehmen" }));
    expect(await screen.findByRole("button", { name: "Zurück zur Einrichtung" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Zurück zur Einrichtung" }));
    expect(await screen.findByRole("heading", { name: "First-Run-Assistent" })).toBeInTheDocument();
  });

  it("beendet die Erste-Route-Einrichtung sobald ein Run runtime-validierbar ist", async () => {
    mocks.getStatus.mockReset().mockResolvedValue({ ...detached, selection: { character: "MrBones", difficulty: "hell" } });
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 3, default_difficulty: "hell", characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }], difficulties: [{ id: "hell", display_name: "Hölle" }], profiles: [],
      runs: [
        { run_id: "countess", display_name: "Countess", status: "runtime_validation_required", reasons: ["route_runtime_validation_required"] },
        { run_id: "mephisto", display_name: "Mephisto", status: "unavailable", reasons: ["route_assignment_missing"] },
      ],
    });
    mocks.getRunAvailabilities.mockResolvedValue({
      schema_version: 1, character: "MrBones", difficulty: "hell",
      runs: [
        { run_id: "countess", display_name: "Countess", status: "runtime_validation_required", reasons: ["route_runtime_validation_required"] },
        { run_id: "mephisto", display_name: "Mephisto", status: "unavailable", reasons: ["route_assignment_missing"] },
      ],
    });
    render(<App />);
    await screen.findByRole("heading", { name: "MrBones ist bereit" });
    expect(screen.queryByText("Einrichtung fortsetzen")).not.toBeInTheDocument();
  });

  it("lädt den Katalog nach einer veröffentlichten Routenmutation live neu", async () => {
    mocks.getStatus.mockReset().mockResolvedValue({ ...detached, selection: { character: "MrBones", difficulty: "hell" } });
    const missing = {
      schema_version: 1, revision: 2, default_difficulty: "hell", characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }], difficulties: [{ id: "hell", display_name: "Hölle" }], profiles: [],
      runs: [{ run_id: "countess", display_name: "Countess", status: "unavailable", reasons: ["route_assignment_missing"] }],
    };
    const published = {
      ...missing, revision: 3,
      runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required", reasons: ["route_runtime_validation_required"] }],
    };
    mocks.getCatalog.mockReset().mockResolvedValueOnce(missing).mockResolvedValue(published);
    mocks.getRunAvailabilities.mockResolvedValueOnce({ schema_version: 1, character: "MrBones", difficulty: "hell", runs: missing.runs })
      .mockResolvedValue({ schema_version: 1, character: "MrBones", difficulty: "hell", runs: published.runs });
    render(<App />);
    expect(await screen.findByText("Einrichtung fortsetzen")).toBeInTheDocument();
    await waitFor(() => expect(mocks.connect).toHaveBeenCalledOnce());
    const onEvent = mocks.connect.mock.calls[0][1] as (event: unknown) => void;
    await act(async () => onEvent({ sequence: 3, timestamp: new Date().toISOString(), event: "route_library_changed" }));
    await waitFor(() => expect(screen.queryByText("Einrichtung fortsetzen")).not.toBeInTheDocument());
  });

  it("stellt alle fünf iconbeschrifteten Hash-Ziele tastaturzugänglich bereit", async () => {
    render(<App />);
    await screen.findByRole("heading", { level: 1, name: "Farming vorbereiten" });
    const targets = [
      ["Dashboard", "dashboard", "Farming vorbereiten"],
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
    await screen.findByRole("heading", { level: 1, name: "Farming vorbereiten" });
    await act(async () => navigate?.("settings"));
    expect(window.location.hash).toBe("#settings");
    expect(await screen.findByRole("heading", { level: 1, name: "Einstellungen" })).toBeInTheDocument();
  });

  it("lädt nach einem Live-Delta die aktuelle Statusprojektion neu", async () => {
    mocks.getStatus.mockResolvedValueOnce(attached);
    render(<App />);
    expect(await screen.findByText("D2R nicht bereit")).toBeInTheDocument();
    await waitFor(() => expect(mocks.connect).toHaveBeenCalledOnce());
    const onEvent = mocks.connect.mock.calls[0][1] as (event: unknown) => void;
    await act(async () => onEvent({ sequence: 1, timestamp: new Date().toISOString(), event: "d2r_state_changed" }));
    await waitFor(() => expect(screen.getByText("D2R bereit")).toBeInTheDocument());
    expect(screen.getByText("Lager der Jägerinnen")).toBeInTheDocument();
  });

  it("zeigt gesperrte Charaktere und sendet nur eine freigegebene Auswahl", async () => {
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 3, default_difficulty: "nightmare", profiles: [{ id: "necro_bone_spear", character_class: "necromancer" }], runs: [], difficulties: [{ id: "nightmare", display_name: "Alptraum" }], characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }, { name: "MrHammer", slug: "mrhammer", selectable: false, reasons: ["character_class_unsupported", "character_anchor_missing"] }] });
    mocks.applySelection.mockResolvedValue(undefined);
    mocks.previewSelection.mockResolvedValue({ schema_version: 1, character: "MrBones", new_difficulty: "nightmare", affected_routes: [], requires_confirmation: false, confirmation_token: "safe-preview", catalog_revision: 3, lifecycle_revision: 1 });
    mocks.getStatus.mockResolvedValue(detached);
    render(<App />);
    expect(await screen.findByRole("option", { name: "MrHammer – nicht verfügbar" })).toBeDisabled();
    expect(screen.getByLabelText("Charakter")).toHaveValue("MrBones");
    expect(screen.queryByText("Nicht nutzbare Charaktere")).not.toBeInTheDocument();
    expect(screen.queryByText(/Für diese Klasse gibt es noch kein freigegebenes Kampfprofil/)).not.toBeInTheDocument();
    const apply = await screen.findByRole("button", { name: "In D2R verwenden" });
    await waitFor(() => expect(apply).toBeEnabled());
    fireEvent.click(apply);
    await waitFor(() => expect(mocks.applySelection).toHaveBeenCalledWith("MrBones", "nightmare", 3, 0, "safe-preview"));
  });

  it("übernimmt die bestätigte D2R-Auswahl statt des ersten Katalog-Charakters", async () => {
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 3, default_difficulty: "nightmare",
      profiles: [{ id: "necro_bone_spear", character_class: "necromancer" }, { id: "paladin_hammerdin", character_class: "paladin" }],
      runs: [],
      difficulties: [{ id: "nightmare", display_name: "Alptraum" }, { id: "hell", display_name: "Hölle" }],
      characters: [
        { name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true },
        { name: "MrHammer", slug: "mrhammer", selectable: true, farm_ready: true },
      ],
    });
    mocks.getStatus.mockReset().mockResolvedValue({ ...detached, selection: { character: "MrHammer", difficulty: "hell" } });
    render(<App />);
    await waitFor(() => expect(screen.getByLabelText("Charakter")).toHaveValue("MrHammer"));
    expect(screen.getByLabelText("Schwierigkeit")).toHaveValue("hell");
    expect(screen.queryByText("Andere Auswahl gewählt")).not.toBeInTheDocument();
    expect(screen.queryByText(/Entwurf/)).not.toBeInTheDocument();
  });

  it("aktualisiert alle Vorschauen appweit und hält den Start bis zur D2R-Bestätigung gesperrt", async () => {
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 3, default_difficulty: "nightmare", profiles: [],
      difficulties: [{ id: "nightmare", display_name: "Alptraum" }, { id: "hell", display_name: "Hölle" }],
      characters: [
        { name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true, expected_class: "necromancer" },
        { name: "MrHammer", slug: "mrhammer", selectable: true, farm_ready: true, expected_class: "paladin" },
      ],
      runs: [{ run_id: "countess", display_name: "Countess" }, { run_id: "mephisto", display_name: "Mephisto" }],
    });
    const bonesStatus = {
      ...detached,
      state: "idle_in_game",
      selection: { character: "MrBones", difficulty: "nightmare" },
      queue: { ...queue, default_entries: ["countess"] },
    };
    mocks.getStatus.mockReset().mockResolvedValue(bonesStatus);
    mocks.getOperatorSettings.mockResolvedValue({
      ...operatorSettings,
      characters: {
        mrbones: { last_difficulty: "nightmare", queue: ["countess"] },
        mrhammer: { last_difficulty: "hell", queue: ["mephisto"] },
      },
    });
    mocks.getRunAvailabilities.mockImplementation(async (name: string) => ({
      schema_version: 1, character: name, difficulty: name === "MrHammer" ? "hell" : "nightmare",
      runs: name === "MrHammer"
        ? [{ run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required" }]
        : [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }],
    }));
    mocks.previewSelection.mockResolvedValue({
      schema_version: 1, character: "MrHammer", new_difficulty: "hell",
      affected_routes: [], requires_confirmation: false, confirmation_token: "switch-preview",
      catalog_revision: 3, lifecycle_revision: 1,
    });
    mocks.applySelection.mockResolvedValue(undefined);
    render(<App />);
    await screen.findByRole("option", { name: "MrHammer" });
    fireEvent.change(screen.getByLabelText("Charakter"), { target: { value: "MrHammer" } });
    await screen.findByText("MrHammer ist in der App ausgewählt");
    expect(screen.getByLabelText("Charakter")).toHaveValue("MrHammer");
    expect(screen.getByLabelText("Schwierigkeit")).toHaveValue("hell");
    expect(screen.getByRole("heading", { name: "MrHammer ist bereit" })).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Jetzt farmen" })).toBeDisabled();
    expect(mocks.validateQueue).not.toHaveBeenCalled();
    expect(mocks.startQueue).not.toHaveBeenCalled();
    for (const [target, context] of [
      ["routes", "Routen-Kontext MrHammer"],
      ["pickit", "Pickit-Kontext MrHammer"],
      ["history", "Historien-Kontext MrHammer / hell"],
      ["settings", "Einstellungs-Kontext mrhammer"],
    ] as const) {
      await act(async () => {
        window.location.hash = `#${target}`;
        window.dispatchEvent(new HashChangeEvent("hashchange"));
      });
      expect(screen.getByText(context)).toBeInTheDocument();
    }
  });

  it("sperrt Charakter und Schwierigkeit während einer laufenden Session", async () => {
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 3, default_difficulty: "nightmare", profiles: [], runs: [],
      difficulties: [{ id: "nightmare", display_name: "Alptraum" }, { id: "hell", display_name: "Hölle" }],
      characters: [
        { name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true },
        { name: "MrHammer", slug: "mrhammer", selectable: true, farm_ready: true },
      ],
    });
    mocks.getStatus.mockReset().mockResolvedValue({
      ...detached,
      state: "running_run",
      lifecycle_phase: "running_run",
      selection: { character: "MrBones", difficulty: "nightmare" },
    });

    render(<App />);

    const character = await screen.findByLabelText("Charakter");
    await waitFor(() => expect(character).toHaveValue("MrBones"));
    expect(character).toBeDisabled();
    expect(screen.getByLabelText("Schwierigkeit")).toBeDisabled();
    expect(screen.queryByRole("button", { name: "In D2R verwenden" })).not.toBeInTheDocument();
    expect(screen.getByText("Während der Session gesperrt")).toBeInTheDocument();
  });

  it("rekonstruiert nach einem Renderer-Reload die bestätigte Auswahl und deren Queue", async () => {
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 3, default_difficulty: "nightmare", profiles: [],
      difficulties: [{ id: "nightmare", display_name: "Alptraum" }, { id: "hell", display_name: "Hölle" }],
      characters: [
        { name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true },
        { name: "MrHammer", slug: "mrhammer", selectable: true, farm_ready: true },
      ],
      runs: [{ run_id: "countess", display_name: "Countess" }, { run_id: "mephisto", display_name: "Mephisto" }],
    });
    mocks.getOperatorSettings.mockResolvedValue({
      ...operatorSettings,
      characters: {
        mrbones: { last_difficulty: "nightmare", queue: ["countess"] },
        mrhammer: { last_difficulty: "hell", queue: ["mephisto"] },
      },
    });
    mocks.getRunAvailabilities.mockImplementation(async (name: string) => ({
      schema_version: 1,
      character: name,
      difficulty: name === "MrHammer" ? "hell" : "nightmare",
      runs: [{ run_id: name === "MrHammer" ? "mephisto" : "countess", display_name: name === "MrHammer" ? "Mephisto" : "Countess", status: "runtime_validation_required" }],
    }));
    mocks.getStatus.mockReset().mockResolvedValue({
      ...detached,
      state: "idle_in_game",
      selection: { character: "MrHammer", difficulty: "hell" },
      queue: { ...queue, default_entries: ["mephisto"] },
    });

    const firstRenderer = render(<App />);
    await waitFor(() => expect(screen.getByLabelText("Charakter")).toHaveValue("MrHammer"));
    fireEvent.change(screen.getByLabelText("Charakter"), { target: { value: "MrBones" } });
    expect(screen.getByText("MrBones ist in der App ausgewählt")).toBeInTheDocument();
    firstRenderer.unmount();

    render(<App />);

    await waitFor(() => expect(screen.getByLabelText("Charakter")).toHaveValue("MrHammer"));
    expect(screen.getByLabelText("Schwierigkeit")).toHaveValue("hell");
    expect(screen.queryByText("MrBones ist in der App ausgewählt")).not.toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: "Deine Run-Reihenfolge" })).toBeInTheDocument();
    await waitFor(() => expect(screen.getAllByRole("listitem").some((entry) => entry.textContent?.includes("1Mephisto"))).toBe(true));
  });

  it("stellt die gespeicherte App-Auswahl wieder her, ohne sie als D2R-Bestätigung zu behandeln", async () => {
    const updateDesktopSettings = vi.fn(async (request: Parameters<D2RDesktopBridge["updateDesktopSettings"]>[0]) => ({
      schema_version: 3, language: "de" as const, autostart: false, onboarding_completed: true,
      selected_character: request.selected_character,
      selected_difficulty: request.selected_difficulty,
    }));
    window.d2rDesktop = desktopBridge({
      schema_version: 3, language: "de", autostart: false, onboarding_completed: true,
      selected_character: "MrHammer", selected_difficulty: "hell",
    }, updateDesktopSettings);
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 3, default_difficulty: "nightmare", profiles: [],
      difficulties: [{ id: "nightmare", display_name: "Alptraum" }, { id: "hell", display_name: "Hölle" }],
      characters: [
        { name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true },
        { name: "MrHammer", slug: "mrhammer", selectable: true, farm_ready: true },
      ],
      runs: [{ run_id: "countess", display_name: "Countess" }, { run_id: "mephisto", display_name: "Mephisto" }],
    });
    mocks.getOperatorSettings.mockResolvedValue({
      ...operatorSettings,
      characters: {
        mrbones: { last_difficulty: "nightmare", queue: ["countess"] },
        mrhammer: { last_difficulty: "hell", queue: ["mephisto"] },
      },
    });
    mocks.getRunAvailabilities.mockResolvedValue({
      schema_version: 1, character: "MrHammer", difficulty: "hell",
      runs: [{ run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required" }],
    });
    mocks.getStatus.mockReset().mockResolvedValue({
      ...detached,
      state: "idle_in_game",
      selection: { character: "MrBones", difficulty: "nightmare" },
      queue: { ...queue, default_entries: ["countess"] },
    });

    render(<App />);

    await waitFor(() => expect(screen.getByLabelText("Charakter")).toHaveValue("MrHammer"));
    expect(screen.getByLabelText("Schwierigkeit")).toHaveValue("hell");
    expect(screen.getByText("MrHammer ist in der App ausgewählt")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "MrHammer ist bereit" })).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Jetzt farmen" })).toBeDisabled();
    expect(mocks.applySelection).not.toHaveBeenCalled();
    expect(updateDesktopSettings).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("Schwierigkeit"), { target: { value: "nightmare" } });
    await waitFor(() => expect(updateDesktopSettings).toHaveBeenCalledWith({ selected_character: "MrHammer", selected_difficulty: "nightmare" }));
  });

  it("verwirft unbekannte gespeicherte Auswahlwerte gegen den Katalog", async () => {
    const updateDesktopSettings = vi.fn(async (request: Parameters<D2RDesktopBridge["updateDesktopSettings"]>[0]) => ({
      schema_version: 3, language: request.language ?? "de", autostart: false, onboarding_completed: true, ...request,
    }));
    window.d2rDesktop = desktopBridge({
      schema_version: 3, language: "de", autostart: false, onboarding_completed: true,
      selected_character: "Unbekannt", selected_difficulty: "torment",
    }, updateDesktopSettings);
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 3, default_difficulty: "hell", profiles: [],
      runs: [{ run_id: "countess", display_name: "Countess" }, { run_id: "mephisto", display_name: "Mephisto" }],
      difficulties: [{ id: "nightmare", display_name: "Alptraum" }, { id: "hell", display_name: "Hölle" }],
      characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }],
    });
    mocks.getRunAvailabilities.mockResolvedValue({
      schema_version: 1, character: "MrBones", difficulty: "nightmare",
      runs: [
        { run_id: "countess", display_name: "Countess", status: "runtime_validation_required" },
        { run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required" },
      ],
    });
    mocks.getStatus.mockReset().mockResolvedValue({ ...detached, state: "idle_in_game", selection: { character: "MrBones", difficulty: "nightmare" } });

    render(<App />);

    await waitFor(() => expect(screen.getByLabelText("Charakter")).toHaveValue("MrBones"));
    expect(screen.getByLabelText("Schwierigkeit")).toHaveValue("nightmare");
    await waitFor(() => expect(updateDesktopSettings).toHaveBeenCalledWith({ selected_character: "MrBones", selected_difficulty: "nightmare" }));
    expect(screen.queryByText(/ist in der App ausgewählt/)).not.toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("button", { name: "Jetzt farmen" })).toBeEnabled());
  });

  it("fordert vor einer Routen-Invalidierung eine explizite Bestätigung", async () => {
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 4, default_difficulty: "nightmare", profiles: [], runs: [], difficulties: [{ id: "nightmare", display_name: "Alptraum" }, { id: "hell", display_name: "Hölle" }], characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }] });
    mocks.getStatus.mockResolvedValue({ ...detached, selection: { character: "MrBones", difficulty: "nightmare" } });
    mocks.previewSelection.mockResolvedValue({ schema_version: 1, character: "MrBones", old_difficulty: "nightmare", new_difficulty: "hell", affected_routes: ["countess.yaml", "mephisto.yaml"], invalidation_reason: "difficulty_changed", requires_confirmation: true, confirmation_token: "impact-preview", catalog_revision: 4, lifecycle_revision: 7 });
    mocks.applySelection.mockResolvedValue(undefined);
    render(<App />);
    const apply = await screen.findByRole("button", { name: "In D2R verwenden" });
    await waitFor(() => expect(apply).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Schwierigkeit"), { target: { value: "hell" } });
    await waitFor(() => expect(screen.getByLabelText("Schwierigkeit")).toHaveValue("hell"));
    fireEvent.click(apply);
    expect(await screen.findByRole("dialog", { name: "Routen werden unbrauchbar" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(mocks.applySelection).not.toHaveBeenCalled();
  });

  it("behält einen Selection-Fehler trotz folgendem Live-Refresh sichtbar", async () => {
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 1, default_difficulty: "nightmare", profiles: [], runs: [], difficulties: [{ id: "nightmare", display_name: "Alptraum" }], characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }] });
    mocks.previewSelection.mockRejectedValue(apiError("character_selection_unconfirmed"));
    mocks.getStatus.mockResolvedValue(detached);
    render(<App />);
    const apply = await screen.findByRole("button", { name: "In D2R verwenden" });
    await waitFor(() => expect(apply).toBeEnabled());
    fireEvent.click(apply);
    await waitFor(() => expect(mocks.previewSelection).toHaveBeenCalledOnce());
    expect(await screen.findByText("Die Charakterauswahl konnte nicht sicher bestätigt werden.", {}, { timeout: 10_000 })).toBeInTheDocument();
    const onEvent = mocks.connect.mock.calls[0][1] as (event: unknown) => void;
    await act(async () => onEvent({ sequence: 2, timestamp: new Date().toISOString(), event: "selection_failed" }));
    expect(screen.getByText("Die Charakterauswahl konnte nicht sicher bestätigt werden.")).toBeInTheDocument();
  }, 15_000);

  it("startet die persistente Charakter-Queue nach vollständigem Preflight genau einmal", async () => {
    const ready = { ...detached, state: "idle_in_game", generation: 5, selection: { character: "MrBones", difficulty: "nightmare" } };
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 9, default_difficulty: "nightmare", profiles: [], characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }], difficulties: [{ id: "nightmare", display_name: "Alptraum" }], runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }, { run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required" }] });
    mocks.getRunAvailabilities.mockResolvedValue({
      schema_version: 1, character: "MrBones", difficulty: "nightmare",
      runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }, { run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required" }],
    });
    mocks.getStatus.mockReset().mockResolvedValue(ready);
    mocks.validateQueue.mockResolvedValue({ entries: [], budgets: {} });
    mocks.startQueue.mockResolvedValue({ state: "starting_run", generation: 6 });
    render(<App />);
    expect(await screen.findByRole("heading", { name: "Deine Run-Reihenfolge" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Bearbeiten" })).toHaveAttribute("href", "#settings");
    const start = await screen.findByRole("button", { name: "Jetzt farmen" });
    await waitFor(() => expect(start).toBeEnabled());
    fireEvent.click(start);
    fireEvent.click(start);
    await waitFor(() => expect(mocks.validateQueue).toHaveBeenCalledOnce());
    expect(mocks.validateQueue).toHaveBeenCalledWith(["countess", "mephisto"], "MrBones", "nightmare", 9);
    await waitFor(() => expect(mocks.startQueue).toHaveBeenCalledOnce());
  });

  it("hält einen Queue-Startfehler sichtbar und übersetzt game_start_failed", async () => {
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 9, default_difficulty: "nightmare", profiles: [],
      characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }],
      difficulties: [{ id: "nightmare", display_name: "Alptraum" }],
      runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }],
    });
    mocks.getRunAvailabilities.mockResolvedValue({
      schema_version: 1, character: "MrBones", difficulty: "nightmare",
      runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }],
    });
    mocks.getStatus.mockReset().mockResolvedValue({
      ...detached,
      state: "stopped_error",
      lifecycle_phase: "stopped_error",
      selection: { character: "MrBones", difficulty: "nightmare" },
      last_result: { disposition: "stop", reason: "game_start_failed" },
      last_error: { code: "game_start_failed", message: "Kein laufendes Spiel im Rogue Encampment. D2R muss im Lager stehen oder auf dem Offline-Charakterbildschirm, damit der Bot das Spiel öffnet." },
      input: { enabled: true, paused: false, stopped: false },
    });
    render(<App />);
    expect(await screen.findByText("Farming konnte nicht gestartet werden")).toBeInTheDocument();
    expect(screen.getByText(/Das Spiel konnte nicht sicher gestartet werden/)).toBeInTheDocument();
    expect(screen.queryByText("Spielsteuerung nicht bereit")).not.toBeInTheDocument();
    const onEvent = mocks.connect.mock.calls[0][1] as (event: unknown) => void;
    await act(async () => onEvent({ sequence: 2, timestamp: new Date().toISOString(), event: "d2r_state_changed" }));
    expect(screen.getByText("Farming konnte nicht gestartet werden")).toBeInTheDocument();
  });

  it("sperrt den Queue-Start bei fehlender Loadout-Farm-Readiness", async () => {
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1,
      revision: 9,
      default_difficulty: "nightmare",
      profiles: [],
      characters: [{
        name: "MrBones",
        slug: "mrbones",
        selectable: true,
        farm_ready: false,
        farm_ready_reasons: ["profile_bindings_incomplete"],
      }],
      difficulties: [],
      runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }],
    });
    mocks.getStatus.mockReset().mockResolvedValue({
      ...detached,
      selection: { character: "MrBones", difficulty: "nightmare" },
    });

    render(<App />);
    const start = await screen.findByRole("button", { name: "Jetzt farmen" });
    await waitFor(() => expect(start).toBeDisabled());
    expect(screen.getByText("Charakter nicht farmbereit")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Charaktereinstellungen öffnen" })).toHaveAttribute("href", "#settings");
    expect(mocks.validateQueue).not.toHaveBeenCalled();
  });

  it("sperrt den Queue-Start wenn ein Queue-Eintrag nicht startfähig ist", async () => {
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 9, default_difficulty: "nightmare", profiles: [],
      characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }],
      difficulties: [{ id: "nightmare", display_name: "Alptraum" }],
      runs: [{ run_id: "countess", display_name: "Countess", status: "unavailable", reasons: ["route_assignment_missing"] }, { run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required" }],
    });
    mocks.getRunAvailabilities.mockResolvedValue({
      schema_version: 1, character: "MrBones", difficulty: "nightmare",
      runs: [{ run_id: "countess", display_name: "Countess", status: "unavailable", reasons: ["route_assignment_missing"] }, { run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required" }],
    });
    mocks.getStatus.mockReset().mockResolvedValue({
      ...detached,
      state: "idle_in_game",
      selection: { character: "MrBones", difficulty: "nightmare" },
    });
    render(<App />);
    expect(await screen.findByText("Noch nicht eingerichtet")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Jetzt farmen" })).toBeDisabled();
    expect(mocks.validateQueue).not.toHaveBeenCalled();
  });

  it("zeigt die persistente Queue read-only in ihrer gespeicherten Reihenfolge", async () => {
    const ready = { ...detached, state: "idle_in_game", selection: { character: "MrBones", difficulty: "nightmare" } };
    mocks.getCatalog.mockResolvedValue({
      schema_version: 1, revision: 1, default_difficulty: "nightmare", profiles: [],
      characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }],
      difficulties: [{ id: "nightmare", display_name: "Alptraum" }],
      runs: [{ run_id: "countess", display_name: "Countess" }, { run_id: "mephisto", display_name: "Mephisto" }],
    });
    mocks.getRunAvailabilities.mockResolvedValue({
      schema_version: 1, character: "MrBones", difficulty: "nightmare",
      runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }, { run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required" }],
    });
    mocks.getStatus.mockReset().mockResolvedValue(ready);
    render(<App />);
    expect(await screen.findByRole("heading", { name: "Deine Run-Reihenfolge" })).toBeInTheDocument();
    await waitFor(() => {
      const entries = screen.getAllByRole("listitem");
      expect(entries.some((entry) => entry.textContent?.includes("1Gräfin"))).toBe(true);
      expect(entries.some((entry) => entry.textContent?.includes("2Mephisto"))).toBe(true);
    });
    expect(screen.queryByRole("button", { name: /entfernen/ })).not.toBeInTheDocument();
  });

  it("rekonstruiert die Core-Etappe und zeigt Session-Aktionen nur als effektive Hotkeys", async () => {
    const running = { ...detached, state: "running_run", lifecycle_phase: "running_run", generation: 12, pending_intent: "pause_after_run", active_run_id: "countess", run_id: "run-1", run_progress: { stage_code: "cellar_floor", params: { floor: 3, floors: 5 }, current: 6, total: 13 }, selection: { character: "MrBones", difficulty: "nightmare" } };
    mocks.getCatalog.mockResolvedValue({ schema_version: 1, revision: 2, default_difficulty: "nightmare", profiles: [], characters: [], difficulties: [], runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required" }] });
    mocks.getStatus.mockReset().mockResolvedValue(running);
    const first = render(<App />);
    expect(await screen.findByLabelText("Etappe 6 von 13")).toBeInTheDocument();
    expect(screen.getByText("Pause nach diesem Run vorgemerkt")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Jetzt farmen" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /pausieren|stoppen|Emergency Stop/i })).not.toBeInTheDocument();
    expect(screen.getByText("Pause", { selector: "kbd" })).toBeInTheDocument();
    expect(screen.getByText("F10", { selector: "kbd" })).toBeInTheDocument();
    expect(screen.getByText("F11", { selector: "kbd" })).toBeInTheDocument();
    first.unmount();

    render(<App />);
    expect(await screen.findByLabelText("Etappe 6 von 13")).toBeInTheDocument();
  });
});

function desktopBridge(settings: DesktopSettingsView, updateDesktopSettings: D2RDesktopBridge["updateDesktopSettings"]): D2RDesktopBridge {
  return {
    getProvisioningState: vi.fn().mockResolvedValue({ required: false, import_selected: false, import_label: "" }),
    chooseImportRoot: vi.fn(),
    provision: vi.fn(),
    getAppInfo: vi.fn(),
    getDesktopSettings: vi.fn().mockResolvedValue(settings),
    updateDesktopSettings,
    showWindow: vi.fn(),
    restartCore: vi.fn(),
    restartAsAdministrator: vi.fn(),
    onNavigate: vi.fn(() => vi.fn()),
  };
}
