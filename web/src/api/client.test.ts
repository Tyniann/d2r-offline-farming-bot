import { afterEach, describe, expect, it, vi } from "vitest";
import { connectLiveEvents, consumeBootstrapToken, controlHeaders } from "./client";

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

describe("bootstrap token", () => {
  it("übernimmt das Fragment nur in Memory und entfernt es sofort", () => {
    const replaceState = vi.spyOn(history, "replaceState");
    const location = { hash: "#control_token=secret", pathname: "/", search: "" } as Location;
    expect(consumeBootstrapToken(location)).toBe("secret");
    expect(controlHeaders()).toMatchObject({ "X-D2RBot-Control-Token": "secret" });
    expect(replaceState).toHaveBeenCalledWith(null, "", "/");
    consumeBootstrapToken({ hash: "", pathname: "/", search: "" } as Location);
    expect(controlHeaders()).toMatchObject({ "X-D2RBot-Control-Token": "secret" });
  });
});

describe("control bootstrap", () => {
  it("erneuert den Memory-Token nach einem Browser-Refresh", async () => {
    vi.resetModules();
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ control_token: "restored" }) })
      .mockResolvedValueOnce({ ok: true });
    vi.stubGlobal("fetch", fetchMock);
    const client = await import("./client");
    await client.applySelection("MrBones", "nightmare", 1, 0, "confirmation");
    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/v1/control/bootstrap", {
      headers: { Accept: "application/json", "X-D2RBot-Bootstrap": "1" },
    });
    expect(fetchMock.mock.calls[1][1].headers).toMatchObject({ "X-D2RBot-Control-Token": "restored" });
    expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toMatchObject({ payload: { confirmation_token: "confirmation" } });
  });

  it("validiert die Queue read-only und sendet den identischen Kontext geschützt zum Start", async () => {
    vi.resetModules();
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ entries: ["countess", "mephisto"] }) })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ control_token: "queue-token" }) })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ state: "starting_run", generation: 4 }) });
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("crypto", { randomUUID: () => "queue-command" });
    const client = await import("./client");
    await client.validateQueue(["countess", "mephisto"], "MrBones", "nightmare", 7);
    await client.startQueue(["countess", "mephisto"], "MrBones", "nightmare", 7, 3);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/v1/queue/validate");
    expect(fetchMock.mock.calls[0][1].headers).not.toHaveProperty("X-D2RBot-Control-Token");
    expect(fetchMock.mock.calls[2][0]).toBe("/api/v1/session/start");
    expect(fetchMock.mock.calls[2][1].headers).toMatchObject({ "X-D2RBot-Control-Token": "queue-token" });
    expect(JSON.parse(fetchMock.mock.calls[2][1].body)).toEqual({
      command_id: "queue-command", expected_generation: 3,
      payload: { entries: ["countess", "mephisto"], character: "MrBones", difficulty: "nightmare", catalog_revision: 7 },
    });
  });
});

describe("live events", () => {
  it("verbindet den Stream und schließt ihn beim Unmount", () => {
    const listeners = new Map<string, EventListener>();
    const close = vi.fn();
    class FakeEventSource {
      onopen: (() => void) | null = null;
      onerror: (() => void) | null = null;
      addEventListener(name: string, listener: EventListener) { listeners.set(name, listener); }
      close = close;
    }
    vi.stubGlobal("EventSource", FakeEventSource);
    const states: string[] = [];
    const snapshots: unknown[] = [];
    const disconnect = connectLiveEvents(snapshots.push.bind(snapshots), vi.fn(), states.push.bind(states));
    listeners.get("snapshot")?.({ data: '{"state":"idle"}' } as unknown as Event);
    expect(states).toEqual(["wird verbunden"]);
    expect(snapshots).toEqual([{ state: "idle" }]);
    disconnect();
    expect(close).toHaveBeenCalledOnce();
  });
});
