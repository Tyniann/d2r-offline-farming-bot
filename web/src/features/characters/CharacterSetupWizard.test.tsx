import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { CatalogDTO, StatusDTO } from "../../api/generated";
import { CharacterSetupWizard } from "./CharacterSetupWizard";

const mocks = vi.hoisted(() => ({
  preview: vi.fn(),
  settings: vi.fn(),
  reload: vi.fn(),
  confirm: vi.fn(),
  capture: vi.fn(),
  save: vi.fn(),
}));

vi.mock("../../api/generated", async () => {
  const actual = await vi.importActual<typeof import("../../api/generated")>("../../api/generated");
  return {
    ...actual,
    previewCharacterSetup: mocks.preview,
    getOperatorSettings: mocks.settings,
    reloadCharacters: mocks.reload,
  };
});
vi.mock("../../api/client", () => ({
  confirmCharacterSetup: mocks.confirm,
  captureCharacterSelection: mocks.capture,
  saveOperatorSettings: mocks.save,
}));

const status = {
  schema_version: 1, app_version: "t", core_version: "t", state: "idle", lifecycle_phase: "idle", generation: 3,
  d2r: { state: "attached", window_bound: true }, input: { enabled: true, paused: false, stopped: false },
  world: { valid: false, phase: "unknown" }, compatibility: { state: "compatible" },
  selection: { character: "MrBones", difficulty: "nightmare" },
  queue: { entries: [], default_entries: [], index: 0, cycle: 0, retry: 0, started_runs: 0, consecutive_failures: 0, total_restarts: 0, budgets: { max_runs: 10, max_duration_ms: 1, max_consecutive_failures: 1, max_total_restarts: 1 } },
} as unknown as StatusDTO;

const catalog = {
  schema_version: 1, revision: 2, default_difficulty: "nightmare",
  characters: [
    { name: "MrBones", slug: "mrbones", selectable: true, farm_ready: false, farm_ready_reasons: ["profile_bindings_incomplete"] },
    { name: "MrSpare", slug: "mrspare", selectable: false, reasons: ["character_profile_missing"] },
  ],
  difficulties: [{ id: "nightmare", display_name: "Alptraum" }],
  profiles: [{ id: "necro_bone_spear", character_class: "necromancer" }],
  runs: [],
} as CatalogDTO;

function setupPreview(character: string, overrides: Record<string, unknown> = {}) {
  return {
    schema_version: 1, catalog_revision: 2, operator_settings_revision: 4, pickit_assignment_revision: 1,
    character: { name: character, slug: character.toLowerCase(), character_class: "necromancer", class_display_name: "Totenbeschwörer" },
    supported: true,
    profiles: [
      { id: "necro_bone_spear", display_name: "Knochen-Speer", is_default: true, is_selected: character === "MrBones", standard_attack: "bone_spear", required_skills: [{ skill: "teleport", skill_id: 54, display_name: "Teleport" }] },
      { id: "necro_bone_spirit", display_name: "Knochengeist", is_default: false, is_selected: false, standard_attack: "bone_spirit", required_skills: [{ skill: "teleport", skill_id: 54, display_name: "Teleport" }] },
    ],
    selected_profile_id: character === "MrBones" ? "necro_bone_spear" : "",
    default_profile_id: "necro_bone_spear",
    pickit_defaults: [{ run_id: "countess", run_display_name: "Countess", profile_names: ["gems"], state: "missing" }],
    anchor_state: "missing", setup_state: character === "MrBones" ? "ready" : "needs_setup", reasons: [],
    ...overrides,
  };
}

describe("CharacterSetupWizard", () => {
  afterEach(() => cleanup());
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.settings.mockResolvedValue({
      schema_version: 3, revision: 4,
      characters: {
        mrbones: { character_class: "necromancer", combat_profile: "necro_bone_spear", last_difficulty: "nightmare", queue: ["countess", "cows"] },
        mrspare: { last_difficulty: "normal", queue: ["countess"] },
      },
      budgets: { max_runs: 10, max_duration_ms: 1, max_consecutive_failures: 1, max_total_restarts: 1 },
      input: { enabled: true, pause_hotkey: "pause", stop_after_run_hotkey: "f10", recording_finish_hotkey: "f9", emergency_stop_hotkey: "f11" },
      history: { retention_enabled: true, retention_days: 60 },
    });
    mocks.reload.mockResolvedValue({ schema_version: 1, catalog });
    mocks.confirm.mockResolvedValue(setupPreview("MrSpare", { setup_state: "needs_anchor", selected_profile_id: "necro_bone_spear" }));
  });

  it("richtet einen zweiten Fixture-Charakter über denselben Wizard-Code ein", async () => {
    mocks.preview.mockResolvedValue(setupPreview("MrSpare"));
    render(<CharacterSetupWizard character="MrSpare" catalog={catalog} status={status} mode="dashboard" />);
    expect(await screen.findByRole("heading", { name: /MrSpare/ })).toBeInTheDocument();
    expect(screen.getByText("Kampfprofil festlegen")).toBeInTheDocument();
    expect(screen.getByRole("combobox")).toHaveDisplayValue(/Knochen-Speer/);
    fireEvent.click(screen.getByRole("button", { name: "Profil und Lootprofile bestätigen" }));
    await waitFor(() => expect(mocks.confirm).toHaveBeenCalledWith(expect.objectContaining({ character: "MrSpare", profile_id: "necro_bone_spear" })));
  });

  it("zeigt Mehrfachprofile und erhält den Hinweis auf bewahrte Queue beim Wechsel", async () => {
    mocks.preview.mockResolvedValue(setupPreview("MrBones"));
    render(<CharacterSetupWizard character="MrBones" catalog={catalog} status={status} mode="settings" allowDeferBindings={false} showReload={false} />);
    expect(await screen.findByText("Kampfprofil wechseln")).toBeInTheDocument();
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "necro_bone_spirit" } });
    expect(screen.getByRole("button", { name: "Profil wechseln" })).toBeInTheDocument();
    expect(screen.getByText(/Queue, Routen, Inventarschutz/)).toBeInTheDocument();
  });

  it("erlaubt Später im Onboarding ohne sofortiges Speichern", async () => {
    mocks.preview.mockResolvedValue(setupPreview("MrBones"));
    render(<CharacterSetupWizard character="MrBones" catalog={catalog} status={status} mode="onboarding" />);
    expect(await screen.findByRole("button", { name: "Später" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Später" }));
    expect(screen.getByText(/Queue bleibt gesperrt/)).toBeInTheDocument();
    expect(mocks.save).not.toHaveBeenCalled();
  });
});
