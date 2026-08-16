import { useEffect, useMemo, useState } from "react";
import { captureCharacterSelection, confirmCharacterSetup, saveOperatorSettings } from "../../api/client";
import {
  getOperatorSettings, previewCharacterSetup, reloadCharacters,
  type CatalogDTO, type CharacterSetupPreviewDTO, type OperatorSettingsDTO, type StatusDTO,
} from "../../api/generated";
import { Button, StateMessage } from "../../app/ui";
import { characterReasonText } from "../../app/characterReasons";
import { BindingEditor, bindingsFromDTO, bindingsToDTO, emptyBindings, type BindingEditorValue } from "./BindingEditor";
import { farmReadyReasonText } from "./characterReasonText";
import { RequiredSkillsList } from "./RequiredSkillsList";

export type CharacterSetupWizardMode = "onboarding" | "dashboard" | "settings";

/** CharacterSetupWizard ist der gemeinsame Setup-/Profilwechsel-Flow ohne Core-Fachlogik in React. */
export function CharacterSetupWizard({
  character, catalog, status, mode = "onboarding", allowDeferBindings = true, showReload = true, onChanged,
}: {
  character: string;
  catalog: CatalogDTO;
  status: StatusDTO;
  mode?: CharacterSetupWizardMode;
  allowDeferBindings?: boolean;
  showReload?: boolean;
  onChanged?: () => Promise<void> | void;
}) {
  const [setupPreview, setSetupPreview] = useState<CharacterSetupPreviewDTO | null>(null);
  const [settings, setSettings] = useState<OperatorSettingsDTO | null>(null);
  const [profileID, setProfileID] = useState("");
  const [captureConfirmed, setCaptureConfirmed] = useState(false);
  const [bindingDraft, setBindingDraft] = useState<BindingEditorValue>(emptyBindings());
  const [bindingsDeferred, setBindingsDeferred] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const selectedProfile = useMemo(
    () => setupPreview?.profiles.find((profile) => profile.id === profileID)
      ?? setupPreview?.profiles.find((profile) => profile.is_selected)
      ?? setupPreview?.profiles[0],
    [setupPreview, profileID],
  );
  const catalogEntry = catalog.characters.find((entry) => entry.name === character);
  const farmReady = catalogEntry?.farm_ready === true;
  const profileSwitchable = !!setupPreview?.supported
    && setupPreview.setup_state === "ready"
    && setupPreview.profiles.length > 0
    && (mode === "settings" || mode === "dashboard");

  async function refresh(options?: { reloadCatalog?: boolean }) {
    if (!character) {
      setSetupPreview(null);
      setSettings(null);
      return;
    }
    if (options?.reloadCatalog) {
      await reloadCharacters();
    }
    const [preview, operator] = await Promise.all([
      previewCharacterSetup({ character }),
      getOperatorSettings(),
    ]);
    setSetupPreview(preview);
    setSettings(operator);
    setProfileID(preview.selected_profile_id || preview.default_profile_id || preview.profiles[0]?.id || "");
    setError("");
  }

  useEffect(() => {
    setCaptureConfirmed(false);
    setBindingsDeferred(false);
    setBindingDraft(emptyBindings());
    let cancelled = false;
    void (async () => {
      try {
        if (!character) return;
        const [preview, operator] = await Promise.all([
          previewCharacterSetup({ character }),
          getOperatorSettings(),
        ]);
        if (cancelled) return;
        setSetupPreview(preview);
        setSettings(operator);
        setProfileID(preview.selected_profile_id || preview.default_profile_id || preview.profiles[0]?.id || "");
        setError("");
      } catch (reason) {
        if (!cancelled) setError(reason instanceof Error ? reason.message : "Der Charakterstatus konnte nicht geladen werden.");
      }
    })();
    return () => { cancelled = true; };
  }, [character, catalog.revision]);

  useEffect(() => {
    if (!settings || !character || !profileID) {
      setBindingDraft(emptyBindings());
      return;
    }
    const key = character.trim().toLowerCase();
    setBindingDraft(bindingsFromDTO(settings.characters[key]?.profile_bindings?.[profileID]));
  }, [settings, character, profileID]);

  async function run(action: () => Promise<void>) {
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await action();
      await onChanged?.();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Der Schritt konnte nicht abgeschlossen werden.");
    } finally {
      setBusy(false);
    }
  }

  async function reloadCharacterData() {
    await run(async () => {
      await refresh({ reloadCatalog: true });
    });
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
      await refresh({ reloadCatalog: true });
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
      await refresh({ reloadCatalog: true });
    });
  }

  async function saveBindings() {
    if (!settings || !profileID) return;
    await run(async () => {
      const key = character.trim().toLowerCase();
      const next = structuredClone(settings);
      const value = next.characters[key];
      if (!value) throw new Error("Der Charakter ist in den Einstellungen noch nicht vorhanden.");
      value.profile_bindings = { ...(value.profile_bindings ?? {}), [profileID]: bindingsToDTO(bindingDraft) };
      next.characters[key] = value;
      const result = await saveOperatorSettings({
        expected_revision: settings.revision,
        expected_generation: status.generation,
        settings: next,
      });
      setSettings(result.settings);
      setBindingsDeferred(false);
      await refresh();
    });
  }

  const title = mode === "settings" ? "Kampfprofil wechseln" : "Charakter einrichten";

  return <div className="character-setup" data-mode={mode}>
    <div className="section-heading">
      <div>
        <p className="eyebrow">{mode === "onboarding" ? "Gefundener Spielstand" : "Charaktersetup"}</p>
        <h3>{setupPreview ? `${setupPreview.character.name} · ${setupPreview.character.class_display_name}` : character || "Kein Charakter gefunden"}</h3>
      </div>
      {showReload && <Button variant="secondary" disabled={busy || !character} onClick={() => void reloadCharacterData()}>Spielstände neu laden</Button>}
    </div>
    {error && <StateMessage kind="error" title="Setup fehlgeschlagen">{error}</StateMessage>}

    {setupPreview?.supported && (setupPreview.setup_state === "needs_setup" || profileSwitchable) && <>
      <h4>{setupPreview.setup_state === "ready" ? title : "Kampfprofil festlegen"}</h4>
      {setupPreview.profiles.length <= 1
        ? <p>Profil: <strong>{setupPreview.profiles[0]?.display_name || "–"}</strong>{setupPreview.profiles[0]?.is_default ? " (Standard)" : ""}</p>
        : <label>Kampfprofil
          <select value={profileID} onChange={(event) => setProfileID(event.target.value)} disabled={busy}>
            {setupPreview.profiles.map((profile) => <option key={profile.id} value={profile.id}>{profile.display_name}{profile.is_default ? " – Standard" : ""}</option>)}
          </select>
        </label>}
      {setupPreview.setup_state === "needs_setup" && <>
        <h4>Lootprofile</h4>
        <ul>{setupPreview.pickit_defaults.map((entry) => <li key={entry.run_id}><strong>{entry.run_display_name}</strong>: {entry.profile_names.join(" → ")}{entry.state === "ready" ? " · bereits eingerichtet" : " · wird ergänzt"}</li>)}</ul>
      </>}
      {(setupPreview.setup_state === "needs_setup" || (profileSwitchable && profileID !== setupPreview.selected_profile_id)) && (
        <Button disabled={busy || !profileID} onClick={() => void confirmSetup()}>
          {setupPreview.setup_state === "ready" ? "Profil wechseln" : "Profil und Lootprofile bestätigen"}
        </Button>
      )}
      {setupPreview.setup_state === "ready" && profileSwitchable && (
        <p className="hint">Queue, Routen, Inventarschutz und Tasten anderer Profile bleiben erhalten. Nicht unterstützte Queue-Einträge werden sichtbar gesperrt, aber nicht gelöscht.</p>
      )}
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

    {setupPreview?.setup_state === "ready" && <>
      {mode !== "settings" && <StateMessage kind="empty" title="Charakter ist eingerichtet">Kampfprofil, Lootprofile und Auswahlbild sind bereit. Wähle oben die gewünschte Schwierigkeit und klicke anschließend auf „Über Core bestätigen“.</StateMessage>}
      {selectedProfile && mode !== "settings" && <>
        <p className="character-profile-summary">Kampfprofil: <strong>{selectedProfile.display_name}</strong></p>
        <h4>Pflichtskills</h4>
        <RequiredSkillsList skills={selectedProfile.required_skills ?? []} standardAttack={selectedProfile.standard_attack} />
        <h4>Tastenbelegung</h4>
        <BindingEditor
          requiredSkills={selectedProfile.required_skills ?? []}
          optionalSkillPairs={selectedProfile.optional_skill_pairs ?? []}
          standardAttack={selectedProfile.standard_attack}
          requiresMercenary={selectedProfile.requires_mercenary}
          bindingsReady={selectedProfile.bindings_ready}
          bindingReasons={selectedProfile.binding_reasons ?? []}
          value={bindingDraft}
          mutable={!busy}
          onChange={setBindingDraft}
        />
        <div className="inline-actions">
          <Button disabled={busy || !profileID} onClick={() => void saveBindings()}>Tasten speichern</Button>
          {allowDeferBindings && <Button variant="secondary" disabled={busy} onClick={() => setBindingsDeferred(true)}>Später</Button>}
        </div>
        {bindingsDeferred && !farmReady && <StateMessage kind="error" title="Queue bleibt gesperrt">Du kannst fortfahren. Farming startet erst, wenn Tasten und Inventarschutz gespeichert sind.</StateMessage>}
        {farmReady && <StateMessage kind="empty" title="Charakter farmbereit">Die Queue darf starten, sobald Route und übrige Voraussetzungen bereit sind.</StateMessage>}
        {!farmReady && (catalogEntry?.farm_ready_reasons?.length ?? 0) > 0 && (
          <StateMessage kind="error" title="Noch nicht farmbereit">
            {(catalogEntry?.farm_ready_reasons ?? []).map((reason) => farmReadyReasonText(reason)).join(" ")}
          </StateMessage>
        )}
      </>}
    </>}

    {setupPreview?.setup_state === "blocked" && <StateMessage kind="error" title="Einrichtung nicht möglich">{setupPreview.reasons.map((reason) => characterReasonText(reason, catalog)).join(" ")}</StateMessage>}
  </div>;
}
