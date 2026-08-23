import { useEffect, useMemo, useState } from "react";
import { Check, ExternalLink, ShieldCheck } from "lucide-react";
import { applySelection, previewSelection, saveOperatorSettings } from "../../api/client";
import {
  getHotkeyHelp, getOperatorSettings, getRecordingOptions, getRouteWorkflow,
  type CatalogDTO, type OperatorSettingsDTO, type RecordingOptionDTO, type SelectionPreviewDTO, type StatusDTO,
} from "../../api/generated";
import { Button, Dialog, StateMessage, StatusBadge } from "../../app/ui";
import { characterAvailabilityText, supportedCharacterClasses } from "../../app/characterReasons";
import { farmReadyReasonText } from "../characters/characterReasonText";
import { CharacterSetupWizard } from "../characters/CharacterSetupWizard";
import { prepareOnboardingResume } from "./onboardingResume";
import { useTranslation } from "react-i18next";
import { presentApiError, presentDifficultyName, presentRecordingInstruction, presentRunName, type AppTranslator } from "../../i18n/presenters";

interface Props {
  status: StatusDTO;
  catalog: CatalogDTO;
  onRefresh(): Promise<void>;
  onClose(): void;
  onOpenRoutes(runID: string): void;
  initialStep?: number;
}

const stepKeys = ["welcome", "system", "d2r", "safety", "input", "character", "readiness", "firstRoute", "finish"] as const;

function prerequisiteLabel(id: string, t: AppTranslator): string {
  return t(id === "waypoint" ? "onboarding.waypoint" : id === "teleport" ? "onboarding.teleport" : id === "town_portal" ? "onboarding.townPortal" : id === "pickit" ? "onboarding.pickit" : "onboarding.otherPrerequisite");
}

function prerequisiteStatus(entry: { ready: boolean; reason?: string }, t: AppTranslator): string {
  if (entry.ready) return t("onboarding.ready");
  const reason = entry.reason ?? "";
  if (reason === "onboarding_waypoint_required" || reason === "onboarding_waypoint_missing") return t("onboarding.waypointMissing");
  if (reason === "onboarding_teleport_binding_missing" || reason === "onboarding_teleport_missing") return t("onboarding.teleportMissing");
  if (reason === "onboarding_town_portal_binding_missing" || reason === "onboarding_town_portal_missing") return t("onboarding.townPortalMissing");
  if (reason === "pickit_assignment_missing") return t("onboarding.pickitMissing");
  return t("onboarding.prerequisiteMissing");
}

