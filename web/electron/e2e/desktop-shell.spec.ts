import { _electron as electron, expect, test, type ElectronApplication, type Page } from "@playwright/test";
import electronExecutable from "electron";
import { spawn } from "node:child_process";
import { access, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const main = join(webRoot, "dist-electron", "main.js");
const fakeCore = join(webRoot, "electron", "test-fixtures", "fake-core.mjs");

test("lädt genau ein sandboxed Fenster und blockiert Navigation, Fenster und zweite Instanz", async () => {
  const launched = await launch("valid");
  const { app, page, root } = launched;
  try {
    await expect(page.getByRole("heading", { name: "Fake-Core-Snapshot" })).toBeVisible();
    await expect(page.getByTestId("state")).toHaveText("idle");
    await expect.poll(() => app.windows().length).toBe(1);
    expect(await page.evaluate(() => ({ process: typeof process, require: typeof require, bridge: Object.keys((window as unknown as { d2rDesktop: object }).d2rDesktop ?? {}).sort() }))).toEqual({ process: "undefined", require: "undefined", bridge: ["checkForUpdates", "chooseImportRoot", "getAppInfo", "getDesktopSettings", "getProvisioningState", "getUpdateStatus", "onNavigate", "onUpdateStatus", "openReleasePage", "provision", "restartAsAdministrator", "restartCore", "revealDiagnosticBundle", "showWindow", "updateDesktopSettings"] });
    await expect(page.evaluate(() => (window as unknown as { d2rDesktop: { restartAsAdministrator(): Promise<void> } }).d2rDesktop.restartAsAdministrator())).rejects.toThrow("nicht nachgewiesen");
    expect(await page.evaluate(() => (window as unknown as { d2rDesktop: { getDesktopSettings(): Promise<unknown> } }).d2rDesktop.getDesktopSettings())).toEqual({ schema_version: 3, language: "de", autostart: false, onboarding_completed: false });
    expect(await page.evaluate(() => (window as unknown as { d2rDesktop: { updateDesktopSettings(value: unknown): Promise<unknown> } }).d2rDesktop.updateDesktopSettings({ autostart: true, onboarding_completed: true }))).toEqual({ schema_version: 3, language: "de", autostart: true, onboarding_completed: true });
    expect(await page.evaluate(() => (window as unknown as { d2rDesktop: { updateDesktopSettings(value: unknown): Promise<unknown> } }).d2rDesktop.updateDesktopSettings({ selected_character: "MrBones", selected_difficulty: "nightmare" }))).toEqual({ schema_version: 3, language: "de", autostart: true, onboarding_completed: true, selected_character: "MrBones", selected_difficulty: "nightmare" });
    await expect(page.evaluate(() => (window as unknown as { d2rDesktop: { updateDesktopSettings(value: unknown): Promise<unknown> } }).d2rDesktop.updateDesktopSettings({ autostart: true, onboarding_completed: true, input_enabled: true }))).rejects.toThrow("Unbekanntes");

    const original = page.url();
    await page.evaluate(() => { location.href = "https://example.com/"; });
    await page.waitForTimeout(250);
    expect(page.url()).toBe(original);
    expect(await page.evaluate(() => window.open("https://example.com/"))).toBeNull();

    const second = spawn(String(electronExecutable), [main, "--data-root", root, "--fake-core", fakeCore, "--fake-core-mode", "valid"], { windowsHide: true, stdio: "ignore" });
    const exitCode = await new Promise<number | null>((resolveExit) => second.once("exit", resolveExit));
    expect(exitCode).toBe(0);
    await expect.poll(() => app.windows().length).toBe(1);
    expect(await page.evaluate(() => fetch("/api/test/commands").then((response) => response.json()))).toMatchObject({ count: 0 });
    expect(JSON.parse(await readFile(join(root, "desktop-settings.json"), "utf8"))).toMatchObject({ schema_version: 3, language: "de", autostart: true, onboarding_completed: true, selected_character: "MrBones", selected_difficulty: "nightmare" });
  } finally {
    await close(launched);
  }
});

test("frischer Root zeigt dieselbe React-App vor dem produktiven Core", async () => {
  const base = await mkdtemp(join(tmpdir(), "d2rbot-electron-provision-"));
  const root = join(base, "data-root");
  const defaults = join(base, "defaults");
  await mkdir(defaults);
  const app = await electron.launch({ args: [main, "--data-root", root, "--fake-core", fakeCore, "--fake-core-mode", "valid", "--defaults-root", defaults] });
  const page = await app.firstWindow();
  try {
    await expect(page.getByRole("heading", { name: "Datenbasis einrichten" })).toBeVisible();
    expect(await page.evaluate(() => typeof process)).toBe("undefined");
    await expect(access(root)).rejects.toMatchObject({ code: "ENOENT" });
    await page.getByRole("button", { name: "Neuen Datenroot anlegen" }).click();
    await expect(page.getByRole("heading", { name: "Fake-Core-Snapshot" })).toBeVisible();
    expect(await readFile(join(root, "configs", "config.yaml"), "utf8")).toContain("fake");
  } finally {
    await app.close().catch(() => undefined);
    await rm(base, { recursive: true, force: true });
  }
});

test("Import übernimmt den abgeschlossenen Onboarding-Zustand vor dem produktiven Renderer", async () => {
  const base = await mkdtemp(join(tmpdir(), "d2rbot-electron-import-"));
  const root = join(base, "data-root");
  const importRoot = join(base, "import-root");
  await mkdir(importRoot);
  await writeFile(join(importRoot, "desktop-settings.json"), JSON.stringify({
    schema_version: 1,
    window_bounds: { x: 10, y: 20, width: 1280, height: 800 },
    autostart: true,
    onboarding_completed: true,
  }), "utf8");
  const app = await electron.launch({ args: [main, "--data-root", root, "--fake-core", fakeCore, "--fake-core-mode", "valid"] });
  const page = await app.firstWindow();
  try {
    await expect(page.getByRole("heading", { name: "Datenbasis einrichten" })).toBeVisible();
    await app.evaluate(({ dialog }, selectedRoot) => {
      dialog.showOpenDialog = async () => ({ canceled: false, filePaths: [selectedRoot], bookmarks: [] });
    }, importRoot);
    await page.getByRole("button", { name: "Bestehenden Datenroot auswählen" }).click();
    await page.getByRole("button", { name: "Ausgewählten Datenroot importieren" }).click();
    await expect(page.getByRole("heading", { name: "Fake-Core-Snapshot" })).toBeVisible();
    await expect.poll(() => page.evaluate(() => (
      window as unknown as { d2rDesktop: { getDesktopSettings(): Promise<unknown> } }
    ).d2rDesktop.getDesktopSettings())).toEqual({
      schema_version: 3,
      language: "de",
      autostart: false,
      onboarding_completed: true,
    });
  } finally {
    await app.close().catch(() => undefined);
    await rm(base, { recursive: true, force: true });
  }
});

for (const [mode, expectedReason, timeout] of [
  ["delayed", "core_handshake_timeout", 120],
  ["wrong", "core_handshake_invalid", 2_000],
  ["aborted", "core_handshake_invalid", 2_000],
] as const) {
  test(`${mode} Handshake endet in lokaler Recovery`, async () => {
    const launched = await launch(mode, timeout);
    try {
      await expect(launched.page.getByRole("heading", { name: "Core-Wiederherstellung erforderlich" })).toBeVisible();
      await expect.poll(() => launched.page.locator("body").getAttribute("data-reason")).toBe(expectedReason);
      await expect.poll(() => launched.app.windows().length).toBe(1);
    } finally {
      await close(launched);
    }
  });
}

test("inaktiver Core-Exit startet genau einmal neu", async () => {
  const launched = await launch("exit-idle");
  try {
    await expect(launched.page.getByRole("heading", { name: "Core-Wiederherstellung erforderlich" })).toBeVisible({ timeout: 8_000 });
    await expect.poll(() => launched.page.locator("body").getAttribute("data-restarts")).toBe("1");
  } finally {
    await close(launched);
  }
});

test("aktiver Core-Exit bleibt ohne Neustart fail-closed", async () => {
  const launched = await launch("exit-active");
  try {
    await expect(launched.page.getByRole("heading", { name: "Core-Wiederherstellung erforderlich" })).toBeVisible();
    await expect.poll(() => launched.page.locator("body").getAttribute("data-restarts")).toBe("0");
    await expect.poll(() => launched.page.locator("body").getAttribute("data-reason")).toBe("core_recovery_required");
  } finally {
    await close(launched);
  }
});

test("aktive X-Aktion versteckt in den Tray und erzeugt keinen Core-Command", async () => {
  const launched = await launch("steady-active");
  try {
    await expect(launched.page.getByTestId("state")).toHaveText("running_run");
    await launched.app.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0].close());
    await expect.poll(() => launched.app.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0].isVisible())).toBe(false);
    expect(await launched.page.evaluate(() => fetch("/api/test/commands").then((response) => response.json()))).toMatchObject({ count: 0 });
    await launched.page.waitForTimeout(1_700);
  } finally {
    await close(launched);
  }
});

