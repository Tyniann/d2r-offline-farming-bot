// @vitest-environment node

import { describe, expect, it } from "vitest";
import { decideCoreExit, DesktopCoreError, isAllowedIPCSender, isAllowedNavigation, parseCoreHandshake, secureWebPreferences } from "./core-contract.js";

const valid = (pid = 42) => JSON.stringify({
  schema_version: 1,
  core_pid: pid,
  generation: 1,
  base_url: "http://127.0.0.1:43123",
  bootstrap_url: `http://127.0.0.1:43123/#control_token=${"a".repeat(43)}`,
});

describe("Core-Handshake", () => {
  it("akzeptiert nur den engen PID- und Loopbackvertrag", () => {
    expect(parseCoreHandshake(valid(), 42).base_url).toBe("http://127.0.0.1:43123");
    expect(() => parseCoreHandshake(valid(43), 42)).toThrow(DesktopCoreError);
    expect(() => parseCoreHandshake(valid().replace("127.0.0.1", "localhost"), 42)).toThrow(DesktopCoreError);
    expect(() => parseCoreHandshake(valid().replace('"generation":1', '"generation":1,"unknown":true'), 42)).toThrow(DesktopCoreError);
  });
});

describe("Desktop-Security", () => {
  it("friert Sandbox, Context Isolation und die Node-Grenze ein", () => {
    expect(secureWebPreferences("preload.js")).toMatchObject({ nodeIntegration: false, contextIsolation: true, sandbox: true, webSecurity: true, webviewTag: false });
  });

  it("erlaubt Navigation und IPC nur vom exakten Core-Origin", () => {
    expect(isAllowedNavigation("http://127.0.0.1:43123/#history", "http://127.0.0.1:43123")).toBe(true);
    expect(isAllowedNavigation("https://example.com", "http://127.0.0.1:43123")).toBe(false);
    expect(isAllowedIPCSender("http://127.0.0.1:43123/", "http://127.0.0.1:43123", "file:///recovery.html", "file:///ui/index.html")).toBe(true);
    expect(isAllowedIPCSender("file:///ui/index.html", "http://127.0.0.1:43123", "file:///recovery.html", "file:///ui/index.html")).toBe(true);
    expect(isAllowedIPCSender("file:///other.html", "http://127.0.0.1:43123", "file:///recovery.html", "file:///ui/index.html")).toBe(false);
  });
});

describe("Core-Exit", () => {
  it("startet ausschließlich nach sicher inaktivem Exit genau einmal neu", () => {
    expect(decideCoreExit({ expectedShutdown: false, handshakeComplete: true, lastState: "idle", routeWorkflowState: "idle", restartCount: 0 })).toBe("restart_once");
    expect(decideCoreExit({ expectedShutdown: false, handshakeComplete: true, lastState: "idle", routeWorkflowState: "idle", restartCount: 1 })).toBe("recovery_required");
    expect(decideCoreExit({ expectedShutdown: false, handshakeComplete: true, lastState: "running_run", routeWorkflowState: "idle", restartCount: 0 })).toBe("recovery_required");
    expect(decideCoreExit({ expectedShutdown: false, handshakeComplete: true, lastState: "idle", routeWorkflowState: "recording", restartCount: 0 })).toBe("recovery_required");
    expect(decideCoreExit({ expectedShutdown: true, handshakeComplete: false, restartCount: 0 })).toBe("expected");
  });
});
