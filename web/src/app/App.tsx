import { useEffect, useRef, useState } from "react";
import { CircleAlert, CircleArrowUp, History, LayoutDashboard, Map, Settings, SlidersHorizontal, Wifi, WifiOff } from "lucide-react";
import {
  applySelection, connectLiveEvents, consumeBootstrapToken, emergencyStop, pauseAfterRun,
  previewSelection, resumeQueue, startQueue, stopAfterRun, validateQueue, type LiveConnectionState,
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
import { characterAvailabilityText } from "./characterReasons";
import { CharacterSetupWizard } from "../features/characters/CharacterSetupWizard";
import { farmReadyReasonText } from "../features/characters/characterReasonText";
import { isRunStartable, queueStartErrorText, runAvailabilityText, selectionErrorText } from "./runReasons";
import { runResultReasonText } from "./runResultReasons";
import { terminalWorkflowStates } from "../features/routes/routePresentation";

const editableStates = new Set(["idle", "idle_in_game", "stopped_error"]);
const emergencyStates = new Set(["starting_game", "starting_run", "running_run", "paused_between_runs", "exiting_game"]);
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
  const [events, setEvents] = useState<LiveEvent[]>([]);
  const [connection, setConnection] = useState<LiveConnectionState>("wird verbunden");
  const [character, setCharacter] = useState("");
  const [difficulty, setDifficulty] = useState("");
  const [applying, setApplying] = useState(false);
  const [commandPending, setCommandPending] = useState(false);
  const commandLock = useRef(false);
  const [error, setError] = useState("");
  const [selectionError, setSelectionError] = useState("");
  const [queueError, setQueueError] = useState("");
  const [queueWarning, setQueueWarning] = useState("");
  const [preview, setPreview] = useState<SelectionPreviewDTO | null>(null);
  const [confirmEmergency, setConfirmEmergency] = useState(false);
  const [routeRefreshKey, setRouteRefreshKey] = useState(0);
  const [pickitRefreshKey, setPickitRefreshKey] = useState(0);
  const [historyRefreshKey, setHistoryRefreshKey] = useState(0);
  const [onboardingOpen, setOnboardingOpen] = useState(false);
  const [routeOpenedFromOnboarding, setRouteOpenedFromOnboarding] = useState(false);
  const [preferredRecordingRun, setPreferredRecordingRun] = useState("countess");
  const [updateAvailable, setUpdateAvailable] = useState(false);
  const [settingsDirty, setSettingsDirty] = useState(false);
  const [pendingNav, setPendingNav] = useState<AppTarget | null>(null);
  const [setupCharacter, setSetupCharacter] = useState("");
  const selectionEdited = useRef(false);
  const emergencyConfirmRef = useRef<HTMLButtonElement>(null);
  const contentRef = useRef<HTMLElement>(null);
  const settingsDirtyRef = useRef(false);
  const targetRef = useRef(target);
  targetRef.current = target;
  settingsDirtyRef.current = settingsDirty;

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
    let active = true;
    void window.d2rDesktop?.getDesktopSettings().then((settings) => {
      if (active && !settings.onboarding_completed) setOnboardingOpen(true);
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
    contentRef.current?.focus();
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
      const confirmed = nextStatus.selection.character;
      setCharacter(confirmed || nextCatalog.characters.find((entry) => entry.selectable)?.name || "");
      setDifficulty(nextStatus.selection.difficulty || nextCatalog.default_difficulty);
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
    const selectedCharacter = status?.selection.character;
    const selectedDifficulty = status?.selection.difficulty;
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
  }, [status?.selection.character, status?.selection.difficulty, catalog?.revision, routeRefreshKey]);

  useEffect(() => {
    if (selectionEdited.current || !status?.selection.character) return;
    setCharacter(status.selection.character);
    if (status.selection.difficulty) setDifficulty(status.selection.difficulty);
  }, [status?.selection.character, status?.selection.difficulty]);

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
      selectionEdited.current = false;
      setCharacter(selectionPreview.character);
      setDifficulty(selectionPreview.new_difficulty);
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

  const selectionLocked = applying || commandPending || (!!status && !editableStates.has(status.state));
  const confirmedSelection = !!status?.selection.character && !!status.selection.difficulty;
  const draftDiffers = confirmedSelection && (character !== status?.selection.character || difficulty !== status?.selection.difficulty);
  const farmCatalogEntry = catalog?.characters.find((entry) => entry.name === status?.selection.character);
  const storedFarmSettings = storedCharacterSettings(operatorSettings, catalog, status?.selection.character ?? "");
  const configuredQueue = storedFarmSettings?.queue ?? (status?.queue.default_entries ?? []);
  const hasPendingIntent = !!status?.pending_intent && status.pending_intent !== "none";
  const compatibilityState = status?.compatibility?.state ?? "not_detected";
  const liveLocked = compatibilityState !== "compatible";
  const farmReadyBlocked = !!farmCatalogEntry && farmCatalogEntry.selectable && !farmCatalogEntry.farm_ready;
  const viewedCatalogEntry = catalog?.characters.find((entry) => entry.name === character);
  const viewedFarmReadyBlocked = !!viewedCatalogEntry && viewedCatalogEntry.selectable && !viewedCatalogEntry.farm_ready;
  const inputHandoff = commandPending || (!!status && !editableStates.has(status.state));
  const inputNotReady = !!status && !inputHandoff && (!!status.input.paused || !!status.input.stopped || !status.input.enabled);
  const routeWorkflowBusy = !!routeWorkflow && !terminalWorkflowStates.has(routeWorkflow.state);
  const queueEntriesStartable = configuredQueue.length > 0 && (selectionRuns ?? []).length > 0
    && configuredQueue.every((runID) => {
      const run = (selectionRuns ?? []).find((entry) => entry.run_id === runID);
      return !!run && isRunStartable(run.status);
    });
  const startFailureText = queueError || (status?.state === "stopped_error" && status.last_error && status.last_error.code !== "runtime_read_failed"
    ? queueStartErrorText(status.last_error.message || status.last_error.code)
    : "");
  const queueStartLocked = !status || !catalog || !editableStates.has(status.state) || commandPending || liveLocked
    || farmReadyBlocked || inputNotReady || routeWorkflowBusy || !queueEntriesStartable || selectionRuns === null || draftDiffers;
  const effectiveSelectionLocked = selectionLocked || liveLocked || inputNotReady;
  const confirmedDifficultyLabel = catalog?.difficulties.find((entry) => entry.id === status?.selection.difficulty)?.display_name
    ?? status?.selection.difficulty
    ?? "";
  const focusedCatalogEntry = viewedCatalogEntry;
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
    <div className="app-shell">
      <aside className="sidebar">
        <a className="brand" href="#dashboard" aria-label="D2R Offline Farming Bot – Dashboard">
          <img src="/portal-mark.svg" alt="" width="46" height="46" />
          <span><strong>D2R Offline</strong><small>Farming Bot</small></span>
        </a>
        <nav className="main-navigation" aria-label="Hauptnavigation">
          {navigation.map(({ target: itemTarget, label, icon: Icon }) => <a key={itemTarget} href={`#${itemTarget}`} aria-current={target === itemTarget ? "page" : undefined}><Icon aria-hidden="true" size={19} /><span>{label}</span></a>)}
        </nav>
        <div className="sidebar-meta">
          <StatusBadge tone={connection === "verbunden" ? "success" : "danger"} icon={connection === "verbunden" ? Wifi : WifiOff}>Core {connection}</StatusBadge>
          {updateAvailable && <a href="#settings" aria-label="Neue App-Version verfügbar"><StatusBadge tone="warning" icon={CircleArrowUp}>Update verfügbar</StatusBadge></a>}
          <small>App {status?.app_version ?? "–"}</small><small>Core {status?.core_version ?? "–"}</small>
        </div>
      </aside>

      <main ref={contentRef} id="app-content" className="app-content" tabIndex={-1}>
        {onboardingOpen && status && catalog && <OnboardingFeature status={status} catalog={catalog} initialStep={onboardingStep} onRefresh={refreshAfterCommand} onClose={() => { setRouteOpenedFromOnboarding(false); setOnboardingOpen(false); }} onOpenRoutes={openRoutes} />}
        {!onboardingOpen && <>{target === "dashboard" && <>
          <PageHeader eyebrow="Betrieb" title="Lokales Dashboard" description="Core-autoritärer Überblick für Auswahl, Queue und Session. Keine Anzeige berechnet einen zweiten Fachzustand." actions={<StatusBadge tone={connection === "verbunden" ? "success" : "danger"} icon={connection === "verbunden" ? Wifi : WifiOff}>Live: {connection}</StatusBadge>} />
          {needsFirstRoute && <section className="first-route-cta"><div><p className="eyebrow">Einrichtung fortsetzen</p><h2>Erste Route aufnehmen</h2><p>Für den bestätigten Kontext fehlt noch eine verwendbare Farming-Route. Die geführte Aufnahme verwendet denselben Core-Workflow wie die Routenbibliothek.</p></div><Button onClick={() => openRoutes("countess")}>Erste Route aufnehmen</Button></section>}
          {status && liveLocked && <section className="compatibility-block" role="alert" aria-labelledby="compatibility-title"><CircleAlert aria-hidden="true" size={28} /><div><h2 id="compatibility-title">D2R-Kompatibilität blockiert Input</h2><p>Zustand <strong>{compatibilityState}</strong>{status.compatibility?.reason ? ` · ${status.compatibility.reason}` : ""}. Einstellungen, Historie und Diagnose bleiben verfügbar.</p><small>Erwartet {status.compatibility?.expected_version || "–"} · Offsets {status.compatibility?.offset_version || "–"} · Erkannt {status.compatibility?.actual_version || "–"}</small></div></section>}
          <section aria-live="polite">
            <div className="section-heading"><div><p className="eyebrow">Live-Projektion</p><h2>Core-Status</h2></div>{status && <StatusBadge tone={compatibilityState === "compatible" ? "success" : compatibilityState === "not_detected" ? "warning" : "danger"}>{compatibilityState}</StatusBadge>}</div>
            {error && <StateMessage kind="error" title="Statusabfrage fehlgeschlagen">{error}</StateMessage>}
            {!error && !status && <StateMessage kind="loading" title="Verbindung wird hergestellt">Der lokale Core wird kontaktiert.</StateMessage>}
            {status && <div className="cards">
              <article><span>Core</span><strong>{status.state}</strong><small>Generation {status.generation}</small></article>
              <article><span>D2R</span><strong>{status.d2r.state}</strong><small>{status.d2r.window_bound ? `${status.d2r.client_width ?? 0} × ${status.d2r.client_height ?? 0}` : "Kein Fenster gebunden"}</small></article>
              <article><span>Input</span><strong>{status.input.stopped ? "gestoppt" : status.input.paused ? "pausiert" : status.input.enabled ? "freigegeben" : "deaktiviert"}</strong><small>Safety-Gates aus dem Core</small></article>
              <article><span>Gebiet</span><strong>{status.world.area_name || "Unbekannt"}</strong><small>{status.world.valid ? status.world.phase : "World Model noch ungültig"}</small></article>
            </div>}
          </section>

          <section>
            <div className="section-heading"><div><p className="eyebrow">Voraussetzung</p><h2>Charakter und Schwierigkeit</h2></div></div>
            <p>D2R muss auf dem Offline-Charakterbildschirm bei 1280 × 720 stehen. Die Auswahl wird vor jedem Klick visuell und anschließend im Spiel über Memory bestätigt.</p>
            {selectionError && <p role="alert">{selectionError}</p>}
            <p><strong>In D2R bestätigt:</strong> {status?.selection.character ? `${status.selection.character} / ${confirmedDifficultyLabel}` : "Noch keine Auswahl bestätigt"}</p>
            {draftDiffers && <StateMessage kind="error" title="Auswahl noch nicht in D2R">Queue und Start gelten für {status?.selection.character}. Nach dem Anwenden lädt die Queue von {character}.</StateMessage>}
            <div className="selection-grid">
              <label>Charakter<select value={character} onChange={(event) => {
                const next = event.target.value;
                selectionEdited.current = true;
                setCharacter(next);
                const stored = storedCharacterSettings(operatorSettings, catalog, next);
                if (stored?.last_difficulty) setDifficulty(stored.last_difficulty);
              }} disabled={effectiveSelectionLocked}>{catalog?.characters.map((entry) => <option key={entry.slug} value={entry.name} disabled={!entry.selectable}>{entry.name}{entry.selectable ? "" : " – nicht verfügbar"}</option>)}</select></label>
              <label>Schwierigkeit<select value={difficulty} onChange={(event) => { selectionEdited.current = true; setDifficulty(event.target.value); }} disabled={effectiveSelectionLocked}>{catalog?.difficulties.map((entry) => <option key={entry.id} value={entry.id}>{entry.display_name}</option>)}</select></label>
              <button type="button" disabled={effectiveSelectionLocked || !character || (status?.state !== "idle" && status?.state !== "idle_in_game" && status?.state !== "stopped_error")} onClick={() => void submitSelection()}>{applying ? "Auswahl wird geprüft …" : "Auswahl in D2R anwenden"}</button>
            </div>
            {catalog && focusedCatalogEntry && !focusedCatalogEntry.selectable && !(focusedCatalogEntry.reasons ?? []).includes("character_class_unsupported") && status && (
              <ul className="character-list"><li key={focusedCatalogEntry.slug}>
                <strong>{focusedCatalogEntry.name}</strong>
                <span>{characterAvailabilityText(focusedCatalogEntry, catalog)}</span>
                <Button variant="secondary" onClick={() => setSetupCharacter(focusedCatalogEntry.name)}>Charakter einrichten</Button>
              </li></ul>
            )}
            {viewedFarmReadyBlocked && <StateMessage kind="error" title="Charakter nicht farmbereit">{(viewedCatalogEntry?.farm_ready_reasons ?? []).map((reason) => farmReadyReasonText(reason)).join(" ") || "Tasten oder Inventarschutz fehlen noch."} <a href="#settings">Einstellungen → Charaktere</a>{status && viewedCatalogEntry && <Button variant="secondary" onClick={() => setSetupCharacter(viewedCatalogEntry.name)}>Charakter einrichten</Button>}</StateMessage>}
            {setupCharacter && status && catalog && <CharacterSetupWizard
              character={setupCharacter}
              catalog={catalog}
              status={status}
              mode="dashboard"
              onChanged={async () => { await refreshAfterCommand(); }}
            />}
          </section>

          <section>
            <div className="section-heading"><div><p className="eyebrow">Farming</p><h2>Run-Reihenfolge pro Spiel</h2></div></div>
            <p>Die Reihenfolge wird persistent pro Charakter gespeichert. Änderungen erfolgen zentral unter <a href="#settings">Einstellungen</a>.</p>
            {startFailureText && <StateMessage kind="error" title="Queue-Start fehlgeschlagen">{startFailureText}</StateMessage>}
            {queueWarning && <StateMessage kind="error" title="Queue-Hinweis">{queueWarning}</StateMessage>}
            {inputNotReady && !startFailureText && <StateMessage kind="error" title="Spielsteuerung nicht bereit">Prüfe Freigabe, Pause und Notstopp oder warte den kontrollierten Core-Neustart ab.</StateMessage>}
            {routeWorkflowBusy && <StateMessage kind="error" title="Routenvorgang aktiv">Zuerst den Routenvorgang beenden.</StateMessage>}
            {!confirmedSelection
              ? <StateMessage kind="empty" title="Zuerst Charakter in D2R bestätigen">Ohne bestätigte Auswahl gibt es keine Farm-Queue.</StateMessage>
              : selectionRuns === null
                ? <StateMessage kind="loading" title="Katalog wird geladen" />
                : <div className="run-grid">{selectionRuns.map((run) => {
                  const availability = runAvailabilityText(run.status, run.reasons, farmCatalogEntry?.expected_class);
                  return <article key={run.run_id}><strong>{run.display_name}</strong><span>{availability.title}</span><small>{availability.detail}</small></article>;
                })}</div>}
            {confirmedSelection && <>
            <h3>Reihenfolge für {status?.selection.character} / {confirmedDifficultyLabel} (in D2R bestätigt)</h3>
            {configuredQueue.length === 0 ? <StateMessage kind="empty" title="Keine Queue konfiguriert">Lege die Run-Reihenfolge in den Einstellungen für {status?.selection.character} fest.</StateMessage> : <ol className="queue-list">{configuredQueue.map((runID, index) => <li key={`${runID}-${index}`}><span>{index + 1}</span><strong>{(selectionRuns ?? []).find((run) => run.run_id === runID)?.display_name ?? runID}</strong></li>)}</ol>}
            <div className="queue-toolbar"><a className="button secondary" href="#settings">Queue in Einstellungen ändern</a><button type="button" disabled={queueStartLocked || configuredQueue.length === 0 || !status?.selection.character} onClick={() => void submitQueue()}>{commandPending ? "Core bestätigt …" : "Queue prüfen und starten"}</button></div>
            </>}
            {status && <div className="queue-status" aria-live="polite"><strong>Core-Queue:</strong> {(status.queue.entries ?? []).length ? (status.queue.entries ?? []).join(" → ") : "keine aktive Queue"}<span>Spiel {status.game_id || "–"} · Spielzyklus {status.queue.cycle + 1} · Lifecycle {status.lifecycle_phase}</span><span>Index {status.queue.index + 1} · Retry {status.queue.retry} · Run-ID {status.run_id || "–"}</span><span>Gestartet {status.queue.started_runs}/{status.queue.budgets.max_runs} · Restarts {status.queue.total_restarts}/{status.queue.budgets.max_total_restarts}</span>{status.active_run_id && <span>Aktiv: {status.active_run_id}{status.step ? ` · ${status.step}` : ""}</span>}{hasPendingIntent && <span>Vorgemerkt: {status.pending_intent}</span>}{status.last_result && <span>Letztes Ergebnis: {status.last_result.disposition}{status.last_result.reason ? ` · ${runResultReasonText(status.last_result.reason)}` : ""}</span>}</div>}
            <div className="session-controls"><button type="button" disabled={commandPending || status?.state !== "running_run" || hasPendingIntent} onClick={() => status && void runCommand(() => pauseAfterRun(status.generation))}>Nach aktuellem Run pausieren</button><button type="button" disabled={liveLocked || commandPending || status?.state !== "paused_between_runs"} onClick={() => status && void runCommand(() => resumeQueue(status.generation))}>Queue fortsetzen</button><button type="button" disabled={commandPending || status?.state !== "running_run" || hasPendingIntent} onClick={() => status && void runCommand(() => stopAfterRun(status.generation))}>Nach aktuellem Run stoppen</button><button type="button" className="danger" disabled={commandPending || !status || !emergencyStates.has(status.state)} onClick={() => setConfirmEmergency(true)}>Emergency Stop</button></div>
            <p className="hint">Pause und Stop warten auf die sichere Run-Grenze. Emergency Stop und F11 brechen sofort ab und garantieren kein Save &amp; Exit.</p>
          </section>
        </>}

        {target === "routes" && <>{liveLocked && <StateMessage kind="error" title="Live-Routenaktionen sind gesperrt">Die Routenbibliothek bleibt read-only, bis D2R kompatibel bestätigt ist.</StateMessage>}<RouteFeature characters={catalog?.characters.map((entry) => entry.name) ?? []} selectedCharacter={status?.selection.character ?? character} refreshKey={routeRefreshKey} liveLocked={liveLocked} preferredRecordingRun={routeOpenedFromOnboarding ? preferredRecordingRun : ""} onReturnToOnboarding={routeOpenedFromOnboarding ? returnToOnboarding : undefined} /></>}
        {target === "pickit" && <><PageHeader eyebrow="Loot-Policy" title="Pickit" description="Profile, Regeln und Zuordnungen bleiben Core-validiert und gelten erst an einer sicheren Run-Grenze." /><PickitFeature characters={catalog?.characters.map((entry) => entry.name) ?? []} selectedCharacter={status?.selection.character ?? character} runs={catalog?.runs.map((entry) => entry.run_id) ?? []} locked={!!status && !editableStates.has(status.state)} refreshKey={pickitRefreshKey} /></>}
        {target === "history" && <><PageHeader eyebrow="Auswertung" title="Historie" description="Core-berechnete Runs, Itemertrag, Vergleiche und Exporte ohne UI-eigene Aggregation." /><HistoryFeature characters={catalog?.characters.map((entry) => entry.name) ?? []} runs={catalog?.runs.map((entry) => entry.run_id) ?? []} refreshKey={historyRefreshKey} /></>}
        {target === "settings" && <><PageHeader eyebrow="System" title="Einstellungen" description="Bot-Verhalten, Charaktere, App und Wartung – getrennt nach Speicherziel." /><SettingsFeature generation={status?.generation ?? 0} coreState={status?.state ?? ""} status={status} characters={catalog?.characters.map((entry) => entry.slug) ?? []} catalog={catalog} runs={catalog?.runs.map((entry) => ({ id: entry.run_id, label: entry.display_name, status: entry.status, reasons: entry.reasons, routeCombat: entry.route_combat })) ?? []} events={events} onOpenOnboarding={() => { setOnboardingStep(0); setOnboardingOpen(true); }} onSettingsApplied={() => { void refreshAfterCommand(); }} onHistoryDeleted={() => setHistoryRefreshKey((value) => value + 1)} onDirtyChange={setSettingsDirty} /></>}
        </>}
      </main>

      {preview && <Dialog title="Routen werden unbrauchbar" onClose={() => !applying && setPreview(null)}><p>Der Wechsel von <strong>{preview.old_difficulty || "unbestätigt"}</strong> auf <strong>{preview.new_difficulty}</strong> markiert folgende Farming-Routen als <code>stale</code>:</p><ul>{preview.affected_routes.map((route) => <li key={route}>{route}</li>)}</ul><p>Die Dateien werden nicht gelöscht oder verändert. Neue Aufnahmen sind vor Farming erforderlich.</p><div className="modal-actions"><Button variant="secondary" onClick={() => setPreview(null)} disabled={applying}>Abbrechen</Button><Button onClick={() => void applyPreview(preview)} disabled={applying}>{applying ? "Wird angewendet …" : "Auswirkungen bestätigen und anwenden"}</Button></div></Dialog>}
      {confirmEmergency && <Dialog title="Session sofort abbrechen?" onClose={() => setConfirmEmergency(false)} initialFocusRef={emergencyConfirmRef}><p>Der aktuelle Input wird sofort gesperrt. Save &amp; Exit ist nicht garantiert. Dies entspricht F11 im Spiel.</p><div className="modal-actions"><Button variant="secondary" onClick={() => setConfirmEmergency(false)}>Abbrechen</Button><Button ref={emergencyConfirmRef} variant="danger" onClick={() => { setConfirmEmergency(false); if (status) void runCommand(() => emergencyStop(status.generation)); }}>Emergency Stop bestätigen</Button></div></Dialog>}
      {pendingNav && <Dialog title="Ungespeicherte Änderungen" onClose={() => setPendingNav(null)}><p>Deine Änderungen an der Run-Reihenfolge sind noch nicht gespeichert. Ohne Speichern startet der Bot mit der alten Reihenfolge.</p><div className="modal-actions"><Button variant="secondary" onClick={() => setPendingNav(null)}>Zurück zu den Einstellungen</Button><Button variant="danger" onClick={discardSettingsAndNavigate}>Änderungen verwerfen</Button></div></Dialog>}
    </div>
  );
}

function storedCharacterSettings(settings: OperatorSettingsDTO | null, catalog: CatalogDTO | null | undefined, name: string) {
  const slug = catalog?.characters.find((entry) => entry.name === name)?.slug;
  if (!slug) return undefined;
  return settings?.characters[slug];
}
