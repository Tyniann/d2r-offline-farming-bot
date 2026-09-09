import type { ReactNode } from "react";
import { FolderOpen, RefreshCw, RotateCcw } from "lucide-react";
import { Button, Dialog, StateMessage, StatusBadge } from "../../app/ui";
import { formatDate, formatNumber } from "../../i18n/format";
import { presentDifficultyName } from "../../i18n/presenters";
import { BindingEditor } from "../characters/BindingEditor";
import { CharacterSetupWizard } from "../characters/CharacterSetupWizard";
import { farmReadyReasonText } from "../characters/characterReasonText";
import { InventoryLockEditor } from "../characters/InventoryLockEditor";
import { RequiredSkillsList } from "../characters/RequiredSkillsList";
import { QueueEditor } from "./QueueEditor";
import { minutesToMs, msToMinutes, summarizeChangedFields } from "./settingsDiff";
import type { CharacterContext, SettingsModel } from "./settingsModel";
import necromancerBadge from "../../assets/classes/necromancer-build-badge.png";
import paladinBadge from "../../assets/classes/paladin-build-badge.png";
import sorceressBadge from "../../assets/classes/blizzard-build-badges.png";

// Bausteine der Einstellungsseite: Felder, Editoren, Dialoge. Kein Layout – das ordnet SettingsOverview an.

/** profileBadgeSource wählt das grafische Kampfprofil-Badge nach Profil-ID, sonst nach Klasse. */
export function profileBadgeSource(profileID: string, characterClass: string): string | null {
  if (profileID.startsWith("necro") || characterClass === "necromancer") return necromancerBadge;
  if (profileID.startsWith("paladin") || characterClass === "paladin") return paladinBadge;
  if (characterClass === "sorceress") return sorceressBadge;
  return null;
}

/**
 * ProfileBadge zeigt das Medaillon eines Kampfprofils als runden Ausschnitt. Ohne `label` ist es dekorativ
 * (der begleitende Text benennt den Charakter); mit `label` wird es als eigenständiges Bild vorgelesen.
 */
export function ProfileBadge({ profileID = "", characterClass = "", size = "md", muted = false, label }: { profileID?: string; characterClass?: string; size?: "sm" | "md"; muted?: boolean; label?: string }) {
  const source = profileBadgeSource(profileID, characterClass);
  return <span className={`profile-badge profile-badge-${size}${muted ? " profile-badge-muted" : ""}`} aria-hidden={label ? undefined : true} role={label ? "img" : undefined} aria-label={label}>
    {source ? <img src={source} alt="" /> : <span className="profile-badge-fallback">{(characterClass || "?").slice(0, 1).toUpperCase()}</span>}
  </span>;
}

/** Switch ist eine Checkbox in Schalteroptik; die Beschriftung bleibt Teil des Labels. */
export function Switch({ checked, disabled, onChange, label }: { checked: boolean; disabled?: boolean; onChange: (value: boolean) => void; label: ReactNode }) {
  return <label className="settings-switch">
    <input type="checkbox" checked={checked} disabled={disabled} onChange={(event) => onChange(event.target.checked)} />
    <span className="settings-switch-track" aria-hidden="true"><span className="settings-switch-knob" /></span>
    <span className="settings-switch-text">{label}</span>
  </label>;
}

/** KeyCap zeigt eine Taste in Tastenkappenoptik. */
export function KeyCap({ children }: { children: ReactNode }) {
  return <kbd className="settings-key">{String(children).toUpperCase()}</kbd>;
}

/** ChangeDot markiert eine noch nicht gespeicherte Änderung. */
export function ChangeDot({ title }: { title: string }) {
  return <span className="settings-change-dot" title={title} aria-label={title} />;
}

