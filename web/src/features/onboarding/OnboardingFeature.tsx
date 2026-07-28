import { useEffect, useMemo, useState } from "react";
import { Check, CircleAlert, ExternalLink, ShieldCheck } from "lucide-react";
import { applySelection, captureCharacterSelection, confirmCharacterSetup, previewSelection, saveOperatorSettings } from "../../api/client";
import {
  getHotkeyHelp, getOperatorSettings, getRecordingOptions, getRouteWorkflow, previewCharacterSetup, reloadCharacters,
  type CatalogDTO, type CharacterSetupPreviewDTO, type OperatorSettingsDTO, type RecordingOptionDTO, type SelectionPreviewDTO, type StatusDTO,
} from "../../api/generated";
import { Button, Dialog, StateMessage, StatusBadge } from "../../app/ui";
import { characterAvailabilityText, characterReasonText, supportedCharacterClasses } from "../../app/characterReasons";
import { prepareOnboardingResume } from "./onboardingResume";

interface Props {
  status: StatusDTO;
  catalog: CatalogDTO;
  onRefresh(): Promise<void>;
  onClose(): void;
  onOpenRoutes(runID: string): void;
  initialStep?: number;
}

const steps = ["Willkommen", "System", "D2R", "Safety", "Input", "Charakter", "Readiness", "Erste Route", "Abschluss"];
const prerequisiteLabels: Record<string, string> = { waypoint: "Startwegpunkt", teleport: "Teleport-Binding", town_portal: "Town-Portal-Binding", pickit: "Pickit-Zuordnung" };
const prerequisiteReasons: Record<string, string> = {
  onboarding_waypoint_required: "Der erforderliche Startwegpunkt ist noch nicht verfügbar.",
  onboarding_waypoint_missing: "Der erforderliche Startwegpunkt ist noch nicht verfügbar.",
  onboarding_teleport_binding_missing: "Die Teleport-Tastenbelegung fehlt.",
  onboarding_teleport_missing: "Die Teleport-Tastenbelegung fehlt.",
  onboarding_town_portal_binding_missing: "Die Town-Portal-Tastenbelegung fehlt.",
  onboarding_town_portal_missing: "Die Town-Portal-Tastenbelegung fehlt.",
  pickit_assignment_missing: "Für diesen Charakter und Run ist noch kein Lootprofil zugeordnet.",
};

function prerequisiteStatus(entry: { ready: boolean; reason?: string }): string {
  return entry.ready ? "bereit" : prerequisiteReasons[entry.reason ?? ""] ?? "Diese Voraussetzung fehlt noch.";
}

