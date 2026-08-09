import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { CatalogDTO, OperatorSettingsDTO, RecordingOptionDTO, StatusDTO } from "../../api/generated";
import { OnboardingFeature } from "./OnboardingFeature";

const mocks = vi.hoisted(() => ({
  getSettings: vi.fn(), getOptions: vi.fn(), getHotkeys: vi.fn(), getWorkflow: vi.fn(),
  previewSetup: vi.fn(), reloadCharacters: vi.fn(), confirmSetup: vi.fn(), captureSelection: vi.fn(),
  previewSelection: vi.fn(), applySelection: vi.fn(), saveSettings: vi.fn(),
}));
vi.mock("../../api/generated", () => ({
  getOperatorSettings: mocks.getSettings, getRecordingOptions: mocks.getOptions, getHotkeyHelp: mocks.getHotkeys, getRouteWorkflow: mocks.getWorkflow,
  previewCharacterSetup: mocks.previewSetup, reloadCharacters: mocks.reloadCharacters,
}));
vi.mock("../../api/client", () => ({
  previewSelection: mocks.previewSelection, applySelection: mocks.applySelection, saveOperatorSettings: mocks.saveSettings,
  confirmCharacterSetup: mocks.confirmSetup, captureCharacterSelection: mocks.captureSelection,
}));

const operator: OperatorSettingsDTO = {
  schema_version: 2, revision: 2,
  characters: { mrbones: { character_class: "necromancer", combat_profile: "necro_bone_spear", last_difficulty: "nightmare", queue: ["countess"] } },
  budgets: { max_runs: 10, max_duration_ms: 3_600_000, max_consecutive_failures: 3, max_total_restarts: 2 },
  input: { enabled: false, pause_hotkey: "pause", stop_after_run_hotkey: "f10", recording_finish_hotkey: "f9", emergency_stop_hotkey: "f11" },
  history: { retention_enabled: true, retention_days: 60 },
};
const catalog = {
  schema_version: 1, revision: 3, default_difficulty: "nightmare", profiles: [{ id: "necro_bone_spear", character_class: "necromancer" }],
  characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }],
  difficulties: [{ id: "nightmare", display_name: "Alptraum" }],
  runs: [{ run_id: "countess", display_name: "Countess", status: "unavailable", reasons: ["route_assignment_missing"] }],
} as CatalogDTO;
const status = {
  schema_version: 1, app_version: "test", core_version: "test", state: "idle", lifecycle_phase: "idle", generation: 4,
  d2r: { state: "attached", window_bound: true }, input: { enabled: true, paused: false, stopped: false },
  world: { valid: false, phase: "unknown" }, compatibility: { state: "compatible", supported_version: "3.2.92777", expected_version: "3.2.92777", offset_version: "3.2.92777", actual_version: "3.2.92777", privilege_mismatch: false },
  selection: { character: "MrBones", difficulty: "nightmare" },
  queue: { entries: ["countess"], default_entries: ["countess"], index: 0, cycle: 0, retry: 0, started_runs: 0, consecutive_failures: 0, total_restarts: 0, budgets: { max_runs: 10, max_duration_ms: 3_600_000, max_consecutive_failures: 3, max_total_restarts: 2 } },
} as StatusDTO;

const readySetup = {
  schema_version: 1, catalog_revision: 3, operator_settings_revision: 2, pickit_assignment_revision: 2,
  character: { name: "MrBones", slug: "mrbones", character_class: "necromancer", class_display_name: "Totenbeschwörer" },
  supported: true,
  profiles: [{ id: "necro_bone_spear", display_name: "Knochen-Speer", is_default: true, is_selected: true }],
  selected_profile_id: "necro_bone_spear", default_profile_id: "necro_bone_spear",
  pickit_defaults: [
    { run_id: "countess", run_display_name: "Countess", profile_names: ["Runen"], state: "ready" },
    { run_id: "mephisto", run_display_name: "Mephisto", profile_names: ["Ausrüstung"], state: "ready" },
  ],
  anchor_state: "ready", setup_state: "ready", reasons: [],
} as const;

function option(missing = ""): RecordingOptionDTO {
  return {
    run_id: "countess", display_name: "Countess", instructions_de: "Vom Wegpunkt zum Boss.", start_waypoint: "black_marsh",
    allowed_start_area_id: 6, allowed_route_area_ids: [6], terminal_area_id: 25, terminal_max_distance_tiles: 20, available: true,
    prerequisites: ["waypoint", "teleport", "town_portal", "pickit"].map((id) => ({
      id: id as "waypoint",
      ready: id !== missing,
      reason: id === missing ? (id === "pickit" ? "pickit_assignment_missing" : `onboarding_${id}_missing`) : undefined,
    })),
  };
}