/** NumberInput ist ein Zahlenfeld mit optionaler Einheit hinter dem Feld. */
export function NumberInput({ value, onChange, disabled, changed, min = 1, suffix, ariaLabel }: { value: number; onChange: (value: number) => void; disabled?: boolean; changed?: boolean; min?: number; suffix?: string; ariaLabel: string }) {
  return <span className={`settings-number${changed ? " settings-changed" : ""}`}>
    <input type="number" min={min} step={1} value={value} disabled={disabled} aria-label={ariaLabel} onChange={(event) => onChange(Number(event.target.value))} />
    {suffix && <span className="settings-suffix">{suffix}</span>}
  </span>;
}

/** SettingsNotices zeigt Sperre, veraltete Revision, Fehler, Erfolg und Neustartpflicht oberhalb der Flächen. */
export function SettingsNotices({ model }: { model: SettingsModel }) {
  const { t } = model;
  return <div className="settings-notices">
    {!model.mutable && <StateMessage kind="error" title={t("settings.lockedTitle")}>{t("settings.lockedSession")} {t("settings.saveWhenIdle")}</StateMessage>}
    {model.stale && <div className="settings-feedback"><StateMessage kind="error" title={t("settings.staleTitle")}>{t("settings.staleDetail")}</StateMessage><Button variant="secondary" onClick={() => void model.load()}>{t("settings.reload")}</Button></div>}
    {model.error && !model.stale && <StateMessage kind="error" title={t("settings.changeFailedTitle")}>{model.error}</StateMessage>}
    {model.message && <div className="settings-success" role="status"><strong>{model.message}</strong></div>}
    {model.restartRequired && <div className="restart-required" role="status"><div><strong>{t("settings.restartCoreTitle")}</strong><p>{t("settings.restartCoreDetail")}</p></div><Button onClick={() => void model.restartCore()} disabled={!model.mutable || model.busy || !window.d2rDesktop}>{t("settings.restartCore")}</Button></div>}
  </div>;
}

/** SettingsDialogs enthält die Bestätigungen für Speichern/Zurücksetzen und für das Löschen der Historie. */
export function SettingsDialogs({ model }: { model: SettingsModel }) {
  const { t, preview, deletePreview, busy } = model;
  return <>
    {deletePreview && <Dialog title={t("settings.deleteConfirmTitle")} onClose={() => !busy && model.closeDeletePreview()}>
      <p>{t("settings.deletePreviewDetail", { files: formatNumber(deletePreview.candidate_files), bytes: formatNumber(deletePreview.candidate_bytes), protected: formatNumber(deletePreview.protected_files) })}</p>
      <p>{t("settings.categories", { categories: Object.entries(deletePreview.categories).map(([name, count]) => `${name}: ${formatNumber(count)}`).join(" · ") || t("history.none") })}</p>
      <StateMessage kind="error" title={t("settings.irreversible")}>{t("settings.irreversibleDetail")}</StateMessage>
      <div className="modal-actions">
        <Button variant="secondary" onClick={model.closeDeletePreview} disabled={busy}>{t("common.cancel")}</Button>
        <Button variant="danger" onClick={() => void model.deleteHistory()} disabled={busy || deletePreview.candidate_files === 0}>{t("settings.deleteAll")}</Button>
      </div>
    </Dialog>}
    {preview && <Dialog title={t(preview.mode === "reset" ? "settings.applyDefaultsTitle" : "settings.saveChangesTitle")} onClose={() => !busy && model.closePreview()}>
      <p>{t("settings.changedFields")}<strong>{summarizeChangedFields(preview.change.changed_fields) || t("history.none")}</strong></p>
      <details className="effective-settings"><summary>{t("settings.technicalFields")}</summary><pre>{preview.change.changed_fields.join("\n") || t("history.none")}</pre></details>
      {preview.change.restart_required && <StateMessage kind="error" title={t("settings.restartCoreTitle")}>{t("settings.restartEffectiveDetail")}</StateMessage>}
      <div className="modal-actions">
        <Button variant="secondary" onClick={model.closePreview} disabled={busy}>{t("common.cancel")}</Button>
        <Button variant={preview.mode === "reset" ? "danger" : "primary"} onClick={() => void model.applyPreview()} disabled={busy}>{t(preview.mode === "reset" ? "settings.applyDefaults" : "settings.saveNow")}</Button>
      </div>
    </Dialog>}
  </>;
}

