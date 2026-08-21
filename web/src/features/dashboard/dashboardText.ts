/** dashboardRunName returns the product-facing German name for a stable run ID. */
export function dashboardRunName(runID: string, fallback = ""): string {
  return ({
    countess: "Gräfin",
    summoner: "Beschwörer",
    mephisto: "Mephisto",
    nihlathak: "Nihlathak",
    "lower-kurast": "Unter-Kurast",
    cows: "Kuh-Level",
    "cow-level": "Kuh-Level",
  } as Record<string, string>)[runID.toLowerCase()] ?? (fallback || runID);
}

/** dashboardDifficultyName returns the German label for a stable difficulty ID. */
export function dashboardDifficultyName(difficulty: string): string {
  return ({ normal: "Normal", nightmare: "Alptraum", hell: "Hölle" } as Record<string, string>)[difficulty.toLowerCase()] ?? difficulty;
}
