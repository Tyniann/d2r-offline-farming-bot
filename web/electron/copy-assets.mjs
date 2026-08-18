import { copyFile, cp, mkdir, rm } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const source = join(root, "electron");
const target = join(root, "dist-electron");
await mkdir(target, { recursive: true });
// Vite content hashes change on every relevant renderer build. Remove the old
// projection first so an installer never carries unreachable historical UI
// bundles alongside the index.html-selected build.
await rm(join(target, "ui"), { recursive: true, force: true });
await Promise.all([
  ...["recovery.html", "recovery.css"].map((name) => copyFile(join(source, name), join(target, name))),
  copyFile(join(root, "public", "portal-mark.png"), join(target, "portal-mark.png")),
  cp(join(root, "..", "internal", "api", "ui", "dist"), join(target, "ui"), { recursive: true, force: true }),
]);