// ---------------------------------------------------------------------------
// Charakterbezogene Bausteine
// ---------------------------------------------------------------------------

export function DifficultySelect({ ctx, model }: { ctx: CharacterContext; model: SettingsModel }) {
  const { t } = model;
  return <select aria-label={t("settings.lastDifficulty")} value={ctx.settings?.last_difficulty ?? "normal"} disabled={!model.mutable || !ctx.settings} onChange={(event) => ctx.setDifficulty(event.target.value as "normal" | "nightmare" | "hell")}>
    {["normal", "nightmare", "hell"].map((id) => <option key={id} value={id}>{presentDifficultyName(id, t)}</option>)}
  </select>;
}

export function PlayersSelect({ ctx, model }: { ctx: CharacterContext; model: SettingsModel }) {
  const { t } = model;
  return <select aria-label={t("characters.playersAria")} value={ctx.players} disabled={!model.mutable || !ctx.settings} onChange={(event) => ctx.setPlayers(Number(event.target.value))}>
    {[1, 2, 3, 4, 5, 6, 7, 8].map((value) => <option key={value} value={value}>{value}</option>)}
  </select>;
}

/** QueueBlock bettet den Routen-Editor ein und warnt, wenn nicht die in D2R bestätigte Reihenfolge bearbeitet wird. */
export function QueueBlock({ ctx, model }: { ctx: CharacterContext; model: SettingsModel }) {
  const { t } = model;
  if (!ctx.settings) return null;
  const confirmedName = model.props.status?.selection.character ?? "";
  const editingLive = !!confirmedName && ctx.name.toLowerCase() === confirmedName.toLowerCase();
  return <div className="settings-embedded-editor">
    {!editingLive && confirmedName && <StateMessage kind="error" title={t("settings.otherQueueTitle")}>{t("settings.otherQueueDetail", { edited: ctx.name, active: confirmedName })}</StateMessage>}
    <QueueEditor queue={ctx.settings.queue} runs={ctx.runs} mutable={model.mutable} changed={ctx.queueChanged} characterClass={ctx.settings.character_class} onChange={ctx.setQueue} />
  </div>;
}

/** ProfileSwitchBlock bettet den Profilwechsel des Charakter-Setups ein (eigener Core-Command, nicht Teil des Drafts). */
export function ProfileSwitchBlock({ ctx, model }: { ctx: CharacterContext; model: SettingsModel }) {
  const { status, catalog } = model.props;
  if (!ctx.settings || !status || !catalog) return null;
  return <div className="settings-embedded-editor settings-plain-setup">
    <CharacterSetupWizard character={ctx.name} catalog={catalog} status={status} mode="settings" allowDeferBindings={false} showReload={false} onChanged={model.props.onSettingsApplied} />
  </div>;
}

export function SkillsBlock({ ctx }: { ctx: CharacterContext }) {
  if (!ctx.profile) return null;
  return <RequiredSkillsList skills={ctx.profile.required_skills ?? []} standardAttack={ctx.profile.standard_attack} />;
}

export function BindingsBlock({ ctx, model }: { ctx: CharacterContext; model: SettingsModel }) {
  const { t } = model;
  if (ctx.previewError) return <StateMessage kind="error" title={t("characters.previewFailed")}>{ctx.previewError}</StateMessage>;
  if (!ctx.profile) return <StateMessage kind="loading" title={t("characters.loading")} />;
  if (!ctx.profileID) return <StateMessage kind="empty" title={t("characters.noProfileTitle")}>{t("characters.noProfileDetail")}</StateMessage>;
  return <BindingEditor
    requiredSkills={ctx.profile.required_skills ?? []}
    optionalSkillPairs={ctx.profile.optional_skill_pairs ?? []}
    standardAttack={ctx.profile.standard_attack}
    requiresMercenary={ctx.profile.requires_mercenary}
    bindingsReady={ctx.profile.bindings_ready}
    bindingReasons={ctx.profile.binding_reasons ?? []}
    value={ctx.bindingValue}
    mutable={model.mutable}
    onChange={ctx.setBindings}
  />;
}

