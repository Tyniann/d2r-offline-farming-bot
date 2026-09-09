import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { confirmHistoryDeleteAll, createDiagnosticBundle, previewHistoryDeleteAll, restoreOperatorSettings, saveOperatorSettings } from "../../api/client";
import {
  getOperatorSettings, getRunAvailabilities, previewCharacterSetup, previewOperatorSettings, previewResetOperatorSettings,
  type CatalogDTO, type CharacterCatalogEntry, type CharacterSetupPreviewDTO, type CharacterSetupProfileDTO, type HistoryDeletePreviewDTO,
  type LiveEvent, type OperatorCharacterSettingsDTO, type OperatorSettingsChangeDTO, type OperatorSettingsDTO, type StatusDTO,
} from "../../api/generated";
import { formatNumber } from "../../i18n/format";
import { apiErrorCode, presentApiError, presentClassName, presentProfileName, presentRunName } from "../../i18n/presenters";
import { bindingsFromDTO, bindingsToDTO, emptyBindings, type BindingEditorValue } from "../characters/BindingEditor";
import { characterStatusLabel } from "../characters/characterReasonText";
import { inventoryConfigured, inventoryCowSuitable, type InventoryGrid } from "../characters/InventoryLockEditor";
import { cloneSettings, collectLocalDiffPaths, pathChanged, settingsEqual, summarizeChangedFields } from "./settingsDiff";
import type { SettingsRun } from "./settingsTypes";

/** SettingsFeatureProps ist die Props-API der Einstellungsseite; die App-Shell liefert Katalog, Status und Auswahlkontext. */
export type SettingsFeatureProps = {
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
};

/** SettingsModel ist der von [useSettingsModel] gelieferte Zustand samt Aktionen. */
export type SettingsModel = ReturnType<typeof useSettingsModel>;

/**
 * useSettingsModel bündelt Laden, Draft, Dirty-Tracking und alle Aktionen der Einstellungen.
 *
 * Invarianten: `settings` ist der zuletzt vom Core bestätigte Stand, `draft` die lokale Kopie. Jede Mutation
 * geht über [SettingsModel.changeDraft], damit Vorschau und Erfolgsmeldung verworfen werden. Speichern und
 * Zurücksetzen laufen immer über Vorschau → Bestätigung; ein Revisions- oder Generationskonflikt bleibt als
 * `stale` sichtbar, bis der Operator ausdrücklich neu lädt.
 */
