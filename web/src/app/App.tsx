import { useCallback, useEffect, useRef, useState } from "react";
import { CircleArrowUp, History, LayoutDashboard, Map, Settings, SlidersHorizontal, Wifi, WifiOff } from "lucide-react";
import {
  applySelection, connectLiveEvents, consumeBootstrapToken,
  previewSelection, resumeQueue, startQueue, validateQueue, type LiveConnectionState,
} from "../api/client";
import { getCatalog, getOperatorSettings, getRunAvailabilities, getRouteWorkflow, getStatus, type CatalogDTO, type LiveEvent, type OperatorSettingsDTO, type RouteWorkflowDTO, type RunCatalogEntry, type SelectionPreviewDTO, type StatusDTO } from "../api/generated";
import "./app.css";
import { RouteFeature } from "../features/routes/RouteFeature";
import { PickitFeature } from "../features/pickit/PickitFeature";
import { HistoryFeature } from "../features/history/HistoryFeature";
import { SettingsFeature } from "../features/settings/SettingsFeature";
import { ProvisioningFeature } from "../features/onboarding/ProvisioningFeature";
import { OnboardingFeature } from "../features/onboarding/OnboardingFeature";
import { clearOnboardingResume, readOnboardingResumeStep } from "../features/onboarding/onboardingResume";
import { targetFromHash, type AppTarget } from "./navigation";
import { Button, Dialog, PageHeader, StateMessage, StatusBadge } from "./ui";
import { isRunStartable, queueStartErrorText, selectionErrorText } from "./runReasons";
import { terminalWorkflowStates } from "../features/routes/routePresentation";
import { DashboardFeature } from "../features/dashboard/DashboardFeature";
import { SessionSummaryDialog, sessionSummaryFromTransition } from "../features/dashboard/SessionSummaryDialog";
import { AppSelectionProvider, useAppSelectionState } from "./AppSelectionContext";
import { LanguageSwitcher } from "./LanguageSwitcher";
import { useTranslation } from "react-i18next";
import { presentApiError, presentDifficultyName, presentProblem, presentRunName } from "../i18n/presenters";

const editableStates = new Set(["idle", "idle_in_game", "stopped_error"]);
const navigation = [
  { target: "dashboard", icon: LayoutDashboard },
  { target: "routes", icon: Map },
  { target: "pickit", icon: SlidersHorizontal },
  { target: "history", icon: History },
  { target: "settings", icon: Settings },
] as const;

export function App() {
  const { t } = useTranslation();
  const bridge = window.d2rDesktop;
  const [provisioning, setProvisioning] = useState<boolean | null>(() => bridge?.getProvisioningState ? null : false);

  useEffect(() => {
    if (!bridge?.getProvisioningState) return;
    let active = true;
    void bridge.getProvisioningState()
      .then((state) => { if (active) setProvisioning(state.required); })
      .catch(() => { if (active) setProvisioning(false); });
    return () => { active = false; };
  }, [bridge]);

  if (provisioning === null) return <main className="provisioning-shell"><StateMessage kind="loading" title={t("app.provisioningCheck")} /></main>;
  if (provisioning) return <ProvisioningFeature />;
  return <CoreApp />;
}

