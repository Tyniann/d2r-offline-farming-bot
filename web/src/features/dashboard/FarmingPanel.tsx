import { ExternalLink, Play } from "lucide-react";
import type { RunCatalogEntry, StatusDTO } from "../../api/generated";
import { StateMessage } from "../../app/ui";
import { runAvailabilityText } from "../../app/runReasons";
import { dashboardRunName } from "./dashboardText";

interface Props {
  status: StatusDTO | null;
  character: string;
  difficulty: string;
  queue: string[];
  runs: RunCatalogEntry[] | null;
  expectedClass?: string;
  startLocked: boolean;
  commandPending: boolean;
  onStart(): void;
}

/** FarmingPanel shows the persistent queue and its single start and edit actions. */
export function FarmingPanel({ status, character, difficulty, queue, runs, expectedClass, startLocked, commandPending, onStart }: Props) {
  const sessionActive = !!status && !["idle", "idle_in_game", "stopped_error"].includes(status.state);
  const rows = queue.map((runID) => {
    const run = runs?.find((entry) => entry.run_id === runID);
    return { id: runID, name: dashboardRunName(runID, run?.display_name), availability: run ? runAvailabilityText(run.status, run.reasons, expectedClass).title : "Wird geprüft" };
  });

  return <section className="dashboard-panel dashboard-farming-panel" aria-labelledby="dashboard-farming-title">
    <div className="dashboard-panel-heading"><div><span>Farming</span><h2 id="dashboard-farming-title">Deine Run-Reihenfolge</h2></div>{queue.length > 0 && <a href="#settings" className="dashboard-text-link">Bearbeiten <ExternalLink aria-hidden="true" size={14} /></a>}</div>
    {!character || !difficulty
      ? <StateMessage kind="empty" title="Charakter auswählen">Wähle Charakter und Schwierigkeit in der Seitenleiste.</StateMessage>
      : runs === null
        ? <StateMessage kind="loading" title="Run-Reihenfolge wird geprüft" />
        : rows.length === 0
          ? <div className="dashboard-empty-queue"><strong>Noch keine Run-Reihenfolge</strong><p>Lege fest, welche Routen pro Spiel gefarmt werden.</p><a className="button" href="#settings">Run-Reihenfolge festlegen</a></div>
          : <>
            <ol className="dashboard-queue-list">{rows.map((row, index) => <li key={`${row.id}-${index}`}><span>{index + 1}</span><strong>{row.name}</strong><small>{status?.state === "running_run" && status.active_run_id && status.queue.index === index ? "Läuft" : row.availability}</small></li>)}</ol>
            {!sessionActive && <button className="dashboard-start" type="button" disabled={startLocked || !status?.selection.character} onClick={onStart}><Play aria-hidden="true" size={18} />{commandPending ? "Start wird bestätigt …" : "Jetzt farmen"}</button>}
            <p className="dashboard-panel-summary">{rows.length} {rows.length === 1 ? "Run" : "Runs"} pro Spiel</p>
          </>}
  </section>;
}
