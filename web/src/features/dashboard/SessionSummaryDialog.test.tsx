import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { changeAppLanguage } from "../../i18n";
import { SessionSummaryDialog, sessionSummaryFromTransition } from "./SessionSummaryDialog";

const mocks = vi.hoisted(() => ({ summary: vi.fn(), items: vi.fn() }));
vi.mock("../../api/generated", () => ({
  getHistorySummary: mocks.summary,
  getHistoryItems: mocks.items,
}));

const funnel = { seen: 5, matched: 5, picked_up: 5, stashed: 4, sold: 3, keep_return: 4, pickup_lost: 0, post_pickup_lost: 0 };
const summary = {
  summary: {
    runs: 2, terminal_runs: 2, successful: 2, failed: 0, aborted: 0, incomplete: 0, running: 0, boss_kills: 2,
    durations: { count: 2, total_ms: 120000, average_ms: 60000, median_ms: 60000, minimum_ms: 30000, maximum_ms: 90000 },
    stages: { travel_ms: 1, combat_ms: 1, loot_ms: 1, return_town_ms: 1, other_ms: 1 },
    funnel,
  },
};
const items = {
  items: [
    { item_key: "base:r15:normal", item_name: "Ko Rune", base_code: "r15", seen: 3, matched: 3, picked_up: 3, stashed: 3, sold: 0, pickup_lost: 0, post_pickup_lost: 0 },
    { item_key: "base:gpw:normal", item_name: "Flawless Diamond", base_code: "gpw", seen: 1, matched: 1, picked_up: 1, stashed: 1, sold: 0, pickup_lost: 0, post_pickup_lost: 0 },
    { item_key: "base:gld:normal", item_name: "Gold", base_code: "gld", seen: 3, matched: 3, picked_up: 3, stashed: 0, sold: 3, pickup_lost: 0, post_pickup_lost: 0 },
  ],
};

describe("sessionSummaryFromTransition", () => {
  it("öffnet nur beim Wechsel aus einer aktiven Session", () => {
    const idle = { state: "idle", last_result: { disposition: "stop", session_id: "session-a", duration_ms: 5000 } };
    expect(sessionSummaryFromTransition(undefined, idle)).toBeNull();
    expect(sessionSummaryFromTransition("idle", idle)).toBeNull();
    expect(sessionSummaryFromTransition("running_run", { state: "running_run", last_result: idle.last_result })).toBeNull();
    expect(sessionSummaryFromTransition("running_run", idle)).toEqual({ sessionID: "session-a", durationMs: 5000 });
    expect(sessionSummaryFromTransition("running_run", { state: "idle", last_result: { disposition: "stop" } })).toBeNull();
  });
});

describe("SessionSummaryDialog", () => {
  afterEach(cleanup);
  beforeEach(async () => {
    await changeAppLanguage("de");
    vi.clearAllMocks();
    mocks.summary.mockResolvedValue(summary);
    mocks.items.mockResolvedValue(items);
  });

  it("zeigt Dauer und klappt Itemlisten auf", async () => {
    render(<SessionSummaryDialog sessionID="session-a" durationMs={3_661_000} refreshKey={0} onClose={() => undefined} />);

    expect(await screen.findByText("Sitzungsdauer: 01:01:01")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Aufgehobene Items anzeigen" })).toHaveTextContent("4 aufgehobene Items");
    expect(screen.getByRole("button", { name: "Verkaufte Items anzeigen" })).toHaveTextContent("3 verkaufte Items");
    expect(screen.queryByText(/Ko-Rune|Ko Rune/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Aufgehobene Items anzeigen" }));
    expect(await screen.findByText("3 × Hel-Rune")).toBeInTheDocument();
    expect(screen.getByText("1 × perfekter Diamant")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Verkaufte Items anzeigen" }));
    expect(await screen.findByText("3 × Gold")).toBeInTheDocument();
  });

  it("lädt Core-Daten nur für die beendete Session", async () => {
    render(<SessionSummaryDialog sessionID="session-a" durationMs={1000} refreshKey={4} onClose={() => undefined} />);
    await screen.findByText("Sitzungsdauer: 00:00:01");
    expect(mocks.summary).toHaveBeenCalledWith({ session: ["session-a"] }, expect.any(AbortSignal));
    expect(mocks.items).toHaveBeenCalledWith({ session: ["session-a"], limit: 200 }, expect.any(AbortSignal));
  });

  it("zeigt die Itemzahlen, wenn Summary Limit als ungültigen Filter ablehnt", async () => {
    mocks.summary.mockImplementation((query: { limit?: number }) => {
      if (query.limit !== undefined) return Promise.reject(new Error("filter_invalid"));
      return Promise.resolve(summary);
    });
    render(<SessionSummaryDialog sessionID="session-a" durationMs={1000} refreshKey={0} onClose={() => undefined} />);
    expect(await screen.findByRole("button", { name: "Aufgehobene Items anzeigen" })).toHaveTextContent("4 aufgehobene Items");
    expect(screen.queryByText("Die Zusammenfassung konnte nicht geladen werden.")).not.toBeInTheDocument();
  });
});