function CoreApp() {
  const { t } = useTranslation();
  const [onboardingStep, setOnboardingStep] = useState(() => readOnboardingResumeStep(8));
  const [target, setTarget] = useState<AppTarget>(() => targetFromHash(window.location.hash));
  const [status, setStatus] = useState<StatusDTO | null>(null);
  const [catalog, setCatalog] = useState<CatalogDTO | null>(null);
  const [operatorSettings, setOperatorSettings] = useState<OperatorSettingsDTO | null>(null);
  const [selectionRuns, setSelectionRuns] = useState<RunCatalogEntry[] | null>(null);
  const [routeWorkflow, setRouteWorkflow] = useState<RouteWorkflowDTO | null>(null);
  const [desktopSettings, setDesktopSettings] = useState<DesktopSettingsView | null>(() => window.d2rDesktop?.getDesktopSettings
    ? null
    : { schema_version: 3, language: "de", autostart: false, onboarding_completed: true });
  const [events, setEvents] = useState<LiveEvent[]>([]);
  const [connection, setConnection] = useState<LiveConnectionState>("connecting");
  const [applying, setApplying] = useState(false);
  const [commandPending, setCommandPending] = useState(false);
  const commandLock = useRef(false);
  const [error, setError] = useState("");
  const [selectionError, setSelectionError] = useState("");
  const [queueError, setQueueError] = useState("");
  const [queueWarning, setQueueWarning] = useState("");
  const [preview, setPreview] = useState<SelectionPreviewDTO | null>(null);
  const [routeRefreshKey, setRouteRefreshKey] = useState(0);
  const [pickitRefreshKey, setPickitRefreshKey] = useState(0);
  const [historyRefreshKey, setHistoryRefreshKey] = useState(0);
  const [sessionSummary, setSessionSummary] = useState<{ sessionID: string; durationMs: number } | null>(null);
  const previousCoreState = useRef<string | undefined>(undefined);
  const [onboardingOpen, setOnboardingOpen] = useState(false);
  const [routeOpenedFromOnboarding, setRouteOpenedFromOnboarding] = useState(false);
  const [preferredRecordingRun, setPreferredRecordingRun] = useState("countess");
  const [updateAvailable, setUpdateAvailable] = useState(false);
  const [settingsDirty, setSettingsDirty] = useState(false);
  const [pendingNav, setPendingNav] = useState<AppTarget | null>(null);
  const contentRef = useRef<HTMLElement>(null);
  const settingsDirtyRef = useRef(false);
  const targetRef = useRef(target);
  targetRef.current = target;
  settingsDirtyRef.current = settingsDirty;
  const persistAppSelection = useCallback((selection: { character: string; difficulty: string }) => {
    if (!window.d2rDesktop) return;
    void window.d2rDesktop.updateDesktopSettings({
      selected_character: selection.character,
      selected_difficulty: selection.difficulty,
    }).then(setDesktopSettings).catch(() => setSelectionError(t("app.selectionSaveFailed")));
  }, [t]);
  const appSelection = useAppSelectionState(
    catalog,
    status,
    desktopSettings === null ? undefined : { character: desktopSettings.selected_character, difficulty: desktopSettings.selected_difficulty },
    persistAppSelection,
  );
  const { character, difficulty, selectCharacter, selectDifficulty } = appSelection;

  useEffect(() => {
    clearOnboardingResume();
  }, []);

  useEffect(() => {
    if (!window.location.hash) window.history.replaceState(null, "", "#dashboard");
    const syncTarget = () => {
      const next = targetFromHash(window.location.hash);
      if (settingsDirtyRef.current && targetRef.current === "settings" && next !== "settings") {
        window.history.replaceState(null, "", "#settings");
        setPendingNav(next);
        return;
      }
      setTarget(next);
      document.title = t("app.documentTitle", { page: t(`navigation.${next}`) });
    };
    syncTarget();
    window.addEventListener("hashchange", syncTarget);
    return () => window.removeEventListener("hashchange", syncTarget);
  }, []);

  useEffect(() => {
    if (!window.d2rDesktop) return;
    let active = true;
    void window.d2rDesktop.getDesktopSettings().then((settings) => {
      if (!active) return;
      setDesktopSettings(settings);
      if (!settings.onboarding_completed) setOnboardingOpen(true);
    }).catch(() => {
      if (active) setDesktopSettings({ schema_version: 3, language: "de", autostart: false, onboarding_completed: true });
    });
    return () => { active = false; };
  }, [t]);

  useEffect(() => window.d2rDesktop?.onNavigate((next) => {
    const resolved = targetFromHash(`#${next}`);
    if (settingsDirtyRef.current && targetRef.current === "settings" && resolved !== "settings") {
      setPendingNav(resolved);
      return;
    }
    if (window.location.hash !== `#${resolved}`) window.location.hash = resolved;
    else setTarget(resolved);
  }), []);

  const discardSettingsAndNavigate = () => {
    if (!pendingNav) return;
    const next = pendingNav;
    setPendingNav(null);
    setSettingsDirty(false);
    settingsDirtyRef.current = false;
    if (window.location.hash !== `#${next}`) window.location.hash = next;
    else setTarget(next);
  };

  useEffect(() => {
    void window.d2rDesktop?.getUpdateStatus?.().then((value) => setUpdateAvailable(value.status === "available"));
    return window.d2rDesktop?.onUpdateStatus?.((value) => setUpdateAvailable(value.status === "available"));
  }, []);

  useEffect(() => {
    contentRef.current?.focus({ preventScroll: true });
  }, [target]);

  useEffect(() => {
    consumeBootstrapToken();
    const controller = new AbortController();
    let disconnect = () => {};
    let refreshing = false;
    let refreshPending = false;

    const reportError = (reason: unknown) => {
      if (!controller.signal.aborted) setError(presentApiError(reason, t, t("app.statusFailed")));
    };
    const refreshStatus = async () => {
      if (refreshing) {
        refreshPending = true;
        return;
      }
      refreshing = true;
      try {
        do {
          refreshPending = false;
          setStatus(await getStatus(controller.signal));
          setError("");
        } while (refreshPending && !controller.signal.aborted);
      } catch (reason: unknown) {
        reportError(reason);
      } finally {
        refreshing = false;
      }
    };

    void Promise.all([
      getStatus(controller.signal),
      getCatalog(controller.signal),
      getRouteWorkflow(controller.signal),
      getOperatorSettings(controller.signal).catch(() => null),
    ]).then(([nextStatus, nextCatalog, nextWorkflow, nextSettings]) => {
      if (controller.signal.aborted) return;
      setStatus(nextStatus);
      setCatalog(nextCatalog);
      setRouteWorkflow(nextWorkflow);
      setOperatorSettings(nextSettings);
      disconnect = connectLiveEvents(
        (data) => setStatus(data as StatusDTO),
        (data) => {
          setEvents((current) => [data as LiveEvent, ...current].slice(0, 40));
          if ((data as LiveEvent).event.startsWith("route_")) {
            setRouteRefreshKey((value) => value + 1);
            void getCatalog(controller.signal).then(setCatalog).catch(reportError);
            void getRouteWorkflow(controller.signal).then(setRouteWorkflow).catch(reportError);
          }
          if ((data as LiveEvent).event === "catalog_changed") {
            void getCatalog(controller.signal).then(setCatalog).catch(reportError);
          }
          if ((data as LiveEvent).event === "operator_settings_changed") {
            void getOperatorSettings(controller.signal).then(setOperatorSettings).catch(() => setOperatorSettings(null));
          }
          if ((data as LiveEvent).event.startsWith("pickit_")) setPickitRefreshKey((value) => value + 1);
          if ((data as LiveEvent).event === "history_changed") setHistoryRefreshKey((value) => value + 1);
          void refreshStatus();
        },
        setConnection,
      );
    }).catch(reportError);
    return () => { controller.abort(); disconnect(); };
  }, [t]);

  useEffect(() => {
    const selectedCharacter = character;
    const selectedDifficulty = difficulty;
    if (!selectedCharacter || !selectedDifficulty) {
      setSelectionRuns(null);
      return;
    }
    const controller = new AbortController();
    setSelectionRuns(null);
    void getRunAvailabilities(selectedCharacter, selectedDifficulty, controller.signal)
      .then((value) => { if (!controller.signal.aborted) setSelectionRuns(value.runs ?? []); })
      .catch(() => { if (!controller.signal.aborted) setSelectionRuns([]); });
    return () => controller.abort();
  }, [character, difficulty, catalog?.revision, routeRefreshKey]);

  useEffect(() => {
    const before = previousCoreState.current;
    previousCoreState.current = status?.state;
    if (status && !editableStates.has(status.state)) {
      setSessionSummary(null);
      return;
    }
    const next = sessionSummaryFromTransition(before, status);
    if (next) setSessionSummary(next);
  }, [status]);

  const refreshAfterCommand = async () => {
    const [nextStatus, nextCatalog, nextWorkflow, nextSettings] = await Promise.all([
      getStatus(),
      getCatalog(),
      getRouteWorkflow(),
      getOperatorSettings().catch(() => null),
    ]);
    setStatus(nextStatus);
    setCatalog(nextCatalog);
    setRouteWorkflow(nextWorkflow);
    setOperatorSettings(nextSettings);
  };

  const applyPreview = async (selectionPreview: SelectionPreviewDTO) => {
    if (!catalog || !status) return;
    setApplying(true);
    setSelectionError("");
    try {
      await applySelection(selectionPreview.character, selectionPreview.new_difficulty, catalog.revision, status.generation, selectionPreview.confirmation_token);
      await refreshAfterCommand();
      appSelection.confirm({ character: selectionPreview.character, difficulty: selectionPreview.new_difficulty });
      setPreview(null);
    } catch (reason: unknown) {
      setSelectionError(selectionErrorText(reason, t));
    } finally {
      setApplying(false);
    }
  };

  const submitSelection = async () => {
    if (!catalog || !status || !character || !difficulty) return;
    setApplying(true);
    setSelectionError("");
    try {
      const nextPreview = await previewSelection(character, difficulty, catalog.revision);
      if (nextPreview.requires_confirmation) setPreview(nextPreview);
      else {
        setApplying(false);
        await applyPreview(nextPreview);
        return;
      }
    } catch (reason: unknown) {
      setSelectionError(selectionErrorText(reason, t, t("app.previewFailed")));
    } finally {
      setApplying(false);
    }
  };

  const runCommand = async (action: () => Promise<unknown>) => {
    if (commandLock.current) return;
    commandLock.current = true;
    setCommandPending(true);
    setQueueError("");
    try {
      await action();
      await refreshAfterCommand();
    } catch (reason: unknown) {
      setQueueError(queueStartErrorText(reason, t));
    } finally {
      commandLock.current = false;
      setCommandPending(false);
    }
  };

  const submitQueue = async () => {
    if (!status || !catalog || !status.selection.character || !status.selection.difficulty) return;
    const entries = configuredQueue;
    if (entries.length === 0) return;
    await runCommand(async () => {
      setQueueWarning("");
      const validation = await validateQueue(entries, status.selection.character!, status.selection.difficulty!, catalog.revision);
      if ((validation.warnings ?? []).includes("inventory_layout_unsuitable_for_cows")) {
        setQueueWarning(t("app.inventoryWarning"));
      }
      await startQueue(entries, status.selection.character!, status.selection.difficulty!, catalog.revision, status.generation);
    });
  };

  const submitResume = async () => {
    if (!status || status.state !== "paused_between_runs") return;
    await runCommand(async () => {
      await resumeQueue(status.generation);
    });
  };

  const sessionSelectionLocked = !!status && !editableStates.has(status.state);
  const selectionLocked = applying || commandPending || sessionSelectionLocked;
  const confirmedSelection = !!status?.selection.character && !!status.selection.difficulty;
  const draftDiffers = confirmedSelection && (character !== status?.selection.character || difficulty !== status?.selection.difficulty);
  const selectionNeedsApply = !!character && (!confirmedSelection || draftDiffers);
  const farmCatalogEntry = catalog?.characters.find((entry) => entry.name === character);
  const storedFarmSettings = storedCharacterSettings(operatorSettings, catalog, character);
  const configuredQueue = storedFarmSettings?.queue ?? (status?.queue.default_entries ?? []);
  const compatibilityState = status?.compatibility?.state ?? "not_detected";
  const liveLocked = compatibilityState !== "compatible";
  const farmReadyBlocked = !!farmCatalogEntry && farmCatalogEntry.selectable && !farmCatalogEntry.farm_ready;
  const inputHandoff = commandPending || (!!status && !editableStates.has(status.state));
  const inputNotReady = !!status && !inputHandoff && (!!status.input.paused || !!status.input.stopped || !status.input.enabled);
  const routeWorkflowBusy = !!routeWorkflow && !terminalWorkflowStates.has(routeWorkflow.state);
  const queueEntriesStartable = configuredQueue.length > 0 && (selectionRuns ?? []).length > 0
    && configuredQueue.every((runID) => {
      const run = (selectionRuns ?? []).find((entry) => entry.run_id === runID);
      return !!run && isRunStartable(run.status);
    });
  const startFailureText = queueError || (status?.state === "stopped_error" && status.last_error && status.last_error.code !== "runtime_read_failed"
    ? presentProblem(status.last_error, t)
    : "");
  const queueStartLocked = !status || !catalog || !editableStates.has(status.state) || commandPending || liveLocked
    || farmReadyBlocked || inputNotReady || routeWorkflowBusy || !queueEntriesStartable || selectionRuns === null || draftDiffers;
  const effectiveSelectionLocked = selectionLocked || liveLocked || inputNotReady;
  const selectionApplyLocked = effectiveSelectionLocked || !character || (!draftDiffers && confirmedSelection)
    || (status?.state !== "idle" && status?.state !== "idle_in_game" && status?.state !== "stopped_error");
  const confirmedDifficultyLabel = status?.selection.difficulty ? presentDifficultyName(status.selection.difficulty, t) : "";
  const appDifficultyLabel = presentDifficultyName(difficulty, t);
  const needsFirstRoute = confirmedSelection && !draftDiffers && selectionRuns !== null && selectionRuns.length > 0
    && !selectionRuns.some((run) => isRunStartable(run.status));
  const openRoutes = (runID = "countess") => {
    setPreferredRecordingRun(runID);
    setRouteOpenedFromOnboarding(true);
    setOnboardingOpen(false);
    if (window.location.hash !== "#routes") window.location.hash = "routes";
    else setTarget("routes");
  };
  const returnToOnboarding = () => {
    setOnboardingStep(7);
    setRouteOpenedFromOnboarding(false);
    setOnboardingOpen(true);
    if (window.location.hash !== "#dashboard") window.location.hash = "dashboard";
    else setTarget("dashboard");
  };

  return (
    <AppSelectionProvider value={appSelection}>
    <div className="app-shell">
      <aside className="sidebar">
        <a className="brand" href="#dashboard" aria-label={t("sidebar.brandDashboard")}>
          <img src="./portal-mark.png" alt="" width="46" height="46" />
          <span><strong>D2R Offline</strong><small>Farming Bot</small></span>
        </a>
        <nav className="main-navigation" aria-label={t("sidebar.mainNavigation")}>
          {navigation.map(({ target: itemTarget, icon: Icon }) => <a key={itemTarget} href={`#${itemTarget}`} aria-current={target === itemTarget ? "page" : undefined}><Icon aria-hidden="true" size={19} /><span>{t(`navigation.${itemTarget}`)}</span></a>)}
        </nav>
        <div className="sidebar-context">
          <strong>{t("sidebar.selectedCharacter")}</strong>
          <label>{t("sidebar.character")}<select value={character} onChange={(event) => {
            const next = event.target.value;
            selectCharacter(next, storedCharacterSettings(operatorSettings, catalog, next)?.last_difficulty);
          }} disabled={effectiveSelectionLocked}>{catalog?.characters.map((entry) => <option key={entry.slug} value={entry.name} disabled={!entry.selectable}>{entry.name}{entry.selectable ? "" : ` – ${t("sidebar.unavailable")}`}</option>)}</select></label>
          <label>{t("sidebar.difficulty")}<select value={difficulty} onChange={(event) => selectDifficulty(event.target.value)} disabled={effectiveSelectionLocked}>{catalog?.difficulties.map((entry) => <option key={entry.id} value={entry.id}>{presentDifficultyName(entry.id, t)}</option>)}</select></label>
          {sessionSelectionLocked && <small>{t("sidebar.lockedDuringSession")}</small>}
          {!sessionSelectionLocked && confirmedSelection && !draftDiffers && <small className="sidebar-context-confirmed">{t("sidebar.activeInD2R")}</small>}
          {draftDiffers && <small className="sidebar-context-pending">{t("sidebar.notActiveInD2R")}</small>}
        </div>
        <div className="sidebar-meta">
          <LanguageSwitcher />
          <StatusBadge tone={connection === "connected" ? "success" : "danger"} icon={connection === "connected" ? Wifi : WifiOff}>{t(connection === "connected" ? "sidebar.connected" : connection === "connecting" ? "sidebar.connecting" : "sidebar.disconnected")}</StatusBadge>
          {updateAvailable && <a href="#settings" aria-label={t("sidebar.updateAvailableAria")}><StatusBadge tone="warning" icon={CircleArrowUp}>{t("sidebar.updateAvailable")}</StatusBadge></a>}
          <small>{t("sidebar.version", { version: status?.app_version ?? "–" })}</small>
        </div>
      </aside>

      <main ref={contentRef} id="app-content" className="app-content" tabIndex={-1}>
        {onboardingOpen && status && catalog && <OnboardingFeature status={status} catalog={catalog} initialStep={onboardingStep} onRefresh={refreshAfterCommand} onClose={() => { setRouteOpenedFromOnboarding(false); setOnboardingOpen(false); }} onOpenRoutes={openRoutes} />}
        {!onboardingOpen && <>{target === "dashboard" && <DashboardFeature
          status={status}
          catalog={catalog}
          connection={connection}
          error={error}
          selectionError={selectionError}
          character={character}
          difficulty={difficulty}
          confirmedDifficultyLabel={confirmedDifficultyLabel}
          appDifficultyLabel={appDifficultyLabel}
          draftDiffers={draftDiffers}
          selectionNeedsApply={selectionNeedsApply}
          needsFirstRoute={needsFirstRoute}
          liveLocked={liveLocked}
          compatibilityState={compatibilityState}
          selectionRuns={selectionRuns}
          configuredQueue={configuredQueue}
          queueStartLocked={queueStartLocked}
          selectionApplyLocked={selectionApplyLocked}
          applying={applying}
          commandPending={commandPending}
          hotkeys={{
            pause: operatorSettings?.input.pause_hotkey ?? "Pause",
            stopAfterRun: operatorSettings?.input.stop_after_run_hotkey ?? "F10",
            emergencyStop: operatorSettings?.input.emergency_stop_hotkey ?? "F11",
          }}
          startFailureText={startFailureText}
          queueWarning={queueWarning}
          inputNotReady={inputNotReady}
          routeWorkflowBusy={routeWorkflowBusy}
          onOpenRoutes={openRoutes}
          onRefresh={refreshAfterCommand}
          onApplySelection={() => void submitSelection()}
          onStartQueue={() => void submitQueue()}
          onResumeQueue={() => void submitResume()}
        />}

        {target === "routes" && <>{liveLocked && <StateMessage kind="error" title={t("app.routesLockedTitle")}>{t("app.routesLockedDetail")}</StateMessage>}<RouteFeature characters={catalog?.characters.map((entry) => entry.name) ?? []} selectedCharacter={character} onSelectedCharacterChange={(next) => selectCharacter(next, storedCharacterSettings(operatorSettings, catalog, next)?.last_difficulty)} refreshKey={routeRefreshKey} liveLocked={liveLocked} preferredRecordingRun={routeOpenedFromOnboarding ? preferredRecordingRun : ""} onReturnToOnboarding={routeOpenedFromOnboarding ? returnToOnboarding : undefined} /></>}
        {target === "pickit" && <PickitFeature characters={catalog?.characters.map((entry) => entry.name) ?? []} selectedCharacter={character} onSelectedCharacterChange={(next) => selectCharacter(next, storedCharacterSettings(operatorSettings, catalog, next)?.last_difficulty)} runs={catalog?.runs.map((entry) => entry.run_id) ?? []} locked={!!status && !editableStates.has(status.state)} refreshKey={pickitRefreshKey} />}
        {target === "history" && <><PageHeader eyebrow={t("app.historyEyebrow")} title={t("navigation.history")} description={t("app.historyDescription")} /><HistoryFeature characters={catalog?.characters.map((entry) => entry.name) ?? []} selectedCharacter={character} selectedDifficulty={difficulty} onSelectedCharacterChange={(next) => selectCharacter(next, storedCharacterSettings(operatorSettings, catalog, next)?.last_difficulty)} onSelectedDifficultyChange={selectDifficulty} runs={catalog?.runs.map((entry) => entry.run_id) ?? []} refreshKey={historyRefreshKey} /></>}
        {target === "settings" && <><PageHeader eyebrow={t("app.settingsEyebrow")} title={t("navigation.settings")} description={t("app.settingsDescription")} /><SettingsFeature generation={status?.generation ?? 0} coreState={status?.state ?? ""} status={status} characters={catalog?.characters.map((entry) => entry.slug) ?? []} selectedCharacter={catalog?.characters.find((entry) => entry.name === character)?.slug ?? ""} onSelectedCharacterChange={(slug) => { const next = catalog?.characters.find((entry) => entry.slug === slug)?.name; if (next) selectCharacter(next, storedCharacterSettings(operatorSettings, catalog, next)?.last_difficulty); }} catalog={catalog} runs={catalog?.runs.map((entry) => ({ id: entry.run_id, label: presentRunName(entry.run_id, t), status: entry.status, reasons: entry.reasons, routeCombat: entry.route_combat })) ?? []} events={events} onOpenOnboarding={() => { setOnboardingStep(0); setOnboardingOpen(true); }} onSettingsApplied={() => { void refreshAfterCommand(); }} onHistoryDeleted={() => setHistoryRefreshKey((value) => value + 1)} onDirtyChange={setSettingsDirty} /></>}
        </>}
      </main>

      {preview && <Dialog title={t("app.routeInvalidationTitle")} onClose={() => !applying && setPreview(null)}><p>{t("app.routeInvalidationIntro", { oldDifficulty: preview.old_difficulty || t("app.unconfirmed"), newDifficulty: preview.new_difficulty })}</p><ul>{preview.affected_routes.map((route) => <li key={route}>{route}</li>)}</ul><p>{t("app.routeInvalidationDetail")}</p><div className="modal-actions"><Button variant="secondary" onClick={() => setPreview(null)} disabled={applying}>{t("common.cancel")}</Button><Button onClick={() => void applyPreview(preview)} disabled={applying}>{t(applying ? "app.applying" : "app.confirmApply")}</Button></div></Dialog>}
      {pendingNav && <Dialog title={t("app.unsavedTitle")} onClose={() => setPendingNav(null)}><p>{t("app.unsavedDetail")}</p><div className="modal-actions"><Button variant="secondary" onClick={() => setPendingNav(null)}>{t("app.returnToSettings")}</Button><Button variant="danger" onClick={discardSettingsAndNavigate}>{t("app.discardChanges")}</Button></div></Dialog>}
      {sessionSummary && <SessionSummaryDialog sessionID={sessionSummary.sessionID} durationMs={sessionSummary.durationMs} refreshKey={historyRefreshKey} onClose={() => setSessionSummary(null)} />}
    </div>
    </AppSelectionProvider>
  );
}

function storedCharacterSettings(settings: OperatorSettingsDTO | null, catalog: CatalogDTO | null | undefined, name: string) {
  const slug = catalog?.characters.find((entry) => entry.name === name)?.slug;
  if (!slug) return undefined;
  return settings?.characters[slug];
}