export function InventoryBlock({ ctx, model }: { ctx: CharacterContext; model: SettingsModel }) {
  const { t } = model;
  return <>
    <InventoryLockEditor value={ctx.grid} configured={ctx.inventoryIsConfigured} mutable={model.mutable} onChange={ctx.setInventory} />
    {ctx.cowWarning && <StateMessage kind="error" title={t("characters.cowWarningTitle")}>{t("characters.cowWarningDetail")}</StateMessage>}
  </>;
}

/** ReadinessNote erklärt, warum die Queue eines Charakters (noch) nicht starten darf. */
export function ReadinessNote({ ctx, model }: { ctx: CharacterContext; model: SettingsModel }) {
  const { t } = model;
  const entry = ctx.entry;
  if (!entry) return null;
  if (!entry.farm_ready && (entry.farm_ready_reasons?.length ?? 0) > 0) {
    return <StateMessage kind="error" title={t("characters.queueLocked")}>{(entry.farm_ready_reasons ?? []).map((reason) => farmReadyReasonText(reason, t)).join(" ")}</StateMessage>;
  }
  if (entry.farm_ready) return <StateMessage kind="empty" title={t("characters.farmReady")}>{t("characters.farmReadyDetail")}</StateMessage>;
  return null;
}

/** InventoryPreview zeigt das 4×10-Raster read-only in Miniatur; unbestätigt gilt alles als geschützt. */
export function InventoryPreview({ ctx }: { ctx: CharacterContext }) {
  const grid = ctx.inventoryIsConfigured ? ctx.grid! : Array.from({ length: 4 }, () => Array.from({ length: 10 }, () => 1));
  return <div className="settings-inventory-mini" aria-hidden="true">{grid.map((row, rowIndex) => <div key={rowIndex}>{row.map((cell, colIndex) => <span key={colIndex} className={cell === 1 ? "locked" : "free"} />)}</div>)}</div>;
}

// ---------------------------------------------------------------------------
// Gemeinsam gespeicherte Werte (Grenzen, Steuerung, Verlauf)
// ---------------------------------------------------------------------------

export function BudgetFields({ model }: { model: SettingsModel }) {
  const { t, draft, diffPaths, mutable, changeDraft } = model;
  if (!draft) return null;
  return <div className="settings-fields">
    <label className={diffPaths.includes("budgets.max_runs") ? "settings-changed" : undefined}>{t("settings.maxRuns")}<input type="number" min={1} value={draft.budgets.max_runs} disabled={!mutable} onChange={(event) => changeDraft((next) => { next.budgets.max_runs = Number(event.target.value); return next; })} /></label>
    <label className={diffPaths.includes("budgets.max_duration_ms") ? "settings-changed" : undefined}>{t("settings.maxDuration")} ({t("settings.minutes")})<input type="number" min={1} value={msToMinutes(draft.budgets.max_duration_ms)} disabled={!mutable} onChange={(event) => changeDraft((next) => { next.budgets.max_duration_ms = minutesToMs(Number(event.target.value)); return next; })} /></label>
    <label className={diffPaths.includes("budgets.max_consecutive_failures") ? "settings-changed" : undefined}>{t("settings.consecutiveErrors")}<input type="number" min={1} value={draft.budgets.max_consecutive_failures} disabled={!mutable} onChange={(event) => changeDraft((next) => { next.budgets.max_consecutive_failures = Number(event.target.value); return next; })} /></label>
    <label className={diffPaths.includes("budgets.max_total_restarts") ? "settings-changed" : undefined}>{t("settings.totalRestarts")}<input type="number" min={1} value={draft.budgets.max_total_restarts} disabled={!mutable} onChange={(event) => changeDraft((next) => { next.budgets.max_total_restarts = Number(event.target.value); return next; })} /></label>
  </div>;
}

