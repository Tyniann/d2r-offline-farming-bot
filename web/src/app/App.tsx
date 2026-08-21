import { useCallback, useEffect, useRef, useState } from "react";
import { CircleArrowUp, History, LayoutDashboard, Map, Settings, SlidersHorizontal, Wifi, WifiOff } from "lucide-react";
import {
  applySelection, connectLiveEvents, consumeBootstrapToken,
  previewSelection, startQueue, validateQueue, type LiveConnectionState,
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
import { AppSelectionProvider, useAppSelectionState } from "./AppSelectionContext";

const editableStates = new Set(["idle", "idle_in_game", "stopped_error"]);
const navigation = [
  { target: "dashboard", label: "Dashboard", icon: LayoutDashboard },
  { target: "routes", label: "Routen", icon: Map },
  { target: "pickit", label: "Pickit", icon: SlidersHorizontal },
  { target: "history", label: "Historie", icon: History },
  { target: "settings", label: "Einstellungen", icon: Settings },
] as const;

export function App() {
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

  if (provisioning === null) return <main className="provisioning-shell"><StateMessage kind="loading" title="Lokaler Datenroot wird geprüft" /></main>;
  if (provisioning) return <ProvisioningFeature />;
  return <CoreApp />;
}

function CoreApp() {
  const [onboardingStep, setOnboardingStep] = useState(() => readOnboardingResumeStep(8));
  const [target, setTarget] = useState<AppTarget>(() => targetFromHash(window.location.hash));
  const [status, setStatus] = useState<StatusDTO | null>(null);
  const [catalog, setCatalog] = useState<CatalogDTO | null>(null);
  const [operatorSettings, setOperatorSettings] = useState<OperatorSettingsDTO | null>(null);
  const [selectionRuns, setSelectionRuns] = useState<RunCatalogEntry[] | null>(null);
  const [routeWorkflow, setRouteWorkflow] = useState<RouteWorkflowDTO | null>(null);
  const [desktopSettings, setDesktopSettings] = useState<DesktopSettingsView | null>(() => window.d2rDesktop?.getDesktopSettings
    ? null
    : { schema_version: 2, autostart: false, onboarding_completed: true });
  const [events, setEvents] = useState<LiveEvent[]>([]);
  const [connection, setConnection] = useState<LiveConnectionState>("wird verbunden");
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
    }).then(setDesktopSettings).catch(() => setSelectionError("App-Auswahl konnte nicht gespeichert werden."));
  }, []);
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
      document.title = `${navigation.find((entry) => entry.target === next)?.label ?? "Dashboard"} · D2R Offline Farming Bot`;
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
      if (active) setDesktopSettings({ schema_version: 2, autostart: false, onboarding_completed: true });
    });
    return () => { active = false; };
  }, []);

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
      if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : "Statusabfrage fehlgeschlagen");
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
  }, []);

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
      setSelectionError(reason instanceof Error ? selectionErrorText(reason.message) : "Auswahl fehlgeschlagen");
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
      setSelectionError(reason instanceof Error ? selectionErrorText(reason.message) : "Vorschau fehlgeschlagen");
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
      setQueueError(reason instanceof Error ? queueStartErrorText(reason.message) : "Session-Befehl fehlgeschlagen");
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
        setQueueWarning("Inventarlayout für Cow-Runs ungeeignet. Countess und andere Runs bleiben möglich.");
      }
      await startQueue(entries, status.selection.character!, status.selection.difficulty!, catalog.revision, status.generation);
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
    ? queueStartErrorText(`${status.last_error.code}: ${status.last_error.message}`)
    : "");
  const queueStartLocked = !status || !catalog || !editableStates.has(status.state) || commandPending || liveLocked
    || farmReadyBlocked || inputNotReady || routeWorkflowBusy || !queueEntriesStartable || selectionRuns === null || draftDiffers;
  const effectiveSelectionLocked = selectionLocked || liveLocked || inputNotReady;
  const selectionApplyLocked = effectiveSelectionLocked || !character || (!draftDiffers && confirmedSelection)
    || (status?.state !== "idle" && status?.state !== "idle_in_game" && status?.state !== "stopped_error");
  const confirmedDifficultyLabel = catalog?.difficulties.find((entry) => entry.id === status?.selection.difficulty)?.display_name
    ?? status?.selection.difficulty
    ?? "";
  const appDifficultyLabel = catalog?.difficulties.find((entry) => entry.id === difficulty)?.display_name ?? difficulty;
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
        <a className="brand" href="#dashboard" aria-label="D2R Offline Farming Bot – Dashboard">
          <img src="./portal-mark.png" alt="" width="46" height="46" />
          <span><strong>D2R Offline</strong><small>Farming Bot</small></span>
        </a>
        <nav className="main-navigation" aria-label="Hauptnavigation">
          {navigation.map(({ target: itemTarget, label, icon: Icon }) => <a key={itemTarget} href={`#${itemTarget}`} aria-current={target === itemTarget ? "page" : undefined}><Icon aria-hidden="true" size={19} /><span>{label}</span></a>)}
        </nav>
        <div className="sidebar-context">
          <strong>Ausgewählter Charakter</strong>
          <label>Charakter<select value={character} onChange={(event) => {
            const next = event.target.value;
            selectCharacter(next, storedCharacterSettings(operatorSettings, catalog, next)?.last_difficulty);
          }} disabled={effectiveSelectionLocked}>{catalog?.characters.map((entry) => <option key={entry.slug} value={entry.name} disabled={!entry.selectable}>{entry.name}{entry.selectable ? "" : " – nicht verfügbar"}</option>)}</select></label>
          <label>Schwierigkeit<select value={difficulty} onChange={(event) => selectDifficulty(event.target.value)} disabled={effectiveSelectionLocked}>{catalog?.difficulties.map((entry) => <option key={entry.id} value={entry.id}>{entry.display_name}</option>)}</select></label>
          {sessionSelectionLocked && <small>Während der Session gesperrt</small>}
          {!sessionSelectionLocked && confirmedSelection && !draftDiffers && <small className="sidebar-context-confirmed">In D2R aktiv</small>}
          {draftDiffers && <small className="sidebar-context-pending">Noch nicht in D2R aktiv</small>}
        </div>
        <div className="sidebar-meta">
          <StatusBadge tone={connection === "verbunden" ? "success" : "danger"} icon={connection === "verbunden" ? Wifi : WifiOff}>{connection === "verbunden" ? "App verbunden" : "Verbindung getrennt"}</StatusBadge>
          {updateAvailable && <a href="#settings" aria-label="Neue App-Version verfügbar"><StatusBadge tone="warning" icon={CircleArrowUp}>Update verfügbar</StatusBadge></a>}
          <small>Version {status?.app_version ?? "–"}</small>
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
        />}

        {target === "routes" && <>{liveLocked && <StateMessage kind="error" title="Live-Routenaktionen sind gesperrt">Die Routenbibliothek bleibt read-only, bis D2R kompatibel bestätigt ist.</StateMessage>}<RouteFeature characters={catalog?.characters.map((entry) => entry.name) ?? []} selectedCharacter={character} onSelectedCharacterChange={(next) => selectCharacter(next, storedCharacterSettings(operatorSettings, catalog, next)?.last_difficulty)} refreshKey={routeRefreshKey} liveLocked={liveLocked} preferredRecordingRun={routeOpenedFromOnboarding ? preferredRecordingRun : ""} onReturnToOnboarding={routeOpenedFromOnboarding ? returnToOnboarding : undefined} /></>}
        {target === "pickit" && <><PageHeader eyebrow="Loot-Policy" title="Pickit" description="Profile, Regeln und Zuordnungen bleiben Core-validiert und gelten erst an einer sicheren Run-Grenze." /><PickitFeature characters={catalog?.characters.map((entry) => entry.name) ?? []} selectedCharacter={character} onSelectedCharacterChange={(next) => selectCharacter(next, storedCharacterSettings(operatorSettings, catalog, next)?.last_difficulty)} runs={catalog?.runs.map((entry) => entry.run_id) ?? []} locked={!!status && !editableStates.has(status.state)} refreshKey={pickitRefreshKey} /></>}
        {target === "history" && <><PageHeader eyebrow="Auswertung" title="Historie" description="Core-berechnete Runs, Itemertrag, Vergleiche und Exporte ohne UI-eigene Aggregation." /><HistoryFeature characters={catalog?.characters.map((entry) => entry.name) ?? []} selectedCharacter={character} selectedDifficulty={difficulty} onSelectedCharacterChange={(next) => selectCharacter(next, storedCharacterSettings(operatorSettings, catalog, next)?.last_difficulty)} onSelectedDifficultyChange={selectDifficulty} runs={catalog?.runs.map((entry) => entry.run_id) ?? []} refreshKey={historyRefreshKey} /></>}
        {target === "settings" && <><PageHeader eyebrow="System" title="Einstellungen" description="Bot-Verhalten, Charaktere, App und Wartung – getrennt nach Speicherziel." /><SettingsFeature generation={status?.generation ?? 0} coreState={status?.state ?? ""} status={status} characters={catalog?.characters.map((entry) => entry.slug) ?? []} selectedCharacter={catalog?.characters.find((entry) => entry.name === character)?.slug ?? ""} onSelectedCharacterChange={(slug) => { const next = catalog?.characters.find((entry) => entry.slug === slug)?.name; if (next) selectCharacter(next, storedCharacterSettings(operatorSettings, catalog, next)?.last_difficulty); }} catalog={catalog} runs={catalog?.runs.map((entry) => ({ id: entry.run_id, label: entry.display_name, status: entry.status, reasons: entry.reasons, routeCombat: entry.route_combat })) ?? []} events={events} onOpenOnboarding={() => { setOnboardingStep(0); setOnboardingOpen(true); }} onSettingsApplied={() => { void refreshAfterCommand(); }} onHistoryDeleted={() => setHistoryRefreshKey((value) => value + 1)} onDirtyChange={setSettingsDirty} /></>}
        </>}
      </main>

      {preview && <Dialog title="Routen werden unbrauchbar" onClose={() => !applying && setPreview(null)}><p>Der Wechsel von <strong>{preview.old_difficulty || "unbestätigt"}</strong> auf <strong>{preview.new_difficulty}</strong> markiert folgende Farming-Routen als <code>stale</code>:</p><ul>{preview.affected_routes.map((route) => <li key={route}>{route}</li>)}</ul><p>Die Dateien werden nicht gelöscht oder verändert. Neue Aufnahmen sind vor Farming erforderlich.</p><div className="modal-actions"><Button variant="secondary" onClick={() => setPreview(null)} disabled={applying}>Abbrechen</Button><Button onClick={() => void applyPreview(preview)} disabled={applying}>{applying ? "Wird angewendet …" : "Auswirkungen bestätigen und anwenden"}</Button></div></Dialog>}
      {pendingNav && <Dialog title="Ungespeicherte Änderungen" onClose={() => setPendingNav(null)}><p>Deine Änderungen an der Run-Reihenfolge sind noch nicht gespeichert. Ohne Speichern startet der Bot mit der alten Reihenfolge.</p><div className="modal-actions"><Button variant="secondary" onClick={() => setPendingNav(null)}>Zurück zu den Einstellungen</Button><Button variant="danger" onClick={discardSettingsAndNavigate}>Änderungen verwerfen</Button></div></Dialog>}
    </div>
    </AppSelectionProvider>
  );
}

function storedCharacterSettings(settings: OperatorSettingsDTO | null, catalog: CatalogDTO | null | undefined, name: string) {
  const slug = catalog?.characters.find((entry) => entry.name === name)?.slug;
  if (!slug) return undefined;
  return settings?.characters[slug];
}
