import type { WindowBounds } from "./desktop-settings.js";

export interface VisibleWorkArea {
  x: number;
  y: number;
  width: number;
  height: number;
}

export const DEFAULT_WINDOW_BOUNDS: Readonly<WindowBounds> = Object.freeze({ x: 80, y: 60, width: 1440, height: 900 });

export const DEFAULT_ZOOM_FACTOR = 1;
export const ZOOM_FACTOR_MIN = 0.5;
export const ZOOM_FACTOR_MAX = 2;
export const ZOOM_FACTOR_STEP = 0.1;

export type ZoomDirection = "in" | "out" | "reset";

export interface ZoomKeyInput {
  type: string;
  key: string;
  code: string;
  control: boolean;
  meta: boolean;
  alt: boolean;
}

export function clampWindowBounds(saved: WindowBounds | undefined, workAreas: readonly VisibleWorkArea[]): WindowBounds {
  const source = saved ?? DEFAULT_WINDOW_BOUNDS;
  const areas = workAreas.filter((area) => area.width > 0 && area.height > 0);
  if (areas.length === 0) return { ...source };
  const target = areas.reduce((best, candidate) => intersectionArea(source, candidate) > intersectionArea(source, best) ? candidate : best, areas[0]);
  const width = Math.min(Math.max(source.width, 1100), target.width);
  const height = Math.min(Math.max(source.height, 700), target.height);
  return {
    x: clamp(source.x, target.x, target.x + target.width - width),
    y: clamp(source.y, target.y, target.y + target.height - height),
    width,
    height,
  };
}

function intersectionArea(bounds: WindowBounds, area: VisibleWorkArea): number {
  const width = Math.max(0, Math.min(bounds.x + bounds.width, area.x + area.width) - Math.max(bounds.x, area.x));
  const height = Math.max(0, Math.min(bounds.y + bounds.height, area.y + area.height) - Math.max(bounds.y, area.y));
  return width * height;
}

function clamp(value: number, minimum: number, maximum: number): number {
  return Math.min(Math.max(value, minimum), Math.max(minimum, maximum));
}

export function clampZoomFactor(value: number): number {
  if (!Number.isFinite(value)) return DEFAULT_ZOOM_FACTOR;
  return clamp(Math.round(value * 100) / 100, ZOOM_FACTOR_MIN, ZOOM_FACTOR_MAX);
}

export function nextZoomFactor(current: number, direction: ZoomDirection): number {
  if (direction === "reset") return DEFAULT_ZOOM_FACTOR;
  return clampZoomFactor(current + (direction === "in" ? ZOOM_FACTOR_STEP : -ZOOM_FACTOR_STEP));
}

export function parseZoomDirection(value: unknown): ZoomDirection {
  if (value !== "in" && value !== "out" && value !== "reset") {
    throw new Error("Unbekannte Zoomrichtung.");
  }
  return value;
}

export function zoomDirectionFromInput(input: ZoomKeyInput): ZoomDirection | undefined {
  if (input.type !== "keyDown" || input.alt || !(input.control || input.meta)) return undefined;
  if (input.key === "+" || input.key === "=" || input.code === "NumpadAdd") return "in";
  if (input.key === "-" || input.key === "_" || input.code === "NumpadSubtract") return "out";
  if (input.key === "0" || input.code === "Digit0" || input.code === "Numpad0") return "reset";
}

export function zoomDirectionFromWheel(ctrlKey: boolean, deltaY: number): ZoomDirection | undefined {
  if (!ctrlKey || deltaY === 0 || !Number.isFinite(deltaY)) return undefined;
  return deltaY < 0 ? "in" : "out";
}