export function OnboardingFeature({ status, catalog, onRefresh, onClose, onOpenRoutes, initialStep = 0 }: Props) {
  const [step, setStep] = useState(() => Math.min(Math.max(initialStep, 0), steps.length - 1));
  const [settings, setSettings] = useState<OperatorSettingsDTO | null>(null);
  const [options, setOptions] = useState<RecordingOptionDTO[]>([]);
  const [hotkeys, setHotkeys] = useState<{ recording_finish: string; stop_after_run: string; emergency_stop: string; pause: string } | null>(null);
  const [workflowState, setWorkflowState] = useState("");
  const [routeID, setRouteID] = useState("countess");
  const [character, setCharacter] = useState(status.selection.character || catalog.characters.find((entry) => entry.selectable)?.name || catalog.characters[0]?.name || "");
  const [difficulty, setDifficulty] = useState(status.selection.difficulty || catalog.default_difficulty);
  const [selectionPreview, setSelectionPreview] = useState<SelectionPreviewDTO | null>(null);
  const [setupPreview, setSetupPreview] = useState<CharacterSetupPreviewDTO | null>(null);
  const [profileID, setProfileID] = useState("");
  const [captureConfirmed, setCaptureConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const selectedOption = useMemo(() => options.find((option) => option.run_id === routeID), [options, routeID]);
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
      setError(message(reason, "Onboarding-Daten konnten nicht geladen werden."));
    }
  }
  useEffect(() => { void load(); }, [catalog.revision]);
  useEffect(() => {
    if (catalog.characters.some((entry) => entry.name === character)) return;
    setCharacter(status.selection.character || catalog.characters.find((entry) => entry.selectable)?.name || catalog.characters[0]?.name || "");
  }, [catalog.revision, character, status.selection.character]);
  useEffect(() => {
    setCaptureConfirmed(false);
    if (!character) {
      setSetupPreview(null);
      return;
    }
    void previewCharacterSetup({ character })
      .then((preview) => {
        setSetupPreview(preview);
        setProfileID(preview.selected_profile_id || preview.default_profile_id || preview.profiles[0]?.id || "");
      })
      .catch((reason) => setError(message(reason, "Der Charakterstatus konnte nicht geladen werden.")));
  }, [character, catalog.revision]);

  async function run(action: () => Promise<void>) {
    if (busy) return;
    setBusy(true);
    setError("");
    try { await action(); } catch (reason) { setError(message(reason, "Der Schritt konnte nicht abgeschlossen werden.")); } finally { setBusy(false); }
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

  async function refreshCharacterData() {
    await reloadCharacters();
    await onRefresh();
    const [preview, operator, recording] = await Promise.all([
      previewCharacterSetup({ character }),
      getOperatorSettings(),
      getRecordingOptions(),
    ]);
    setSetupPreview(preview);
    setProfileID(preview.selected_profile_id || preview.default_profile_id || preview.profiles[0]?.id || "");
    setSettings(operator);
    setOptions(recording);
  }

  async function reloadCharacterData() {
    await run(refreshCharacterData);
  }

  async function confirmSetup() {
    if (!setupPreview) return;
    await run(async () => {
      await confirmCharacterSetup({
        command_id: crypto.randomUUID(),
        character,
        profile_id: profileID || undefined,
        expected_catalog_revision: setupPreview.catalog_revision,
        expected_operator_settings_revision: setupPreview.operator_settings_revision,
        expected_pickit_assignment_revision: setupPreview.pickit_assignment_revision,
        expected_generation: status.generation,
      });
      await refreshCharacterData();
    });
  }

  async function captureSelectionAnchor() {
    if (!setupPreview || !captureConfirmed) return;
    await run(async () => {
      await captureCharacterSelection({
        command_id: crypto.randomUUID(),
        character,
        expected_catalog_revision: setupPreview.catalog_revision,
        expected_generation: status.generation,
      });
      setCaptureConfirmed(false);
      await refreshCharacterData();
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
  const canAdvance = (step !== 2 || compatibility === "compatible")
    && (step !== 4 || effectiveInputReady)
    && (step !== 5 || !!status.selection.character);
  return <section className="onboarding" aria-labelledby="onboarding-title">
    <div className="section-heading">
      <div><p className="eyebrow">First Run · Schritt {step + 1} von {steps.length}</p><h1 id="onboarding-title">{steps[step]}</h1></div>
      <StatusBadge tone={step === steps.length - 1 ? "success" : "warning"}>{step + 1}/{steps.length}</StatusBadge>
    </div>
    <ol className="onboarding-progress" aria-label="Fortschritt">
      {steps.map((label, index) => <li key={label} aria-current={index === step ? "step" : undefined} className={index < step ? "complete" : ""}><span>{index < step ? <Check aria-hidden="true" size={15} /> : index + 1}</span>{label}</li>)}
    </ol>
    {error && <StateMessage kind="error" title="Schritt nicht abgeschlossen">{error}</StateMessage>}

    {step === 0 && <div className="onboarding-panel"><ShieldCheck aria-hidden="true" size={32} /><h2>Sicher zum ersten echten Run</h2><p>Der Assistent erklärt Einrichtung und Safety. Er startet weder D2R noch Farming automatisch und verwendet für jede Fachentscheidung den bestehenden Core.</p></div>}
    {step === 1 && <div className="onboarding-panel"><h2>Lokale Installation</h2><ul><li>Windows 10/11 x64</li><li>App und Core verwenden denselben expliziten LocalAppData-Datenroot.</li><li>Die Datenbasis wurde vor dem produktiven Core atomar provisioniert.</li><li>Administratorrechte sind nur bei einem nachgewiesenen Privilegienkonflikt nötig.</li></ul></div>}
    {step === 2 && <div className="onboarding-panel"><h2>D2R manuell starten</h2><p>Zustand: <strong>{compatibility}</strong></p><p>Erwartet {status.compatibility.expected_version || "–"} · Erkannt {status.compatibility.actual_version || "–"}</p>{compatibility !== "compatible" && <StateMessage kind="error" title="D2R ist noch nicht kompatibel bestätigt">{status.compatibility.reason || "Starte D2R manuell und warte auf die read-only Versionsprüfung."}</StateMessage>}{status.compatibility.privilege_mismatch && <Button variant="secondary" onClick={() => void window.d2rDesktop?.restartAsAdministrator()}>App als Administrator neu starten</Button>}</div>}
    {step === 3 && <div className="onboarding-panel"><h2>Safety vor Komfort</h2><ul><li>D2R im Fenstermodus mit 1280×720 verwenden.</li><li><strong>{hotkeys?.pause ?? "Pause"}</strong>: Pause nach Run; <strong>{hotkeys?.stop_after_run ?? "F10"}</strong>: Stop nach Run.</li><li><strong>{hotkeys?.emergency_stop ?? "F11"}</strong>: sofortiger Emergency Stop ohne Save-&amp;-Exit-Garantie.</li><li><strong>{hotkeys?.recording_finish ?? "F9"}</strong>: laufende Routenaufnahme beenden.</li></ul></div>}
    {step === 4 && <div className="onboarding-panel"><h2>Gameplay-Input bewusst freigeben</h2><p>Gespeicherte Freigabe: <strong>{settings?.input.enabled ? "aktiv" : "deaktiviert"}</strong>. Effektiver Core-Zustand: <strong>{status.input.stopped ? "gestoppt" : status.input.paused ? "pausiert" : status.input.enabled ? "freigegeben" : "deaktiviert"}</strong>. Eine Änderung wird revisionsgebunden gespeichert und löst bei Bedarf einen kontrollierten Core-Neustart aus. Erst wenn der laufende Core die Freigabe bestätigt, wird die Charakterauswahl aktiv.</p>{settings?.input.enabled && !effectiveInputReady && <StateMessage kind="error" title="Core-Freigabe noch nicht aktiv">Warte den kontrollierten Core-Neustart ab oder lade diesen Schritt neu. Die Auswahl bleibt bis zur effektiven Bestätigung gesperrt.</StateMessage>}<div className="inline-actions"><Button disabled={busy || settings?.input.enabled} onClick={() => void setInput(true)}>Input ausdrücklich freigeben</Button><Button variant="secondary" disabled={busy || !settings?.input.enabled} onClick={() => void setInput(false)}>Input deaktiviert lassen</Button></div></div>}
    {step === 5 && <div className="onboarding-panel">
      <h2>Charakter und Schwierigkeit bestätigen</h2>
      <p>Alle gefundenen lokalen Charaktere werden angezeigt. Verfügbar sind nur Charaktere mit freigegebenem Kampfprofil und sicherer D2R-Auswahl. Derzeit unterstützt: <strong>{supportedCharacterClasses(catalog)}</strong>.</p>
      <div className="selection-grid">
        <label>Charakter
          <select value={character} onChange={(event) => setCharacter(event.target.value)}>
            {catalog.characters.map((entry) => <option key={entry.slug} value={entry.name}>{entry.name}{entry.selectable ? "" : " – Einrichtung nötig"}</option>)}
          </select>
        </label>
        <label>Schwierigkeit
          <select value={difficulty} onChange={(event) => setDifficulty(event.target.value)}>
            {catalog.difficulties.map((entry) => <option key={entry.id} value={entry.id}>{entry.display_name}</option>)}
          </select>
        </label>
        <Button disabled={busy || !character || !effectiveInputReady || !selectedCharacterReady} onClick={() => void submitSelection()}>Über Core bestätigen</Button>
      </div>
      <div className="character-setup">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Gefundener Spielstand</p>
            <h3>{setupPreview ? `${setupPreview.character.name} · ${setupPreview.character.class_display_name}` : character || "Kein Charakter gefunden"}</h3>
          </div>
          <Button variant="secondary" disabled={busy || !character} onClick={() => void reloadCharacterData()}>Spielstände neu laden</Button>
        </div>
        {setupPreview?.supported && setupPreview.setup_state === "needs_setup" && <>
          <h4>Kampfprofil festlegen</h4>
          {setupPreview.profiles.length === 1
            ? <p>Profil: <strong>{setupPreview.profiles[0].display_name}</strong></p>
            : <label>Kampfprofil
              <select value={profileID} onChange={(event) => setProfileID(event.target.value)}>
                {setupPreview.profiles.map((profile) => <option key={profile.id} value={profile.id}>{profile.display_name}{profile.is_default ? " – Standard" : ""}</option>)}
              </select>
            </label>}
          <h4>Lootprofile</h4>
          <ul>{setupPreview.pickit_defaults.map((entry) => <li key={entry.run_id}><strong>{entry.run_display_name}</strong>: {entry.profile_names.join(" → ")}{entry.state === "ready" ? " · bereits eingerichtet" : " · wird ergänzt"}</li>)}</ul>
          <Button disabled={busy || !profileID} onClick={() => void confirmSetup()}>Profil und Lootprofile bestätigen</Button>
        </>}
        {setupPreview?.supported && setupPreview.setup_state === "needs_anchor" && <>
          <h4>Auswahlbild speichern</h4>
          <ol>
            <li>D2R öffnen.</li>
            <li>Die Charakterauswahl öffnen.</li>
            <li><strong>{setupPreview.character.name}</strong> einmal anklicken, aber das Spiel nicht starten.</li>
            <li>Hier bestätigen und das Auswahlbild speichern.</li>
          </ol>
          <label className="capture-confirmation"><input type="checkbox" checked={captureConfirmed} onChange={(event) => setCaptureConfirmed(event.target.checked)} /> {setupPreview.character.name} ist in der Charakterauswahl markiert.</label>
          <Button disabled={busy || !captureConfirmed} onClick={() => void captureSelectionAnchor()}>Auswahlbild jetzt speichern</Button>
        </>}
        {setupPreview?.setup_state === "ready" && <StateMessage kind="empty" title="Charakter ist eingerichtet">Kampfprofil, Lootprofile und Auswahlbild sind bereit. Wähle oben die gewünschte Schwierigkeit und klicke anschließend auf „Über Core bestätigen“.</StateMessage>}
        {setupPreview?.setup_state === "blocked" && <StateMessage kind="error" title="Einrichtung nicht möglich">{setupPreview.reasons.map((reason) => characterReasonText(reason, catalog)).join(" ")}</StateMessage>}
      </div>
      <p>Aktiv: {status.selection.character ? `${status.selection.character} / ${status.selection.difficulty}` : "noch nicht bestätigt"}</p>
      {catalog.characters.some((entry) => !entry.selectable) && <>
        <h3>Warum sind Charaktere nicht verfügbar?</h3>
        <ul className="character-availability">{catalog.characters.filter((entry) => !entry.selectable).map((entry) => <li key={entry.slug}><strong>{entry.name}</strong><span>{characterAvailabilityText(entry, catalog)}</span></li>)}</ul>
      </>}
    </div>}
    {step === 6 && <div className="onboarding-panel"><h2>Core-Readiness</h2><p>Die Liste stammt aus Status-, Katalog- und Recording-API.</p><ul className="readiness-list"><li><span>Versionsgate</span><strong>{compatibility === "compatible" ? "bereit" : "fehlt"}</strong></li><li><span>Charakter/Difficulty</span><strong>{status.selection.character ? "bereit" : "fehlt"}</strong></li><li><span>Input</span><strong>{effectiveInputReady ? "bereit" : "abgelehnt/deaktiviert"}</strong></li>{(selectedOption?.prerequisites ?? []).map((entry) => <li key={entry.id}><span>{prerequisiteLabels[entry.id] ?? "Weitere Voraussetzung"}</span><strong>{prerequisiteStatus(entry)}</strong></li>)}</ul><Button variant="secondary" onClick={() => void load()}>Readiness neu laden</Button></div>}
    {step === 7 && <div className="onboarding-panel"><h2>Erste echte Route</h2><p><strong>So startest du:</strong> Öffne unten den Routenbereich und klicke dort beim gewünschten Run auf „Aufnahme starten“. Bleibe am Startwegpunkt stehen, bis der Core „Aufnahme läuft“ meldet. <strong>{hotkeys?.recording_finish ?? "F9"} startet keine Aufnahme</strong>, sondern beendet ausschließlich eine bereits laufende Aufnahme an der gewünschten Kampfposition.</p><div className="route-choice"><label><input type="radio" name="first-route" checked={routeID === "countess"} onChange={() => setRouteID("countess")} /> Countess <strong>empfohlen</strong></label><label><input type="radio" name="first-route" checked={routeID === "mephisto"} onChange={() => setRouteID("mephisto")} /> Mephisto</label></div>{selectedOption && <><p>{selectedOption.instructions_de}</p><ul>{(selectedOption.prerequisites ?? []).map((entry) => <li key={entry.id}>{prerequisiteLabels[entry.id] ?? "Weitere Voraussetzung"}: {prerequisiteStatus(entry)}</li>)}</ul></>} {["failed_safe", "emergency_cancelled"].includes(workflowState) && <StateMessage kind="error" title="Vorherige Aufnahme wurde verworfen">Es gibt kein Resume. Starte die Aufnahme sauber neu.</StateMessage>}<Button disabled={!allPrerequisitesReady || !settings?.input.enabled || compatibility !== "compatible"} onClick={() => onOpenRoutes(routeID)}>Routenbereich öffnen und Aufnahme starten <ExternalLink aria-hidden="true" size={17} /></Button></div>}
    {step === 8 && <div className="onboarding-panel"><h2>Bereit für das Dashboard</h2><p>Du kannst den Assistenten ohne fertige Route abschließen. Das Dashboard zeigt dann weiterhin den konkreten Einstieg „Erste Route aufnehmen“. Es startet kein Testlauf und keine Farming-Session.</p><div className="inline-actions"><Button onClick={() => void finish(false)}>Assistent abschließen</Button><Button variant="secondary" onClick={() => onOpenRoutes(routeID)}>Jetzt zur Routenaufnahme</Button></div></div>}

    <footer className="onboarding-actions"><Button variant="secondary" disabled={step === 0 || busy} onClick={() => setStep((value) => value - 1)}>Zurück</Button><Button variant="secondary" disabled={busy} onClick={() => void finish(true)}>Überspringen – Input bleibt aus</Button>{step < steps.length - 1 && <Button disabled={!canAdvance || busy} onClick={() => setStep((value) => value + 1)}>Weiter</Button>}</footer>
    {selectionPreview && <Dialog title="Routenwirkung bestätigen" onClose={() => setSelectionPreview(null)}><p>Der Difficulty-Wechsel betrifft {selectionPreview.affected_routes.length} Route(n). Die vorhandene Core-Vorschau bleibt autoritativ.</p><div className="modal-actions"><Button variant="secondary" onClick={() => setSelectionPreview(null)}>Abbrechen</Button><Button onClick={() => void confirmSelection()}>Auswahl bestätigen</Button></div></Dialog>}
  </section>;
}

function message(reason: unknown, fallback: string): string {
  return reason instanceof Error ? reason.message : fallback;
}
