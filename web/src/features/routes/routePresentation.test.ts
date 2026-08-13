import { describe, expect, it } from "vitest";

import { reasonLabel } from "./routePresentation";

describe("reasonLabel", () => {
  it("übersetzt einen fehlenden Teleport-Binding aus dem Core", () => {
    expect(reasonLabel("bindings precheck: teleport not configured: set the character loadout Teleport F-key in Charaktere"))
      .toBe("Vervollständige für diesen Charakter unter „Charaktere“ die Tastenbelegung des Kampfprofils.");
  });
});
