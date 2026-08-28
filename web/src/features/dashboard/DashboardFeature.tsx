import { useState } from "react";
import { useTranslation } from "react-i18next";
import { CircleAlert } from "lucide-react";
import type { LiveConnectionState } from "../../api/client";
import type { CatalogDTO, RunCatalogEntry, StatusDTO } from "../../api/generated";
import { Button, StateMessage } from "../../app/ui";
import { characterAvailabilityText } from "../../app/characterReasons";
import { CharacterSetupWizard } from "../characters/CharacterSetupWizard";
import { farmReadyReasonText } from "../characters/characterReasonText";
import { DashboardHeader } from "./DashboardHeader";
import { DashboardStats } from "./DashboardStats";
import { FarmingPanel } from "./FarmingPanel";
import { ActiveRunPanel } from "./ActiveRunPanel";
import { dashboardRunName } from "./dashboardText";
import "./dashboard.css";
import type { AppTranslator } from "../../i18n/presenters";

interface Props {
  status: StatusDTO | null;
  catalog: CatalogDTO | null;
  connection: LiveConnectionState;
  error: string;
  selectionError: string;
  character: string;
  difficulty: string;
  confirmedDifficultyLabel: string;
  appDifficultyLabel: string;
  draftDiffers: boolean;
  selectionNeedsApply: boolean;
  needsFirstRoute: boolean;
  liveLocked: boolean;
  compatibilityState: string;
  selectionRuns: RunCatalogEntry[] | null;
  configuredQueue: string[];
  queueStartLocked: boolean;
  selectionApplyLocked: boolean;
  applying: boolean;
  commandPending: boolean;
  hotkeys: { pause: string; stopAfterRun: string; emergencyStop: string };
  startFailureText: string;
  queueWarning: string;
  inputNotReady: boolean;
  routeWorkflowBusy: boolean;
  onOpenRoutes(runID: string): void;
  onRefresh(): Promise<void>;
  onApplySelection(): void;
  onStartQueue(): void;
  onResumeQueue(): void;
}

