import { OctagonX, Pause, Play, Square } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { StatusDTO } from "../../api/generated";
import { type AppTranslator, presentRunStage } from "../../i18n/presenters";

interface Hotkeys {
  pause: string;
  stopAfterRun: string;
  emergencyStop: string;
}

interface Props {
  status: StatusDTO;
  runName?: string;
  hotkeys: Hotkeys;
  commandPending?: boolean;
  resumeLocked?: boolean;
  onResume?: () => void;
}

/** ActiveRunPanel renders Core-owned progress, keyboard safety hints, and the paused-session resume control. */
export function ActiveRunPanel({ status, runName, hotkeys, commandPending = false, resumeLocked = false, onResume }: Props) {
  const { t } = useTranslation();
  const elapsed = useObservedElapsed(status.run_id);
  const paused = status.state === "paused_between_runs";
  const progress = !paused && validProgress(status.run_progress) ? status.run_progress : undefined;
  const queueTotal = status.queue.entries.length;
  const queueCurrent = queueTotal > 0 ? Math.min(Math.max(status.queue.index + 1, 1), queueTotal) : 0;
  const maxRuns = status.queue.budgets.max_runs;
  const sessionCurrent = maxRuns > 0 ? Math.min(Math.max(status.queue.started_runs, 1), maxRuns) : 0;
  const pendingText = status.pending_intent === "pause_after_run"
    ? t("dashboard.active.pausePending")
    : status.pending_intent === "stop_after_run"
      ? t("dashboard.active.stopPending")
      : "";
  const recoveryText = presentRecoveryStep(status.recovery_step, status.world?.area_id, t);
  const title = paused
    ? t("dashboard.active.paused")
    : runName ? t("dashboard.active.runActive", { run: runName }) : t("dashboard.active.preparing");
  const detail = paused
    ? runName ? t("dashboard.active.pausedNext", { run: runName }) : t("dashboard.active.pausedDetail")
    : progress
      ? t("dashboard.active.stageDetail", { label: presentRunStage(progress.stage_code, progress.params, t), current: progress.current, total: progress.total, elapsed: formatElapsed(elapsed) })
      : t("dashboard.active.runningDetail", { elapsed: formatElapsed(elapsed) });

  return <section className="dashboard-active-run" aria-labelledby="dashboard-active-run-title" aria-live="polite">
    <div className="dashboard-active-run-icon">{paused ? <Pause aria-hidden="true" size={21} /> : <Play aria-hidden="true" size={21} fill="currentColor" />}</div>
    <div className="dashboard-active-run-copy">
      <div className="dashboard-active-run-meta">
        <span>{queueTotal > 0 ? t("dashboard.active.queuePosition", { current: queueCurrent, total: queueTotal }) : t("dashboard.active.activeSession")}</span>
        {maxRuns > 0 && <span>{t("dashboard.active.sessionProgress", { current: sessionCurrent, total: maxRuns })}</span>}
      </div>
      <h2 id="dashboard-active-run-title">{title}</h2>
      {progress && <div className="dashboard-active-run-steps" aria-label={t("dashboard.active.stageAria", { current: progress.current, total: progress.total })}>
        {Array.from({ length: progress.total }, (_, index) => <i key={index} className={index < progress.current - 1 ? "is-complete" : index === progress.current - 1 ? "is-current" : undefined} />)}
      </div>}
      <small>{detail}</small>
      {recoveryText && <strong className="dashboard-active-intent" role="status">{recoveryText}</strong>}
      {!paused && pendingText && <strong className="dashboard-active-intent" role="status">{pendingText}</strong>}
      {status.input.stopped && <strong className="dashboard-active-intent is-danger" role="alert">{t("dashboard.active.emergencyActive")}</strong>}
    </div>
    <div className="dashboard-active-hotkeys" aria-label={t("dashboard.active.controls")}>
      {paused && onResume
        ? <button type="button" className="dashboard-active-resume" disabled={resumeLocked} onClick={onResume}>
          <Play aria-hidden="true" size={16} fill="currentColor" />
          {commandPending ? t("dashboard.active.resumePending") : t("dashboard.active.resume")}
        </button>
        : <HotkeyHint icon={Pause} keyLabel={hotkeys.pause} label={t("dashboard.active.pauseAfterRun")} active={status.pending_intent === "pause_after_run"} />}
      <HotkeyHint icon={Square} keyLabel={hotkeys.stopAfterRun} label={t("dashboard.active.stopAfterRun")} active={status.pending_intent === "stop_after_run"} />
      <HotkeyHint icon={OctagonX} keyLabel={hotkeys.emergencyStop} label={t("dashboard.active.stopImmediately")} danger active={status.input.stopped} />
    </div>
  </section>;
}

function presentRecoveryStep(step: string | undefined, areaID: number | undefined, t: AppTranslator): string {
  switch (step) {
    case "retry_return": return t("dashboard.active.recovery.retryReturn");
    case "local_recovery_clear": return t("dashboard.active.recovery.localClear");
    case "return_portal_reposition": return t("dashboard.active.recovery.portalReposition");
    case "return_portal_retry": return t("dashboard.active.recovery.portalRetry");
    case "direct_exit": return t("dashboard.active.recovery.directExit");
    case "start_town_normalization": return t("dashboard.active.recovery.startTownNormalization", { act: townAct(areaID) });
    case "return_to_act1": return t("dashboard.active.recovery.returnToAct1");
    case "restart_game": return t("dashboard.active.recovery.restartGame");
    default: return "";
  }
}

function townAct(areaID: number | undefined): number | string {
  switch (areaID) {
    case 40: return 2;
    case 75: return 3;
    case 103: return 4;
    case 109: return 5;
    default: return "?";
  }
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
