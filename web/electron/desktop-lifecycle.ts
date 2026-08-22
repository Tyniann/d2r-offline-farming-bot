export type DesktopLifecycleState = "idle" | "running" | "pause_pending" | "paused" | "stop_pending" | "cancelling" | "error" | "core_down";
export type DesktopNotificationKind = "session_completed" | "terminal_error" | "pause_reached" | "update_available";
export type StableAppTarget = "dashboard" | "history" | "settings";

export interface DesktopCoreSnapshot {
  connected: boolean;
  state?: string;
  pendingIntent?: string;
  generation?: number;
}

export interface DesktopLifecyclePolicy {
  state: DesktopLifecycleState;
  closeAction: "hide" | "confirm_exit";
  canPauseAfterRun: boolean;
  canStopAfterRun: boolean;
  canEmergencyStop: boolean;
  canQuit: boolean;
}

const runningStates = new Set(["starting_game", "starting_run", "running_run", "exiting_game"]);

export function desktopLifecycleState(snapshot: DesktopCoreSnapshot): DesktopLifecycleState {
  if (!snapshot.connected) return "core_down";
  if (snapshot.state === "stopped_error") return "error";
  if (snapshot.state === "cancelling") return "cancelling";
  if (snapshot.state === "paused_between_runs") return "paused";
  if (runningStates.has(snapshot.state ?? "")) {
    if (snapshot.pendingIntent === "pause_after_run") return "pause_pending";
    if (snapshot.pendingIntent === "stop_after_run") return "stop_pending";
    return "running";
  }
  if (snapshot.state === "idle" || snapshot.state === "idle_in_game") return "idle";
  // Ein verbundener, aber unbekannter Corezustand darf nie als sicher inaktiv
  // behandelt werden. Die restriktive Cancelling-Policy verhindert Blind-Quit.
  return "cancelling";
}

export function desktopLifecyclePolicy(snapshot: DesktopCoreSnapshot): DesktopLifecyclePolicy {
  const state = desktopLifecycleState(snapshot);
  const active = state === "running" || state === "pause_pending" || state === "paused" || state === "stop_pending" || state === "cancelling";
  return {
    state,
    closeAction: active ? "hide" : "confirm_exit",
    canPauseAfterRun: state === "running" && snapshot.state === "running_run",
    canStopAfterRun: state === "running" && snapshot.state === "running_run",
    canEmergencyStop: state === "running" || state === "pause_pending" || state === "paused" || state === "stop_pending",
    canQuit: state === "idle" || state === "error" || state === "core_down",
  };
}

export function notificationForTransition(previous: DesktopCoreSnapshot | undefined, current: DesktopCoreSnapshot): DesktopNotificationKind | undefined {
  if (!previous) return undefined;
  const before = desktopLifecycleState(previous);
  const after = desktopLifecycleState(current);
  if (after === "error" && before !== "error") return "terminal_error";
  if (after === "paused" && before !== "paused") return "pause_reached";
  if (["running", "pause_pending", "stop_pending"].includes(before) && after === "idle") return "session_completed";
  return undefined;
}

export function desktopNotificationTarget(kind: DesktopNotificationKind): StableAppTarget {
  switch (kind) {
    case "session_completed": return "history";
    case "terminal_error":
    case "pause_reached": return "dashboard";
    case "update_available": return "settings";
  }
}

export function shouldShowDesktopNotification(windowFocused: boolean, nativeSupported: boolean): boolean {
  return !windowFocused && nativeSupported;
}
