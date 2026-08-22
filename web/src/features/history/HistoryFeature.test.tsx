import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { changeAppLanguage } from "../../i18n";
import { apiError } from "../../test/apiError";
import { HistoryFeature } from "./HistoryFeature";

const mocks = vi.hoisted(() => ({ summary: vi.fn(), comparisons: vi.fn(), items: vi.fn(), runs: vi.fn(), detail: vi.fn(), download: vi.fn() }));
vi.mock("../../api/generated", () => ({
  getHistorySummary: mocks.summary, getHistoryComparisons: mocks.comparisons, getHistoryItems: mocks.items,
  getHistoryRuns: mocks.runs, getHistoryRun: mocks.detail, downloadHistoryExport: mocks.download,
}));

const meta = { schema_version: 4, generated_at: "2026-07-22T12:00:00Z", timezone: "UTC", index_generation: 4, filter: { timezone: "UTC", runs: [], characters: [], difficulties: [], outcomes: [], reasons: [], pickit_profiles: [] }, diagnostics: [{ file: "broken.jsonl", code: "history_file_invalid" }], ignored_files: 2 };
const durations = { count: 5, total_ms: 360000, average_ms: 72000, median_ms: 60000, minimum_ms: 30000, maximum_ms: 120000 };
const stages = { travel_ms: 150000, combat_ms: 55000, loot_ms: 55000, return_town_ms: 40000, other_ms: 60000 };
const funnel = { seen: 6, matched: 6, picked_up: 5, stashed: 3, sold: 1, keep_return: 3, pickup_lost: 1, post_pickup_lost: 1 };
const dailyBuckets = [
  { date: "2026-07-20", start_utc: "2026-07-20T00:00:00Z", end_utc: "2026-07-21T00:00:00Z", terminal_runs: 3, successful: 2, success_rate: 2 / 3, active_duration_ms: 180000, active_hours: .05, keep_return: 2, keep_per_hour: 40 },
  { date: "2026-07-21", start_utc: "2026-07-21T00:00:00Z", end_utc: "2026-07-22T00:00:00Z", terminal_runs: 0, successful: 0, active_duration_ms: 0, active_hours: 0, keep_return: 0 },
  { date: "2026-07-22", start_utc: "2026-07-22T00:00:00Z", end_utc: "2026-07-23T00:00:00Z", terminal_runs: 2, successful: 1, success_rate: .5, active_duration_ms: 180000, active_hours: .05, keep_return: 1, keep_per_hour: 20 },
];
const failure = { step: "acquire_boss", reason: "boss_not_found", count: 1, lost_duration_ms: 120000 };
const routeA = { id: "a", character: "MrBones", difficulty: "nightmare", definition_id: "countess", run: "countess", route_id: "countess-route-a", terminal_runs: 2, successful: 1, failed: 1, aborted: 0, success_rate: .5, boss_kills: 1, low_sample: true, durations: { ...durations, count: 2, total_ms: 180000, average_ms: 90000 }, stages, funnel, keep_per_run: .5, keep_per_kill: 1, keep_per_hour: 20, top_failure: failure };
const routeB = { ...routeA, id: "b", route_id: "countess-route-b", terminal_runs: 1, successful: 1, failed: 0, success_rate: 1, durations: { ...durations, count: 1, total_ms: 90000, average_ms: 90000 }, keep_per_run: 1, keep_per_hour: 40, top_failure: undefined };
const routeMephisto = { ...routeA, id: "m", definition_id: "mephisto", run: "mephisto", route_id: "mephisto-route-a", boss_kills: 2, keep_per_kill: .5, keep_per_hour: 40 };
const item = { item_key: "base:r01:normal", item_name: "El-Rune", seen: 2, matched: 2, picked_up: 1, stashed: 1, sold: 1, pickup_lost: 1, post_pickup_lost: 1, yield_per_hour: 30 };
const run = { run_id: "run-1", started_at: "2026-07-22T10:00:00Z", observed_at: "2026-07-22T10:02:00Z", character: "MrBones", difficulty: "nightmare", run: "countess", definition_id: "countess", route_id: "route-b", outcome: "failed", reason: "boss_not_found", last_step: "acquire_boss", duration_ms: 120000, boss_kills: 0, funnel };

