import { URL } from "node:url";

export const CORE_HANDSHAKE_SCHEMA_VERSION = 1;
export const DESKTOP_ACTIVE_STATES = new Set([
  "starting_game", "starting_run", "running_run", "paused_between_runs", "exiting_game", "cancelling",
]);
export const CORE_SAFE_INACTIVE_STATES = new Set(["idle", "idle_in_game", "stopped_error"]);

export type DesktopCoreReason =
  | "core_start_failed"
  | "core_handshake_timeout"
  | "core_handshake_invalid"
  | "core_exited"
  | "core_recovery_required"
  | "core_shutdown_failed";

export interface CoreHandshake {
  schema_version: typeof CORE_HANDSHAKE_SCHEMA_VERSION;
  core_pid: number;
  generation: number;
  base_url: string;
  bootstrap_url: string;
}

export class DesktopCoreError extends Error {
  constructor(readonly code: DesktopCoreReason, message: string) {
    super(message);
    this.name = "DesktopCoreError";
  }
}

export function parseCoreHandshake(raw: string, expectedPID: number): CoreHandshake {
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    throw new DesktopCoreError("core_handshake_invalid", "Der Core-Handshake ist kein gültiges JSON.");
  }
  if (!isRecord(value)) {
    throw new DesktopCoreError("core_handshake_invalid", "Der Core-Handshake muss ein Objekt sein.");
  }
  requireExactKeys(value, ["schema_version", "core_pid", "generation", "base_url", "bootstrap_url"]);
  if (value.schema_version !== CORE_HANDSHAKE_SCHEMA_VERSION || value.core_pid !== expectedPID || value.generation !== 1) {
    throw new DesktopCoreError("core_handshake_invalid", "Schema, PID oder Generation des Core-Handshakes stimmen nicht.");
  }
  if (typeof value.base_url !== "string" || typeof value.bootstrap_url !== "string") {
    throw new DesktopCoreError("core_handshake_invalid", "Core-URLs fehlen im Handshake.");
  }
  const base = safeCoreURL(value.base_url, false);
  const bootstrap = safeCoreURL(value.bootstrap_url, true);
  if (base.origin !== bootstrap.origin || base.pathname !== "/" || base.search || base.hash || bootstrap.pathname !== "/" || bootstrap.search) {
    throw new DesktopCoreError("core_handshake_invalid", "Core- und Bootstrap-URL haben nicht denselben engen Loopback-Origin.");
  }
  const token = new URLSearchParams(bootstrap.hash.slice(1)).get("control_token") ?? "";
  if (!/^[A-Za-z0-9_-]{43}$/.test(token)) {
    throw new DesktopCoreError("core_handshake_invalid", "Der einmalige Control-Token ist ungültig.");
  }
  return value as unknown as CoreHandshake;
}

export type CoreExitAction = "expected" | "restart_once" | "recovery_required";

export function decideCoreExit(input: {
  expectedShutdown: boolean;
  handshakeComplete: boolean;
  lastState?: string;
  routeWorkflowState?: string;
  restartCount: number;
}): CoreExitAction {
  if (input.expectedShutdown) return "expected";
  if (!input.handshakeComplete) return "recovery_required";
  const workflowIdle = input.routeWorkflowState === undefined || input.routeWorkflowState === "idle";
  if (input.restartCount === 0 && workflowIdle && input.lastState !== undefined && CORE_SAFE_INACTIVE_STATES.has(input.lastState)) {
    return "restart_once";
  }
  return "recovery_required";
}

export function secureWebPreferences(preload: string) {
  return Object.freeze({
    preload,
    nodeIntegration: false,
    contextIsolation: true,
    sandbox: true,
    webSecurity: true,
    allowRunningInsecureContent: false,
    webviewTag: false,
  });
}

export function isAllowedNavigation(target: string, coreOrigin: string): boolean {
  try {
    const url = new URL(target);
    return url.origin === coreOrigin && url.protocol === "http:" && url.hostname === "127.0.0.1";
  } catch {
    return false;
  }
}

export function isAllowedIPCSender(target: string, coreOrigin: string, ...localURLs: string[]): boolean {
  if (localURLs.includes(target)) return true;
  return isAllowedNavigation(target, coreOrigin);
}

function safeCoreURL(raw: string, bootstrap: boolean): URL {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new DesktopCoreError("core_handshake_invalid", "Der Core meldet eine ungültige URL.");
  }
  const port = Number(url.port);
  if (url.protocol !== "http:" || url.hostname !== "127.0.0.1" || url.username || url.password || !Number.isInteger(port) || port < 1 || port > 65535) {
    throw new DesktopCoreError("core_handshake_invalid", "Der Core darf nur einen zufälligen IPv4-Loopback-Port melden.");
  }
  if (!bootstrap && url.hash) {
    throw new DesktopCoreError("core_handshake_invalid", "Die sichere Core-URL darf keinen Token enthalten.");
  }
  return url;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function requireExactKeys(value: Record<string, unknown>, expected: readonly string[]): void {
  const keys = Object.keys(value).sort();
  const wanted = [...expected].sort();
  if (keys.length !== wanted.length || keys.some((key, index) => key !== wanted[index])) {
    throw new DesktopCoreError("core_handshake_invalid", "Der Core-Handshake enthält fehlende oder unbekannte Felder.");
  }
}
