import { spawn, type ChildProcess } from "node:child_process";
import { randomUUID } from "node:crypto";
import { createServer } from "node:net";
import { once } from "node:events";
import { DesktopCoreError, parseCoreHandshake, type CoreHandshake } from "./core-contract.js";

const MAX_HANDSHAKE_BYTES = 16 * 1024;

export interface CoreLaunchOptions {
  executable: string;
  executableArgs?: string[];
  dataRoot: string;
  handshakeTimeoutMs: number;
  environment?: NodeJS.ProcessEnv;
}

export interface CoreConnection {
  child: ChildProcess;
  handshake: CoreHandshake;
}

export interface CoreProvisioningRequest {
  mode: "new" | "import";
  defaultsRoot?: string;
  importRoot?: string;
}

export interface CoreProvisioningResult {
  schema_version: 1;
  status: "published" | "existing";
  diagnostic_count: number;
}

// provisionDataRoot startet nur den kurzlebigen Go-Provisionierungsmodus. Er
// öffnet weder API noch Hotkeys, Runtime, D2R-Prozessbindung oder Input.
export async function provisionDataRoot(options: CoreLaunchOptions, request: CoreProvisioningRequest): Promise<CoreProvisioningResult> {
  if (request.mode === "new" ? !request.defaultsRoot || request.importRoot !== undefined : !request.importRoot || request.defaultsRoot !== undefined) {
    throw new DesktopCoreError("core_start_failed", "Die Provisionierungsquelle ist unvollständig.");
  }
  const sourceArgs = request.mode === "new"
    ? ["--defaults-root", request.defaultsRoot!]
    : ["--import-root", request.importRoot!];
  const child = spawn(options.executable, [
    ...(options.executableArgs ?? []),
    "--provision-data-root", "--data-root", options.dataRoot, ...sourceArgs,
  ], {
    windowsHide: true,
    stdio: ["ignore", "pipe", "pipe"],
    env: options.environment,
  });
  let stdout = "";
  let stderr = "";
  child.stdout?.setEncoding("utf8");
  child.stderr?.setEncoding("utf8");
  child.stdout?.on("data", (chunk: string) => {
    stdout += chunk;
    if (Buffer.byteLength(stdout, "utf8") > MAX_HANDSHAKE_BYTES) child.kill();
  });
  child.stderr?.on("data", (chunk: string) => {
    stderr += chunk;
    if (Buffer.byteLength(stderr, "utf8") > MAX_HANDSHAKE_BYTES) child.kill();
  });
  const exit = await new Promise<{ code: number | null; signal: NodeJS.Signals | null }>((resolveExit, rejectExit) => {
    child.once("error", (error) => rejectExit(new DesktopCoreError("core_start_failed", `Der Provisionierungsprozess konnte nicht gestartet werden: ${error.message}`)));
    child.once("exit", (code, signal) => resolveExit({ code, signal }));
  });
  if (exit.code !== 0 || Buffer.byteLength(stdout, "utf8") > MAX_HANDSHAKE_BYTES || Buffer.byteLength(stderr, "utf8") > MAX_HANDSHAKE_BYTES) {
    const detail = stderr.trim().replace(/^error:\s*/i, "");
    throw new DesktopCoreError(
      "core_start_failed",
      detail
        ? `Der Go-Core konnte den Datenroot nicht vollständig provisionieren: ${detail}`
        : "Der Go-Core konnte den Datenroot nicht vollständig provisionieren.",
    );
  }
  return parseProvisioningResult(stdout);
}

export async function launchCore(options: CoreLaunchOptions): Promise<CoreConnection> {
  const pipeName = process.platform === "win32"
    ? `\\\\.\\pipe\\d2rbot-desktop-${randomUUID()}`
    : `/tmp/d2rbot-desktop-${randomUUID()}.sock`;

  let settleHandshake: ((value: string) => void) | undefined;
  let rejectHandshake: ((error: Error) => void) | undefined;
  const handshakeBody = new Promise<string>((resolve, reject) => {
    settleHandshake = resolve;
    rejectHandshake = reject;
  });
  const server = createServer((socket) => {
    let body = "";
    socket.setEncoding("utf8");
    socket.on("data", (chunk: string) => {
      body += chunk;
      if (Buffer.byteLength(body, "utf8") > MAX_HANDSHAKE_BYTES) {
        socket.destroy();
        rejectHandshake?.(new DesktopCoreError("core_handshake_invalid", "Der Core-Handshake ist zu groß."));
        return;
      }
      const newline = body.indexOf("\n");
      if (newline >= 0) {
        const trailing = body.slice(newline + 1).trim();
        socket.end();
        if (trailing) rejectHandshake?.(new DesktopCoreError("core_handshake_invalid", "Der Core sendete mehr als einen Handshake."));
        else settleHandshake?.(body.slice(0, newline));
      }
    });
    socket.on("end", () => {
      if (!body.includes("\n")) rejectHandshake?.(new DesktopCoreError("core_handshake_invalid", "Der Core brach den Handshake ab."));
    });
    socket.on("error", (error) => rejectHandshake?.(new DesktopCoreError("core_handshake_invalid", `Handshake-Pipe: ${error.message}`)));
  });
  server.maxConnections = 1;
  server.listen(pipeName);
  await once(server, "listening");

  const child = spawn(options.executable, [
    ...(options.executableArgs ?? []),
    "--data-root", options.dataRoot, "--desktop-handshake-pipe", pipeName,
  ], {
    windowsHide: true,
    stdio: ["ignore", "ignore", "pipe"],
    env: options.environment,
  });
  child.stderr?.resume();

  const timeout = setTimeout(() => {
    rejectHandshake?.(new DesktopCoreError("core_handshake_timeout", "Der Core antwortete nicht rechtzeitig."));
  }, options.handshakeTimeoutMs);
  const earlyExit = new Promise<never>((_, reject) => child.once("exit", (code, signal) => {
    reject(new DesktopCoreError("core_start_failed", `Der Core endete vor dem Handshake (${code ?? signal ?? "unbekannt"}).`));
  }));
  const spawnError = new Promise<never>((_, reject) => child.once("error", (error) => {
    reject(new DesktopCoreError("core_start_failed", `Der Core konnte nicht gestartet werden: ${error.message}`));
  }));

  try {
    const raw = await Promise.race([handshakeBody, earlyExit, spawnError]);
    const handshake = parseCoreHandshake(raw, child.pid ?? 0);
    return { child, handshake };
  } catch (error) {
    if (!child.killed) child.kill();
    throw error;
  } finally {
    clearTimeout(timeout);
    server.close();
  }
}

function parseProvisioningResult(raw: string): CoreProvisioningResult {
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    throw new DesktopCoreError("core_start_failed", "Der Provisionierungsprozess lieferte kein gültiges Ergebnis.");
  }
  if (!isRecord(value)) {
    throw new DesktopCoreError("core_start_failed", "Das Provisionierungsergebnis muss ein Objekt sein.");
  }
  const keys = Object.keys(value).sort().join(",");
  if (keys !== "diagnostic_count,schema_version,status" || value.schema_version !== 1 || !["published", "existing"].includes(String(value.status)) || !Number.isSafeInteger(value.diagnostic_count) || Number(value.diagnostic_count) < 0) {
    throw new DesktopCoreError("core_start_failed", "Das Provisionierungsergebnis verletzt den Desktopvertrag.");
  }
  return value as unknown as CoreProvisioningResult;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
