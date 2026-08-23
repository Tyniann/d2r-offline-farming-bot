import { ExternalLink, Play } from "lucide-react";
import { useTranslation } from "react-i18next";
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
  const { t } = useTranslation();
  const sessionActive = !!status && !["idle", "idle_in_game", "stopped_error"].includes(status.state);
  const rows = queue.map((runID) => {
    const run = runs?.find((entry) => entry.run_id === runID);
    return { id: runID, name: dashboardRunName(runID, t), availability: run ? runAvailabilityText(run.status, run.reasons, expectedClass, t).title : t("dashboard.farming.checking") };
  });

  return <section className="dashboard-panel dashboard-farming-panel" aria-labelledby="dashboard-farming-title">
    <div className="dashboard-panel-heading"><div><span>{t("dashboard.farming.eyebrow")}</span><h2 id="dashboard-farming-title">{t("dashboard.farming.title")}</h2></div>{queue.length > 0 && <a href="#settings" className="dashboard-text-link">{t("dashboard.farming.edit")} <ExternalLink aria-hidden="true" size={14} /></a>}</div>
    {!character || !difficulty
      ? <StateMessage kind="empty" title={t("dashboard.farming.chooseCharacter")}>{t("dashboard.farming.chooseContext")}</StateMessage>
      : runs === null
        ? <StateMessage kind="loading" title={t("dashboard.farming.checkingQueue")} />
        : rows.length === 0
          ? <div className="dashboard-empty-queue"><strong>{t("dashboard.farming.emptyTitle")}</strong><p>{t("dashboard.farming.emptyDetail")}</p><a className="button" href="#settings">{t("dashboard.farming.configure")}</a></div>
          : <>
            <ol className="dashboard-queue-list">{rows.map((row, index) => <li key={`${row.id}-${index}`}><span>{index + 1}</span><strong>{row.name}</strong><small>{status?.state === "running_run" && status.active_run_id && status.queue.index === index ? t("dashboard.farming.running") : row.availability}</small></li>)}</ol>
            {!sessionActive && <button className="dashboard-start" type="button" disabled={startLocked || !status?.selection.character} onClick={onStart}><Play aria-hidden="true" size={18} />{commandPending ? t("dashboard.farming.startPending") : t("dashboard.farming.start")}</button>}
          </>}
  </section>;
}
