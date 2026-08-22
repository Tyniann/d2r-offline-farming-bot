import { describe, expect, it } from "vitest";

import { reasonLabel, runPurposeLabels } from "./routePresentation";

describe("reasonLabel", () => {
  it("übersetzt einen fehlenden Teleport-Binding aus dem Core", () => {
    expect(reasonLabel("bindings precheck: teleport not configured: set the character loadout Teleport F-key in Charaktere"))
      .toBe("Vervollständige für diesen Charakter unter „Charaktere“ die Tastenbelegung des Kampfprofils.");
  });

  it("übersetzt die Recording-Voraussetzungsgründe des Core", () => {
    expect(reasonLabel("onboarding_teleport_binding_missing"))
      .toBe("Vervollständige für diesen Charakter unter „Charaktere“ die Tastenbelegung des Kampfprofils.");
    expect(reasonLabel("onboarding_town_portal_binding_missing"))
      .toBe("Hinterlege für diesen Charakter unter „Charaktere“ die Taste für das Stadtportal.");
  });

  it("übersetzt einen geänderten Entwurfskontext", () => {
    expect(reasonLabel("live candidate context changed"))
      .toBe("Charakter oder Schwierigkeit passen nicht mehr zu diesem Entwurf.");
    expect(reasonLabel("route_candidate_changed"))
      .toBe("Die Zuordnung für diesen Run hat sich seit der Aufnahme geändert.");
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
