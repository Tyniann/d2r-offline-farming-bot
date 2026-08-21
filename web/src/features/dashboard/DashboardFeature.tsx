import { useState } from "react";
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
}

/** DashboardFeature coordinates the idle dashboard while App keeps Core commands and navigation. */
export function DashboardFeature(props: Props) {
  const {
    status, catalog, connection, error, selectionError, character, confirmedDifficultyLabel,
    appDifficultyLabel, selectionNeedsApply, needsFirstRoute, liveLocked, compatibilityState, selectionRuns,
    configuredQueue, queueStartLocked, selectionApplyLocked, applying, commandPending, startFailureText,
    queueWarning, inputNotReady, routeWorkflowBusy, hotkeys, onOpenRoutes, onRefresh,
    onApplySelection, onStartQueue,
  } = props;
  const [setupCharacter, setSetupCharacter] = useState("");
  const viewedCatalogEntry = catalog?.characters.find((entry) => entry.name === character);
  const viewedFarmReadyBlocked = !!viewedCatalogEntry && viewedCatalogEntry.selectable && !viewedCatalogEntry.farm_ready;
  const activeRun = (selectionRuns ?? catalog?.runs ?? []).find((entry) => entry.run_id === status?.active_run_id);
  const activeRunName = activeRun ? dashboardRunName(activeRun.run_id, activeRun.display_name) : undefined;

  return <div className="dashboard-feature">
    <DashboardHeader
      status={status}
      connection={connection}
      character={character}
      difficultyLabel={appDifficultyLabel}
      confirmedDifficultyLabel={confirmedDifficultyLabel}
      queueSize={configuredQueue.length}
      selectionNeedsApply={selectionNeedsApply}
      selectionError={selectionError}
      applyLocked={selectionApplyLocked}
      applying={applying}
      onApplySelection={onApplySelection}
      activeRunName={activeRunName}
    />

    {status && !["idle", "idle_in_game", "stopped_error"].includes(status.state) && <ActiveRunPanel status={status} runName={activeRunName} hotkeys={hotkeys} />}

    {needsFirstRoute && <section className="first-route-cta"><div><p className="eyebrow">Einrichtung fortsetzen</p><h2>Erste Route aufnehmen</h2><p>Für den bestätigten Kontext fehlt noch eine verwendbare Farming-Route.</p></div><Button onClick={() => onOpenRoutes("countess")}>Erste Route aufnehmen</Button></section>}
    {error && <StateMessage kind="error" title="Statusabfrage fehlgeschlagen">{error}</StateMessage>}
    {!error && !status && <StateMessage kind="loading" title="Verbindung wird hergestellt">Die lokale App wird kontaktiert.</StateMessage>}
    {status && liveLocked && <section className="compatibility-block" role="alert" aria-labelledby="compatibility-title"><CircleAlert aria-hidden="true" size={28} /><div><h2 id="compatibility-title">D2R ist nicht bereit</h2><p>Prüfe, ob D2R läuft und die unterstützte Version verwendet. Einstellungen, Historie und Diagnose bleiben verfügbar.</p><small>{compatibilityMessage(compatibilityState)}</small></div></section>}
    {catalog && viewedCatalogEntry && !viewedCatalogEntry.selectable && !(viewedCatalogEntry.reasons ?? []).includes("character_class_unsupported") && status && <StateMessage kind="error" title={`${viewedCatalogEntry.name} ist noch nicht eingerichtet`}>{characterAvailabilityText(viewedCatalogEntry, catalog)} <Button variant="secondary" onClick={() => setSetupCharacter(viewedCatalogEntry.name)}>Charakter einrichten</Button></StateMessage>}
    {viewedFarmReadyBlocked && <StateMessage kind="error" title="Charakter nicht farmbereit">{(viewedCatalogEntry.farm_ready_reasons ?? []).map((reason) => farmReadyReasonText(reason)).join(" ") || "Tasten oder Inventarschutz fehlen noch."} <a href="#settings">Charaktereinstellungen öffnen</a>{status && <Button variant="secondary" onClick={() => setSetupCharacter(viewedCatalogEntry.name)}>Charakter einrichten</Button>}</StateMessage>}
    {setupCharacter && status && catalog && <CharacterSetupWizard character={setupCharacter} catalog={catalog} status={status} mode="dashboard" onChanged={onRefresh} />}

    {startFailureText && <StateMessage kind="error" title="Farming konnte nicht gestartet werden">{startFailureText}</StateMessage>}
    {queueWarning && <StateMessage kind="error" title="Hinweis zur Run-Reihenfolge">{queueWarning}</StateMessage>}
    {inputNotReady && !startFailureText && <StateMessage kind="error" title="Spielsteuerung nicht bereit">Prüfe Freigabe, Pause und Notstopp.</StateMessage>}
    {routeWorkflowBusy && <StateMessage kind="error" title="Routenvorgang aktiv">Beende zuerst den Routenvorgang.</StateMessage>}

    <DashboardStats character={character} difficulty={props.difficulty} runNames={Object.fromEntries((catalog?.runs ?? selectionRuns ?? []).map((run) => [run.run_id, dashboardRunName(run.run_id, run.display_name)]))} farming={<FarmingPanel
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

function compatibilityMessage(state: string): string {
  return ({ not_detected: "D2R wurde nicht gefunden.", incompatible: "Diese D2R-Version wird nicht unterstützt.", unreadable: "Die D2R-Version konnte nicht gelesen werden." } as Record<string, string>)[state] ?? "D2R ist nicht bereit.";
}
