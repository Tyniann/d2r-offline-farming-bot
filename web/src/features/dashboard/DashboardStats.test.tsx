import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DashboardStats, dashboardHistoryQueries } from "./DashboardStats";

const mocks = vi.hoisted(() => ({ summary: vi.fn(), comparisons: vi.fn(), runs: vi.fn() }));
vi.mock("../../api/generated", () => ({
  getHistorySummary: mocks.summary,
  getHistoryComparisons: mocks.comparisons,
  getHistoryRuns: mocks.runs,
}));

const durations = { count: 5, total_ms: 390000, average_ms: 78000, median_ms: 76000, minimum_ms: 60000, maximum_ms: 100000 };
const stages = { travel_ms: 1, combat_ms: 1, loot_ms: 1, return_town_ms: 1, other_ms: 1 };
const funnel = { seen: 12, matched: 8, picked_up: 6, stashed: 4, sold: 0, keep_return: 4, pickup_lost: 2, post_pickup_lost: 0 };
const summary = { runs: 5, terminal_runs: 5, successful: 4, failed: 1, aborted: 0, incomplete: 0, running: 0, success_rate: .8, boss_kills: 4, durations, stages, funnel, keep_per_hour: 2.7 };
const comparison = { id: "countess-a", character: "MrBones", difficulty: "nightmare", definition_id: "countess", run: "countess", route_id: "route-a", terminal_runs: 5, successful: 4, failed: 1, aborted: 0, success_rate: .8, boss_kills: 4, low_sample: true, durations, stages, funnel, keep_per_hour: 2.7 };
const recent = { run_id: "run-1", started_at: "2026-08-22T10:00:00Z", observed_at: "2026-08-22T10:01:18Z", character: "MrBones", difficulty: "nightmare", run: "countess", definition_id: "countess", route_id: "route-a", outcome: "success", duration_ms: 78000, boss_kills: 1, funnel };

describe("DashboardStats", () => {
  afterEach(cleanup);
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.summary.mockResolvedValue({ summary });
    mocks.comparisons.mockResolvedValue({ comparisons: [comparison] });
    mocks.runs.mockResolvedValue({ runs: [recent] });
  });

  it("lädt 30 Tage parallel und hält die letzten Runs vom Zeitraum unabhängig", async () => {
    render(<DashboardStats farming={<div>Farming bleibt sichtbar</div>} character="MrBones" difficulty="nightmare" runNames={{ countess: "Gräfin" }} />);

    expect(screen.getByRole("button", { name: "30 Tage" })).toHaveAttribute("aria-pressed", "true");
    await waitFor(() => expect(mocks.summary).toHaveBeenCalledOnce());
    expect(mocks.comparisons).toHaveBeenCalledOnce();
    expect(mocks.runs).toHaveBeenCalledOnce();
    expect(mocks.summary.mock.calls[0][0]).toEqual(expect.objectContaining({ character: ["MrBones"], difficulty: ["nightmare"], from: expect.any(String), to: expect.any(String) }));
    expect(mocks.comparisons.mock.calls[0][0]).toEqual(expect.objectContaining({ from: expect.any(String), to: expect.any(String), sort: "keep_per_hour" }));
    expect(mocks.runs.mock.calls[0][0]).toEqual(expect.objectContaining({ character: ["MrBones"], difficulty: ["nightmare"], limit: 3 }));
    expect(mocks.runs.mock.calls[0][0]).not.toHaveProperty("from");
    expect(mocks.runs.mock.calls[0][0]).not.toHaveProperty("to");
    expect(mocks.summary.mock.calls[0][1]).toBeInstanceOf(AbortSignal);
    expect(mocks.comparisons.mock.calls[0][1]).toBeInstanceOf(AbortSignal);
    expect(mocks.runs.mock.calls[0][1]).toBeInstanceOf(AbortSignal);

    expect(await screen.findByText("2,7")).toBeInTheDocument();
    expect(screen.getAllByText("2,7")).toHaveLength(1);
    expect(screen.getByRole("img", { name: /Gesicherte Items pro Stunde: Gräfin 2,7/ })).toBeInTheDocument();
    expect(screen.getAllByText("Gräfin")).toHaveLength(1);
    expect(screen.getByText("4 gesichert")).toBeInTheDocument();
  });

  it("behält alte Werte beim Zeitraumwechsel und ersetzt sie erst nach der Antwort", async () => {
    render(<DashboardStats farming={<div>Farming bleibt sichtbar</div>} character="MrBones" difficulty="nightmare" runNames={{ countess: "Gräfin" }} />);
    expect(await screen.findByText("2,7")).toBeInTheDocument();
    expect(screen.getAllByText("2,7")).toHaveLength(1);

    let resolveSummary!: (value: unknown) => void;
    let resolveComparisons!: (value: unknown) => void;
    let resolveRuns!: (value: unknown) => void;
    mocks.summary.mockImplementationOnce(() => new Promise((resolve) => { resolveSummary = resolve; }));
    mocks.comparisons.mockImplementationOnce(() => new Promise((resolve) => { resolveComparisons = resolve; }));
    mocks.runs.mockImplementationOnce(() => new Promise((resolve) => { resolveRuns = resolve; }));
    fireEvent.click(screen.getByRole("button", { name: "7 Tage" }));

    expect(await screen.findByText("Statistik wird aktualisiert")).toBeInTheDocument();
    expect(screen.getAllByText("2,7")).toHaveLength(1);
    resolveSummary({ summary: { ...summary, keep_per_hour: 3.4 } });
    resolveComparisons({ comparisons: [{ ...comparison, keep_per_hour: 3.4 }] });
    resolveRuns({ runs: [recent] });
    expect(await screen.findByText("3,4")).toBeInTheDocument();
    expect(screen.getAllByText("3,4")).toHaveLength(1);
    expect(screen.getByRole("img", { name: /Gesicherte Items pro Stunde: Gräfin 3,4/ })).toBeInTheDocument();
  });

  it("zeigt Statistikfehler formstabil und blockiert Farming nicht", async () => {
    mocks.summary.mockRejectedValueOnce(new Error("Historie vorübergehend nicht verfügbar"));
    render(<DashboardStats farming={<button>Jetzt farmen</button>} character="MrBones" difficulty="nightmare" runNames={{}} />);
    expect(await screen.findByRole("alert")).toHaveTextContent("Statistik ist gerade nicht verfügbar");
    expect(screen.getByRole("button", { name: "Jetzt farmen" })).toBeEnabled();
    expect(screen.getByRole("heading", { name: "Auf einen Blick" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Verteilung" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Welche Route lohnt sich?" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Erneut versuchen" }));
    await waitFor(() => expect(mocks.summary).toHaveBeenCalledTimes(2));
  });

  it("bildet Gesamt ohne Zeitraumgrenzen und berechnet 7 Tage deterministisch", () => {
    const now = new Date("2026-08-22T12:00:00Z");
    const seven = dashboardHistoryQueries("7", "MrBones", "hell", now);
    expect(seven.period.from).toBe("2026-08-15T12:00:00.000Z");
    expect(seven.period.to).toBe("2026-08-22T12:00:00.000Z");
    expect(seven.recent).not.toHaveProperty("from");
    const all = dashboardHistoryQueries("all", "MrBones", "hell", now);
    expect(all.period).not.toHaveProperty("from");
    expect(all.period).not.toHaveProperty("to");
  });
});
