// @vitest-environment node

import { expect, it } from "vitest";
import { readPortalMarkPNG } from "./portal-icon.js";

it("liefert ein gültig dimensioniertes Town-Portal-PNG für Tray, Notification und UI", () => {
  const png = readPortalMarkPNG();
  expect(png.subarray(0, 8).toString("hex")).toBe("89504e470d0a1a0a");
  expect(png.readUInt32BE(16)).toBe(512);
  expect(png.readUInt32BE(20)).toBe(512);
  expect(png.length).toBeGreaterThan(200);
});
