import { CircleAlert, Gamepad2, MapPin, ShieldCheck, Wifi, WifiOff } from "lucide-react";
import type { LiveConnectionState } from "../../api/client";
import type { StatusDTO } from "../../api/generated";
import { useTranslation } from "react-i18next";
import { gameAreaName } from "../../i18n/game";

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
  const { t, i18n } = useTranslation();
  const connected = connection === "connected";
  const inputReady = !!status?.input.enabled && !status.input.paused && !status.input.stopped;
  const d2rReady = status?.compatibility?.state === "compatible" && status.d2r.state !== "detached";

  return <>
    <header className="dashboard-header">
      <div>
        <p className="dashboard-kicker">{t("dashboard.header.kicker")}</p>
        <h1>{activeRunName ? t("dashboard.header.runActive", { run: activeRunName }) : character ? t("dashboard.header.characterReady", { character }) : t("dashboard.header.prepare")}</h1>
        <p>{character ? t("dashboard.header.context", { difficulty: difficultyLabel, count: queueSize }) : t("dashboard.header.chooseCharacter")}</p>
      </div>
      <div className="dashboard-readiness" aria-label={t("dashboard.header.readiness")}>
        <span className={connected ? "is-ready" : "is-warning"}>{connected ? <Wifi aria-hidden="true" size={15} /> : <WifiOff aria-hidden="true" size={15} />}{connected ? t("dashboard.header.connected") : t("dashboard.header.connectionMissing")}</span>
        <span className={d2rReady ? "is-ready" : "is-warning"}><Gamepad2 aria-hidden="true" size={15} />{d2rReady ? t("dashboard.header.d2rReady") : t("dashboard.header.d2rNotReady")}</span>
        <span className={inputReady ? "is-ready" : "is-warning"}><ShieldCheck aria-hidden="true" size={15} />{inputReady ? t("dashboard.header.inputReady") : t("dashboard.header.inputLocked")}</span>
        {!!status?.world.area_id && <span><MapPin aria-hidden="true" size={15} />{gameAreaName(status.world.area_id, status.world.area_name ?? "", i18n.resolvedLanguage)}</span>}
      </div>
    </header>
    {selectionNeedsApply && <div className="dashboard-selection-notice" role="status">
      <CircleAlert aria-hidden="true" size={19} />
      <div><strong>{t("dashboard.header.appSelection", { character })}</strong><span>{t("dashboard.header.d2rSelection", { character: status?.selection.character || t("dashboard.header.noCharacter"), difficulty: confirmedDifficultyLabel ? ` · ${confirmedDifficultyLabel}` : "" })}</span></div>
      <button type="button" disabled={applyLocked} onClick={onApplySelection}>{applying ? t("dashboard.header.selectionChecking") : t("dashboard.header.useInD2R")}</button>
    </div>}
    {selectionError && <p className="dashboard-inline-error" role="alert">{selectionError}</p>}
  </>;
}