test("klemmt gespeicherte Fensterbounds auf sichtbare Monitore", async () => {
  const launched = await launch("valid", 2_000, { schema_version: 1, window_bounds: { x: 5000, y: 4000, width: 1600, height: 900 }, autostart: false, onboarding_completed: false });
  try {
    const projection = await launched.app.evaluate(({ BrowserWindow, screen }) => ({ bounds: BrowserWindow.getAllWindows()[0].getBounds(), areas: screen.getAllDisplays().map((display) => display.workArea) }));
    expect(projection.bounds.x).not.toBe(5000);
    expect(projection.bounds.y).not.toBe(4000);
    expect(projection.areas.some((area) => projection.bounds.x >= area.x && projection.bounds.y >= area.y && projection.bounds.x + projection.bounds.width <= area.x + area.width && projection.bounds.y + projection.bounds.height <= area.y + area.height)).toBe(true);
  } finally {
    await close(launched);
  }
});

async function launch(mode: string, timeout = 2_000, settings?: object): Promise<{ app: ElectronApplication; page: Page; root: string }> {
  const root = await mkdtemp(join(tmpdir(), `d2rbot-electron-${mode}-`));
  await mkdir(join(root, "configs"), { recursive: true });
  await writeFile(join(root, "configs", "config.yaml"), "existing: true\n", "utf8");
  if (settings) await writeFile(join(root, "desktop-settings.json"), JSON.stringify(settings), "utf8");
  const app = await electron.launch({ args: [main, "--data-root", root, "--fake-core", fakeCore, "--fake-core-mode", mode, "--handshake-timeout-ms", String(timeout)] });
  const page = await app.firstWindow();
  return { app, page, root };
}

async function close(launched: { app: ElectronApplication; root: string }): Promise<void> {
  await launched.app.close().catch(() => undefined);
  await rm(launched.root, { recursive: true, force: true });
}
