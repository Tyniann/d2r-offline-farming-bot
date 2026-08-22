import type { ChildProcess } from "node:child_process";
import { randomUUID } from "node:crypto";
import { decideCoreExit, DesktopCoreError, type CoreHandshake, type DesktopCoreReason } from "./core-contract.js";
import { desktopLifecyclePolicy, type DesktopCoreSnapshot } from "./desktop-lifecycle.js";
import { launchCore, type CoreLaunchOptions } from "./core-process.js";

export interface CoreControllerCallbacks {
  onReady: (handshake: CoreHandshake) => void | Promise<void>;
  onRecoveryRequired: (reason: DesktopCoreReason, restartCount: number) => void | Promise<void>;
  onRestarting?: (restartCount: number) => void;
  onStatusChanged?: (previous: DesktopCoreSnapshot | undefined, current: DesktopCoreSnapshot) => void;
}

export type DesktopSessionIntent = "pause_after_run" | "stop_after_run" | "emergency_stop";

export class DesktopCoreController {
  readonly #options: CoreLaunchOptions;
  readonly #callbacks: CoreControllerCallbacks;
  #child?: ChildProcess;
  #handshake?: CoreHandshake;
  #pollTimer?: ReturnType<typeof setInterval>;
  #expectedShutdown = false;
  #restartCount = 0;
  #lastState?: string;
  #lastGeneration?: number;
  #pendingIntent?: string;
  #routeWorkflowState?: string;
  #privilegeMismatch = false;
  #commandPending = false;
  #publishedStatus?: DesktopCoreSnapshot;

  constructor(options: CoreLaunchOptions, callbacks: CoreControllerCallbacks) {
    this.#options = options;
    this.#callbacks = callbacks;
  }

