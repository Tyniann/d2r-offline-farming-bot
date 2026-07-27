// @vitest-environment node

import { describe, expect, it } from "vitest";
import { clampWindowBounds, DEFAULT_WINDOW_BOUNDS } from "./desktop-window.js";

const primary = { x: 0, y: 0, width: 1920, height: 1080 };
const left = { x: -1280, y: 40, width: 1280, height: 1024 };

describe("clampWindowBounds", () => {
  it("verwendet sichere Desktop-Defaults", () => {
    expect(clampWindowBounds(undefined, [primary])).toEqual(DEFAULT_WINDOW_BOUNDS);
  });

  it("erhält sichtbare Bounds auf einem sekundären Monitor", () => {
    expect(clampWindowBounds({ x: -1200, y: 80, width: 1200, height: 800 }, [primary, left])).toEqual({ x: -1200, y: 80, width: 1200, height: 800 });
  });

  it("klemmt vollständig verschwundene Bounds auf einen sichtbaren Monitor", () => {
    expect(clampWindowBounds({ x: 5000, y: 4000, width: 1600, height: 1000 }, [primary])).toEqual({ x: 320, y: 80, width: 1600, height: 1000 });
  });

  it("passt übergroße Bounds an die Work Area an", () => {
    expect(clampWindowBounds({ x: -300, y: -200, width: 4000, height: 3000 }, [primary])).toEqual(primary);
  });
});
