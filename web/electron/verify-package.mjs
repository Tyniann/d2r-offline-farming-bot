import { extractFile, listPackage } from "@electron/asar";
import process from "node:process";

const [archive, expectedVersion] = process.argv.slice(2);
if (!archive || !expectedVersion) throw new Error("verify-package requires app.asar and expected version");

const entries = listPackage(archive).map((entry) => entry.replaceAll("\\", "/"));
const forbidden = entries.find((entry) =>
  entry.includes("/node_modules/") ||
  /(^|\/)(\.git|\.env|logs|diagnostics|test-results|playwright-report)(\/|$)/i.test(entry) ||
  /(vitest|playwright|typescript|electron-builder)/i.test(entry),
);
if (forbidden) throw new Error(`app.asar contains forbidden development/workspace content: ${forbidden}`);

for (const required of [
  "/dist-electron/locales/de.json",
  "/dist-electron/locales/en.json",
  "/dist-electron/recovery.html",
  "/dist-electron/recovery.js",
  "/dist-electron/ui/index.html",
]) {
  if (!entries.includes(required)) throw new Error(`app.asar is missing required content: ${required}`);
}

const metadata = JSON.parse(extractFile(archive, "package.json").toString("utf8"));
if (metadata.version !== expectedVersion) {
  throw new Error(`app.asar version ${metadata.version} differs from ${expectedVersion}`);
}
