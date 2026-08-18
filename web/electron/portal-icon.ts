import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const moduleDirectory = dirname(fileURLToPath(import.meta.url));

// Packed Electron copies `portal-mark.png` next to this module. Tests and the
// icon builder still read the source file from `web/public`.
const packedMark = join(moduleDirectory, "portal-mark.png");
const sourceMark = join(moduleDirectory, "..", "public", "portal-mark.png");

/** portalMarkPath returns the Town-Portal PNG used as the app mark. */
export function portalMarkPath(): string {
  if (existsSync(packedMark)) return packedMark;
  return sourceMark;
}

/** readPortalMarkPNG reads the original Town-Portal app mark as a PNG buffer. */
export function readPortalMarkPNG(): Buffer {
  return readFileSync(portalMarkPath());
}
