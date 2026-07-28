import { afterEach, expect, it, vi } from "vitest";
import {
  getOperatorSettings,
  previewOperatorSettings,
  previewResetOperatorSettings,
  resetOperatorSettings,
  updateOperatorSettings,
  type OperatorSettingsDTO,
} from "./generated";

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("verwendet für Settings nur bei Update und Reset den Control-Token", async () => {
  const settings: OperatorSettingsDTO = {
    schema_version: 2,
    revision: 1,
    characters: { mrbones: { character_class: "necromancer", combat_profile: "necro_bone_spear", last_difficulty: "nightmare", queue: ["countess", "mephisto"] } },
    budgets: { max_runs: 3, max_duration_ms: 7_200_000, max_consecutive_failures: 2, max_total_restarts: 3 },
    input: { enabled: false, pause_hotkey: "pause", stop_after_run_hotkey: "f10", recording_finish_hotkey: "f9", emergency_stop_hotkey: "f11" },
    history: { retention_enabled: true, retention_days: 60 },
  };
  const response = (body: unknown) => new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
  const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => response(String(input).endsWith("/settings/operator") ? settings : { schema_version: 1, generation: 4, settings, changed_fields: [], restart_required: false }));
  vi.stubGlobal("fetch", fetchMock);

  await getOperatorSettings();
  await previewOperatorSettings({ expected_revision: 1, expected_generation: 4, settings });
  await previewResetOperatorSettings({ expected_revision: 1, expected_generation: 4 });
  await updateOperatorSettings({ expected_revision: 1, expected_generation: 4, settings }, "control");
  await resetOperatorSettings({ expected_revision: 1, expected_generation: 4 }, "control");

  expect(fetchMock.mock.calls[1][1]?.headers).not.toHaveProperty("X-D2RBot-Control-Token");
  expect(fetchMock.mock.calls[2][1]?.headers).not.toHaveProperty("X-D2RBot-Control-Token");
  expect(fetchMock.mock.calls[3][1]?.headers).toMatchObject({ "X-D2RBot-Control-Token": "control" });
  expect(fetchMock.mock.calls[4][1]?.headers).toMatchObject({ "X-D2RBot-Control-Token": "control" });
});