export function InputSwitch({ model }: { model: SettingsModel }) {
  const { t, draft, mutable, changeDraft } = model;
  if (!draft) return null;
  return <Switch checked={draft.input.enabled} disabled={!mutable} onChange={(value) => changeDraft((next) => { next.input.enabled = value; return next; })} label={t("settings.enableInput")} />;
}

export function HotkeyFields({ model }: { model: SettingsModel }) {
  const { t, draft, diffPaths, mutable, changeDraft } = model;
  if (!draft) return null;
  const field = (key: "pause_hotkey" | "stop_after_run_hotkey" | "recording_finish_hotkey" | "emergency_stop_hotkey", label: string) => (
    <label key={key} className={diffPaths.includes(`input.${key}`) ? "settings-changed" : undefined}>{label}<input value={draft.input[key]} disabled={!mutable} onChange={(event) => changeDraft((next) => { next.input[key] = event.target.value; return next; })} /></label>
  );
  return <div className="settings-fields">
    {field("pause_hotkey", t("settings.pause"))}
    {field("stop_after_run_hotkey", t("settings.stopAfterRun"))}
    {field("recording_finish_hotkey", t("settings.finishRecording"))}
    {field("emergency_stop_hotkey", t("settings.emergencyStop"))}
  </div>;
}

export function RetentionSwitch({ model }: { model: SettingsModel }) {
  const { t, draft, mutable, changeDraft } = model;
  if (!draft) return null;
  return <Switch checked={draft.history.retention_enabled} disabled={!mutable} onChange={(value) => changeDraft((next) => { next.history.retention_enabled = value; return next; })} label={t("settings.enableRetention")} />;
}

export function RetentionDays({ model }: { model: SettingsModel }) {
  const { t, draft, diffPaths, mutable, changeDraft } = model;
  if (!draft) return null;
  return <NumberInput ariaLabel={t("settings.retentionDays")} value={draft.history.retention_days} disabled={!mutable || !draft.history.retention_enabled} changed={diffPaths.includes("history.retention_days")} suffix={t("settings.days")} onChange={(value) => changeDraft((next) => { next.history.retention_days = value; return next; })} />;
}

export function ResetButton({ model }: { model: SettingsModel }) {
  const { t } = model;
  return <Button variant="secondary" className="settings-quiet-danger" onClick={() => void model.requestReset()} disabled={!model.mutable || model.busy}><RotateCcw aria-hidden="true" size={15} /> {t("settings.resetSafe")}</Button>;
}

// ---------------------------------------------------------------------------
// System (sofort wirksam) und Wartung
// ---------------------------------------------------------------------------

export function AutostartSwitch({ model }: { model: SettingsModel }) {
  const { t, desktop } = model;
  if (!desktop) return <StatusBadge tone="neutral">{t("settings.bridgeMissing")}</StatusBadge>;
  return <span className="settings-inline">
    <Switch checked={desktop.autostart} disabled={model.busy || !window.d2rDesktop} onChange={(value) => void model.toggleAutostart(value)} label={t("settings.autostart")} />
    {model.autostartFlash && <span className="settings-inline-saved" role="status">{t("settings.saved")}</span>}
  </span>;
}

export function OnboardingRow({ model }: { model: SettingsModel }) {
  const { t, desktop } = model;
  return <span className="settings-inline">
    <span className="hint">{desktop ? t("settings.onboardingStatus", { status: t(desktop.onboarding_completed ? "settings.completed" : "settings.stillOpen") }) : t("settings.bridgeInactive")}</span>
    {model.props.onOpenOnboarding && <Button variant="secondary" onClick={model.props.onOpenOnboarding}>{t("settings.openOnboarding")}</Button>}
  </span>;
}

export function UpdateBadge({ model }: { model: SettingsModel }) {
  const status = model.updateStatus?.status;
  return <StatusBadge tone={status === "available" ? "warning" : status === "up_to_date" ? "success" : "neutral"}>{model.updateLabel()}</StatusBadge>;
}