export function useSettingsModel(props: SettingsFeatureProps) {
  const { generation, coreState, characters, selectedCharacter, onSelectedCharacterChange, onSettingsApplied, onHistoryDeleted, onDirtyChange } = props;
  const { t, i18n } = useTranslation();
  const [settings, setSettings] = useState<OperatorSettingsDTO | null>(null);
  const [draft, setDraft] = useState<OperatorSettingsDTO | null>(null);
  const [desktop, setDesktop] = useState<DesktopSettingsView | null>(null);
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
  const [autostartFlash, setAutostartFlash] = useState(false);
  const dirtyRef = useRef(false);
  const activeCharacter = selectedCharacter || characters[0] || "";
  const mutable = coreState === "idle" || coreState === "stopped_error";

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
      setError(presentApiError(reason, t, t("settings.loadFailed")));
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => { void load(); }, [t]);
  useEffect(() => window.d2rDesktop?.onUpdateStatus?.(setUpdateStatus), []);

  const dirty = !!(settings && draft && !settingsEqual(settings, draft));
  const diffPaths = useMemo(() => settings && draft ? collectLocalDiffPaths(settings, draft) : [], [settings, draft]);
  const dirtySummary = summarizeChangedFields(diffPaths);
  const changeCount = dirty ? dirtySummary.split(", ").filter(Boolean).length : 0;

  useEffect(() => {
    dirtyRef.current = dirty;
    onDirtyChange?.(dirty);
  }, [dirty, onDirtyChange]);

  useEffect(() => {
    const protect = (event: BeforeUnloadEvent) => { if (dirtyRef.current) event.preventDefault(); };
    window.addEventListener("beforeunload", protect);
    return () => window.removeEventListener("beforeunload", protect);
  }, []);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (!(event.ctrlKey || event.metaKey) || event.key.toLowerCase() !== "s") return;
      if (!dirty || !mutable || busy) return;
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

  // run serialisiert alle Core-/Desktop-Aktionen: nie zwei parallel, Fehler bleiben bis zur nächsten Aktion sichtbar.
  const run = async (action: () => void | Promise<void>) => {
    if (busy) return;
    setBusy(true);
    setError("");
    setMessage("");
    setStale(false);
    try {
      await action();
    } catch (reason) {
      setError(presentApiError(reason, t, t("settings.changeFailed")));
      if (["config_revision_conflict", "revision_conflict", "state_changed"].includes(apiErrorCode(reason) ?? "")) setStale(true);
    } finally {
      setBusy(false);
    }
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
      setAutostartFlash(true);
      window.setTimeout(() => setAutostartFlash(false), 1800);
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

  const updateLabel = (): string => {
    if (!updateStatus) return t("settings.desktopBridgeMissing");
    if (updateStatus.status === "checking") return t("settings.checking");
    if (updateStatus.status === "available") return t("settings.newVersion");
    if (updateStatus.status === "up_to_date") return t("settings.upToDate");
    return t("settings.unavailable");
  };
  const updateDescription = (): string => {
    if (!updateStatus) return t("settings.updateDesktopOnly");
    if (updateStatus.status === "checking") return t("settings.updateChecking", { current: updateStatus.current_version });
    if (updateStatus.status === "available") return t("settings.updateAvailable", { current: updateStatus.current_version, latest: updateStatus.latest_version });
    if (updateStatus.status === "up_to_date") return t("settings.updateCurrent", { current: updateStatus.current_version });
    return t("settings.updateUnavailable", { current: updateStatus.current_version });
  };

  return {
    t, props, settings, draft, desktop, updateStatus, mutable, dirty, diffPaths, dirtySummary, changeCount, busy, message, error, stale, restartRequired,
    preview, deletePreview, includeTelemetry, includeRoutes, autostartFlash, activeCharacter, characterNames,
    setIncludeTelemetry, setIncludeRoutes,
    load, changeDraft, discardDraft, requestSave, requestReset, applyPreview, closePreview: () => setPreview(null),
    toggleAutostart, restartCore, previewDeleteHistory, deleteHistory, closeDeletePreview: () => setDeletePreview(null),
    checkForUpdates, buildDiagnosticBundle, updateLabel, updateDescription,
    selectCharacter: onSelectedCharacterChange ?? (() => {}),
    characterChanged: (slug: string) => pathChanged(diffPaths, `characters.${slug}`),
    budgetsChanged: pathChanged(diffPaths, "budgets"),
    inputChanged: pathChanged(diffPaths, "input"),
    historyChanged: pathChanged(diffPaths, "history"),
  };
}

/** CharacterContext ist der von [useCharacterContext] gelieferte Charakterzustand. */
export type CharacterContext = ReturnType<typeof useCharacterContext>;

/**
 * useCharacterContext liefert Katalogeintrag, Setup-Vorschau, Run-Verfügbarkeit und Editor-Bindings eines Charakters.
 *
 * Die Run-Verfügbarkeit stammt aus `GET /api/v1/runs` für Charakter und gespeicherte Schwierigkeit; schlägt der
 * Abruf fehl, bleiben die Katalognamen sichtbar, gelten aber als nicht startfähig (leerer Status).
 */
