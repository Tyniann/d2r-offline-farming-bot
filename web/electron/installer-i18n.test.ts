// @vitest-environment node

import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const webRoot = join(dirname(fileURLToPath(import.meta.url)), "..");

describe("NSIS-Internationalisierung", () => {
  it("baut genau einen Installer mit deutscher und englischer Sprachauswahl ohne erzwungene LCID", async () => {
    const packageJSON = JSON.parse(await readFile(join(webRoot, "package.json"), "utf8")) as {
      build: { nsis: { installerLanguages: string[]; displayLanguageSelector?: boolean; language?: string; artifactName?: string } };
    };
    expect(packageJSON.build.nsis.installerLanguages).toEqual(["de_DE", "en_US"]);
    expect(packageJSON.build.nsis.displayLanguageSelector).toBe(true);
    expect(packageJSON.build.nsis).not.toHaveProperty("language");
  });

  it("hält beide Löschwarnungen in Deutsch und Englisch gleich streng", async () => {
    const script = await readFile(join(webRoot, "build", "installer.nsh"), "utf8");
    expect(script).toContain("LangString uninstallDeleteDataPrompt 1031");
    expect(script).toContain("LangString uninstallDeleteDataPrompt 1033");
    expect(script).toContain("LangString uninstallDeleteDataConfirm 1031");
    expect(script).toContain("LangString uninstallDeleteDataConfirm 1033");
    expect(script).toContain('"$(uninstallDeleteDataPrompt)"');
    expect(script).toContain('"$(uninstallDeleteDataConfirm)"');
    expect(script.match(/MB_DEFBUTTON2/g)).toHaveLength(2);
    expect(script).toContain("IfSilent preserve_phase15_data");
  });
});
