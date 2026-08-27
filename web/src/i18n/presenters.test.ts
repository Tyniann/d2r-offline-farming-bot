import { beforeEach, describe, expect, it } from "vitest";

import { changeAppLanguage, i18n } from ".";
import {
  presentHistoryReason,
  presentProblem,
  presentClassName,
  presentDifficultyName,
  presentProfileName,
  presentRecordingHint,
  presentRecordingInstruction,
  presentRunName,
  presentRunStage,
} from "./presenters";

describe("semantische Presenter", () => {
  beforeEach(() => changeAppLanguage("de"));

  it("übersetzt Fortschrittscode und Parameter in beiden Sprachen", async () => {
    expect(presentRunStage("cellar_floor", { floor: 3, floors: 5 }, i18n.t)).toBe("Kellergeschoss 3 von 5");
    await changeAppLanguage("en");
    expect(presentRunStage("cellar_floor", { floor: 3, floors: 5 }, i18n.t)).toBe("Cellar level 3 of 5");
  });

  it("interpoliert Aufnahme-Hotkeys und trennt Cow-Rollen", async () => {
    expect(presentRecordingInstruction("record_cow_leg", "F8", i18n.t)).toMatch(/Tristram.*F8/);
    expect(presentRecordingInstruction("record_cow_sweep", "F9", i18n.t)).toMatch(/Farming-Schleife.*F9/);
    expect(presentRecordingHint("cow_leg_do_not_click_wirt", i18n.t)).toContain("Wirt");
    await changeAppLanguage("en");
    expect(presentRecordingInstruction("record_cow_leg", "F8", i18n.t)).toMatch(/Tristram.*F8/);
    expect(presentRecordingInstruction("record_cow_sweep", "F9", i18n.t)).toMatch(/farming loop.*F9/);
  });

  it("übersetzt denselben Historiengrund in Liste und Detail", async () => {
    expect(presentHistoryReason("boss_not_found", i18n.t)).toBe("Der Boss wurde nicht gefunden.");
    expect(presentHistoryReason("future_reason", i18n.t)).toBe("Reason-Code: future_reason");
    await changeAppLanguage("en");
    expect(presentHistoryReason("boss_not_found", i18n.t)).toBe("The boss was not found.");
    expect(presentHistoryReason("future_reason", i18n.t)).toBe("Reason code: future_reason");
  });

  it("zeigt beide Ursachen eines fehlgeschlagenen kontrollierten Rückwegs", () => {
    expect(presentProblem({
      code: "retry_return_failed",
      params: { original_reason: "mercenary_died_during_run", recovery_reason: "town_portal_not_found" },
    }, i18n.t)).toContain("Söldner");
    expect(presentProblem({
      code: "retry_return_failed",
      params: { original_reason: "mercenary_died_during_run", recovery_reason: "town_portal_not_found" },
    }, i18n.t)).toContain("kein Stadtportal");
  });

  it("zeigt die Recovery-Grenzfehler als kurze Bedienhinweise", () => {
    expect(presentProblem({ code: "game_exit_failed" }, i18n.t)).toBe("Das Spiel konnte nicht sicher beendet werden. Die Session wurde gestoppt.");
    expect(presentProblem({ code: "start_town_normalization_failed" }, i18n.t)).toBe("Die Rückkehr nach Akt 1 ist fehlgeschlagen.");
  });

  it("übersetzt Difficulty, Run, Klasse und eingebautes Profil", async () => {
    expect([presentDifficultyName("nightmare", i18n.t), presentRunName("countess", i18n.t), presentClassName("necromancer", i18n.t), presentProfileName("necro_bone_spear", "Fallback", i18n.t)]).toEqual(["Alptraum", "Gräfin", "Totenbeschwörer", "Knochen-Totenbeschwörer"]);
    await changeAppLanguage("en");
    expect([presentDifficultyName("nightmare", i18n.t), presentRunName("countess", i18n.t), presentClassName("necromancer", i18n.t), presentProfileName("necro_bone_spear", "Fallback", i18n.t)]).toEqual(["Nightmare", "Countess", "Necromancer", "Bone Necromancer"]);
  });
});
