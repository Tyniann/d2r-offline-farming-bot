import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { StatusDTO } from "../../api/generated";
import { ActiveRunPanel } from "./ActiveRunPanel";

const status = {
  run_id: "run-1",
  pending_intent: "pause_after_run",
  run_progress: { stage_code: "cellar_floor", params: { floor: 3, floors: 5 }, current: 6, total: 13 },
  input: { enabled: true, paused: false, stopped: false },
  queue: { entries: ["countess", "mephisto", "summoner"], index: 0, started_runs: 1, budgets: { max_runs: 8, max_duration_ms: 7200000, max_consecutive_failures: 2, max_total_restarts: 3 } },
} as unknown as StatusDTO;

describe("ActiveRunPanel", () => {
  afterEach(cleanup);

  it("zeigt Core-Etappe, Queue-Position, effektive Hotkeys und Vormerkung ohne Schaltflächen", () => {
    render(<ActiveRunPanel status={status} runName="Gräfin" hotkeys={{ pause: "pause", stopAfterRun: "f8", emergencyStop: "f12" }} />);
    expect(screen.getByRole("heading", { name: "Gräfin läuft" })).toBeInTheDocument();
    expect(screen.getByText("Route 1 von 3")).toBeInTheDocument();
    expect(screen.getByText("Routenausführung 1 von 8")).toBeInTheDocument();
    expect(screen.getByLabelText("Etappe 6 von 13")).toBeInTheDocument();
    expect(screen.getByText(/Kellergeschoss 3 von 5 · Etappe 6 von 13 · 0:00 vergangen/)).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Pause nach dieser Route vorgemerkt");
    expect(screen.getByText("Pause", { selector: "kbd" })).toBeInTheDocument();
    expect(screen.getByText("F8", { selector: "kbd" })).toBeInTheDocument();
    expect(screen.getByText("F12", { selector: "kbd" })).toBeInTheDocument();
    expect(screen.getByRole("note", { name: "F8: Nach dieser Route stoppen" })).toBeInTheDocument();
    expect(screen.queryAllByRole("button")).toHaveLength(0);
  });

  it("fällt bei fehlender Projektion ohne internen Schrittnamen zurück", () => {
    render(<ActiveRunPanel status={{ ...status, run_progress: undefined, pending_intent: "stop_after_run" }} hotkeys={{ pause: "Pause", stopAfterRun: "F10", emergencyStop: "F11" }} />);
    expect(screen.getByText(/Route wird ausgeführt · 0:00 vergangen/)).toBeInTheDocument();
    expect(screen.getByText("Stopp nach dieser Route vorgemerkt")).toBeInTheDocument();
    expect(screen.queryByLabelText(/Etappe/)).not.toBeInTheDocument();
  });
});
