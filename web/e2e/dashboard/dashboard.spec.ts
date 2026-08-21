import { expect, test, type Page, type Route } from "@playwright/test";

test("Leerlauf bleibt an allen Zielbreiten fokussierbar und ohne horizontalen Überlauf", async ({ page }) => {
  const browserErrors = captureBrowserErrors(page);
  await mockDashboard(page, false);
  await page.goto("/?variant=A&state=active#dashboard");
  await expect(page.getByRole("heading", { name: "MrBones ist bereit" })).toBeVisible();

  for (const [width, height] of [[1440, 900], [1100, 700], [900, 760], [760, 800]] as const) {
    await page.setViewportSize({ width, height });
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    const columns = await page.locator(".dashboard-main-grid").evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length);
    expect(columns).toBe(width >= 1181 ? 3 : width >= 1000 ? 2 : 1);
  }

  await page.setViewportSize({ width: 1440, height: 900 });
  await page.waitForTimeout(250);
  const period = page.getByRole("button", { name: "30 Tage" });
  await period.focus();
  expect(await period.evaluate((element) => getComputedStyle(element).outlineStyle)).not.toBe("none");
  await page.screenshot({ path: "../.tmp/dashboard-final-1440x900.png", fullPage: true });
  await page.setViewportSize({ width: 1100, height: 700 });
  await page.waitForTimeout(250);
  await page.screenshot({ path: "../.tmp/dashboard-final-1100x700.png", fullPage: true });
  expect(browserErrors).toEqual([]);
});

