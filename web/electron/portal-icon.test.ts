// @vitest-environment node

import { expect, it } from "vitest";
import { createPortalIconPNG } from "./portal-icon.js";

it("erzeugt ein gültig dimensioniertes originales Portal-PNG für Tray und Notification", () => {
  const png = createPortalIconPNG(32);
  expect(png.subarray(0, 8).toString("hex")).toBe("89504e470d0a1a0a");
  expect(png.readUInt32BE(16)).toBe(32);
  expect(png.readUInt32BE(20)).toBe(32);
  expect(png.length).toBeGreaterThan(200);
});
