export type AppTarget = "dashboard" | "routes" | "pickit" | "history" | "settings";

export const appTargets: readonly AppTarget[] = ["dashboard", "routes", "pickit", "history", "settings"];

export function targetFromHash(hash: string): AppTarget {
  const candidate = hash.replace(/^#/, "").toLowerCase();
  if (candidate === "betrieb") return "dashboard";
  return appTargets.includes(candidate as AppTarget) ? candidate as AppTarget : "dashboard";
}