test("aktiver Run bleibt bei reduzierter Bewegung ruhig und verwendet nur Hotkey-Hinweise", async ({ page }) => {
  const browserErrors = captureBrowserErrors(page);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await mockDashboard(page, true);
  await page.setViewportSize({ width: 760, height: 800 });
  await page.goto("/#dashboard");
  await expect(page.getByLabel("Etappe 6 von 13")).toBeVisible();
  await expect(page.getByRole("status").filter({ hasText: "Pause nach diesem Run vorgemerkt" })).toBeVisible();
  await expect(page.getByRole("button", { name: /pausieren|stoppen|Emergency Stop/i })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Jetzt farmen" })).toHaveCount(0);
  for (const [width, height] of [[1440, 900], [1100, 700], [760, 800]] as const) {
    await page.setViewportSize({ width, height });
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await expect(page.getByLabel("Etappe 6 von 13")).toBeVisible();
    if (width === 1100) await page.screenshot({ path: "../.tmp/dashboard-active-1100x700.png", fullPage: true });
  }
  const layout = await page.evaluate(() => {
    const sidebar = document.querySelector(".sidebar")!.getBoundingClientRect();
    const header = document.querySelector(".dashboard-header")!.getBoundingClientRect();
    const activeRun = document.querySelector(".dashboard-active-run")!.getBoundingClientRect();
    return { sidebarBottom: sidebar.bottom, headerTop: header.top, headerBottom: header.bottom, activeTop: activeRun.top };
  });
  expect(layout.headerTop).toBeGreaterThanOrEqual(layout.sidebarBottom - 1);
  expect(layout.activeTop).toBeGreaterThanOrEqual(layout.headerBottom);
  expect(await page.locator(".dashboard-active-run-steps .is-current").evaluate((element) => getComputedStyle(element).animationName)).toBe("none");
  await page.screenshot({ path: "../.tmp/dashboard-active-760x800.png" });
  expect(browserErrors).toEqual([]);
});

function captureBrowserErrors(page: Page): string[] {
  const errors: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  page.on("pageerror", (error) => errors.push(error.message));
  return errors;
}

async function mockDashboard(page: Page, active: boolean): Promise<void> {
  await page.addInitScript(() => {
    class DashboardEventSource {
      onopen: (() => void) | null = null;
      onerror: (() => void) | null = null;
      constructor() { window.setTimeout(() => this.onopen?.(), 0); }
      addEventListener() {}
      close() {}
    }
    Object.defineProperty(window, "EventSource", { value: DashboardEventSource, configurable: true });
  });
  const queue = { entries: ["countess", "mephisto"], default_entries: ["countess", "mephisto"], index: 0, cycle: 0, retry: 0, started_runs: active ? 1 : 0, consecutive_failures: 0, total_restarts: 0, budgets: { max_runs: 8, max_duration_ms: 7200000, max_consecutive_failures: 2, max_total_restarts: 3 } };
  const status = { schema_version: 1, app_version: "0.1.0", core_version: "0.1.0", state: active ? "running_run" : "idle_in_game", lifecycle_phase: active ? "running_run" : "idle_in_game", generation: 7, pending_intent: active ? "pause_after_run" : "none", active_run_id: active ? "countess" : undefined, run_id: active ? "run-1" : undefined, run_progress: active ? { label: "Kellergeschoss 3 von 5", current: 6, total: 13 } : undefined, d2r: { state: "attached", pid: 42, window_bound: true, client_width: 1280, client_height: 720 }, compatibility: { state: "compatible", supported_version: "3.2", expected_version: "3.2", offset_version: "3.2", actual_version: "3.2", privilege_mismatch: false }, input: { enabled: true, paused: false, stopped: false }, world: { valid: true, phase: "in_game", area_id: active ? 23 : 1, area_name: active ? "Kellergeschoss 3" : "Lager der Jägerinnen" }, selection: { character: "MrBones", difficulty: "nightmare" }, queue };
  const catalog = { schema_version: 1, revision: 3, default_difficulty: "nightmare", characters: [{ name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true, reasons: [], farm_ready_reasons: [] }], difficulties: [{ id: "nightmare", display_name: "Alptraum" }], profiles: [], runs: [{ run_id: "countess", display_name: "Countess", status: "runtime_validation_required", reasons: [], route_combat: {} }, { run_id: "mephisto", display_name: "Mephisto", status: "runtime_validation_required", reasons: [], route_combat: {} }] };
  const durations = { count: 5, total_ms: 390000, average_ms: 78000, median_ms: 76000, minimum_ms: 60000, maximum_ms: 100000 };
  const stages = { travel_ms: 180000, combat_ms: 90000, loot_ms: 60000, return_town_ms: 60000, other_ms: 0 };
  const funnel = { seen: 12, matched: 8, picked_up: 6, stashed: 4, sold: 0, keep_return: 4, pickup_lost: 2, post_pickup_lost: 0 };
  const summary = { runs: 5, terminal_runs: 5, successful: 4, failed: 1, aborted: 0, incomplete: 0, running: 0, success_rate: .8, boss_kills: 4, durations, stages, funnel, keep_per_hour: 2.7 };
  const comparison = { id: "countess-a", character: "MrBones", difficulty: "nightmare", definition_id: "countess", run: "countess", route_id: "route-a", terminal_runs: 5, successful: 4, failed: 1, aborted: 0, success_rate: .8, boss_kills: 4, low_sample: false, durations, stages, funnel, keep_per_hour: 2.7 };
  const recent = { run_id: "history-1", started_at: "2026-08-22T10:00:00Z", observed_at: "2026-08-22T10:01:18Z", character: "MrBones", difficulty: "nightmare", run: "countess", definition_id: "countess", route_id: "route-a", outcome: "success", duration_ms: 78000, boss_kills: 1, funnel };
  await page.route("**/api/v1/**", async (route: Route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/v1/status") return json(route, status);
    if (path === "/api/v1/catalog") return json(route, catalog);
    if (path === "/api/v1/settings/operator") return json(route, { schema_version: 1, revision: 1, characters: { mrbones: { last_difficulty: "nightmare", queue: ["countess", "mephisto"] } }, budgets: queue.budgets, input: { enabled: true, pause_hotkey: "pause", stop_after_run_hotkey: "f10", recording_finish_hotkey: "f9", emergency_stop_hotkey: "f11" }, history: { retention_enabled: false, retention_days: 30 } });
    if (path === "/api/v1/routes/workflow") return json(route, { workflow_id: "", generation: 1, state: "idle", run_id: "", character: "" });
    if (path === "/api/v1/history/summary") return json(route, { summary });
    if (path === "/api/v1/history/comparisons") return json(route, { comparisons: [comparison] });
    if (path === "/api/v1/history/runs") return json(route, { runs: [recent] });
    if (path === "/api/v1/runs") return json(route, { schema_version: 1, character: "MrBones", difficulty: "nightmare", runs: catalog.runs });
    if (path === "/api/v1/events") return route.fulfill({ status: 200, contentType: "text/event-stream", body: `event: snapshot\ndata: ${JSON.stringify(status)}\n\n` });
    return json(route, {});
  });
}

function json(route: Route, value: unknown): Promise<void> {
  return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(value) });
}