describe("OnboardingFeature", () => {
  const onRefresh = vi.fn().mockResolvedValue(undefined);
  const onClose = vi.fn();
  const onOpenRoutes = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    window.history.replaceState(null, "", "/");
    mocks.getSettings.mockResolvedValue(operator);
    mocks.getOptions.mockResolvedValue([option()]);
    mocks.getHotkeys.mockResolvedValue({ recording_finish: "f9", stop_after_run: "f10", emergency_stop: "f11", pause: "pause" });
    mocks.getWorkflow.mockResolvedValue({ workflow_id: "", generation: 1, state: "idle", run_id: "", character: "" });
    mocks.previewSetup.mockResolvedValue(readySetup);
    mocks.reloadCharacters.mockResolvedValue({ schema_version: 1, catalog });
    mocks.confirmSetup.mockResolvedValue(readySetup);
    mocks.captureSelection.mockResolvedValue(readySetup);
    window.d2rDesktop = {
      getDesktopSettings: vi.fn().mockResolvedValue({ schema_version: 1, autostart: false, onboarding_completed: false }),
      updateDesktopSettings: vi.fn().mockResolvedValue({ schema_version: 1, autostart: false, onboarding_completed: true }),
      restartCore: vi.fn(),
    } as unknown as D2RDesktopBridge;
  });
  afterEach(() => { cleanup(); delete window.d2rDesktop; });

  it("bewahrt den Input-Schritt über den kontrollierten Core-Neustart", async () => {
    const enabled = { ...operator, revision: 3, input: { ...operator.input, enabled: true } };
    mocks.saveSettings.mockResolvedValue({
      schema_version: 1, generation: 5, settings: enabled, changed_fields: ["input.enabled"], restart_required: true,
    });
    const restartCore = vi.fn().mockImplementation(async () => {
      expect(new URLSearchParams(window.location.search).get("onboarding_step")).toBe("4");
    });
    window.d2rDesktop!.restartCore = restartCore;

    render(<OnboardingFeature initialStep={4} status={{ ...status, input: { enabled: false, paused: false, stopped: false } }} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    await waitFor(() => expect(mocks.getSettings).toHaveBeenCalled());
    expect(screen.getByRole("heading", { name: "Input" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Input ausdrücklich freigeben" }));
    await waitFor(() => expect(restartCore).toHaveBeenCalledOnce());
  });

  it("blockiert am inkompatiblen Versionsschritt ohne Override", async () => {
    render(<OnboardingFeature status={{ ...status, compatibility: { ...status.compatibility, state: "incompatible", reason: "d2r_version_unsupported" } }} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(screen.getByRole("heading", { name: "D2R" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Weiter" })).toBeDisabled();
    expect(screen.getByText("d2r_version_unsupported")).toBeInTheDocument();
  });

  it("bietet beim frischen Root einen vorbereiteten Charakter an und erklärt gesperrte Einträge", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    const freshCatalog = {
      ...catalog,
      characters: [
        { name: "MrBones", slug: "mrbones", expected_class: "necromancer", selectable: true, farm_ready: true },
        { name: "MrHammer", slug: "mrhammer", expected_class: "paladin", selectable: false, reasons: ["character_class_unsupported", "character_anchor_missing"] },
      ],
    } as CatalogDTO;
    const freshStatus = { ...status, selection: { character: "", difficulty: "nightmare" } } as StatusDTO;
    render(<OnboardingFeature status={freshStatus} catalog={freshCatalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    await waitFor(() => expect(mocks.getSettings).toHaveBeenCalled());
    for (let index = 0; index < 5; index++) fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(screen.getByRole("combobox", { name: "Charakter" })).toHaveValue("MrBones");
    expect(screen.getByRole("option", { name: "MrBones" })).toBeEnabled();
    expect(screen.getByRole("option", { name: "MrHammer – Einrichtung nötig" })).toBeEnabled();
    expect(screen.getByText(/Für diese Klasse gibt es noch kein freigegebenes Kampfprofil/)).toBeInTheDocument();
    expect(screen.getByText(/Das Auswahlbild für diesen Charakter fehlt noch/)).toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: "MrBones · Totenbeschwörer" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Weiter" })).toBeDisabled();
  });

  it("aktualisiert Recording-Readiness nach der Charakterbestätigung", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    mocks.getOptions.mockResolvedValueOnce([option("pickit")]).mockResolvedValue([option()]);
    mocks.previewSelection.mockResolvedValue({
      schema_version: 1, character: "MrBones", old_difficulty: "", new_difficulty: "nightmare",
      reason: "same_difficulty", requires_confirmation: false, confirmation_token: "token", affected_routes: [],
    });
    mocks.applySelection.mockResolvedValue({ schema_version: 1, state: "idle_in_game", character: "MrBones", difficulty: "nightmare" });

    render(<OnboardingFeature initialStep={5} status={{ ...status, selection: { character: "", difficulty: "nightmare" } }} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    await waitFor(() => expect(mocks.getOptions).toHaveBeenCalledOnce());
    const confirm = screen.getByRole("button", { name: "Über Core bestätigen" });
    await waitFor(() => expect(confirm).toBeEnabled());
    fireEvent.click(confirm);

    await waitFor(() => expect(mocks.getOptions).toHaveBeenCalledTimes(2));
    expect(mocks.applySelection).toHaveBeenCalledOnce();
  });

  it("zeigt bei genau einem Profil nur den festen Standard und bestätigt die lesbaren Lootketten", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    const needsSetup = {
      ...readySetup,
      selected_profile_id: undefined,
      anchor_state: "missing",
      setup_state: "needs_setup",
      reasons: ["character_profile_missing"],
      profiles: [{ id: "necro_bone_spear", display_name: "Knochen-Speer", is_default: true, is_selected: false }],
      pickit_defaults: readySetup.pickit_defaults.map((entry) => ({ ...entry, state: "missing" })),
    };
    mocks.previewSetup.mockResolvedValueOnce(needsSetup).mockResolvedValue(readySetup);
    render(<OnboardingFeature initialStep={5} status={{ ...status, selection: { character: "", difficulty: "nightmare" } }} catalog={{ ...catalog, characters: [{ ...catalog.characters[0], selectable: false, reasons: ["character_profile_missing", "character_anchor_missing"] }] }} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);

    expect(await screen.findByText("Knochen-Speer", { selector: "strong" })).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "Kampfprofil" })).not.toBeInTheDocument();
    expect(screen.getByText((_, element) => element?.tagName === "LI" && element.textContent?.includes("Countess: Runen") === true)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Profil und Lootprofile bestätigen" }));

    await waitFor(() => expect(mocks.confirmSetup).toHaveBeenCalledWith(expect.objectContaining({
      character: "MrBones",
      profile_id: "necro_bone_spear",
      expected_catalog_revision: 3,
      expected_operator_settings_revision: 2,
      expected_pickit_assignment_revision: 2,
      expected_generation: 4,
    })));
    expect(mocks.reloadCharacters).toHaveBeenCalledOnce();
    expect(onRefresh).toHaveBeenCalledOnce();
  });

  it("zeigt bei mehreren Profilen die Core-Auswahl mit vorausgewähltem Standard", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    mocks.previewSetup.mockResolvedValue({
      ...readySetup,
      selected_profile_id: undefined,
      default_profile_id: "necro_bone_spear",
      setup_state: "needs_setup",
      anchor_state: "missing",
      reasons: ["character_profile_missing"],
      profiles: [
        { id: "necro_bone_spear", display_name: "Knochen-Speer", is_default: true, is_selected: false },
        { id: "necro_summoner", display_name: "Beschwörer", is_default: false, is_selected: false },
      ],
    });
    render(<OnboardingFeature initialStep={5} status={status} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);

    expect(await screen.findByRole("combobox", { name: "Kampfprofil" })).toHaveValue("necro_bone_spear");
  });

  it("bietet für eine nicht unterstützte Klasse weder Setup noch Bilderfassung an und zeigt keine Reason-ID", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    const paladinCatalog = {
      ...catalog,
      characters: [{ name: "MrHammer", slug: "mrhammer", expected_class: "paladin", selectable: false, reasons: ["character_class_unsupported"] }],
    } as CatalogDTO;
    mocks.previewSetup.mockResolvedValue({
      ...readySetup,
      character: { name: "MrHammer", slug: "mrhammer", character_class: "paladin", class_display_name: "Paladin" },
      supported: false,
      profiles: [],
      selected_profile_id: undefined,
      default_profile_id: undefined,
      setup_state: "blocked",
      reasons: ["character_class_unsupported"],
    });
    render(<OnboardingFeature initialStep={5} status={{ ...status, selection: { character: "", difficulty: "nightmare" } }} catalog={paladinCatalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);

    expect(await screen.findByRole("heading", { name: "MrHammer · Paladin" })).toBeInTheDocument();
    expect(screen.getAllByText(/Für diese Klasse gibt es noch kein freigegebenes Kampfprofil/)).toHaveLength(2);
    expect(screen.queryByText("character_class_unsupported")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Profil und Lootprofile bestätigen" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Auswahlbild jetzt speichern" })).not.toBeInTheDocument();
  });

  it("fordert vor der Bilderfassung eine klare Bestätigung und bleibt im Charakterschritt", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    const needsAnchor = { ...readySetup, anchor_state: "missing", setup_state: "needs_anchor", reasons: ["character_anchor_missing"] };
    mocks.previewSetup.mockResolvedValueOnce(needsAnchor).mockResolvedValue(readySetup);
    render(<OnboardingFeature initialStep={5} status={status} catalog={{ ...catalog, characters: [{ ...catalog.characters[0], selectable: false, reasons: ["character_anchor_missing"] }] }} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);

    expect(await screen.findByText("Die Charakterauswahl öffnen.")).toBeInTheDocument();
    const capture = screen.getByRole("button", { name: "Auswahlbild jetzt speichern" });
    expect(capture).toBeDisabled();
    fireEvent.click(screen.getByRole("checkbox", { name: /MrBones ist in der Charakterauswahl markiert/ }));
    fireEvent.click(capture);

    await waitFor(() => expect(mocks.captureSelection).toHaveBeenCalledWith(expect.objectContaining({ character: "MrBones", expected_catalog_revision: 3, expected_generation: 4 })));
    expect(screen.getByRole("heading", { name: "Charakter" })).toBeInTheDocument();
  });

  it("erklärt nach fertiger Einrichtung den nächsten Bestätigungsschritt", async () => {
    render(<OnboardingFeature initialStep={5} status={status} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    expect(await screen.findByText(/Wähle oben die gewünschte Schwierigkeit und klicke anschließend auf „Über Core bestätigen“/)).toBeInTheDocument();
  });

  it("erklärt einen fehlenden Pickit-Default ohne rohen Reason-Code", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    mocks.getOptions.mockResolvedValue([option("pickit")]);
    render(<OnboardingFeature initialStep={6} status={status} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    expect(await screen.findByText("Für diesen Charakter und Run ist noch kein Lootprofil zugeordnet.")).toBeInTheDocument();
    expect(screen.queryByText("pickit_assignment_missing")).not.toBeInTheDocument();
  });

  it("zeigt auch bei einer unbekannten Voraussetzung keine internen IDs", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    mocks.getOptions.mockResolvedValue([{
      ...option(),
      prerequisites: [{ id: "future_gate_internal", ready: false, reason: "future_reason_internal" }],
    } as unknown as RecordingOptionDTO]);
    render(<OnboardingFeature initialStep={6} status={status} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    expect(await screen.findByText("Weitere Voraussetzung")).toBeInTheDocument();
    expect(screen.getByText("Diese Voraussetzung fehlt noch.")).toBeInTheDocument();
    expect(screen.queryByText("future_gate_internal")).not.toBeInTheDocument();
    expect(screen.queryByText("future_reason_internal")).not.toBeInTheDocument();
  });

  it("erklärt Schaltfläche und F9 beim Aufnahmebeginn", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    mocks.getOptions.mockResolvedValue([option("pickit")]);
    render(<OnboardingFeature initialStep={7} status={status} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    expect(screen.getByText(/F9 startet keine Aufnahme/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Routenbereich öffnen und Aufnahme starten/ })).toBeDisabled();
  });

  it("zeigt alle Recording-Optionen des Core und öffnet den gewählten Key-Run", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    mocks.getOptions.mockResolvedValue([
      option(),
      { ...option(), run_id: "summoner", display_name: "Summoner", start_waypoint: "arcane_sanctuary", allowed_start_area_id: 74, allowed_route_area_ids: [74], terminal_area_id: 74 },
      { ...option(), run_id: "nihlathak", display_name: "Nihlathak", start_waypoint: "halls_of_pain", allowed_start_area_id: 123, allowed_route_area_ids: [123, 124], terminal_area_id: 124 },
    ]);
    render(<OnboardingFeature initialStep={7} status={status} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);

    await screen.findByRole("radio", { name: "Nihlathak" });
    expect(screen.getByRole("radio", { name: "Summoner" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("radio", { name: "Summoner" }));
    fireEvent.click(screen.getByRole("button", { name: /Routenbereich öffnen und Aufnahme starten/ }));
    expect(onOpenRoutes).toHaveBeenCalledWith("summoner");
  });

  it("blockiert trotz gespeicherter Freigabe bis der laufende Core Input bestätigt", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    render(<OnboardingFeature status={{ ...status, input: { enabled: false, paused: false, stopped: false } }} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    await waitFor(() => expect(mocks.getSettings).toHaveBeenCalled());
    for (let index = 0; index < 4; index++) fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(screen.getByRole("heading", { name: "Input" })).toBeInTheDocument();
    expect(screen.getByText("Core-Freigabe noch nicht aktiv")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Weiter" })).toBeDisabled();
  });

  it("erklärt Safety vor der Input-Freigabe und blockiert danach ohne Opt-in", async () => {
    render(<OnboardingFeature status={status} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    await waitFor(() => expect(mocks.getSettings).toHaveBeenCalled());
    for (let index = 0; index < 3; index++) fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(screen.getByRole("heading", { name: "Safety" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(screen.getByRole("heading", { name: "Input" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Weiter" })).toBeDisabled();
    expect(screen.getByText(/Erst wenn der laufende Core die Freigabe bestätigt/)).toBeInTheDocument();
  });

  it("erzwingt beim Überspringen deaktivierten Input und markiert erst danach Desktop-Onboarding", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    mocks.saveSettings.mockResolvedValue({ schema_version: 1, generation: 5, settings: operator, changed_fields: ["input.enabled"], restart_required: true });
    render(<OnboardingFeature status={status} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    await waitFor(() => expect(mocks.getSettings).toHaveBeenCalled());
    await waitFor(() => expect(mocks.getOptions).toHaveBeenCalled());
    await waitFor(() => expect(mocks.getHotkeys).toHaveBeenCalled());
    await waitFor(() => expect(mocks.getWorkflow).toHaveBeenCalled());
    await Promise.all([
      ...mocks.getSettings.mock.results.map((result) => result.value),
      ...mocks.getOptions.mock.results.map((result) => result.value),
      ...mocks.getHotkeys.mock.results.map((result) => result.value),
      ...mocks.getWorkflow.mock.results.map((result) => result.value),
    ]);
    const skip = await screen.findByRole("button", { name: "Überspringen – Input bleibt aus" });
    await waitFor(() => expect(skip).toBeEnabled());
    fireEvent.click(skip);
    await waitFor(() => expect(mocks.saveSettings).toHaveBeenCalledWith(expect.objectContaining({ settings: expect.objectContaining({ input: expect.objectContaining({ enabled: false }) }) })));
    expect(window.d2rDesktop?.updateDesktopSettings).toHaveBeenCalledWith({ autostart: false, onboarding_completed: true });
  });

  it("zeigt fehlende Wegpunkt-/Teleport-/TP-Voraussetzungen und übergibt bei Readiness an denselben Routenbereich", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    mocks.getOptions.mockResolvedValue([option("teleport")]);
    const view = render(<OnboardingFeature initialStep={7} status={status} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    expect(await screen.findByText(/Die Teleport-Tastenbelegung fehlt/)).toBeInTheDocument();
    expect(screen.queryByText("onboarding_teleport_missing")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Routenbereich öffnen und Aufnahme starten/ })).toBeDisabled();

    mocks.getOptions.mockResolvedValue([option()]);
    view.unmount();
    render(<OnboardingFeature initialStep={7} status={status} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    await waitFor(() => expect(mocks.getSettings).toHaveBeenCalled());
    await mocks.getSettings.mock.results.at(-1)?.value;
    await waitFor(() => expect(mocks.getOptions).toHaveBeenCalled());
    await mocks.getOptions.mock.results.at(-1)?.value;
    const openRoutes = await screen.findByRole("button", { name: /Routenbereich öffnen und Aufnahme starten/ });
    await waitFor(() => expect(openRoutes).toBeEnabled());
    fireEvent.click(openRoutes);
    expect(onOpenRoutes).toHaveBeenCalledWith("countess");
  });

  it("bietet nach unterbrochener Aufnahme ausschließlich einen sauberen Neustart an", async () => {
    mocks.getSettings.mockResolvedValue({ ...operator, input: { ...operator.input, enabled: true } });
    mocks.getWorkflow.mockResolvedValue({ workflow_id: "old", generation: 8, state: "emergency_cancelled", run_id: "countess", character: "MrBones", reason: "onboarding_route_interrupted" });
    render(<OnboardingFeature status={status} catalog={catalog} onRefresh={onRefresh} onClose={onClose} onOpenRoutes={onOpenRoutes} />);
    await waitFor(() => expect(mocks.getSettings).toHaveBeenCalledTimes(1));
    for (let index = 0; index < 7; index++) fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(await screen.findByText("Es gibt kein Resume. Starte die Aufnahme sauber neu.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Fortsetzen|Resume/ })).not.toBeInTheDocument();
  });
});
