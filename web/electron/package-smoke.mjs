import { _electron as electron } from "@playwright/test";
import { access } from "node:fs/promises";
import process from "node:process";

const [executablePath, dataRoot, expectedVersion, localAppData] = process.argv.slice(2);
if (!executablePath || !dataRoot || !expectedVersion || !localAppData) {
  throw new Error("package-smoke requires executable, data root, version and LocalAppData");
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
    await page.getByRole("heading", { name: "Datenbasis einrichten" }).waitFor({ timeout: 20_000 });
    await access(dataRoot).then(
      () => { throw new Error("packaged Electron created the data root before Core provisioning"); },
      (error) => {
        if (error?.code !== "ENOENT") throw error;
      },
    );
    await page.getByRole("button", { name: "Neuen Datenroot anlegen" }).click();
    await page.waitForSelector(".sidebar-meta", { timeout: 20_000 });
    await access(dataRoot);
  } catch (error) {
    const summary = await page.locator("body").innerText().catch(() => "");
    throw new Error(`packaged window did not reach the React shell: ${page.url()} :: ${summary.slice(0, 500)}`, { cause: error });
  }
  const bridgeVersion = await page.evaluate(async () => (await window.d2rDesktop.getAppInfo()).app_version);
  if (bridgeVersion !== expectedVersion) {
    throw new Error(`packaged Electron version ${bridgeVersion} differs from ${expectedVersion}`);
  }
  const sidebar = await page.locator(".sidebar-meta").innerText();
  if (!sidebar.includes(`App ${expectedVersion}`) || !sidebar.includes(`Core ${expectedVersion}`) || sidebar.includes("dev")) {
    throw new Error(`sidebar versions are inconsistent: ${sidebar}`);
  }
} finally {
  await product.close();
}
