// @vitest-environment node

import { fileURLToPath } from "node:url";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { DesktopCoreError } from "./core-contract.js";
import { launchCore, provisionDataRoot } from "./core-process.js";

const fixture = fileURLToPath(new URL("./test-fixtures/fake-core.mjs", import.meta.url));

describe("private Core-Pipe", () => {
  it("übernimmt einen validen Handshake genau einmal", async () => {
    const connection = await launchCore({ executable: process.execPath, executableArgs: [fixture, "--mode=valid"], dataRoot: process.cwd(), handshakeTimeoutMs: 2_000, environment: process.env });
    expect(connection.handshake.core_pid).toBe(connection.child.pid);
    connection.child.kill();
  });

  it("reicht die Core-Ursache bei einem Start vor dem Handshake weiter", async () => {
    await expect(launchCore({
      executable: process.execPath,
      executableArgs: [fixture, "--mode=start-fail"],
      dataRoot: process.cwd(),
      handshakeTimeoutMs: 2_000,
      environment: process.env,
    })).rejects.toThrow("pickit_assignment_missing");
  });

  it.each([
    ["delayed", 100, "core_handshake_timeout"],
    ["wrong", 2_000, "core_handshake_invalid"],
    ["aborted", 2_000, "core_handshake_invalid"],
  ])("lehnt %s fail-closed ab", async (mode, timeout, code) => {
    try {
      await launchCore({ executable: process.execPath, executableArgs: [fixture, `--mode=${mode}`], dataRoot: process.cwd(), handshakeTimeoutMs: timeout, environment: process.env });
      throw new Error("Handshake wurde unerwartet akzeptiert.");
    } catch (error) {
      expect(error).toBeInstanceOf(DesktopCoreError);
      expect((error as DesktopCoreError).code).toBe(code);
    }
  });
});

describe("Core-Provisionierung", () => {
  it("führt Neu und Import als kurzlebigen Prozess ohne Handshake aus", async () => {
    const root = await mkdtemp(join(tmpdir(), "d2r-provision-"));
    const options = { executable: process.execPath, executableArgs: [fixture], dataRoot: join(root, "target"), handshakeTimeoutMs: 500 };
    await expect(provisionDataRoot(options, { mode: "new", defaultsRoot: join(root, "defaults") })).resolves.toEqual({
      schema_version: 1, status: "published", diagnostic_count: 0,
    });
    await expect(provisionDataRoot(options, { mode: "import", importRoot: join(root, "source") })).resolves.toEqual({
      schema_version: 1, status: "existing", diagnostic_count: 1,
    });
    await expect(provisionDataRoot(options, { mode: "new" })).rejects.toThrow("unvollständig");
  });

  it("reicht die begrenzte Core-Ursache bei einer fehlgeschlagenen Provisionierung weiter", async () => {
    const root = await mkdtemp(join(tmpdir(), "d2r-provision-error-"));
    const options = { executable: process.execPath, executableArgs: [fixture, "--mode=provision-fail"], dataRoot: join(root, "target"), handshakeTimeoutMs: 500 };
    await expect(provisionDataRoot(options, { mode: "new", defaultsRoot: join(root, "defaults") }))
      .rejects.toThrow("data_import_conflict: Testziel ist nicht leer");
  });
});
