import { _electron as electron } from "@playwright/test";
import { listPackage } from "@electron/asar";
import { access } from "node:fs/promises";
import { dirname, join } from "node:path";
import process from "node:process";

const [executablePath, dataRoot, expectedVersion, localAppData] = process.argv.slice(2);
if (!executablePath || !dataRoot || !expectedVersion || !localAppData) {
  throw new Error("package-smoke requires executable, data root, version and LocalAppData");
}

const archive = join(dirname(executablePath), "resources", "app.asar");
const packageEntries = new Set(listPackage(archive).map((entry) => entry.replaceAll("\\", "/").replace(/^\//, "")));
for (const required of [
  "dist-electron/locales/de.json",
  "dist-electron/locales/en.json",
  "dist-electron/recovery.html",
  "dist-electron/recovery.js",
  "dist-electron/ui/index.html",
]) {
  if (!packageEntries.has(required)) throw new Error(`packaged app is missing required content: ${required}`);
}

const product = await electron.launch({
  executablePath,
  args: ["--data-root", dataRoot],
  env: { ...process.env, LOCALAPPDATA: localAppData },
  timeout: 20_000,
});
try {
  const page = await product.firstWindow();
  try {
    await page.waitForFunction(() => typeof window.d2rDesktop?.getDesktopSettings === "function", undefined, { timeout: 20_000 });
    const language = await page.evaluate(async () => (await window.d2rDesktop.getDesktopSettings()).language);
    await page.getByRole("heading", { name: language === "de" ? "Datenbasis einrichten" : "Set up data" }).waitFor({ timeout: 20_000 });
    await access(dataRoot).then(
      () => { throw new Error("packaged Electron created the data root before Core provisioning"); },
      (error) => {
        if (error?.code !== "ENOENT") throw error;
      },
    );
    await page.getByRole("button", { name: language === "de" ? "Neuen Datenroot anlegen" : "Create new data root" }).click();
    await page.waitForSelector(".sidebar-meta", { timeout: 20_000 });
    await access(dataRoot);
    await access(join(dataRoot, "configs", "ui", "character-play.png"));
    await access(join(dataRoot, "configs", "ui", "difficulty-dialog.png"));
    await access(join(dataRoot, "configs", "ui", "characters", "mrbones-selected.png")).then(
      () => { throw new Error("fresh packaged defaults contain character-specific selection evidence"); },
      (error) => {
        if (error?.code !== "ENOENT") throw error;
      },
    );
  } catch (error) {
    const summary = await page.locator("body").innerText().catch(() => "");
    throw new Error(`packaged window did not reach the React shell: ${page.url()} :: ${summary.slice(0, 500)}`, { cause: error });
  }
  const bridgeVersion = await page.evaluate(async () => (await window.d2rDesktop.getAppInfo()).app_version);
  if (bridgeVersion !== expectedVersion) {
    throw new Error(`packaged Electron version ${bridgeVersion} differs from ${expectedVersion}`);
  }
  const sidebar = await page.locator(".sidebar-meta").innerText();
  if (!sidebar.includes(`Version ${expectedVersion}`) || sidebar.includes("dev")) {
    throw new Error(`sidebar versions are inconsistent: ${sidebar}`);
  }
} finally {
  await product.close();
}