export function OnboardingFeature({ status, catalog, onRefresh, onClose, onOpenRoutes, initialStep = 0 }: Props) {
  const { t } = useTranslation();
  const steps = stepKeys.map((key) => t(`onboarding.steps.${key}`));
  const [step, setStep] = useState(() => Math.min(Math.max(initialStep, 0), stepKeys.length - 1));
  const [settings, setSettings] = useState<OperatorSettingsDTO | null>(null);
  const [options, setOptions] = useState<RecordingOptionDTO[]>([]);
  const [hotkeys, setHotkeys] = useState<{ recording_finish: string; stop_after_run: string; emergency_stop: string; pause: string } | null>(null);
  const [workflowState, setWorkflowState] = useState("");
  const [routeID, setRouteID] = useState("countess");
  const [character, setCharacter] = useState(status.selection.character || catalog.characters.find((entry) => entry.selectable)?.name || catalog.characters[0]?.name || "");
  const [difficulty, setDifficulty] = useState(status.selection.difficulty || catalog.default_difficulty);
  const [selectionPreview, setSelectionPreview] = useState<SelectionPreviewDTO | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const selectedOption = useMemo(() => options.find((option) => option.run_id === routeID), [options, routeID]);
  const onboardingOptions = useMemo(() => options.filter((option, index) => options.findIndex((candidate) => candidate.run_id === option.run_id) === index), [options]);
  const allPrerequisitesReady = !!selectedOption && (selectedOption.prerequisites ?? []).every((entry) => entry.ready);

  async function load() {
    try {
      const [operator, recording, keys, workflow] = await Promise.all([getOperatorSettings(), getRecordingOptions(), getHotkeyHelp(), getRouteWorkflow()]);
      setSettings(operator);
      setOptions(recording);
      setHotkeys(keys);
      setWorkflowState(workflow.state);
      setError("");
    } catch (reason) {
      setError(message(reason, t, t("onboarding.loadFailed")));
    }
  }
  useEffect(() => { void load(); }, [catalog.revision, t]);
  useEffect(() => {
    if (catalog.characters.some((entry) => entry.name === character)) return;
    setCharacter(status.selection.character || catalog.characters.find((entry) => entry.selectable)?.name || catalog.characters[0]?.name || "");
  }, [catalog.revision, character, status.selection.character]);

  async function run(action: () => Promise<void>) {
    if (busy) return;
    setBusy(true);
    setError("");
    try { await action(); } catch (reason) { setError(message(reason, t, t("onboarding.stepFailed"))); } finally { setBusy(false); }
  }

  async function submitSelection() {
    await run(async () => {
      const preview = await previewSelection(character, difficulty, catalog.revision);
      if (preview.requires_confirmation) {
        setSelectionPreview(preview);
        return;
      }
      await applySelection(preview.character, preview.new_difficulty, catalog.revision, status.generation, preview.confirmation_token);
      await onRefresh();
      await load();
    });
  }

  async function confirmSelection() {
    if (!selectionPreview) return;
    await run(async () => {
      await applySelection(selectionPreview.character, selectionPreview.new_difficulty, catalog.revision, status.generation, selectionPreview.confirmation_token);
      setSelectionPreview(null);
      await onRefresh();
      await load();
    });
  }

  async function persistInput(enabled: boolean): Promise<boolean> {
    if (!settings || settings.input.enabled === enabled) return false;
    const replacement = structuredClone(settings);
    replacement.input.enabled = enabled;
    const result = await saveOperatorSettings({ expected_revision: settings.revision, expected_generation: status.generation, settings: replacement });
    setSettings(result.settings);
    return result.restart_required;
  }

  async function setInput(enabled: boolean) {
    await run(async () => {
      const restart = await persistInput(enabled);
      if (restart && window.d2rDesktop) {
        prepareOnboardingResume(step);
        await window.d2rDesktop.restartCore();
      }
      else await onRefresh();
    });
  }

  async function finish(skipped: boolean) {
    await run(async () => {
      const restart = skipped ? await persistInput(false) : false;
      const desktop = await window.d2rDesktop?.getDesktopSettings();
      if (desktop && window.d2rDesktop) {
        await window.d2rDesktop.updateDesktopSettings({ autostart: desktop.autostart, onboarding_completed: true });
      }
      if (restart && window.d2rDesktop) await window.d2rDesktop.restartCore();
      onClose();
    });
  }

  const compatibility = status.compatibility.state;
  const effectiveInputReady = settings?.input.enabled === true && status.input.enabled && !status.input.paused && !status.input.stopped;
  const selectedCatalogEntry = catalog.characters.find((entry) => entry.name === character);
  const selectedCharacterReady = selectedCatalogEntry?.selectable === true;
  const farmReady = selectedCatalogEntry?.farm_ready === true;
  const canAdvance = (step !== 2 || compatibility === "compatible")
    && (step !== 4 || effectiveInputReady)
    && (step !== 5 || !!status.selection.character);
  return <section className="onboarding" aria-labelledby="onboarding-title">
    <div className="section-heading">
      <div><p className="eyebrow">{t("onboarding.stepKicker", { current: step + 1, total: steps.length })}</p><h1 id="onboarding-title">{steps[step]}</h1></div>
      <StatusBadge tone={step === steps.length - 1 ? "success" : "warning"}>{step + 1}/{steps.length}</StatusBadge>
    </div>
    <ol className="onboarding-progress" aria-label={t("onboarding.progressAria")}>
      {steps.map((label, index) => <li key={label} aria-current={index === step ? "step" : undefined} className={index < step ? "complete" : ""}><span>{index < step ? <Check aria-hidden="true" size={15} /> : index + 1}</span>{label}</li>)}
    </ol>
    {error && <StateMessage kind="error" title={t("onboarding.stepIncomplete")}>{error}</StateMessage>}

    {step === 0 && <div className="onboarding-panel"><ShieldCheck aria-hidden="true" size={32} /><h2>{t("onboarding.welcomeTitle")}</h2><p>{t("onboarding.welcomeDetail")}</p></div>}
    {step === 1 && <div className="onboarding-panel"><h2>{t("onboarding.installTitle")}</h2><ul><li>{t("onboarding.installWindows")}</li><li>{t("onboarding.installRoot")}</li><li>{t("onboarding.installProvisioned")}</li><li>{t("onboarding.installAdmin")}</li></ul></div>}
    {step === 2 && <div className="onboarding-panel"><h2>{t("onboarding.d2rTitle")}</h2><p>{t("onboarding.state")}<strong>{compatibilityStateText(compatibility, t)}</strong></p><p>{t("onboarding.versions", { expected: status.compatibility.expected_version || "–", actual: status.compatibility.actual_version || "–" })}</p>{compatibility !== "compatible" && <StateMessage kind="error" title={t("onboarding.d2rNotConfirmed")}>{compatibilityDetailText(compatibility, t)}</StateMessage>}{status.compatibility.privilege_mismatch && <Button variant="secondary" onClick={() => void window.d2rDesktop?.restartAsAdministrator()}>{t("onboarding.restartAdmin")}</Button>}</div>}
    {step === 3 && <div className="onboarding-panel"><h2>{t("onboarding.safetyTitle")}</h2><ul><li>{t("onboarding.windowMode")}</li><li>{t("onboarding.pauseStop", { pause: hotkeys?.pause ?? "Pause", stop: hotkeys?.stop_after_run ?? "F10" })}</li><li>{t("onboarding.emergency", { key: hotkeys?.emergency_stop ?? "F11" })}</li><li>{t("onboarding.recordingFinish", { key: hotkeys?.recording_finish ?? "F9" })}</li></ul></div>}
    {step === 4 && <div className="onboarding-panel"><h2>{t("onboarding.inputTitle")}</h2><p>{t("onboarding.inputDetail", { saved: t(settings?.input.enabled ? "onboarding.active" : "onboarding.disabled"), effective: t(status.input.stopped ? "onboarding.stopped" : status.input.paused ? "onboarding.paused" : status.input.enabled ? "onboarding.enabled" : "onboarding.disabled") })}</p>{settings?.input.enabled && !effectiveInputReady && <StateMessage kind="error" title={t("onboarding.inputPending")}>{t("onboarding.inputPendingDetail")}</StateMessage>}<div className="inline-actions"><Button disabled={busy || settings?.input.enabled} onClick={() => void setInput(true)}>{t("onboarding.enableInput")}</Button><Button variant="secondary" disabled={busy || !settings?.input.enabled} onClick={() => void setInput(false)}>{t("onboarding.leaveInputOff")}</Button></div></div>}
    {step === 5 && <div className="onboarding-panel">
      <h2>{t("onboarding.characterTitle")}</h2>
      <p>{t("onboarding.characterDetail", { classes: supportedCharacterClasses(catalog, t) })}</p>
      <div className="selection-grid">
        <label>{t("onboarding.character")}
          <select value={character} onChange={(event) => setCharacter(event.target.value)}>
            {catalog.characters.map((entry) => <option key={entry.slug} value={entry.name}>{entry.name}{entry.selectable ? "" : ` – ${t("onboarding.setupRequired")}`}</option>)}
          </select>
        </label>
        <label>{t("onboarding.difficulty")}
          <select value={difficulty} onChange={(event) => setDifficulty(event.target.value)}>
            {catalog.difficulties.map((entry) => <option key={entry.id} value={entry.id}>{presentDifficultyName(entry.id, t)}</option>)}
          </select>
        </label>
        <Button disabled={busy || !character || !effectiveInputReady || !selectedCharacterReady} onClick={() => void submitSelection()}>{t("onboarding.confirmCore")}</Button>
      </div>
      <CharacterSetupWizard
        character={character}
        catalog={catalog}
        status={status}
        mode="onboarding"
        onChanged={async () => { await onRefresh(); await load(); }}
      />
      <p>{t("onboarding.activeSelection", { selection: status.selection.character ? `${status.selection.character} / ${presentDifficultyName(status.selection.difficulty ?? "", t)}` : t("onboarding.notConfirmed") })}</p>
      {catalog.characters.some((entry) => !entry.selectable) && <>
        <h3>{t("onboarding.whyUnavailable")}</h3>
        <ul className="character-availability">{catalog.characters.filter((entry) => !entry.selectable).map((entry) => <li key={entry.slug}><strong>{entry.name}</strong><span>{characterAvailabilityText(entry, catalog, t)}</span></li>)}</ul>
      </>}
    </div>}
    {step === 6 && <div className="onboarding-panel"><h2>{t("onboarding.readinessTitle")}</h2><p>{t("onboarding.readinessDetail")}</p><ul className="readiness-list"><li><span>{t("onboarding.versionGate")}</span><strong>{t(compatibility === "compatible" ? "onboarding.ready" : "onboarding.missing")}</strong></li><li><span>{t("onboarding.characterDifficulty")}</span><strong>{t(status.selection.character ? "onboarding.ready" : "onboarding.missing")}</strong></li><li><span>{t("onboarding.input")}</span><strong>{t(effectiveInputReady ? "onboarding.ready" : "onboarding.inputRejected")}</strong></li><li><span>{t("onboarding.bindings")}</span><strong>{t(farmReady ? "onboarding.ready" : "onboarding.missing")}</strong></li>{(selectedOption?.prerequisites ?? []).map((entry) => <li key={entry.id}><span>{prerequisiteLabel(entry.id, t)}</span><strong>{prerequisiteStatus(entry, t)}</strong></li>)}</ul>{!farmReady && <StateMessage kind="error" title={t("onboarding.notFarmReady")}>{(selectedCatalogEntry?.farm_ready_reasons ?? ["profile_bindings_incomplete"]).map((reason) => farmReadyReasonText(reason, t)).join(" ")}</StateMessage>}<Button variant="secondary" onClick={() => void load()}>{t("onboarding.reloadReadiness")}</Button></div>}
    {step === 7 && <div className="onboarding-panel"><h2>{t("onboarding.firstRouteTitle")}</h2><p><strong>{t("onboarding.howToStart")}</strong> {t("onboarding.firstRouteDetail", { key: hotkeys?.recording_finish ?? "F9" })}</p><div className="route-choice">{(onboardingOptions.length > 0 ? onboardingOptions : catalog.runs.map((run) => ({ run_id: run.run_id, instruction_code: "", prerequisites: [] as RecordingOptionDTO["prerequisites"] }))).map((option, index) => (
      <label key={option.run_id}><input type="radio" name="first-route" checked={routeID === option.run_id} onChange={() => setRouteID(option.run_id)} /> {presentRunName(option.run_id, t)}{index === 0 ? <> <strong>{t("onboarding.recommended")}</strong></> : null}</label>
    ))}</div>{selectedOption && <><p>{presentRecordingInstruction(selectedOption.instruction_code, hotkeys?.recording_finish ?? "F9", t)}</p><ul>{(selectedOption.prerequisites ?? []).map((entry) => <li key={entry.id}>{prerequisiteLabel(entry.id, t)}: {prerequisiteStatus(entry, t)}</li>)}</ul></>} {["failed_safe", "emergency_cancelled"].includes(workflowState) && <StateMessage kind="error" title={t("onboarding.discardedTitle")}>{t("onboarding.discardedDetail")}</StateMessage>}<Button disabled={!allPrerequisitesReady || !settings?.input.enabled || compatibility !== "compatible"} onClick={() => onOpenRoutes(routeID)}>{t("onboarding.openRoutes")} <ExternalLink aria-hidden="true" size={17} /></Button></div>}
    {step === 8 && <div className="onboarding-panel"><h2>{t("onboarding.finishTitle")}</h2><p>{t("onboarding.finishDetail")}</p>{!farmReady && <StateMessage kind="error" title={t("onboarding.setupMissing")}>{t("onboarding.setupMissingDetail")}</StateMessage>}<div className="inline-actions"><Button onClick={() => void finish(false)}>{t("onboarding.finishAssistant")}</Button><Button variant="secondary" onClick={() => onOpenRoutes(routeID)}>{t("onboarding.recordNow")}</Button>{!farmReady && <Button variant="secondary" onClick={() => setStep(5)}>{t("onboarding.configureNow")}</Button>}</div></div>}

    <footer className="onboarding-actions"><Button variant="secondary" disabled={step === 0 || busy} onClick={() => setStep((value) => value - 1)}>{t("onboarding.back")}</Button><Button variant="secondary" disabled={busy} onClick={() => void finish(true)}>{t("onboarding.skip")}</Button>{step < steps.length - 1 && <Button disabled={!canAdvance || busy} onClick={() => setStep((value) => value + 1)}>{t("onboarding.next")}</Button>}</footer>
    {selectionPreview && <Dialog title={t("onboarding.routeImpactTitle")} onClose={() => setSelectionPreview(null)}><p>{t("onboarding.routeImpactDetail", { count: selectionPreview.affected_routes.length })}</p><div className="modal-actions"><Button variant="secondary" onClick={() => setSelectionPreview(null)}>{t("common.cancel")}</Button><Button onClick={() => void confirmSelection()}>{t("onboarding.confirmSelection")}</Button></div></Dialog>}
  </section>;
}

function compatibilityStateText(state: string, t: AppTranslator): string {
  const keys = { compatible: "onboarding.compatibilityState.compatible", not_detected: "onboarding.compatibilityState.notDetected", incompatible: "onboarding.compatibilityState.incompatible", unreadable: "onboarding.compatibilityState.unreadable" } as const;
  const key = keys[state as keyof typeof keys] ?? "onboarding.compatibilityState.unknown";
  return t(key);
}

function compatibilityDetailText(state: string, t: AppTranslator): string {
  const keys = { not_detected: "onboarding.compatibilityDetail.notDetected", incompatible: "onboarding.compatibilityDetail.incompatible", unreadable: "onboarding.compatibilityDetail.unreadable" } as const;
  const key = keys[state as keyof typeof keys];
  return key ? t(key) : t("onboarding.d2rFallback");
}

function message(reason: unknown, t: AppTranslator, fallback: string): string {
  return presentApiError(reason, t, fallback);
}
