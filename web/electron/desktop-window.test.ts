// @vitest-environment node

import { describe, expect, it } from "vitest";
import { clampWindowBounds, clampZoomFactor, DEFAULT_WINDOW_BOUNDS, DEFAULT_ZOOM_FACTOR, nextZoomFactor, parseZoomDirection, zoomDirectionFromInput, zoomDirectionFromWheel } from "./desktop-window.js";

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

describe("window zoom", () => {
  it("klemmt unsichere Faktoren auf den sichtbaren Bereich", () => {
    expect(clampZoomFactor(Number.NaN)).toBe(DEFAULT_ZOOM_FACTOR);
    expect(clampZoomFactor(0.2)).toBe(0.5);
    expect(clampZoomFactor(3)).toBe(2);
    expect(clampZoomFactor(1.249)).toBe(1.25);
  });

  it("ändert den Faktor in Zehntelschritten und setzt ihn auf 100 Prozent zurück", () => {
    expect(nextZoomFactor(1, "in")).toBe(1.1);
    expect(nextZoomFactor(0.5, "out")).toBe(0.5);
    expect(nextZoomFactor(1.7, "reset")).toBe(1);
  });

  it("erkennt Browser-ähnliche Zoom-Tasten und das Mausrad", () => {
    expect(zoomDirectionFromInput({ type: "keyDown", key: "+", code: "Equal", control: true, meta: false, alt: false })).toBe("in");
    expect(zoomDirectionFromInput({ type: "keyDown", key: "0", code: "Digit0", control: true, meta: false, alt: false })).toBe("reset");
    expect(zoomDirectionFromInput({ type: "keyDown", key: "-", code: "Minus", control: false, meta: false, alt: false })).toBeUndefined();
    expect(zoomDirectionFromWheel(true, -120)).toBe("in");
    expect(zoomDirectionFromWheel(true, 120)).toBe("out");
    expect(zoomDirectionFromWheel(false, -120)).toBeUndefined();
    expect(parseZoomDirection("out")).toBe("out");
    expect(() => parseZoomDirection("bigger")).toThrow("Zoomrichtung");
  });
});
