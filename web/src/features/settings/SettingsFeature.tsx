import { useEffect, useMemo, useRef, useState } from "react";
import { confirmHistoryDeleteAll, createDiagnosticBundle, previewHistoryDeleteAll, restoreOperatorSettings, saveOperatorSettings } from "../../api/client";
import { getOperatorSettings, previewOperatorSettings, previewResetOperatorSettings, type CatalogDTO, type HistoryDeletePreviewDTO, type LiveEvent, type OperatorSettingsChangeDTO, type OperatorSettingsDTO, type StatusDTO } from "../../api/generated";
import { Button, Dialog, StateMessage, StatusBadge } from "../../app/ui";
import { CharactersTab } from "../characters/CharactersTab";
import { AppTab } from "./AppTab";
import { FarmingTab } from "./FarmingTab";
import { MaintenanceTab } from "./MaintenanceTab";
import { SettingsActionBar } from "./SettingsActionBar";
import { cloneSettings, collectLocalDiffPaths, settingsEqual, summarizeChangedFields } from "./settingsDiff";
import type { SettingsRun, SettingsTab } from "./settingsTypes";
import { useTranslation } from "react-i18next";
import { formatNumber } from "../../i18n/format";
import { apiErrorCode, presentApiError, type AppTranslator } from "../../i18n/presenters";

