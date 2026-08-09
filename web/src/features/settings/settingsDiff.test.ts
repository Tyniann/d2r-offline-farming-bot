import { describe, expect, it } from "vitest";
import { collectLocalDiffPaths, labelChangedField, minutesToMs, msToMinutes, summarizeChangedFields } from "./settingsDiff";
import type { OperatorSettingsDTO } from "../../api/generated";

const base: OperatorSettingsDTO = {
  schema_version: 3,
  revision: 1,
  characters: { mrbones: { character_class: "necromancer", combat_profile: "necro_bone_spear", last_difficulty: "nightmare", queue: ["countess"] } },
  budgets: { max_runs: 10, max_duration_ms: 3_600_000, max_consecutive_failures: 2, max_total_restarts: 3 },
  input: { enabled: false, pause_hotkey: "pause", stop_after_run_hotkey: "f10", recording_finish_hotkey: "f9", emergency_stop_hotkey: "f11" },
  history: { retention_enabled: true, retention_days: 60 },
};

describe("settingsDiff", () => {
  it("übersetzt Feldpfade und fasst Änderungen zusammen", () => {
    expect(labelChangedField("input.pause_hotkey")).toBe("Pause-Hotkey");
    expect(labelChangedField("characters.mrbones.queue")).toBe("Run-Reihenfolge (mrbones)");
    expect(summarizeChangedFields(["input.pause_hotkey", "budgets.max_runs"])).toBe("Pause-Hotkey, Maximale Runs");
  });

  it("erkennt Binding-Diffs", () => {
    const draft = structuredClone(base);
    draft.characters.mrbones.profile_bindings = {
      necro_bone_spear: { skills: { teleport: "f7" }, belt: { slot_1: "1" } },
    };
    expect(collectLocalDiffPaths(base, draft)).toEqual(["characters.mrbones.profile_bindings"]);
    expect(labelChangedField("characters.mrbones.profile_bindings")).toBe("Tastenbelegung (mrbones)");
  });
});
