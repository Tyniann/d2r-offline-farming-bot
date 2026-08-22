import { beforeEach, describe, expect, it } from "vitest";

import { changeAppLanguage } from "../../i18n";
import { reasonLabel, runPurposeLabels } from "./routePresentation";

beforeEach(() => changeAppLanguage("de"));

describe("reasonLabel", () => {
  it("übersetzt die Recording-Voraussetzungsgründe des Core", () => {
    expect(reasonLabel("onboarding_teleport_binding_missing"))
      .toBe("Vervollständige für diesen Charakter unter „Charaktere“ die Tastenbelegung des Kampfprofils.");
    expect(reasonLabel("onboarding_town_portal_binding_missing"))
      .toBe("Hinterlege für diesen Charakter unter „Charaktere“ die Taste für das Stadtportal.");
  });

	it("übersetzt einen geänderten Entwurfskontext ausschließlich über den Code", () => {
		expect(reasonLabel("route_candidate_changed"))
			.toBe("Die Zuordnung für diesen Run hat sich seit der Aufnahme geändert.");
	});

	it("übersetzt denselben Code auf Englisch und fällt für unbekannte Codes neutral zurück", async () => {
		await changeAppLanguage("en");
		expect(reasonLabel("route_candidate_changed")).toBe("The assignment for this run changed after recording.");
		expect(reasonLabel("free-form sentence")).toBe("This action is not available right now.");
	});
});

describe("runPurposeLabels", () => {
  it.each([
    ["countess", ["Schlüssel des Terrors", "Runen"]],
    ["summoner", ["Schlüssel des Hasses"]],
    ["lower-kurast", ["Hohe Runen", "Edelsteine"]],
    ["mephisto", ["Set-Items", "Unique-Items"]],
    ["nihlathak", ["Schlüssel der Zerstörung"]],
    ["cows", ["Weiße Rohlinge", "Gesockelte Rohlinge", "Edelsteine", "Erfahrung"]],
  ])("beschreibt den Zweck von %s", (runID, expected) => {
    expect(runPurposeLabels(runID)).toEqual(expected);
  });

  it("erfindet für unbekannte Runs keinen Zweck", () => {
    expect(runPurposeLabels("unknown")).toEqual([]);
  });
});