export function useCharacterContext(model: SettingsModel, slug: string) {
  const { t, draft, props, changeDraft, diffPaths } = model;
  const catalog = props.catalog ?? null;
  const [preview, setPreview] = useState<CharacterSetupPreviewDTO | null>(null);
  const [previewError, setPreviewError] = useState("");
  const [runs, setRuns] = useState<SettingsRun[]>([]);

  const settings: OperatorCharacterSettingsDTO | undefined = draft?.characters[slug];
  const entry: CharacterCatalogEntry | undefined = catalog?.characters.find((item) => item.slug === slug || item.name.toLowerCase() === slug);
  const name = entry?.name ?? slug;
  const characterClass = preview?.character.character_class || settings?.character_class || entry?.expected_class || "";
  const profileID = settings?.combat_profile ?? preview?.selected_profile_id ?? "";
  const profile: CharacterSetupProfileDTO | undefined = preview?.profiles.find((item) => item.id === profileID) ?? preview?.profiles.find((item) => item.is_selected) ?? preview?.profiles[0];
  const supported = !!settings && (preview ? preview.supported : true);

  useEffect(() => {
    if (!settings) { setPreview(null); return; }
    let cancelled = false;
    setPreviewError("");
    void previewCharacterSetup({ character: name }).then((next) => { if (!cancelled) setPreview(next); }).catch((reason) => {
      if (!cancelled) { setPreview(null); setPreviewError(presentApiError(reason, t, t("characters.previewFallback"))); }
    });
    return () => { cancelled = true; };
  }, [slug, settings?.combat_profile, name, t]);

  useEffect(() => {
    const fallback = props.runs.map((run) => ({ ...run, label: presentRunName(run.id, t), status: "", reasons: [] as string[] }));
    const difficulty = settings?.last_difficulty || props.status?.selection.difficulty || catalog?.default_difficulty || "";
    if (!name || !difficulty) { setRuns(fallback); return; }
    const controller = new AbortController();
    setRuns(fallback);
    void getRunAvailabilities(name, difficulty, controller.signal)
      .then((value) => {
        if (controller.signal.aborted) return;
        setRuns((value.runs ?? []).map((run) => ({ id: run.run_id, label: presentRunName(run.run_id, t), status: run.status, reasons: run.reasons, routeCombat: run.route_combat })));
      })
      .catch(() => { if (!controller.signal.aborted) setRuns(fallback); });
    return () => controller.abort();
  }, [name, settings?.last_difficulty, props.status?.selection.difficulty, catalog, props.runs, t]);

  const bindingValue = useMemo<BindingEditorValue>(() => {
    if (!settings || !profileID) return emptyBindings();
    return bindingsFromDTO(settings.profile_bindings?.[profileID], profile?.belt_layout ?? profile?.default_belt_layout, profile ? { healing: profile.default_healing_restock, mana: profile.default_mana_restock } : undefined);
  }, [settings, profileID, profile]);

  const updateCharacter = (mutate: (character: OperatorCharacterSettingsDTO) => OperatorCharacterSettingsDTO) => changeDraft((current) => {
    const character = current.characters[slug];
    if (character) current.characters[slug] = mutate(character);
    return current;
  });

  const grid = settings?.inventory_lock?.grid ?? null;
  const inventoryIsConfigured = inventoryConfigured(grid);
  const lockedCells = inventoryIsConfigured ? grid!.flat().filter((cell) => cell === 1).length : 40;
  const cowWarning = inventoryIsConfigured && (settings?.queue ?? []).includes("cows") && !inventoryCowSuitable(grid!);

  return {
    slug, name, entry, settings, supported, preview, previewError, runs, characterClass, profileID, profile, bindingValue,
    className: presentClassName(characterClass, t),
    profileName: profile ? presentProfileName(profile.id, profile.display_name, t) : (settings?.combat_profile || t("characters.profileUnset")),
    statusLabel: characterStatusLabel(entry, catalog, t),
    farmReady: !!entry?.farm_ready,
    players: settings?.players ?? 1,
    grid, inventoryIsConfigured, lockedCells, cowWarning,
    queueChanged: pathChanged(diffPaths, `characters.${slug}.queue`),
    setQueue: (queue: string[]) => updateCharacter((character) => ({ ...character, queue })),
    setDifficulty: (last_difficulty: OperatorCharacterSettingsDTO["last_difficulty"]) => updateCharacter((character) => ({ ...character, last_difficulty })),
    setPlayers: (players: number) => updateCharacter((character) => ({ ...character, players })),
    setBindings: (next: BindingEditorValue) => { if (profileID) updateCharacter((character) => ({ ...character, profile_bindings: { ...(character.profile_bindings ?? {}), [profileID]: bindingsToDTO(next) } })); },
    setInventory: (next: InventoryGrid) => updateCharacter((character) => ({ ...character, inventory_lock: { grid: next } })),
  };
}
