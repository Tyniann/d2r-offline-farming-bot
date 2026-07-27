import { describe, expect, it } from "vitest";
import { appTargets, targetFromHash } from "./navigation";

describe("targetFromHash", () => {
  it.each(appTargets)("erhält das stabile Deep Link #%s", (target) => {
    expect(targetFromHash(`#${target}`)).toBe(target);
  });

  it("fällt bei leeren und unbekannten Zielen auf das Dashboard zurück", () => {
    expect(targetFromHash("")).toBe("dashboard");
    expect(targetFromHash("#unbekannt")).toBe("dashboard");
  });

  it("hält den bisherigen Betrieb-Link als Dashboard-Alias lesbar", () => {
    expect(targetFromHash("#betrieb")).toBe("dashboard");
  });
});