/** SettingsFeature orchestriert Farming-, Charaktere-, App- und Wartung-Scopes mit sticky Core-Commit. */
export function SettingsFeature({
  generation, coreState, characters, selectedCharacter, onSelectedCharacterChange, runs, events, catalog, status, onOpenOnboarding, onSettingsApplied, onHistoryDeleted, onDirtyChange,
}: {
  generation: number;
  coreState: string;
  characters: string[];
  selectedCharacter?: string;
  onSelectedCharacterChange?(character: string): void;
  runs: SettingsRun[];
  events: LiveEvent[];
  catalog?: CatalogDTO | null;
  status?: StatusDTO | null;
  onOpenOnboarding?: () => void;
  onSettingsApplied?: () => void;
  onHistoryDeleted?: () => void;
  onDirtyChange?: (dirty: boolean) => void;
}) {
  const { t, i18n } = useTranslation();
  const [settings, setSettings] = useState<OperatorSettingsDTO | null>(null);
  const [draft, setDraft] = useState<OperatorSettingsDTO | null>(null);
  const [desktop, setDesktop] = useState<DesktopSettingsView | null>(null);
  const [tab, setTab] = useState<SettingsTab>("farming");
  const [preview, setPreview] = useState<{ mode: "save" | "reset"; change: OperatorSettingsChangeDTO } | null>(null);
  const [restartRequired, setRestartRequired] = useState(false);
  const [stale, setStale] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [deletePreview, setDeletePreview] = useState<HistoryDeletePreviewDTO | null>(null);
  const [updateStatus, setUpdateStatus] = useState<DesktopUpdateStatus | null>(null);
  const [includeTelemetry, setIncludeTelemetry] = useState(false);
  const [includeRoutes, setIncludeRoutes] = useState(false);
  const dirtyRef = useRef(false);
  const activeCharacter = selectedCharacter || characters[0] || "";
  const mutable = coreState === "idle" || coreState === "stopped_error";
  const editableTab = tab === "farming" || tab === "characters";

  const load = async () => {
    setBusy(true);
    setError("");
    try {
      const [operator, desktopSettings, currentUpdateStatus] = await Promise.all([
        getOperatorSettings(),
        window.d2rDesktop?.getDesktopSettings() ?? Promise.resolve(null),
        window.d2rDesktop?.getUpdateStatus?.() ?? Promise.resolve(null),
      ]);
      setSettings(operator);
      setDraft(cloneSettings(operator));
      setDesktop(desktopSettings);
      setUpdateStatus(currentUpdateStatus);
      setPreview(null);
      setStale(false);
      setMessage("");
    } catch (reason) {
      setError(errorText(reason, t, t("settings.loadFailed")));
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => { void load(); }, [t]);
  useEffect(() => window.d2rDesktop?.onUpdateStatus?.(setUpdateStatus), []);

  const dirty = !!(settings && draft && !settingsEqual(settings, draft));
  const diffPaths = useMemo(() => settings && draft ? collectLocalDiffPaths(settings, draft) : [], [settings, draft]);
  const dirtySummary = summarizeChangedFields(diffPaths);

  useEffect(() => {
    dirtyRef.current = dirty;
    onDirtyChange?.(dirty);
  }, [dirty, onDirtyChange]);

  useEffect(() => {
    const protect = (event: BeforeUnloadEvent) => {
      if (dirtyRef.current) event.preventDefault();
    };
    window.addEventListener("beforeunload", protect);
    return () => window.removeEventListener("beforeunload", protect);
  }, []);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (!(event.ctrlKey || event.metaKey) || event.key.toLowerCase() !== "s") return;
      if (!editableTab || !dirty || !mutable || busy) return;
      event.preventDefault();
      void requestSave();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  const characterNames = useMemo(() => {
    const names = new Set([
      ...characters.map((name) => name.trim().toLowerCase()).filter(Boolean),
      ...Object.keys(draft?.characters ?? {}),
    ]);
    return [...names].sort((left, right) => left.localeCompare(right, i18n.resolvedLanguage));
  }, [characters, draft, i18n.resolvedLanguage]);

  const changeDraft = (update: (current: OperatorSettingsDTO) => OperatorSettingsDTO) => {
    setDraft((current) => current ? update(cloneSettings(current)) : current);
    setPreview(null);
    setMessage("");
  };

  const discardDraft = () => {
    if (!settings) return;
    setDraft(cloneSettings(settings));
    setPreview(null);
    setMessage("");
  };

  const requestSave = async () => {
    if (!draft || !settings || !dirty) return;
    await run(async () => setPreview({ mode: "save", change: await previewOperatorSettings({ expected_revision: settings.revision, expected_generation: generation, settings: draft }) }));
  };

  const requestReset = async () => {
    if (!settings) return;
    await run(async () => setPreview({ mode: "reset", change: await previewResetOperatorSettings({ expected_revision: settings.revision, expected_generation: generation }) }));
  };

  const applyPreview = async () => {
    if (!preview || !settings || !draft) return;
    await run(async () => {
      const result = preview.mode === "reset"
        ? await restoreOperatorSettings({ expected_revision: settings.revision, expected_generation: generation })
        : await saveOperatorSettings({ expected_revision: settings.revision, expected_generation: generation, settings: draft });
      setSettings(result.settings);
      setDraft(cloneSettings(result.settings));
      setRestartRequired(Boolean(result.restart_required));
      setPreview(null);
      setMessage(t(result.changed_fields.length ? "settings.savedForNextStart" : "settings.nothingToSave"));
      onSettingsApplied?.();
    });
  };

  const toggleAutostart = async (value: boolean) => {
    if (!desktop || !window.d2rDesktop) return;
    await run(async () => {
      setDesktop(await window.d2rDesktop!.updateDesktopSettings({ autostart: value, onboarding_completed: desktop.onboarding_completed }));
    });
  };

  const restartCore = async () => {
    if (!window.d2rDesktop) return;
    await run(async () => {
      await window.d2rDesktop!.restartCore();
      setMessage(t("settings.coreRestarted"));
      setRestartRequired(false);
    });
  };

  const previewDeleteHistory = async () => {
    await run(async () => setDeletePreview(await previewHistoryDeleteAll(generation)));
  };

  const deleteHistory = async () => {
    if (!deletePreview) return;
    await run(async () => {
      const result = await confirmHistoryDeleteAll({
        expected_generation: generation,
        confirmation_token: deletePreview.confirmation_token,
        index_generation: deletePreview.index_generation,
        candidate_files: deletePreview.candidate_files,
        candidate_bytes: deletePreview.candidate_bytes,
      });
      setDeletePreview(null);
      setMessage(t("settings.historyDeleted", { deleted: formatNumber(result.deleted_files), protected: formatNumber(result.protected_files) }));
      onHistoryDeleted?.();
    });
  };

  const checkForUpdates = async () => {
    if (!window.d2rDesktop?.checkForUpdates) return;
    await run(async () => setUpdateStatus(await window.d2rDesktop!.checkForUpdates!()));
  };

  const buildDiagnosticBundle = async () => {
    await run(async () => {
      const result = await createDiagnosticBundle({ include_telemetry: includeTelemetry, include_routes: includeRoutes });
      setMessage(t("settings.diagnosticCreated", { filename: result.filename, bytes: formatNumber(result.bytes) }));
      await window.d2rDesktop?.revealDiagnosticBundle?.(result.filename);
    });
  };

  const run = async (action: () => void | Promise<void>) => {
    if (busy) return;
    setBusy(true);
    setError("");
    setMessage("");
    setStale(false);
    try {
      await action();
    } catch (reason) {
      const text = errorText(reason, t, t("settings.changeFailed"));
      setError(text);
      if (["config_revision_conflict", "revision_conflict", "state_changed"].includes(apiErrorCode(reason) ?? "")) setStale(true);
    } finally {
      setBusy(false);
    }
  };

  if (!settings || !draft) {
    return error
      ? <StateMessage kind="error" title={t("settings.notAvailable")}>{error}</StateMessage>
      : <StateMessage kind="loading" title={t("settings.loadingSettings")}>{t("settings.loadingContracts")}</StateMessage>;
  }

  return <>
    <div className="settings-status-row">
      <StatusBadge tone="neutral">{t("settings.revision", { revision: settings.revision })}</StatusBadge>
      <StatusBadge tone={mutable ? "success" : "warning"}>{t(mutable ? "settings.editable" : "settings.locked")}</StatusBadge>
    </div>

    {!mutable && <StateMessage kind="error" title={t("settings.lockedTitle")}>{t("settings.lockedSession")} {t("settings.saveWhenIdle")}</StateMessage>}
    {stale && <div className="settings-feedback"><StateMessage kind="error" title={t("settings.staleTitle")}>{t("settings.staleDetail")}</StateMessage><Button variant="secondary" onClick={() => void load()}>{t("settings.reload")}</Button></div>}
    {error && !stale && <StateMessage kind="error" title={t("settings.changeFailedTitle")}>{error}</StateMessage>}
    {message && <div className="settings-success" role="status"><strong>{message}</strong></div>}
    {restartRequired && <div className="restart-required" role="status"><div><strong>{t("settings.restartCoreTitle")}</strong><p>{t("settings.restartCoreDetail")}</p></div><Button onClick={() => void restartCore()} disabled={!mutable || busy || !window.d2rDesktop}>{t("settings.restartCore")}</Button></div>}

    <div className="settings-tabs" role="tablist" aria-label={t("settings.tabsAria")}>
      {([
        { id: "farming" as const, label: t("settings.tabFarming") },
        { id: "characters" as const, label: t("settings.tabCharacters") },
        { id: "app" as const, label: t("settings.tabApp") },
        { id: "maintenance" as const, label: t("settings.tabMaintenance") },
      ]).map((entry) => (
        <button
          key={entry.id}
          type="button"
          role="tab"
          aria-selected={tab === entry.id}
          className={`settings-tab${tab === entry.id ? " active" : ""}${(entry.id === "farming" || entry.id === "characters") && dirty ? " dirty" : ""}`}
          onClick={() => setTab(entry.id)}
        >
          {entry.label}{(entry.id === "farming" || entry.id === "characters") && dirty ? <span className="settings-tab-dot" aria-label={t("settings.unsavedAria")} /> : null}
        </button>
      ))}
    </div>

    {tab === "farming" && <FarmingTab
      draft={draft}
      saved={settings}
      selectedCharacter={activeCharacter}
      characterNames={characterNames}
      catalog={catalog}
      status={status}
      runs={runs}
      mutable={mutable}
      restartRequired={restartRequired}
      diffPaths={diffPaths}
      onSelectCharacter={onSelectedCharacterChange ?? ignoreCharacterChange}
      onChangeDraft={changeDraft}
      onReset={() => void requestReset()}
    />}
    {tab === "characters" && <CharactersTab
      draft={draft}
      catalog={catalog ?? null}
      selectedCharacter={activeCharacter}
      characterNames={characterNames}
      mutable={mutable}
      diffPaths={diffPaths}
      status={status ?? null}
      onSelectCharacter={onSelectedCharacterChange ?? ignoreCharacterChange}
      onChangeDraft={changeDraft}
      onSetupChanged={onSettingsApplied}
    />}
    {tab === "app" && <AppTab
      desktop={desktop}
      updateStatus={updateStatus}
      busy={busy}
      onOpenOnboarding={onOpenOnboarding}
      onToggleAutostart={toggleAutostart}
      onCheckUpdates={() => void checkForUpdates()}
      onOpenRelease={() => void window.d2rDesktop?.openReleasePage?.()}
    />}
    {tab === "maintenance" && <MaintenanceTab
      settings={settings}
      runs={runs}
      events={events}
      mutable={mutable}
      busy={busy}
      includeTelemetry={includeTelemetry}
      includeRoutes={includeRoutes}
      onIncludeTelemetry={setIncludeTelemetry}
      onIncludeRoutes={setIncludeRoutes}
      onBuildDiagnostic={() => void buildDiagnosticBundle()}
      onPreviewDeleteHistory={() => void previewDeleteHistory()}
    />}

    <SettingsActionBar
      dirty={dirty}
      locked={!mutable}
      revision={settings.revision}
      summary={dirtySummary}
      busy={busy}
      collapsed={!editableTab && dirty}
      onDiscard={discardDraft}
      onSave={() => void requestSave()}
      onShowFarming={() => setTab("farming")}
    />

    {deletePreview && <Dialog title={t("settings.deleteConfirmTitle")} onClose={() => !busy && setDeletePreview(null)}>
      <p>{t("settings.deletePreviewDetail", { files: formatNumber(deletePreview.candidate_files), bytes: formatNumber(deletePreview.candidate_bytes), protected: formatNumber(deletePreview.protected_files) })}</p>
      <p>{t("settings.categories", { categories: Object.entries(deletePreview.categories).map(([name, count]) => `${name}: ${formatNumber(count)}`).join(" · ") || t("history.none") })}</p>
      <StateMessage kind="error" title={t("settings.irreversible")}>{t("settings.irreversibleDetail")}</StateMessage>
      <div className="modal-actions">
        <Button variant="secondary" onClick={() => setDeletePreview(null)} disabled={busy}>{t("common.cancel")}</Button>
        <Button variant="danger" onClick={() => void deleteHistory()} disabled={busy || deletePreview.candidate_files === 0}>{t("settings.deleteAll")}</Button>
      </div>
    </Dialog>}

    {preview && <Dialog title={t(preview.mode === "reset" ? "settings.applyDefaultsTitle" : "settings.saveChangesTitle")} onClose={() => !busy && setPreview(null)}>
      <p>{t("settings.changedFields")}<strong>{summarizeChangedFields(preview.change.changed_fields) || t("history.none")}</strong></p>
      <details className="effective-settings"><summary>{t("settings.technicalFields")}</summary><pre>{preview.change.changed_fields.join("\n") || t("history.none")}</pre></details>
      {preview.change.restart_required && <StateMessage kind="error" title={t("settings.restartCoreTitle")}>{t("settings.restartEffectiveDetail")}</StateMessage>}
      <div className="modal-actions">
        <Button variant="secondary" onClick={() => setPreview(null)} disabled={busy}>{t("common.cancel")}</Button>
        <Button variant={preview.mode === "reset" ? "danger" : "primary"} onClick={() => void applyPreview()} disabled={busy}>{t(preview.mode === "reset" ? "settings.applyDefaults" : "settings.saveNow")}</Button>
      </div>
    </Dialog>}
  </>;
}

function errorText(reason: unknown, t: AppTranslator, fallback: string): string {
  return presentApiError(reason, t, fallback);
}

function ignoreCharacterChange() {}