  get restartCount(): number { return this.#restartCount; }
  get lastState(): string | undefined { return this.#lastState; }
  get connected(): boolean { return this.#handshake !== undefined && this.#child !== undefined; }
  get privilegeMismatch(): boolean { return this.#privilegeMismatch; }
  get statusSnapshot(): DesktopCoreSnapshot { return { connected: this.connected, state: this.#lastState, pendingIntent: this.#pendingIntent, generation: this.#lastGeneration }; }

  async start(): Promise<void> {
    this.#expectedShutdown = false;
    await this.#launch();
  }

  async shutdown(timeoutMs = 3_000): Promise<void> {
    this.#expectedShutdown = true;
    this.#stopPolling();
    const child = this.#child;
    this.#child = undefined;
    this.#handshake = undefined;
    this.#publishStatus();
    if (!child || child.exitCode !== null) return;
    const exited = new Promise<void>((resolve) => child.once("exit", () => resolve()));
    if (!child.kill()) {
      throw new DesktopCoreError("core_shutdown_failed", "Der Core konnte nicht zum Beenden aufgefordert werden.");
    }
    const completed = await Promise.race([exited.then(() => true), new Promise<false>((resolve) => setTimeout(() => resolve(false), timeoutMs))]);
    if (!completed) {
      child.kill("SIGKILL");
      throw new DesktopCoreError("core_shutdown_failed", "Der Core-Shutdown hat das Zeitlimit überschritten.");
    }
  }

  async restart(): Promise<void> {
    const policy = desktopLifecyclePolicy(this.statusSnapshot);
    if (!this.connected || !policy.canQuit) {
      throw new DesktopCoreError("core_shutdown_failed", "Der Core kann nur im sicheren inaktiven Zustand neu gestartet werden.");
    }
    await this.shutdown();
    await this.start();
  }

  async sendSessionIntent(intent: DesktopSessionIntent): Promise<void> {
    const policy = desktopLifecyclePolicy(this.statusSnapshot);
    const allowed = intent === "pause_after_run" ? policy.canPauseAfterRun : intent === "stop_after_run" ? policy.canStopAfterRun : policy.canEmergencyStop;
    if (!allowed) throw new Error("Dieser Session-Befehl ist im aktuellen Corezustand gesperrt.");
    if (this.#commandPending) throw new Error("Ein Desktop-Session-Befehl wird bereits bestätigt.");
    const handshake = this.#handshake;
    const generation = this.#lastGeneration;
    if (!handshake || generation === undefined) throw new Error("Der Core ist nicht steuerbar verbunden.");
    const path = intent === "pause_after_run" ? "pause-after-run" : intent === "stop_after_run" ? "stop-after-run" : "emergency-stop";
    const token = new URLSearchParams(new URL(handshake.bootstrap_url).hash.slice(1)).get("control_token") ?? "";
    this.#commandPending = true;
    try {
      const response = await fetch(`${handshake.base_url}/api/v1/session/${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-D2RBot-Control-Token": token },
        body: JSON.stringify({ command_id: randomUUID(), expected_generation: generation }),
        signal: AbortSignal.timeout(2_000),
      });
      if (!response.ok) {
        const body = await response.json().catch(() => null) as { message?: unknown } | null;
        throw new Error(typeof body?.message === "string" ? body.message : `Core-Befehl fehlgeschlagen (${response.status}).`);
      }
      const result = await response.json() as { state?: unknown; generation?: unknown };
      if (typeof result.state === "string") this.#lastState = result.state;
      if (typeof result.generation === "number") this.#lastGeneration = result.generation;
      this.#pendingIntent = intent === "emergency_stop" ? undefined : intent;
      this.#publishStatus();
    } finally {
      this.#commandPending = false;
    }
  }

  async #launch(): Promise<void> {
    try {
      const connection = await launchCore(this.#options);
      this.#child = connection.child;
      this.#handshake = connection.handshake;
      this.#lastGeneration = connection.handshake.generation;
      this.#child.once("exit", () => void this.#handleExit());
      await this.#pollStatus();
      this.#pollTimer = setInterval(() => void this.#pollStatus(), 500);
      await this.#callbacks.onReady(connection.handshake);
    } catch (error) {
      const reason = error instanceof DesktopCoreError ? error.code : "core_start_failed";
      // Rohe Prozessfehler und begrenztes stderr bleiben reine Diagnose. Die
      // Recovery erhält nur den stabilen Reason-Code und löst ihren Bedienertext selbst auf.
      console.error("Core launch failed.", error);
      await this.#callbacks.onRecoveryRequired(reason, this.#restartCount);
    }
  }

  async #pollStatus(): Promise<void> {
    const handshake = this.#handshake;
    if (!handshake) return;
    try {
      const [statusResponse, workflowResponse] = await Promise.all([
        fetch(`${handshake.base_url}/api/v1/status`, { signal: AbortSignal.timeout(400) }),
        fetch(`${handshake.base_url}/api/v1/routes/workflow`, { signal: AbortSignal.timeout(400) }),
      ]);
      if (statusResponse.ok) {
        const status = await statusResponse.json() as { state?: unknown; generation?: unknown; pending_intent?: unknown; compatibility?: { privilege_mismatch?: unknown } };
        if (typeof status.state === "string") this.#lastState = status.state;
        if (typeof status.generation === "number") this.#lastGeneration = status.generation;
        this.#pendingIntent = typeof status.pending_intent === "string" && status.pending_intent !== "none" && status.pending_intent !== "" ? status.pending_intent : undefined;
        this.#privilegeMismatch = status.compatibility?.privilege_mismatch === true;
      }
      this.#publishStatus();
      if (workflowResponse.ok) {
        const workflow = await workflowResponse.json() as { state?: unknown };
        if (typeof workflow.state === "string") this.#routeWorkflowState = workflow.state;
      }
    } catch {
      // Ein einzelner Pollfehler entscheidet nicht über Recovery; der echte
      // Child-Exit und der letzte autoritative Snapshot bleiben maßgeblich.
    }
  }

  async #handleExit(): Promise<void> {
    this.#stopPolling();
    const action = decideCoreExit({
      expectedShutdown: this.#expectedShutdown,
      handshakeComplete: this.#handshake !== undefined,
      lastState: this.#lastState,
      routeWorkflowState: this.#routeWorkflowState,
      restartCount: this.#restartCount,
    });
    this.#child = undefined;
    this.#handshake = undefined;
    this.#publishStatus();
    if (action === "expected") return;
    if (action === "restart_once") {
      this.#restartCount++;
      this.#callbacks.onRestarting?.(this.#restartCount);
      await this.#launch();
      return;
    }
    await this.#callbacks.onRecoveryRequired("core_recovery_required", this.#restartCount);
  }

  #stopPolling(): void {
    if (this.#pollTimer !== undefined) clearInterval(this.#pollTimer);
    this.#pollTimer = undefined;
  }

  #publishStatus(): void {
    const current = this.statusSnapshot;
    const previous = this.#publishedStatus;
    if (previous && previous.connected === current.connected && previous.state === current.state && previous.pendingIntent === current.pendingIntent && previous.generation === current.generation) return;
    this.#publishedStatus = { ...current };
    this.#callbacks.onStatusChanged?.(previous ? { ...previous } : undefined, { ...current });
  }
}