/** DashboardFeature coordinates the idle dashboard while App keeps Core commands and navigation. */
export function DashboardFeature(props: Props) {
  const { t } = useTranslation();
  const {
    status, catalog, connection, error, selectionError, character, confirmedDifficultyLabel,
    appDifficultyLabel, selectionNeedsApply, needsFirstRoute, liveLocked, compatibilityState, selectionRuns,
    configuredQueue, queueStartLocked, selectionApplyLocked, applying, commandPending, startFailureText,
    queueWarning, inputNotReady, routeWorkflowBusy, hotkeys, onOpenRoutes, onRefresh,
    onApplySelection, onStartQueue, onResumeQueue,
  } = props;
  const [setupCharacter, setSetupCharacter] = useState("");
  const viewedCatalogEntry = catalog?.characters.find((entry) => entry.name === character);
  const viewedFarmReadyBlocked = !!viewedCatalogEntry && viewedCatalogEntry.selectable && !viewedCatalogEntry.farm_ready;
  const activeRun = (selectionRuns ?? catalog?.runs ?? []).find((entry) => entry.run_id === status?.active_run_id);
  const nextRunID = status?.state === "paused_between_runs" ? status.queue.entries[status.queue.index] : undefined;
  const activeRunName = activeRun ? dashboardRunName(activeRun.run_id, t) : undefined;
  const nextRunName = nextRunID ? dashboardRunName(nextRunID, t) : undefined;

  return <div className="dashboard-feature">
    <DashboardHeader
      status={status}
      connection={connection}
      character={character}
      difficultyLabel={appDifficultyLabel}
      confirmedDifficultyLabel={confirmedDifficultyLabel}
      selectionNeedsApply={selectionNeedsApply}
      selectionError={selectionError}
      applyLocked={selectionApplyLocked}
      applying={applying}
      onApplySelection={onApplySelection}
      activeRunName={activeRunName}
    />

    {status && !["idle", "idle_in_game", "stopped_error"].includes(status.state) && <ActiveRunPanel status={status} runName={activeRunName ?? nextRunName} hotkeys={hotkeys} commandPending={commandPending} resumeLocked={commandPending || liveLocked} onResume={onResumeQueue} />}

    {needsFirstRoute && <section className="first-route-cta"><div><p className="eyebrow">{t("dashboard.setup.continue")}</p><h2>{t("dashboard.setup.firstRouteTitle")}</h2><p>{t("dashboard.setup.firstRouteDetail")}</p></div><Button onClick={() => onOpenRoutes("countess")}>{t("dashboard.setup.firstRouteTitle")}</Button></section>}
    {error && <StateMessage kind="error" title={t("dashboard.setup.statusFailed")}>{error}</StateMessage>}
    {!error && !status && <StateMessage kind="loading" title={t("dashboard.setup.connectingTitle")}>{t("dashboard.setup.connectingDetail")}</StateMessage>}
    {status && liveLocked && <section className="compatibility-block" role="alert" aria-labelledby="compatibility-title"><CircleAlert aria-hidden="true" size={28} /><div><h2 id="compatibility-title">{t("dashboard.setup.d2rNotReady")}</h2><p>{t("dashboard.setup.d2rNotReadyDetail")}</p><small>{compatibilityMessage(compatibilityState, t)}</small></div></section>}
    {catalog && viewedCatalogEntry && !viewedCatalogEntry.selectable && !(viewedCatalogEntry.reasons ?? []).includes("character_class_unsupported") && status && <StateMessage kind="error" title={t("dashboard.setup.characterNotConfigured", { character: viewedCatalogEntry.name })}>{characterAvailabilityText(viewedCatalogEntry, catalog, t)} <Button variant="secondary" onClick={() => setSetupCharacter(viewedCatalogEntry.name)}>{t("dashboard.setup.configureCharacter")}</Button></StateMessage>}
    {viewedFarmReadyBlocked && <StateMessage kind="error" title={t("dashboard.setup.characterNotFarmReady")}>{(viewedCatalogEntry.farm_ready_reasons ?? []).map((reason) => farmReadyReasonText(reason, t)).join(" ") || t("dashboard.setup.bindingsOrInventoryMissing")} <a href="#settings">{t("dashboard.setup.openCharacterSettings")}</a>{status && <Button variant="secondary" onClick={() => setSetupCharacter(viewedCatalogEntry.name)}>{t("dashboard.setup.configureCharacter")}</Button>}</StateMessage>}
    {setupCharacter && status && catalog && <CharacterSetupWizard character={setupCharacter} catalog={catalog} status={status} mode="dashboard" onChanged={onRefresh} />}

    {startFailureText && <StateMessage kind="error" title={t("dashboard.setup.farmingStartFailed")}>{startFailureText}</StateMessage>}
    {queueWarning && <StateMessage kind="error" title={t("dashboard.setup.queueNotice")}>{queueWarning}</StateMessage>}
    {inputNotReady && !startFailureText && <StateMessage kind="error" title={t("dashboard.setup.inputNotReady")}>{t("dashboard.setup.inputNotReadyDetail")}</StateMessage>}
    {routeWorkflowBusy && <StateMessage kind="error" title={t("dashboard.setup.routeWorkflowActive")}>{t("dashboard.setup.routeWorkflowActiveDetail")}</StateMessage>}

    <DashboardStats character={character} difficulty={props.difficulty} runNames={Object.fromEntries((catalog?.runs ?? selectionRuns ?? []).map((run) => [run.run_id, dashboardRunName(run.run_id, t)]))} farming={<FarmingPanel
        status={status}
        character={character}
        difficulty={props.difficulty}
        queue={configuredQueue}
        runs={selectionRuns}
        expectedClass={viewedCatalogEntry?.expected_class}
        startLocked={queueStartLocked}
        commandPending={commandPending}
        onStart={onStartQueue}
      />} />
  </div>;
}

function compatibilityMessage(state: string, t: AppTranslator): string {
  const keys = { not_detected: "dashboard.setup.compatibilityNotDetected", incompatible: "dashboard.setup.compatibilityIncompatible", unreadable: "dashboard.setup.compatibilityUnreadable" } as const;
  return t(keys[state as keyof typeof keys] ?? "dashboard.setup.compatibilityFallback");
}
