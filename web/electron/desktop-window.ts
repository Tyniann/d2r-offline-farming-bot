import type { WindowBounds } from "./desktop-settings.js";

export interface VisibleWorkArea {
  x: number;
  y: number;
  width: number;
  height: number;
}

export const DEFAULT_WINDOW_BOUNDS: Readonly<WindowBounds> = Object.freeze({ x: 80, y: 60, width: 1440, height: 900 });

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
