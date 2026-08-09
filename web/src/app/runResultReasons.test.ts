import { describe, expect, it } from "vitest";
import { runResultReasonText } from "./runResultReasons";

describe("runResultReasonText", () => {
  it("übersetzt Cow-Rezept-, Kampf- und dynamische Shopfehler", () => {
    expect(runResultReasonText("cow_portal_missing_after_consumption")).toContain("kein neues Kuhportal");
    expect(runResultReasonText("cow_combat_no_progress")).toContain("keinen bestätigten Fortschritt");
    expect(runResultReasonText("cow_tome_shop_close_failed: escape rejected")).toContain("Handelsfenster");
    expect(runResultReasonText("cow_rejuvenation_reserve_missing")).toContain("Regenerationstrank");
    expect(runResultReasonText("cow_belt_layout_unseeded")).toContain("Gürtelspalten");
    expect(runResultReasonText("mercenary_died_during_run")).toContain("Angriff wurde gestoppt");
    expect(runResultReasonText("combat_resource_exhausted")).toContain("Trank ist aufgebraucht");
    expect(runResultReasonText("profile_required_skills_missing")).toContain("Pflichtskills");
    expect(runResultReasonText("profile_skills_read_unavailable")).toContain("nicht sicher geprüft");
  });

  it("lässt unbekannte Gründe unverändert", () => {
    expect(runResultReasonText("future_reason")).toBe("future_reason");
  });
});
