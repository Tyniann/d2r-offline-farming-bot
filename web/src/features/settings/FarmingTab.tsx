import { useEffect, useState } from "react";
import { RotateCcw } from "lucide-react";
import { getRunAvailabilities } from "../../api/generated";
import type { CatalogDTO, OperatorSettingsDTO, StatusDTO } from "../../api/generated";
import { Button, StateMessage, StatusBadge } from "../../app/ui";
import { QueueEditor } from "./QueueEditor";
import { minutesToMs, msToMinutes, pathChanged } from "./settingsDiff";
import type { SettingsRun } from "./settingsTypes";
import { useTranslation } from "react-i18next";
import { presentDifficultyName, presentRunName } from "../../i18n/presenters";

/** FarmingTab enthält das revisionierte Core-Dokument inklusive Queue-Hero. */
export function FarmingTab({
  draft, saved, selectedCharacter, characterNames, catalog, status, runs, mutable, restartRequired, diffPaths,
  onSelectCharacter, onChangeDraft, onReset,
}: {
  draft: OperatorSettingsDTO;
  saved: OperatorSettingsDTO;
  selectedCharacter: string;
  characterNames: string[];
  catalog?: CatalogDTO | null;
  status?: StatusDTO | null;
  runs: SettingsRun[];
  mutable: boolean;
  restartRequired: boolean;
  diffPaths: string[];
  onSelectCharacter: (name: string) => void;
  onChangeDraft: (update: (current: OperatorSettingsDTO) => OperatorSettingsDTO) => void;
  onReset: () => void;
}) {
  const { t } = useTranslation();
  const characterSettings = draft.characters[selectedCharacter];
  const queueChanged = pathChanged(diffPaths, `characters.${selectedCharacter}.queue`) || pathChanged(diffPaths, `characters.${selectedCharacter}.last_difficulty`);
  const budgetsChanged = pathChanged(diffPaths, "budgets");
  const inputChanged = pathChanged(diffPaths, "input");
  const historyChanged = pathChanged(diffPaths, "history");
  const characterLabel = (slug: string) => catalog?.characters.find((entry) => entry.slug === slug)?.name ?? slug;
  const confirmedName = status?.selection.character ?? "";
  const editingLiveCharacter = !!confirmedName && (characterLabel(selectedCharacter) === confirmedName
    || selectedCharacter.toLowerCase() === confirmedName.toLowerCase());
  const unlabeledRuns = () => runs.map((run) => ({
    ...run,
    label: presentRunName(run.id, t),
    status: "",
    reasons: [] as string[],
  }));
  const [characterRuns, setCharacterRuns] = useState<SettingsRun[]>(unlabeledRuns);

  useEffect(() => {
    const name = catalog?.characters.find((entry) => entry.slug === selectedCharacter)?.name ?? selectedCharacter;
    const difficulty = characterSettings?.last_difficulty || status?.selection.difficulty || catalog?.default_difficulty || "";
    const fallback = unlabeledRuns();
    if (!name || !difficulty) {
      setCharacterRuns(fallback);
      return;
    }
    const controller = new AbortController();
    setCharacterRuns(fallback);
    void getRunAvailabilities(name, difficulty, controller.signal)
      .then((value) => {
        if (controller.signal.aborted) return;
        setCharacterRuns((value.runs ?? []).map((run) => ({
          id: run.run_id, label: presentRunName(run.run_id, t), status: run.status, reasons: run.reasons, routeCombat: run.route_combat,
        })));
      })
      .catch(() => { if (!controller.signal.aborted) setCharacterRuns(fallback); });
    return () => controller.abort();
  }, [selectedCharacter, characterSettings?.last_difficulty, catalog, status?.selection.difficulty, runs, t]);

  return <div className="settings-tab-body settings-scope-farming">
    <p className="settings-scope-line">{t("settings.farmingScope")}</p>

    <section className={queueChanged ? "settings-field-changed" : undefined}>
      <div className="section-heading">
        <div><h2>{t("settings.character")}</h2><p>{t("settings.characterDetail")}</p></div>
        {queueChanged && <StatusBadge tone="warning">{t("settings.changed")}</StatusBadge>}
      </div>
      {characterNames.length === 0 || !characterSettings
        ? <p className="hint">{t("settings.noCharacterSettings")}</p>
        : <div className="settings-grid two-columns settings-form-width">
          <label>{t("settings.character")}<select value={selectedCharacter} onChange={(event) => onSelectCharacter(event.target.value)} disabled={!mutable}>{characterNames.map((name) => <option key={name} value={name}>{characterLabel(name)}</option>)}</select></label>
          <label>{t("settings.lastDifficulty")}<select value={characterSettings.last_difficulty} disabled={!mutable} onChange={(event) => onChangeDraft((next) => { next.characters[selectedCharacter].last_difficulty = event.target.value as "normal" | "nightmare" | "hell"; return next; })}>{["normal", "nightmare", "hell"].map((id) => <option key={id} value={id}>{presentDifficultyName(id, t)}</option>)}</select></label>
        </div>}
    </section>

    {characterSettings && <section>
      {!editingLiveCharacter && confirmedName && <StateMessage kind="error" title={t("settings.otherQueueTitle")}>{t("settings.otherQueueDetail", { edited: characterLabel(selectedCharacter), active: confirmedName })}</StateMessage>}
      <QueueEditor
        queue={characterSettings.queue}
        runs={characterRuns}
        mutable={mutable}
        changed={pathChanged(diffPaths, `characters.${selectedCharacter}.queue`)}
        characterClass={characterSettings.character_class}
        onChange={(queue) => onChangeDraft((next) => { next.characters[selectedCharacter].queue = queue; return next; })}
      />
    </section>}

    <section className={budgetsChanged ? "settings-field-changed" : undefined}>
      <div className="section-heading">
        <div><h2>{t("settings.budgets")}</h2><p>{t("settings.budgetsDetail")}</p></div>
        {budgetsChanged && <StatusBadge tone="warning">{t("settings.changed")}</StatusBadge>}
      </div>
      <div className="settings-grid four-columns settings-form-width">
        <NumberField label={t("settings.maxRuns")} value={draft.budgets.max_runs} disabled={!mutable} changed={diffPaths.includes("budgets.max_runs")} onChange={(value) => onChangeDraft((next) => { next.budgets.max_runs = value; return next; })} />
        <NumberField label={t("settings.maxDuration")} value={msToMinutes(draft.budgets.max_duration_ms)} disabled={!mutable} changed={diffPaths.includes("budgets.max_duration_ms")} suffix={t("settings.minutes")} onChange={(value) => onChangeDraft((next) => { next.budgets.max_duration_ms = minutesToMs(value); return next; })} />
        <NumberField label={t("settings.consecutiveErrors")} value={draft.budgets.max_consecutive_failures} disabled={!mutable} changed={diffPaths.includes("budgets.max_consecutive_failures")} onChange={(value) => onChangeDraft((next) => { next.budgets.max_consecutive_failures = value; return next; })} />
        <NumberField label={t("settings.totalRestarts")} value={draft.budgets.max_total_restarts} disabled={!mutable} changed={diffPaths.includes("budgets.max_total_restarts")} onChange={(value) => onChangeDraft((next) => { next.budgets.max_total_restarts = value; return next; })} />
      </div>
    </section>

    <section className={inputChanged ? "settings-field-changed" : undefined}>
      <div className="section-heading">
        <div><h2>{t("settings.inputTitle")}</h2><p>{t("settings.inputDetail")}</p></div>
        <div className="inline-actions">
          {restartRequired && <StatusBadge tone="warning">{t("settings.restartRequired")}</StatusBadge>}
          {inputChanged && <StatusBadge tone="warning">{t("settings.changed")}</StatusBadge>}
        </div>
      </div>
      <div className="settings-form-width">
        <label className={`check${diffPaths.includes("input.enabled") ? " settings-field-changed" : ""}`}><input type="checkbox" checked={draft.input.enabled} disabled={!mutable} onChange={(event) => onChangeDraft((next) => { next.input.enabled = event.target.checked; return next; })} /> {t("settings.enableInput")}</label>
        <div className="settings-grid four-columns">
          <TextField label={t("settings.pause")} value={draft.input.pause_hotkey} disabled={!mutable} changed={diffPaths.includes("input.pause_hotkey")} onChange={(value) => onChangeDraft((next) => { next.input.pause_hotkey = value; return next; })} />
          <TextField label={t("settings.stopAfterRun")} value={draft.input.stop_after_run_hotkey} disabled={!mutable} changed={diffPaths.includes("input.stop_after_run_hotkey")} onChange={(value) => onChangeDraft((next) => { next.input.stop_after_run_hotkey = value; return next; })} />
          <TextField label={t("settings.finishRecording")} value={draft.input.recording_finish_hotkey} disabled={!mutable} changed={diffPaths.includes("input.recording_finish_hotkey")} onChange={(value) => onChangeDraft((next) => { next.input.recording_finish_hotkey = value; return next; })} />
          <TextField label={t("settings.emergencyStop")} value={draft.input.emergency_stop_hotkey} disabled={!mutable} changed={diffPaths.includes("input.emergency_stop_hotkey")} onChange={(value) => onChangeDraft((next) => { next.input.emergency_stop_hotkey = value; return next; })} />
        </div>
      </div>
    </section>

    <section className={historyChanged ? "settings-field-changed" : undefined}>
      <div className="section-heading">
        <div><h2>{t("settings.retentionTitle")}</h2><p>{t("settings.retentionDetail")}</p></div>
        {historyChanged && <StatusBadge tone="warning">{t("settings.changed")}</StatusBadge>}
      </div>
      <div className="settings-grid two-columns settings-form-width">
        <label className={`check${diffPaths.includes("history.retention_enabled") ? " settings-field-changed" : ""}`}><input type="checkbox" checked={draft.history.retention_enabled} disabled={!mutable} onChange={(event) => onChangeDraft((next) => { next.history.retention_enabled = event.target.checked; return next; })} /> {t("settings.enableRetention")}</label>
        <NumberField label={t("settings.retentionDays")} value={draft.history.retention_days} disabled={!mutable} changed={diffPaths.includes("history.retention_days")} suffix={t("settings.days")} onChange={(value) => onChangeDraft((next) => { next.history.retention_days = value; return next; })} />
      </div>
    </section>

    <section className="settings-reset-block">
      <div className="section-heading"><div><h2>{t("settings.resetTitle")}</h2><p>{t("settings.resetDetail")}</p></div></div>
      <Button variant="danger" onClick={onReset} disabled={!mutable}><RotateCcw aria-hidden="true" size={16} /> {t("settings.resetSafe")}</Button>
    </section>

    {/* saved revision badge context for screen readers */}
    <span className="visually-hidden">{t("settings.savedRevision", { revision: saved.revision })}</span>
  </div>;
}

function NumberField({ label, value, onChange, disabled, changed, suffix }: { label: string; value: number; onChange: (value: number) => void; disabled?: boolean; changed?: boolean; suffix?: string }) {
  return <label className={changed ? "settings-field-changed" : undefined}>{label}{suffix ? ` (${suffix})` : ""}<input type="number" min={1} step={1} value={value} disabled={disabled} onChange={(event) => onChange(Number(event.target.value))} /></label>;
}

function TextField({ label, value, onChange, disabled, changed }: { label: string; value: string; onChange: (value: string) => void; disabled?: boolean; changed?: boolean }) {
  return <label className={changed ? "settings-field-changed" : undefined}>{label}<input value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)} /></label>;
}
