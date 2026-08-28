import { afterEach, expect, it, vi } from "vitest";
import {
  captureCharacterSelection,
  confirmCharacterSetup,
  previewCharacterSetup,
  reloadCharacters,
  type CharacterSetupProfileDTO,
} from "./generated";

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("transportiert den Core-Vertrag für Slots, CTA und Readiness", () => {
  const profile: CharacterSetupProfileDTO = {
    id: "paladin_hammerdin",
    display_name: "Hammerdin",
    is_default: true,
    is_selected: true,
    required_skills: [{ skill: "blessed_hammer", skill_id: 112, slot: "left" }],
    optional_skill_pairs: [{ skills: [
      { skill: "battle_command", skill_id: 155, slot: "right" },
      { skill: "battle_orders", skill_id: 149, slot: "right" },
    ] }],
    requires_mercenary: true,
    bindings_ready: false,
    binding_reasons: ["profile_bindings_incomplete"],
    supported_runs: ["countess", "cows", "lower-kurast", "mephisto", "nihlathak", "summoner"],
    default_belt_layout: { slot_1: "healing", slot_2: "mana", slot_3: "mana", slot_4: "rejuvenation" },
    belt_layout: { slot_1: "healing", slot_2: "mana", slot_3: "mana", slot_4: "rejuvenation" },
    default_healing_restock: 2,
    default_mana_restock: 4,
  };

  expect(profile).toMatchObject({ requires_mercenary: true, bindings_ready: false, supported_runs: ["countess", "cows", "lower-kurast", "mephisto", "nihlathak", "summoner"] });
  expect(profile.required_skills?.[0].slot).toBe("left");
  expect(profile.optional_skill_pairs?.[0].skills.map((skill) => skill.skill_id)).toEqual([155, 149]);
});

it("schützt nur Confirm und Capture mit dem Control-Token", async () => {
  const response = new Response(JSON.stringify({ schema_version: 1 }), { status: 200, headers: { "Content-Type": "application/json" } });
  const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => response.clone());
  vi.stubGlobal("fetch", fetchMock);

  await reloadCharacters();
  await previewCharacterSetup({ character: "MrBones" });
  await confirmCharacterSetup({
    command_id: "setup-1", character: "MrBones", expected_catalog_revision: 1,
    expected_operator_settings_revision: 1, expected_pickit_assignment_revision: 1, expected_generation: 0,
  }, "control");
  await captureCharacterSelection({
    command_id: "capture-1", character: "MrBones", expected_catalog_revision: 1, expected_generation: 0,
  }, "control");

  expect(fetchMock.mock.calls[0][1]?.headers).not.toHaveProperty("X-D2RBot-Control-Token");
  expect(fetchMock.mock.calls[1][1]?.headers).not.toHaveProperty("X-D2RBot-Control-Token");
  expect(fetchMock.mock.calls[2][1]?.headers).toMatchObject({ "X-D2RBot-Control-Token": "control" });
  expect(fetchMock.mock.calls[3][1]?.headers).toMatchObject({ "X-D2RBot-Control-Token": "control" });
});
