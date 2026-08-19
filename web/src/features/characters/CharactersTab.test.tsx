import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { CatalogDTO, OperatorSettingsDTO } from "../../api/generated";
import { CharactersTab } from "./CharactersTab";

const mocks = vi.hoisted(() => ({ preview: vi.fn(), settings: vi.fn() }));
vi.mock("../../api/generated", async () => {
  const actual = await vi.importActual<typeof import("../../api/generated")>("../../api/generated");
  return { ...actual, previewCharacterSetup: mocks.preview, getOperatorSettings: mocks.settings, reloadCharacters: vi.fn() };
});
vi.mock("../../api/client", () => ({
  confirmCharacterSetup: vi.fn(), captureCharacterSelection: vi.fn(), saveOperatorSettings: vi.fn(),
}));

const draft: OperatorSettingsDTO = {
  schema_version: 3,
  revision: 4,
  characters: {
    mrbones: {
      character_class: "necromancer",
      combat_profile: "necro_bone_spear",
      last_difficulty: "nightmare",
      queue: ["countess"],
      profile_bindings: {
        necro_bone_spear: {
          skills: {
            teleport: "f7", town_portal: "f6", amplify_damage: "f1", corpse_explosion: "f2",
            bone_prison: "f3", bone_armor: "f5", bone_spear: "f8",
          },
          belt: { slot_1: "1", slot_2: "2", slot_3: "3", slot_4: "4" },
        },
      },
      inventory_lock: {
        grid: [
          [1, 1, 1, 1, 0, 0, 0, 0, 0, 0],
          [1, 1, 1, 1, 0, 0, 0, 0, 0, 0],
          [1, 1, 1, 1, 0, 0, 0, 0, 0, 0],
          [1, 1, 1, 1, 0, 0, 0, 0, 0, 0],
        ],
      },
    },
  },
  budgets: { max_runs: 20, max_duration_ms: 7_200_000, max_consecutive_failures: 3, max_total_restarts: 4 },
  input: { enabled: true, pause_hotkey: "pause", stop_after_run_hotkey: "f10", recording_finish_hotkey: "f9", emergency_stop_hotkey: "f11" },
  history: { retention_enabled: true, retention_days: 60 },
};

const catalog: CatalogDTO = {
  schema_version: 1,
  revision: 1,
  default_difficulty: "nightmare",
  characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }],
  difficulties: [],
  profiles: [{ id: "necro_bone_spear", character_class: "necromancer" }],
  runs: [],
};

describe("CharactersTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.settings.mockResolvedValue(draft);
    mocks.preview.mockResolvedValue({
      schema_version: 1,
      catalog_revision: 1,
      operator_settings_revision: 4,
      pickit_assignment_revision: 1,
      character: { name: "MrBones", slug: "mrbones", character_class: "necromancer", class_display_name: "Totenbeschwörer" },
      supported: true,
      profiles: [{
        id: "necro_bone_spear",
        display_name: "Knochen-Speer",
        is_default: true,
        is_selected: true,
        standard_attack: "bone_spear",
        requires_mercenary: false,
        bindings_ready: true,
        required_skills: [
          { skill: "teleport", skill_id: 54, display_name: "Teleport", slot: "right" },
          { skill: "town_portal", skill_id: 359, display_name: "Stadtportal", slot: "right" },
          { skill: "bone_spear", skill_id: 84, display_name: "Knochen-Speer", slot: "right" },
          { skill: "amplify_damage", skill_id: 66, display_name: "Verstärkter Schaden", slot: "right" },
          { skill: "corpse_explosion", skill_id: 74, display_name: "Kadaverexplosion", slot: "right" },
          { skill: "bone_armor", skill_id: 68, display_name: "Knochenrüstung", slot: "right" },
          { skill: "bone_prison", skill_id: 88, display_name: "Knochengefängnis", slot: "right" },
        ],
        supported_runs: ["countess"],
        default_belt_layout: { slot_1: "healing", slot_2: "mana", slot_3: "mana", slot_4: "rejuvenation" },
        belt_layout: { slot_1: "healing", slot_2: "mana", slot_3: "mana", slot_4: "rejuvenation" },
      }],
      selected_profile_id: "necro_bone_spear",
      default_profile_id: "necro_bone_spear",
      pickit_defaults: [],
      anchor_state: "ready",
      setup_state: "ready",
      reasons: [],
    });
  });
  afterEach(() => cleanup());

  it("zeigt MrBones-Bindings F1–F8 und Gürtel 1–4", async () => {
    render(<CharactersTab
      draft={draft}
      catalog={catalog}
      selectedCharacter="mrbones"
      characterNames={["mrbones"]}
      mutable
      diffPaths={[]}
      status={{
        schema_version: 1, app_version: "test", core_version: "test", state: "idle", lifecycle_phase: "idle", generation: 1,
        d2r: { state: "attached", window_bound: true }, input: { enabled: true, paused: false, stopped: false },
        world: { valid: false, phase: "unknown" }, compatibility: { state: "compatible" },
        selection: { character: "MrBones", difficulty: "nightmare" },
        queue: { entries: [], default_entries: [], index: 0, cycle: 0, retry: 0, started_runs: 0, consecutive_failures: 0, total_restarts: 0, budgets: { max_runs: 10, max_duration_ms: 1, max_consecutive_failures: 1, max_total_restarts: 1 } },
      } as never}
      onSelectCharacter={vi.fn()}
      onChangeDraft={vi.fn()}
    />);
    expect(await screen.findByRole("heading", { name: "MrBones" })).toBeInTheDocument();
    expect(await screen.findByLabelText("Verstärkter Schaden Taste")).toHaveValue("f1");
    expect(screen.getByLabelText("Kadaverexplosion Taste")).toHaveValue("f2");
    expect(screen.getByLabelText("Knochengefängnis Taste")).toHaveValue("f3");
    expect(screen.getByLabelText("Knochenrüstung Taste")).toHaveValue("f5");
    expect(screen.getByLabelText("Stadtportal Taste")).toHaveValue("f6");
    expect(screen.getByLabelText("Teleport Taste")).toHaveValue("f7");
    expect(screen.getByLabelText("Knochen-Speer Taste")).toHaveValue("f8");
    expect(screen.getByLabelText("Gürtel Slot 1 Taste")).toHaveValue("1");
    expect(screen.getByLabelText("Gürtel Slot 2 Taste")).toHaveValue("2");
    expect(screen.getByLabelText("Gürtel Slot 3 Taste")).toHaveValue("3");
    expect(screen.getByLabelText("Gürtel Slot 4 Taste")).toHaveValue("4");
    expect(screen.getByLabelText("Gürtel Slot 1 Trank")).toHaveValue("healing");
    expect(screen.getByLabelText("Gürtel Slot 2 Trank")).toHaveValue("mana");
    expect(screen.getByLabelText("Gürtel Slot 3 Trank")).toHaveValue("mana");
    expect(screen.getByLabelText("Gürtel Slot 4 Trank")).toHaveValue("rejuvenation");
    expect(screen.getByRole("combobox", { name: "Spieleranzahl" })).toHaveValue("1");
    expect(screen.getByRole("grid", { name: /inventarschutz/i })).toBeInTheDocument();
    expect(screen.queryByText("Noch nicht bestätigt")).not.toBeInTheDocument();
    await waitFor(() => expect(mocks.preview).toHaveBeenCalledWith({ character: "MrBones" }));
  });
});
