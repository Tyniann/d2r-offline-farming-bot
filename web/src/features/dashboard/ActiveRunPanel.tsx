import { OctagonX, Pause, Play, Square } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { StatusDTO } from "../../api/generated";
import { presentRunStage } from "../../i18n/presenters";

interface Hotkeys {
  pause: string;
  stopAfterRun: string;
  emergencyStop: string;
}

interface Props {
  status: StatusDTO;
  runName?: string;
  hotkeys: Hotkeys;
}

/** ActiveRunPanel renders Core-owned progress and keyboard-only safety hints. */
export function ActiveRunPanel({ status, runName, hotkeys }: Props) {
  const { t } = useTranslation();
  const elapsed = useObservedElapsed(status.run_id);
  const progress = validProgress(status.run_progress) ? status.run_progress : undefined;
  const queueTotal = status.queue.entries.length;
  const queueCurrent = queueTotal > 0 ? Math.min(Math.max(status.queue.index + 1, 1), queueTotal) : 0;
  const maxRuns = status.queue.budgets.max_runs;
  const sessionCurrent = maxRuns > 0 ? Math.min(Math.max(status.queue.started_runs, 1), maxRuns) : 0;
  const pendingText = status.pending_intent === "pause_after_run"
    ? t("dashboard.active.pausePending")
    : status.pending_intent === "stop_after_run"
      ? t("dashboard.active.stopPending")
      : "";

  return <section className="dashboard-active-run" aria-labelledby="dashboard-active-run-title" aria-live="polite">
    <div className="dashboard-active-run-icon"><Play aria-hidden="true" size={21} fill="currentColor" /></div>
    <div className="dashboard-active-run-copy">
      <div className="dashboard-active-run-meta">
        <span>{queueTotal > 0 ? t("dashboard.active.queuePosition", { current: queueCurrent, total: queueTotal }) : t("dashboard.active.activeSession")}</span>
        {maxRuns > 0 && <span>{t("dashboard.active.sessionProgress", { current: sessionCurrent, total: maxRuns })}</span>}
      </div>
      <h2 id="dashboard-active-run-title">{runName ? t("dashboard.active.runActive", { run: runName }) : t("dashboard.active.preparing")}</h2>
      {progress ? <>
        <div className="dashboard-active-run-steps" aria-label={t("dashboard.active.stageAria", { current: progress.current, total: progress.total })}>
          {Array.from({ length: progress.total }, (_, index) => <i key={index} className={index < progress.current - 1 ? "is-complete" : index === progress.current - 1 ? "is-current" : undefined} />)}
        </div>
        <small>{t("dashboard.active.stageDetail", { label: presentRunStage(progress.stage_code, progress.params, t), current: progress.current, total: progress.total, elapsed: formatElapsed(elapsed) })}</small>
      </> : <small>{t("dashboard.active.runningDetail", { elapsed: formatElapsed(elapsed) })}</small>}
      {pendingText && <strong className="dashboard-active-intent" role="status">{pendingText}</strong>}
      {status.input.stopped && <strong className="dashboard-active-intent is-danger" role="alert">{t("dashboard.active.emergencyActive")}</strong>}
    </div>
    <div className="dashboard-active-hotkeys" aria-label={t("dashboard.active.controls")}>
      <HotkeyHint icon={Pause} keyLabel={hotkeys.pause} label={t("dashboard.active.pauseAfterRun")} active={status.pending_intent === "pause_after_run"} />
      <HotkeyHint icon={Square} keyLabel={hotkeys.stopAfterRun} label={t("dashboard.active.stopAfterRun")} active={status.pending_intent === "stop_after_run"} />
      <HotkeyHint icon={OctagonX} keyLabel={hotkeys.emergencyStop} label={t("dashboard.active.stopImmediately")} danger active={status.input.stopped} />
    </div>
  </section>;
}

function HotkeyHint({ icon: Icon, keyLabel, label, danger = false, active = false }: { icon: typeof Pause; keyLabel: string; label: string; danger?: boolean; active?: boolean }) {
  const formattedKey = formatHotkey(keyLabel);
  return <div className={`dashboard-active-hotkey${danger ? " is-danger" : ""}${active ? " is-active" : ""}`} role="note" aria-label={`${formattedKey}: ${label}`} title={label}>
    <Icon aria-hidden="true" size={16} />
    <kbd>{formattedKey}</kbd>
    <span className="visually-hidden">{label}</span>
  </div>;
}

function useObservedElapsed(runID?: string): number {
  const [startedAt, setStartedAt] = useState(() => Date.now());
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const current = Date.now();
    setStartedAt(current);
    setNow(current);
  }, [runID]);
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);
  return Math.max(0, Math.floor((now - startedAt) / 1000));
}

function validProgress(progress: StatusDTO["run_progress"]): progress is NonNullable<StatusDTO["run_progress"]> {
  return !!progress && progress.current >= 1 && progress.total >= progress.current && typeof progress.stage_code === "string" && progress.stage_code.trim() !== "";
}

function formatElapsed(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  return `${minutes}:${String(seconds % 60).padStart(2, "0")}`;
}

function formatHotkey(value: string): string {
  if (value.toLowerCase() === "pause") return "Pause";
  if (/^f\d+$/i.test(value)) return value.toUpperCase();
  return value;
}