export function UpdateActions({ model }: { model: SettingsModel }) {
  const { t } = model;
  return <span className="settings-inline">
    <Button variant="secondary" onClick={() => void model.checkForUpdates()} disabled={model.busy || !window.d2rDesktop?.checkForUpdates}><RefreshCw aria-hidden="true" size={15} /> {t("settings.checkAgain")}</Button>
    {model.updateStatus?.status === "available" && <Button onClick={() => void window.d2rDesktop?.openReleasePage?.()}>{t("settings.openRelease")}</Button>}
  </span>;
}

export function DiagnosticBlock({ model }: { model: SettingsModel }) {
  const { t } = model;
  return <div className="settings-stack">
    <p className="hint">{t("settings.diagnosticDetail")} {t("settings.diagnosticExcludes")}</p>
    <label className="check"><input type="checkbox" checked={model.includeTelemetry} onChange={(event) => model.setIncludeTelemetry(event.target.checked)} /> {t("settings.includeTelemetry")}</label>
    <label className="check"><input type="checkbox" checked={model.includeRoutes} onChange={(event) => model.setIncludeRoutes(event.target.checked)} /> {t("settings.includeRoutes")}</label>
    <div><Button onClick={() => void model.buildDiagnosticBundle()} disabled={model.busy}><FolderOpen aria-hidden="true" size={15} /> {t("settings.buildDiagnostic")}</Button></div>
  </div>;
}

export function EventsList({ model }: { model: SettingsModel }) {
  const { t } = model;
  const events = model.props.events;
  if (events.length === 0) return <StateMessage kind="empty" title={t("settings.noEvents")}>{t("settings.noEventsDetail")}</StateMessage>;
  return <ol className="event-feed settings-event-feed">{events.map((event) => <li key={event.sequence}><time>{formatDate(event.timestamp, { timeStyle: "medium" })}</time><strong>{event.event}</strong><span>{event.area || event.step || event.reason || t("settings.coreUpdate")}</span></li>)}</ol>;
}

export function EffectiveValues({ model }: { model: SettingsModel }) {
  const { t, settings } = model;
  return <>
    <details className="effective-settings settings-details"><summary>{t("settings.effectiveOperator")}</summary><p>{t("settings.file")} <code>configs/operator-settings.local.yaml</code></p><pre>{JSON.stringify(settings, null, 2)}</pre></details>
    <details className="effective-settings settings-details"><summary>{t("settings.effectiveRouteCombat")}</summary><p>{t("settings.routeCombatDetail")}</p><pre>{JSON.stringify(Object.fromEntries(model.props.runs.map((run) => [run.id, run.routeCombat ?? null])), null, 2)}</pre></details>
  </>;
}

export function DeleteHistoryButton({ model }: { model: SettingsModel }) {
  const { t } = model;
  return <Button variant="danger" onClick={() => void model.previewDeleteHistory()} disabled={!model.mutable || model.busy}>{t("settings.previewDelete")}</Button>;
}

/**
 * SaveBar ist die am unteren Rand angedockte Commit-Leiste. Sie erscheint nur bei offenen Änderungen oder
 * gesperrtem Core; im sauberen Zustand ersetzt sie eine Fußzeile (siehe SettingsOverview).
 */
export function SaveBar({ model }: { model: SettingsModel }) {
  const { t, dirty, mutable, busy } = model;
  return <div className={`settings-savebar${dirty ? " dirty" : ""}${!mutable ? " locked" : ""}`} role="status">
    <div className="settings-savebar-text">
      {!mutable
        ? <><strong>{t("settings.lockedSession")}</strong><small>{t("settings.saveWhenIdle")}</small></>
        : <><strong>{t("settings.changeCount", { count: model.changeCount, summary: model.dirtySummary })}</strong><small>{t("settings.shortcutSave")}</small></>}
    </div>
    <div className="inline-actions">
      {dirty && <Button variant="secondary" onClick={model.discardDraft} disabled={busy}>{t("settings.discard")}</Button>}
      <Button onClick={() => void model.requestSave()} disabled={!dirty || busy || !mutable}>{t(busy ? "settings.coreChecking" : "settings.save")}</Button>
    </div>
  </div>;
}
