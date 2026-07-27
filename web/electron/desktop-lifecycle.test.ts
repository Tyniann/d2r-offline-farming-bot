// @vitest-environment node

import { describe, expect, it } from "vitest";
import { desktopLifecyclePolicy, desktopNotificationSpec, notificationForTransition, shouldShowDesktopNotification, type DesktopCoreSnapshot, type DesktopLifecycleState } from "./desktop-lifecycle.js";

describe("Desktop-State-Matrix", () => {
  it.each([
    ["idle", { connected: true, state: "idle", generation: 1 }, "confirm_exit", false, false, false, true],
    ["running", { connected: true, state: "running_run", generation: 2 }, "hide", true, true, true, false],
    ["pause_pending", { connected: true, state: "running_run", pendingIntent: "pause_after_run", generation: 3 }, "hide", false, false, true, false],
    ["paused", { connected: true, state: "paused_between_runs", generation: 4 }, "hide", false, false, true, false],
    ["stop_pending", { connected: true, state: "running_run", pendingIntent: "stop_after_run", generation: 5 }, "hide", false, false, true, false],
    ["cancelling", { connected: true, state: "cancelling", generation: 6 }, "hide", false, false, false, false],
    ["error", { connected: true, state: "stopped_error", generation: 7 }, "confirm_exit", false, false, false, true],
    ["core_down", { connected: false }, "confirm_exit", false, false, false, true],
  ] as const)("projiziert %s exakt", (expectedState, snapshot, closeAction, pause, stop, emergency, quit) => {
    const policy = desktopLifecyclePolicy(snapshot);
    expect(policy).toMatchObject({ state: expectedState, closeAction, canPauseAfterRun: pause, canStopAfterRun: stop, canEmergencyStop: emergency, canQuit: quit });
  });

  it("behandelt einen unbekannten verbundenen Zustand fail-closed", () => {
    expect(desktopLifecyclePolicy({ connected: true, state: "future_state" })).toMatchObject({ state: "cancelling", canQuit: false, closeAction: "hide" });
  });
});

describe("Desktop-Benachrichtigungen", () => {
  const snapshot = (state: string, pendingIntent?: string): DesktopCoreSnapshot => ({ connected: true, state, pendingIntent, generation: 1 });

  it.each([
    [snapshot("running_run"), snapshot("idle"), "session_completed", "history"],
    [snapshot("running_run"), snapshot("stopped_error"), "terminal_error", "dashboard"],
    [snapshot("running_run", "pause_after_run"), snapshot("paused_between_runs"), "pause_reached", "dashboard"],
  ] as const)("erkennt nur den fachlichen Übergang %s → %s", (before, after, kind, target) => {
    expect(notificationForTransition(before, after)).toBe(kind);
    expect(desktopNotificationSpec(kind).target).toBe(target);
  });

  it("projiziert den vierten erlaubten Updatetyp auf Settings", () => {
    expect(desktopNotificationSpec("update_available")).toMatchObject({ target: "settings", title: "Neue Version verfügbar" });
  });

  it("erlaubt native Benachrichtigungen ausschließlich bei unfokussiertem Fenster", () => {
    expect(shouldShowDesktopNotification(false, true)).toBe(true);
    expect(shouldShowDesktopNotification(true, true)).toBe(false);
    expect(shouldShowDesktopNotification(false, false)).toBe(false);
  });

  it("meldet einen Emergency-/Cancelling-Abschluss nicht als erfolgreiche Session", () => {
    expect(notificationForTransition(snapshot("cancelling"), snapshot("idle"))).toBeUndefined();
    expect(notificationForTransition(snapshot("paused_between_runs"), snapshot("idle"))).toBeUndefined();
  });

  it.each(["idle", "running", "pause_pending", "paused", "stop_pending", "cancelling", "error", "core_down"] satisfies DesktopLifecycleState[])("erzeugt ohne Übergang in %s keine Benachrichtigung", (state) => {
    const snapshots: Record<DesktopLifecycleState, DesktopCoreSnapshot> = {
      idle: snapshot("idle"), running: snapshot("running_run"), pause_pending: snapshot("running_run", "pause_after_run"), paused: snapshot("paused_between_runs"), stop_pending: snapshot("running_run", "stop_after_run"), cancelling: snapshot("cancelling"), error: snapshot("stopped_error"), core_down: { connected: false },
    };
    expect(notificationForTransition(snapshots[state], snapshots[state])).toBeUndefined();
  });
});