describe("HistoryFeature", () => {
  afterEach(cleanup);
  beforeEach(async () => {
    await changeAppLanguage("de");
    vi.clearAllMocks();
    mocks.summary.mockResolvedValue({ meta, daily_buckets: dailyBuckets, summary: { runs: 7, terminal_runs: 5, successful: 3, failed: 2, aborted: 0, incomplete: 1, running: 1, success_rate: .6, boss_kills: 4, durations, stages, funnel, keep_per_run: .6, keep_per_kill: .75, keep_per_hour: 30, top_failure: failure } });
    mocks.comparisons.mockResolvedValue({ meta, comparisons: [routeA, routeB, routeMephisto] });
    mocks.items.mockResolvedValue({ meta, items: [item], next_cursor: "items-next" });
    mocks.runs.mockResolvedValue({ meta, runs: [run], next_cursor: "runs-next" });
    mocks.detail.mockImplementation((_runID: string, includeRaw = false) => Promise.resolve({ meta, run: { ...run, ended_at: "2026-07-22T10:02:00Z", stages, items: [{ unit_id: 7, item_key: item.item_key, item_name: item.item_name, pickit_profile_id: "runes", pickit_rule_id: "el-rune", pickit_action: "keep", pickit_profile_revision: 3, pickit_assignment_revision: 8, seen: true, matched: true, picked_up: true, stashed: false, sold: false, pickup_lost: false, post_pickup_lost: true }], ...(includeRaw ? { raw_context: { schema_version: 4, run_id: "run-1", route_id: "route-b" }, raw_events: [{ timestamp: "2026-07-22T10:00:00Z", event: "run_context" }, { timestamp: "2026-07-22T10:00:01Z", event: "route_clear_action", action_kind: "attack" }] } : {}) } }));
    mocks.download.mockResolvedValue({ blob: new Blob(["ok"]), filename: "d2r-history.json" });
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn(() => "blob:test") });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
  });

  it("rendert die repräsentative Historienoberfläche auf Englisch", async () => {
    await changeAppLanguage("en");
    render(<HistoryFeature characters={["MrBones"]} selectedCharacter="MrBones" selectedDifficulty="nightmare" runs={["countess"]} refreshKey={0} />);

    expect(await screen.findByRole("heading", { name: "History" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Apply filters" })).toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: "Daily trend" })).toBeInTheDocument();
  });

  it("zeigt Route, Sample, Keep/Sell, Verlust und priorisierten Fehler und sortiert interaktiv", async () => {
    mocks.comparisons.mockImplementation((request: { sort?: string }) => Promise.resolve({ meta, comparisons: request.sort === "average_duration" ? [routeB, routeA, routeMephisto] : [routeA, routeB, routeMephisto] }));
    const { container } = render(<HistoryFeature characters={["MrBones"]} selectedCharacter="MrBones" selectedDifficulty="nightmare" runs={["countess"]} refreshKey={0} />);
    expect((await screen.findAllByText("Kleine Stichprobe (< 10 Kills)")).length).toBe(3);
    expect(screen.getAllByText("Der Boss wurde nicht gefunden.").length).toBeGreaterThan(0);
    expect(screen.getAllByText("1 / 1")).toHaveLength(2);
    expect(screen.getByText("Nicht aggregiert: 1 aktiv, 1 unvollständig.")).toBeInTheDocument();
    expect(screen.getByText("Die Telemetriedatei ist beschädigt und wurde vollständig ausgeschlossen.")).toBeInTheDocument();
    expect(screen.getByRole("table", { name: /Keep, Verkauf und Verluste/ })).toBeInTheDocument();
    expect(container.querySelectorAll(".table-scroll")).toHaveLength(7);
    expect(screen.getByRole("heading", { name: "Tagesverlauf" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Routenvergleich" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Run-Stages" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Loot-Funnel" })).toBeInTheDocument();
    const dailyTable = screen.getByRole("table", { name: "Exakte Core-Werte des Tagesverlaufs" });
    expect(within(dailyTable).getByText("2026-07-21").closest("tr")).toHaveTextContent("2026-07-212026-07-21T00:00:00Z – 2026-07-22T00:00:00Z0–00–");
    fireEvent.change(screen.getByLabelText("Tageskennzahl"), { target: { value: "keep_per_hour" } });
    const comparison = screen.getByRole("table", { name: /Vergleich derselben/ });
    expect(within(comparison).getAllByRole("row")[1]).toHaveTextContent("countess-route-a");
    fireEvent.change(screen.getByLabelText("Sortierung"), { target: { value: "average_duration" } });
    await waitFor(() => expect(mocks.comparisons).toHaveBeenLastCalledWith(expect.objectContaining({ sort: "average_duration" }), expect.any(AbortSignal)));
    await waitFor(() => expect(within(comparison).getAllByRole("row")[1]).toHaveTextContent("countess-route-b"));
  });

  it("filtert, paginiert und lädt den Fehler-Drill-down mit eingeklapptem Raw-JSON", async () => {
    mocks.runs.mockImplementation((request: { cursor?: string }) => Promise.resolve(request.cursor ? { meta, runs: [{ ...run, run_id: "run-2", route_id: "route-a" }] } : { meta, runs: [run], next_cursor: "runs-next" }));
    mocks.items.mockImplementation((request: { cursor?: string }) => Promise.resolve(request.cursor ? { meta, items: [{ ...item, item_key: "base:r02:normal", item_name: "Eld-Rune" }] } : { meta, items: [item], next_cursor: "items-next" }));
    render(<HistoryFeature characters={["MrBones"]} selectedCharacter="MrBones" selectedDifficulty="nightmare" runs={["countess"]} refreshKey={0} />);
    await screen.findByRole("table", { name: /Vergleich derselben/ });
    fireEvent.change(screen.getByLabelText("Charakter"), { target: { value: "MrBones" } });
    fireEvent.change(screen.getByLabelText("Ergebnis"), { target: { value: "failed" } });
    fireEvent.change(screen.getByLabelText("Zeitraum"), { target: { value: "custom" } });
    fireEvent.change(screen.getByLabelText("Von"), { target: { value: "2026-07-20T10:00" } });
    fireEvent.change(screen.getByLabelText("Reason-Code"), { target: { value: "boss_not_found" } });
    fireEvent.change(screen.getByLabelText("Pickit-Profil"), { target: { value: "runes" } });
    fireEvent.click(screen.getByRole("button", { name: "Filter anwenden" }));
    await waitFor(() => expect(mocks.summary).toHaveBeenLastCalledWith(expect.objectContaining({ character: ["MrBones"], outcome: ["failed"], reason: ["boss_not_found"], pickit_profile: ["runes"], from: expect.stringMatching(/Z$/) }), expect.any(AbortSignal)));
    expect(screen.getByText("Aktive Filter:").closest("p")).toHaveTextContent("Reason: boss_not_found");
    fireEvent.click(await screen.findByRole("button", { name: "Mehr Runs laden" }));
    await waitFor(() => expect(mocks.runs).toHaveBeenCalledWith(expect.objectContaining({ cursor: "runs-next" })));
    fireEvent.click(screen.getByRole("button", { name: "Mehr Items laden" }));
    await waitFor(() => expect(mocks.items).toHaveBeenCalledWith(expect.objectContaining({ cursor: "items-next" })));
    fireEvent.click(screen.getAllByRole("button", { name: "Run öffnen" })[0]);
    expect(await screen.findByRole("heading", { name: "Run run-1" })).toBeInTheDocument();
    expect(screen.getByText("nach Pickup verloren")).toBeInTheDocument();
    expect(screen.getByText("Fehlerstelle").closest("div")).toHaveTextContent("boss_not_found · Schritt acquire_boss");
    expect(screen.getByText(/Pickit runes Revision 3/)).toHaveTextContent("Regel el-rune · Aktion keep · Assignment-Revision 8");
    const raw = screen.getByText("Rohereignisse anzeigen").closest("details")!;
    Object.defineProperty(raw, "open", { configurable: true, value: true });
    fireEvent(raw, new Event("toggle"));
    await waitFor(() => expect(mocks.detail).toHaveBeenLastCalledWith("run-1", true));
    expect(await screen.findByRole("heading", { name: "Gemeinsamer Run-Kontext" })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Ereignistyp"), { target: { value: "route_clear_action" } });
    expect(screen.getByText("1 von 2 Ereignissen. Gemeinsame Run-Daten werden nur oben angezeigt.")).toBeInTheDocument();
  });

  it("führt Exporte aus und zeigt Export-, Empty- und No-results-Zustände zugänglich an", async () => {
    const { rerender } = render(<HistoryFeature characters={[]} selectedCharacter="" selectedDifficulty="" runs={[]} refreshKey={0} />);
    await screen.findByRole("button", { name: "JSON-Report" });
    fireEvent.click(screen.getByRole("button", { name: "JSON-Report" }));
    await waitFor(() => expect(mocks.download).toHaveBeenCalledWith("json", "", expect.objectContaining({ timezone: expect.any(String), from: expect.stringMatching(/Z$/), to: expect.stringMatching(/Z$/) })));
    mocks.download.mockRejectedValueOnce(apiError("history_unavailable"));
    fireEvent.click(screen.getByRole("button", { name: "Run-CSV" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Die Historie ist derzeit nicht verfügbar.");
    mocks.summary.mockResolvedValue({ meta, daily_buckets: [], summary: { runs: 0, terminal_runs: 0, successful: 0, failed: 0, aborted: 0, incomplete: 0, running: 0, boss_kills: 0, durations: { ...durations, count: 0 }, stages, funnel } });
    rerender(<HistoryFeature characters={[]} selectedCharacter="" selectedDifficulty="" runs={[]} refreshKey={1} />);
    expect(await screen.findByRole("heading", { name: "Noch keine Historie" })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Ergebnis"), { target: { value: "failed" } });
    fireEvent.click(screen.getByRole("button", { name: "Filter anwenden" }));
    expect(await screen.findByRole("heading", { name: "Keine passenden Runs" })).toBeInTheDocument();
  });

  it("kennzeichnet Loading und API-Fehler als zugängliche Zustände", async () => {
    mocks.summary.mockRejectedValueOnce(apiError("history_unavailable"));
    render(<HistoryFeature characters={[]} selectedCharacter="" selectedDifficulty="" runs={[]} refreshKey={0} />);
    expect(screen.getByRole("status")).toHaveTextContent("Historie wird geladen");
    expect(await screen.findByRole("alert")).toHaveTextContent("Die Historie ist derzeit nicht verfügbar.");
  });
});
