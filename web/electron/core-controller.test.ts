// @vitest-environment node

import { fileURLToPath } from "node:url";
import { expect, it } from "vitest";
import { DesktopCoreController } from "./core-controller.js";
import type { CoreHandshake } from "./core-contract.js";

const fixture = fileURLToPath(new URL("./test-fixtures/fake-core.mjs", import.meta.url));

it.each([
  ["pause_after_run", "pause_after_run", "running_run"],
  ["stop_after_run", "stop_after_run", "running_run"],
  ["emergency_stop", "none", "cancelling"],
] as const)("bindet %s an die Coregeneration und unterdrückt Doppelklicke", async (intent, pendingIntent, expectedState) => {
  let handshake: CoreHandshake | undefined;
  let controller!: DesktopCoreController;
  const ready = new Promise<void>((resolve) => {
    controller = new DesktopCoreController({ executable: process.execPath, executableArgs: [fixture, "--mode=active-command"], dataRoot: process.cwd(), handshakeTimeoutMs: 2_000, environment: process.env }, {
      onReady: (value) => { handshake = value; resolve(); },
      onRecoveryRequired: (reason) => { throw new Error(reason); },
    });
  });
  await controller.start();
  await ready;
  try {
    const first = controller.sendSessionIntent(intent);
    await expect(controller.sendSessionIntent(intent)).rejects.toThrow("bereits bestätigt");
    await first;
    const result = await fetch(`${handshake!.base_url}/api/test/commands`).then((response) => response.json()) as { count: number; generation: number; pending_intent: string };
    expect(result).toMatchObject({ count: 1, generation: 2, pending_intent: pendingIntent, state: expectedState });
    expect(controller.statusSnapshot).toMatchObject({ generation: 2, state: expectedState, pendingIntent: intent === "emergency_stop" ? undefined : intent });
  } finally {
    // Der Test-Command lässt den Fake-Core aktiv; der direkte Shutdown ist hier
    // ausschließlich Testfixture-Cleanup, nicht die Desktop-Quit-Policy.
    await controller.shutdown();
  }
});
