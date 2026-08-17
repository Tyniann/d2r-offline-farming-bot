import { useEffect, useState } from "react";
import { RotateCcw } from "lucide-react";
import { getRunAvailabilities } from "../../api/generated";
import type { CatalogDTO, OperatorSettingsDTO, StatusDTO } from "../../api/generated";
import { Button, StateMessage, StatusBadge } from "../../app/ui";
import { QueueEditor } from "./QueueEditor";
import { minutesToMs, msToMinutes, pathChanged } from "./settingsDiff";
import type { SettingsRun } from "./settingsTypes";

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
  const characterSettings = draft.characters[selectedCharacter];
  const queueChanged = pathChanged(diffPaths, `characters.${selectedCharacter}.queue`) || pathChanged(diffPaths, `characters.${selectedCharacter}.last_difficulty`);
  const budgetsChanged = pathChanged(diffPaths, "budgets");
  const inputChanged = pathChanged(diffPaths, "input");
  const historyChanged = pathChanged(diffPaths, "history");
  const characterLabel = (slug: string) => catalog?.characters.find((entry) => entry.slug === slug)?.name ?? slug;
  const confirmedName = status?.selection.character ?? "";
  const editingLiveCharacter = !!confirmedName && (characterLabel(selectedCharacter) === confirmedName
    || selectedCharacter.toLowerCase() === confirmedName.toLowerCase());
  const unlabeledRuns = () => runs.map((run) => ({ ...run, status: "", reasons: [] as string[] }));
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
          id: run.run_id, label: run.display_name, status: run.status, reasons: run.reasons, routeCombat: run.route_combat,
        })));
      })
      .catch(() => { if (!controller.signal.aborted) setCharacterRuns(fallback); });
    return () => controller.abort();
  }, [selectedCharacter, characterSettings?.last_difficulty, catalog, status?.selection.difficulty]);

  return <div className="settings-tab-body settings-scope-farming">
    <p className="settings-scope-line">Alles in diesem Bereich wird gemeinsam als eine Core-Revision gespeichert.</p>

    <section className={queueChanged ? "settings-field-changed" : undefined}>
      <div className="section-heading">
        <div><h2>Charakter</h2><p>Bestimmt, für wen Queue und Schwierigkeit gelten.</p></div>
        {queueChanged && <StatusBadge tone="warning">Geändert</StatusBadge>}
      </div>
      {characterNames.length === 0 || !characterSettings
        ? <p className="hint">Der Core hat noch keine charakterbezogenen Operatorwerte geliefert.</p>
        : <div className="settings-grid two-columns settings-form-width">
          <label>Charakter<select value={selectedCharacter} onChange={(event) => onSelectCharacter(event.target.value)} disabled={!mutable}>{characterNames.map((name) => <option key={name} value={name}>{characterLabel(name)}</option>)}</select></label>
          <label>Letzte Schwierigkeit<select value={characterSettings.last_difficulty} disabled={!mutable} onChange={(event) => onChangeDraft((next) => { next.characters[selectedCharacter].last_difficulty = event.target.value as "normal" | "nightmare" | "hell"; return next; })}><option value="normal">Normal</option><option value="nightmare">Alptraum</option><option value="hell">Hölle</option></select></label>
        </div>}
    </section>

    {characterSettings && <section>
      {!editingLiveCharacter && confirmedName && <StateMessage kind="error" title="Andere gespeicherte Reihenfolge">Du bearbeitest die gespeicherte Reihenfolge von {characterLabel(selectedCharacter)}. Gestartet wird weiterhin {confirmedName}, bis du ihn in D2R anwendest.</StateMessage>}
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
        <div><h2>Sicherheitsgrenzen</h2><p>Harte Limits für Runs, Dauer und Fehlerketten.</p></div>
        {budgetsChanged && <StatusBadge tone="warning">Geändert</StatusBadge>}
      </div>
      <div className="settings-grid four-columns settings-form-width">
        <NumberField label="Maximale Runs" value={draft.budgets.max_runs} disabled={!mutable} changed={diffPaths.includes("budgets.max_runs")} onChange={(value) => onChangeDraft((next) => { next.budgets.max_runs = value; return next; })} />
        <NumberField label="Maximale Dauer" value={msToMinutes(draft.budgets.max_duration_ms)} disabled={!mutable} changed={diffPaths.includes("budgets.max_duration_ms")} suffix="Min." onChange={(value) => onChangeDraft((next) => { next.budgets.max_duration_ms = minutesToMs(value); return next; })} />
        <NumberField label="Fehler in Folge" value={draft.budgets.max_consecutive_failures} disabled={!mutable} changed={diffPaths.includes("budgets.max_consecutive_failures")} onChange={(value) => onChangeDraft((next) => { next.budgets.max_consecutive_failures = value; return next; })} />
        <NumberField label="Gesamte Restarts" value={draft.budgets.max_total_restarts} disabled={!mutable} changed={diffPaths.includes("budgets.max_total_restarts")} onChange={(value) => onChangeDraft((next) => { next.budgets.max_total_restarts = value; return next; })} />
      </div>
    </section>

    <section className={inputChanged ? "settings-field-changed" : undefined}>
      <div className="section-heading">
        <div><h2>Steuerung &amp; Hotkeys</h2><p>Input-Freigabe und Tasten gelten erst nach kontrolliertem Core-Neustart.</p></div>
        <div className="inline-actions">
          {restartRequired && <StatusBadge tone="warning">Neustart nötig</StatusBadge>}
          {inputChanged && <StatusBadge tone="warning">Geändert</StatusBadge>}
        </div>
      </div>
      <div className="settings-form-width">
        <label className={`check${diffPaths.includes("input.enabled") ? " settings-field-changed" : ""}`}><input type="checkbox" checked={draft.input.enabled} disabled={!mutable} onChange={(event) => onChangeDraft((next) => { next.input.enabled = event.target.checked; return next; })} /> Gameplay-Input ausdrücklich freigeben</label>
        <div className="settings-grid four-columns">
          <TextField label="Pause" value={draft.input.pause_hotkey} disabled={!mutable} changed={diffPaths.includes("input.pause_hotkey")} onChange={(value) => onChangeDraft((next) => { next.input.pause_hotkey = value; return next; })} />
          <TextField label="Stop nach Run" value={draft.input.stop_after_run_hotkey} disabled={!mutable} changed={diffPaths.includes("input.stop_after_run_hotkey")} onChange={(value) => onChangeDraft((next) => { next.input.stop_after_run_hotkey = value; return next; })} />
          <TextField label="Aufnahme beenden" value={draft.input.recording_finish_hotkey} disabled={!mutable} changed={diffPaths.includes("input.recording_finish_hotkey")} onChange={(value) => onChangeDraft((next) => { next.input.recording_finish_hotkey = value; return next; })} />
          <TextField label="Emergency Stop" value={draft.input.emergency_stop_hotkey} disabled={!mutable} changed={diffPaths.includes("input.emergency_stop_hotkey")} onChange={(value) => onChangeDraft((next) => { next.input.emergency_stop_hotkey = value; return next; })} />
        </div>
      </div>
    </section>

    <section className={historyChanged ? "settings-field-changed" : undefined}>
      <div className="section-heading">
        <div><h2>Verlauf aufbewahren</h2><p>Vorhandene Historie löschst du unter <strong>Wartung</strong>.</p></div>
        {historyChanged && <StatusBadge tone="warning">Geändert</StatusBadge>}
      </div>
      <div className="settings-grid two-columns settings-form-width">
        <label className={`check${diffPaths.includes("history.retention_enabled") ? " settings-field-changed" : ""}`}><input type="checkbox" checked={draft.history.retention_enabled} disabled={!mutable} onChange={(event) => onChangeDraft((next) => { next.history.retention_enabled = event.target.checked; return next; })} /> Automatische Retention aktiv</label>
        <NumberField label="Retention in Tagen" value={draft.history.retention_days} disabled={!mutable} changed={diffPaths.includes("history.retention_days")} suffix="Tage" onChange={(value) => onChangeDraft((next) => { next.history.retention_days = value; return next; })} />
      </div>
    </section>

    <section className="settings-reset-block">
      <div className="section-heading"><div><h2>Zurücksetzen</h2><p>Ersetzt allgemeine Operatorwerte durch sichere Standardwerte. Charakter-Setup bleibt erhalten.</p></div></div>
      <Button variant="danger" onClick={onReset} disabled={!mutable}><RotateCcw aria-hidden="true" size={16} /> Auf sichere Standardwerte zurücksetzen</Button>
    </section>

    {/* saved revision badge context for screen readers */}
    <span className="visually-hidden">Gespeicherte Revision {saved.revision}</span>
  </div>;
}

function NumberField({ label, value, onChange, disabled, changed, suffix }: { label: string; value: number; onChange: (value: number) => void; disabled?: boolean; changed?: boolean; suffix?: string }) {
  return <label className={changed ? "settings-field-changed" : undefined}>{label}{suffix ? ` (${suffix})` : ""}<input type="number" min={1} step={1} value={value} disabled={disabled} onChange={(event) => onChange(Number(event.target.value))} /></label>;
}

function TextField({ label, value, onChange, disabled, changed }: { label: string; value: string; onChange: (value: string) => void; disabled?: boolean; changed?: boolean }) {
  return <label className={changed ? "settings-field-changed" : undefined}>{label}<input value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)} /></label>;
}
