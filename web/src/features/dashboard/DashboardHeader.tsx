import { CircleAlert, Gamepad2, MapPin, ShieldCheck, Wifi, WifiOff } from "lucide-react";
import type { LiveConnectionState } from "../../api/client";
import type { StatusDTO } from "../../api/generated";

interface Props {
  status: StatusDTO | null;
  connection: LiveConnectionState;
  character: string;
  difficultyLabel: string;
  confirmedDifficultyLabel: string;
  queueSize: number;
  selectionNeedsApply: boolean;
  selectionError: string;
  applyLocked: boolean;
  applying: boolean;
  activeRunName?: string;
  onApplySelection(): void;
}

/** DashboardHeader presents the selected farming context without exposing Core internals. */
export function DashboardHeader({ status, connection, character, difficultyLabel, confirmedDifficultyLabel, queueSize, selectionNeedsApply, selectionError, applyLocked, applying, activeRunName, onApplySelection }: Props) {
  const connected = connection === "verbunden";
  const inputReady = !!status?.input.enabled && !status.input.paused && !status.input.stopped;
  const d2rReady = status?.compatibility?.state === "compatible" && status.d2r.state !== "detached";

  return <>
    <header className="dashboard-header">
      <div>
        <p className="dashboard-kicker">Dashboard</p>
        <h1>{activeRunName ? `${activeRunName} läuft` : character ? `${character} ist bereit` : "Farming vorbereiten"}</h1>
        <p>{character ? `${difficultyLabel} · ${queueSize} ${queueSize === 1 ? "Run" : "Runs"} pro Spiel` : "Wähle einen Charakter in der Seitenleiste."}</p>
      </div>
      <div className="dashboard-readiness" aria-label="Bereitschaft">
        <span className={connected ? "is-ready" : "is-warning"}>{connected ? <Wifi aria-hidden="true" size={15} /> : <WifiOff aria-hidden="true" size={15} />}{connected ? "Verbunden" : "Verbindung fehlt"}</span>
        <span className={d2rReady ? "is-ready" : "is-warning"}><Gamepad2 aria-hidden="true" size={15} />{d2rReady ? "D2R bereit" : "D2R nicht bereit"}</span>
        <span className={inputReady ? "is-ready" : "is-warning"}><ShieldCheck aria-hidden="true" size={15} />{inputReady ? "Steuerung bereit" : "Steuerung gesperrt"}</span>
        {status?.world.area_name && <span><MapPin aria-hidden="true" size={15} />{status.world.area_name}</span>}
      </div>
    </header>
    {selectionNeedsApply && <div className="dashboard-selection-notice" role="status">
      <CircleAlert aria-hidden="true" size={19} />
      <div><strong>{character} ist in der App ausgewählt</strong><span>In D2R ist noch {status?.selection.character || "kein Charakter"}{confirmedDifficultyLabel ? ` · ${confirmedDifficultyLabel}` : ""} aktiv.</span></div>
      <button type="button" disabled={applyLocked} onClick={onApplySelection}>{applying ? "Auswahl wird geprüft …" : "In D2R verwenden"}</button>
    </div>}
    {selectionError && <p className="dashboard-inline-error" role="alert">{selectionError}</p>}
  </>;
}
