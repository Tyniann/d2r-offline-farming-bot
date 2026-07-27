// @vitest-environment node

import { describe, expect, it } from "vitest";
import { carryOnboardingStep } from "./onboarding-resume.js";

describe("Onboarding-Fortsetzung beim Core-Neustart", () => {
  const token = "a".repeat(43);
  const target = `http://127.0.0.1:43124/#control_token=${token}`;

  it("überträgt ausschließlich einen gültigen Schritt auf die neue Loopback-Origin", () => {
    const result = carryOnboardingStep("http://127.0.0.1:43123/?onboarding_step=4#dashboard", target, 8);
    const url = new URL(result);
    expect(url.searchParams.get("onboarding_step")).toBe("4");
    expect(url.hash).toBe(`#control_token=${token}`);
  });

  it("ignoriert fehlende, ungültige und außerhalb liegende Werte", () => {
    expect(carryOnboardingStep("http://127.0.0.1:43123/#dashboard", target, 8)).toBe(target);
    expect(carryOnboardingStep("http://127.0.0.1:43123/?onboarding_step=9#dashboard", target, 8)).toBe(target);
    expect(carryOnboardingStep("keine-url", target, 8)).toBe(target);
  });
});
