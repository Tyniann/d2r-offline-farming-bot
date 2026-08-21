import { OctagonX, Pause, Play, Square } from "lucide-react";
import { useEffect, useState } from "react";
import type { StatusDTO } from "../../api/generated";

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
  const elapsed = useObservedElapsed(status.run_id);
  const progress = validProgress(status.run_progress) ? status.run_progress : undefined;
  const queueTotal = status.queue.entries.length;
  const queueCurrent = queueTotal > 0 ? Math.min(Math.max(status.queue.index + 1, 1), queueTotal) : 0;
  const pendingText = status.pending_intent === "pause_after_run"
    ? "Pause nach diesem Run vorgemerkt"
    : status.pending_intent === "stop_after_run"
      ? "Stopp nach diesem Run vorgemerkt"
      : "";

  return <section className="dashboard-active-run" aria-labelledby="dashboard-active-run-title" aria-live="polite">
    <div className="dashboard-active-run-icon"><Play aria-hidden="true" size={21} fill="currentColor" /></div>
    <div className="dashboard-active-run-copy">
      <span>{queueTotal > 0 ? `Run ${queueCurrent} von ${queueTotal}` : "Aktive Session"}</span>
      <h2 id="dashboard-active-run-title">{runName ? `${runName} läuft` : "Session wird vorbereitet"}</h2>
      {progress ? <>
        <div className="dashboard-active-run-steps" aria-label={`Etappe ${progress.current} von ${progress.total}`}>
          {Array.from({ length: progress.total }, (_, index) => <i key={index} className={index < progress.current - 1 ? "is-complete" : index === progress.current - 1 ? "is-current" : undefined} />)}
        </div>
        <small>{progress.label} · Etappe {progress.current} von {progress.total} · {formatElapsed(elapsed)} vergangen</small>
      </> : <small>Run wird ausgeführt · {formatElapsed(elapsed)} vergangen</small>}
      {pendingText && <strong className="dashboard-active-intent" role="status">{pendingText}</strong>}
      {status.input.stopped && <strong className="dashboard-active-intent is-danger" role="alert">Notstopp aktiv</strong>}
    </div>
    <div className="dashboard-active-hotkeys" aria-label="Steuerung im Spiel">
      <HotkeyHint icon={Pause} keyLabel={hotkeys.pause} label="Nach diesem Run pausieren" active={status.pending_intent === "pause_after_run"} />
      <HotkeyHint icon={Square} keyLabel={hotkeys.stopAfterRun} label="Nach diesem Run stoppen" active={status.pending_intent === "stop_after_run"} />
      <HotkeyHint icon={OctagonX} keyLabel={hotkeys.emergencyStop} label="Sofort stoppen" danger active={status.input.stopped} />
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
  return !!progress && progress.current >= 1 && progress.total >= progress.current && progress.label.trim() !== "";
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
