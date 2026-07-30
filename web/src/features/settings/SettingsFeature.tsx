import { useEffect, useMemo, useRef, useState } from "react";
import { confirmHistoryDeleteAll, createDiagnosticBundle, previewHistoryDeleteAll, restoreOperatorSettings, saveOperatorSettings } from "../../api/client";
import { getOperatorSettings, previewOperatorSettings, previewResetOperatorSettings, type HistoryDeletePreviewDTO, type LiveEvent, type OperatorSettingsChangeDTO, type OperatorSettingsDTO } from "../../api/generated";
import { Button, Dialog, StateMessage, StatusBadge } from "../../app/ui";
import { AppTab } from "./AppTab";
import { FarmingTab } from "./FarmingTab";
import { MaintenanceTab } from "./MaintenanceTab";
import { SettingsActionBar } from "./SettingsActionBar";
import { cloneSettings, collectLocalDiffPaths, settingsEqual, summarizeChangedFields } from "./settingsDiff";
import type { SettingsRun, SettingsTab } from "./settingsTypes";

/** SettingsFeature orchestriert Farming-, App- und Wartung-Scopes mit sticky Core-Commit. */
export function SettingsFeature({
  generation, coreState, characters, runs, events, onOpenOnboarding, onSettingsApplied, onDirtyChange,
}: {
  generation: number;
  coreState: string;
  characters: string[];
  runs: SettingsRun[];
  events: LiveEvent[];
  onOpenOnboarding?: () => void;
  onSettingsApplied?: () => void;
  onDirtyChange?: (dirty: boolean) => void;
}) {
  const [settings, setSettings] = useState<OperatorSettingsDTO | null>(null);
  const [draft, setDraft] = useState<OperatorSettingsDTO | null>(null);
  const [desktop, setDesktop] = useState<DesktopSettingsView | null>(null);
  const [selectedCharacter, setSelectedCharacter] = useState("");
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
      const names = Object.keys(operator.characters).sort((left, right) => left.localeCompare(right, "de"));
      setSelectedCharacter((current) => current && operator.characters[current] ? current : names[0] ?? "");
      setPreview(null);
      setRestartRequired(false);
      setStale(false);
      setMessage("");
    } catch (reason) {
      setError(errorText(reason, "Einstellungen konnten nicht geladen werden."));
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => { void load(); }, []);
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
      if (tab !== "farming" || !dirty || !mutable || busy) return;
      event.preventDefault();
      void requestSave();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  const characterNames = useMemo(() => {
    const names = new Set([...characters, ...Object.keys(draft?.characters ?? {})]);
    return [...names].sort((left, right) => left.localeCompare(right, "de"));
  }, [characters, draft]);

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
      setRestartRequired(result.restart_required);
      setPreview(null);
      setMessage(result.changed_fields.length ? "Gespeichert. Der Bot verwendet ab dem nächsten Start diese Werte." : "Es waren keine Änderungen zu speichern.");
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
      setMessage("Der Core wurde kontrolliert neu gestartet.");
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
      setMessage(`${result.deleted_files} Historiendatei(en) wurden dauerhaft gelöscht; ${result.protected_files} aktive Datei(en) blieben geschützt.`);
    });
  };

  const checkForUpdates = async () => {
    if (!window.d2rDesktop?.checkForUpdates) return;
    await run(async () => setUpdateStatus(await window.d2rDesktop!.checkForUpdates!()));
  };

  const buildDiagnosticBundle = async () => {
    await run(async () => {
      const result = await createDiagnosticBundle({ include_telemetry: includeTelemetry, include_routes: includeRoutes });
      setMessage(`Diagnosepaket ${result.filename} wurde lokal erstellt (${result.bytes.toLocaleString("de-DE")} Byte).`);
      await window.d2rDesktop?.revealDiagnosticBundle?.(result.filename);
    });
  };

  const run = async (action: () => void | Promise<void>) => {
    if (busy) return;
    setBusy(true);
    setError("");
    setStale(false);
    try {
      await action();
    } catch (reason) {
      const text = errorText(reason, "Einstellungsänderung fehlgeschlagen.");
      setError(text);
      if (/revision|zwischenzeitlich|zustand hat sich geändert/i.test(text)) setStale(true);
    } finally {
      setBusy(false);
    }
  };

  if (!settings || !draft) {
    return error
      ? <StateMessage kind="error" title="Einstellungen nicht verfügbar">{error}</StateMessage>
      : <StateMessage kind="loading" title="Einstellungen werden geladen">Core- und Desktopverträge werden gemeinsam gelesen.</StateMessage>;
  }

  return <>
    <div className="settings-status-row">
      <StatusBadge tone="neutral">Revision {settings.revision}</StatusBadge>
      <StatusBadge tone={mutable ? "success" : "warning"}>{mutable ? "Änderbar" : "Gesperrt"}</StatusBadge>
    </div>

    {!mutable && <StateMessage kind="error" title="Diese Einstellungen sind gesperrt">Gesperrt, solange eine Session läuft. Speichern ist wieder möglich, sobald der Bot inaktiv ist.</StateMessage>}
    {stale && <div className="settings-feedback"><StateMessage kind="error" title="Revision ist veraltet">Lade den aktuellen Core-Stand neu, bevor du erneut änderst.</StateMessage><Button variant="secondary" onClick={() => void load()}>Aktuellen Stand laden</Button></div>}
    {error && !stale && <StateMessage kind="error" title="Änderung fehlgeschlagen">{error}</StateMessage>}
    {message && <div className="settings-success" role="status"><strong>{message}</strong></div>}
    {restartRequired && <div className="restart-required" role="status"><div><strong>Core-Neustart erforderlich</strong><p>Input- oder Hotkeyressourcen bleiben bis zu einem kontrollierten Neustart unverändert.</p></div><Button onClick={() => void restartCore()} disabled={!mutable || busy || !window.d2rDesktop}>Core kontrolliert neu starten</Button></div>}

    <div className="settings-tabs" role="tablist" aria-label="Einstellungsbereiche">
      {([
        { id: "farming" as const, label: "Farming" },
        { id: "app" as const, label: "App" },
        { id: "maintenance" as const, label: "Wartung" },
      ]).map((entry) => (
        <button
          key={entry.id}
          type="button"
          role="tab"
          aria-selected={tab === entry.id}
          className={`settings-tab${tab === entry.id ? " active" : ""}${entry.id === "farming" && dirty ? " dirty" : ""}`}
          onClick={() => setTab(entry.id)}
        >
          {entry.label}{entry.id === "farming" && dirty ? <span className="settings-tab-dot" aria-label="ungespeicherte Änderungen" /> : null}
        </button>
      ))}
    </div>

    {tab === "farming" && <FarmingTab
      draft={draft}
      saved={settings}
      selectedCharacter={selectedCharacter}
      characterNames={characterNames}
      runs={runs}
      mutable={mutable}
      restartRequired={restartRequired}
      diffPaths={diffPaths}
      onSelectCharacter={setSelectedCharacter}
      onChangeDraft={changeDraft}
      onReset={() => void requestReset()}
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
      collapsed={tab !== "farming" && dirty}
      onDiscard={discardDraft}
      onSave={() => void requestSave()}
      onShowFarming={() => setTab("farming")}
    />

    {deletePreview && <Dialog title="Gesamte Historie dauerhaft löschen?" onClose={() => !busy && setDeletePreview(null)}>
      <p><strong>{deletePreview.candidate_files}</strong> direkte Datei(en) mit {deletePreview.candidate_bytes.toLocaleString("de-DE")} Byte sind vorgesehen. {deletePreview.protected_files} aktive Datei(en) sind geschützt.</p>
      <p>Kategorien: {Object.entries(deletePreview.categories).map(([name, count]) => `${name}: ${count}`).join(" · ") || "keine"}</p>
      <StateMessage kind="error" title="Nicht rückgängig zu machen">Die Metadaten und Indexgeneration werden unmittelbar erneut geprüft. Veraltete Vorschauen werden vollständig abgelehnt.</StateMessage>
      <div className="modal-actions">
        <Button variant="secondary" onClick={() => setDeletePreview(null)} disabled={busy}>Abbrechen</Button>
        <Button variant="danger" onClick={() => void deleteHistory()} disabled={busy || deletePreview.candidate_files === 0}>Alles endgültig löschen</Button>
      </div>
    </Dialog>}

    {preview && <Dialog title={preview.mode === "reset" ? "Sichere Standardwerte anwenden?" : "Änderungen speichern?"} onClose={() => !busy && setPreview(null)}>
      <p>Geänderte Felder: <strong>{summarizeChangedFields(preview.change.changed_fields) || "keine"}</strong></p>
      <details className="effective-settings"><summary>Technische Feldnamen anzeigen</summary><pre>{preview.change.changed_fields.join("\n") || "keine"}</pre></details>
      {preview.change.restart_required && <StateMessage kind="error" title="Core-Neustart erforderlich">Input und Hotkeys werden erst nach dem kontrollierten Neustart wirksam.</StateMessage>}
      <div className="modal-actions">
        <Button variant="secondary" onClick={() => setPreview(null)} disabled={busy}>Abbrechen</Button>
        <Button variant={preview.mode === "reset" ? "danger" : "primary"} onClick={() => void applyPreview()} disabled={busy}>{preview.mode === "reset" ? "Standardwerte anwenden" : "Jetzt speichern"}</Button>
      </div>
    </Dialog>}
  </>;
}

function errorText(reason: unknown, fallback: string): string {
  return reason instanceof Error ? reason.message : fallback;
}
