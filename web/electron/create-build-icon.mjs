import { copyFile, mkdir } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

// Windows consumes one ICO with several resolutions of the same Town Portal
// artwork. Tray and notifications scale the PNG at runtime; this step only
// stages the prebuilt ICO for electron-builder.
const webRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const source = join(webRoot, "electron", "app-icon.ico");
const target = join(webRoot, "build", "icon.ico");
await mkdir(dirname(target), { recursive: true });
await copyFile(source, target);
