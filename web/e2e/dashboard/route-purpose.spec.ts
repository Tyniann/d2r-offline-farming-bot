import { expect, test, type Page, type Route } from "@playwright/test";

test("Routenzweck bleibt vor der Aufnahme auf Desktop und schmalen Ansichten sichtbar", async ({ page }) => {
  await mockRoutes(page);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/#dashboard");
  await page.getByRole("link", { name: "Routen" }).click();
  await page.getByRole("button", { name: "Route aufnehmen" }).click();

  const countessPurpose = page.getByRole("note", { name: "Geeignet für Gräfin" });
  await expect(countessPurpose).toContainText("Schlüssel des Terrors");
  await expect(countessPurpose).toContainText("Runen");
  await expect(countessPurpose.getByRole("link", { name: "Pickit konfigurieren" })).toHaveAttribute("href", "#pickit");
  await page.screenshot({ path: "../.tmp/route-purpose-1440x900.png", fullPage: true });

  await page.getByRole("button", { name: /^Kuhlevel/ }).click();
  await page.setViewportSize({ width: 760, height: 800 });
  const cowPurpose = page.getByRole("note", { name: "Geeignet für Kuhlevel" });
  for (const label of ["Weiße Rohlinge", "Gesockelte Rohlinge", "Edelsteine", "Erfahrung"]) await expect(cowPurpose).toContainText(label);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: "../.tmp/route-purpose-cows-760x800.png", fullPage: true });
});

async function mockRoutes(page: Page): Promise<void> {
  await page.addInitScript(() => {
    class RouteEventSource {
      onopen: (() => void) | null = null;
      onerror: (() => void) | null = null;
      constructor() { window.setTimeout(() => this.onopen?.(), 0); }
      addEventListener() {}
      close() {}
    }
    Object.defineProperty(window, "EventSource", { value: RouteEventSource, configurable: true });
  });
  const queue = { entries: ["countess"], default_entries: ["countess"], index: 0, cycle: 0, retry: 0, started_runs: 0, consecutive_failures: 0, total_restarts: 0, budgets: { max_runs: 8, max_duration_ms: 7200000, max_consecutive_failures: 2, max_total_restarts: 3 } };
  const status = { schema_version: 1, app_version: "0.1.0", core_version: "0.1.0", state: "idle_in_game", lifecycle_phase: "idle_in_game", generation: 1, pending_intent: "none", d2r: { state: "attached", pid: 42, window_bound: true, client_width: 1280, client_height: 720 }, compatibility: { state: "compatible", supported_version: "3.2", expected_version: "3.2", offset_version: "3.2", actual_version: "3.2", privilege_mismatch: false }, input: { enabled: true, paused: false, stopped: false }, world: { valid: true, phase: "in_game", area_id: 1, area_name: "Lager der Jägerinnen" }, selection: { character: "MrBones", difficulty: "hell" }, queue };
  const runs = ["countess", "mephisto", "lower-kurast", "summoner", "nihlathak", "cows"].map((runID) => ({ run_id: runID, display_name: runID, status: "runtime_validation_required", reasons: [], route_combat: {} }));
  const catalog = { schema_version: 1, revision: 1, default_difficulty: "hell", characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true, reasons: [], farm_ready_reasons: [] }], difficulties: [{ id: "hell", display_name: "Hölle" }], profiles: [], runs };
  const option = (runID: string, startWaypoint: string) => ({ run_id: runID, display_name: runID, instructions_de: "Folge der Aufnahme-Anleitung bis zum Ziel.", start_waypoint: startWaypoint, allowed_start_area_id: 1, allowed_route_area_ids: [1], terminal_area_id: 1, terminal_max_distance_tiles: 80, available: true, prerequisites: [] });
  const options = [option("countess", "black_marsh"), option("mephisto", "durance_of_hate_level_2"), option("lower-kurast", "lower_kurast"), option("summoner", "arcane_sanctuary"), option("nihlathak", "halls_of_pain"), { ...option("cows", "stony_field"), route_role: "leg_acquisition" }, { ...option("cows", ""), route_role: "cow_sweep", start_kind: "object_portal_arrival" }];
  await page.route("**/api/v1/**", async (route: Route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/v1/status") return json(route, status);
    if (path === "/api/v1/catalog") return json(route, catalog);
    if (path === "/api/v1/settings/operator") return json(route, { schema_version: 1, revision: 1, characters: { mrbones: { last_difficulty: "hell", queue: ["countess"] } }, budgets: queue.budgets, input: { enabled: true, pause_hotkey: "pause", stop_after_run_hotkey: "f10", recording_finish_hotkey: "f9", emergency_stop_hotkey: "f11" }, history: { retention_enabled: false, retention_days: 30 } });
    if (path === "/api/v1/routes/workflow") return json(route, { workflow_id: "", generation: 1, state: "idle", run_id: "", character: "MrBones" });
    if (path === "/api/v1/routes") return json(route, { revision: 1, character: "MrBones", routes: [] });
    if (path === "/api/v1/route-recording/options") return json(route, options);
    if (path === "/api/v1/routes/candidates") return json(route, []);
    if (path === "/api/v1/routes/hotkeys") return json(route, { recording_finish: "F9", stop_after_run: "F10", emergency_stop: "F11", pause: "Pause" });
    if (path === "/api/v1/runs") return json(route, { schema_version: 1, character: "MrBones", difficulty: "hell", runs });
    if (path === "/api/v1/events") return route.fulfill({ status: 200, contentType: "text/event-stream", body: "" });
    return json(route, {});
  });
}

function json(route: Route, value: unknown): Promise<void> {
  return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(value) });
}
