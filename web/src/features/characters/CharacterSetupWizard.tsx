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
import { useTranslation } from "react-i18next";
import { presentApiError, presentClassName, presentProfileName, presentRunName } from "../../i18n/presenters";

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
  const { t } = useTranslation();
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
        if (!cancelled) setError(presentApiError(reason, t, t("characters.wizardLoadFailed")));
      }
    })();
    return () => { cancelled = true; };
  }, [character, catalog.revision, t]);

  useEffect(() => {
    if (!settings || !character || !profileID) {
      setBindingDraft(emptyBindings());
      return;
    }
    const key = character.trim().toLowerCase();
    const selected = setupPreview?.profiles.find((profile) => profile.id === profileID);
    setBindingDraft(bindingsFromDTO(
      settings.characters[key]?.profile_bindings?.[profileID],
      selected?.belt_layout ?? selected?.default_belt_layout,
      selected ? { healing: selected.default_healing_restock, mana: selected.default_mana_restock } : undefined,
    ));
  }, [settings, character, profileID, setupPreview]);

  async function run(action: () => Promise<void>) {
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await action();
      await onChanged?.();
    } catch (reason) {
      setError(presentApiError(reason, t, t("characters.wizardStepFailed")));
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
      if (!value) throw new Error(t("characters.wizardMissingSettings"));
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

  const title = t(mode === "settings" ? "characters.wizardSwitchProfile" : "characters.wizardConfigure");

  return <div className="character-setup" data-mode={mode}>
    <div className="section-heading">
      <div>
        <p className="eyebrow">{t(mode === "onboarding" ? "characters.wizardFoundSave" : "characters.wizardSetup")}</p>
        <h3>{setupPreview ? `${setupPreview.character.name} · ${presentClassName(setupPreview.character.character_class, t)}` : character || t("characters.wizardNoCharacter")}</h3>
      </div>
      {showReload && <Button variant="secondary" disabled={busy || !character} onClick={() => void reloadCharacterData()}>{t("characters.wizardReload")}</Button>}
    </div>
    {error && <StateMessage kind="error" title={t("characters.wizardFailed")}>{error}</StateMessage>}

    {setupPreview?.supported && (setupPreview.setup_state === "needs_setup" || profileSwitchable) && <>
      <h4>{setupPreview.setup_state === "ready" ? title : t("characters.wizardSetProfile")}</h4>
      {setupPreview.profiles.length <= 1
        ? <p>{t("characters.wizardProfile")}<strong>{setupPreview.profiles[0] ? presentProfileName(setupPreview.profiles[0].id, setupPreview.profiles[0].display_name, t) : "–"}</strong>{setupPreview.profiles[0]?.is_default ? ` (${t("characters.wizardDefault")})` : ""}</p>
        : <label>{t("characters.wizardCombatProfile")}
          <select value={profileID} onChange={(event) => setProfileID(event.target.value)} disabled={busy}>
            {setupPreview.profiles.map((profile) => <option key={profile.id} value={profile.id}>{presentProfileName(profile.id, profile.display_name, t)}{profile.is_default ? ` – ${t("characters.wizardDefault")}` : ""}</option>)}
          </select>
        </label>}
      {setupPreview.setup_state === "needs_setup" && <>
        <h4>{t("characters.wizardLootProfiles")}</h4>
        <ul>{setupPreview.pickit_defaults.map((entry) => <li key={entry.run_id}><strong>{presentRunName(entry.run_id, t)}</strong>: {entry.profile_names.join(" → ")} · {t(entry.state === "ready" ? "characters.wizardAlreadySetup" : "characters.wizardWillAdd")}</li>)}</ul>
      </>}
      {(setupPreview.setup_state === "needs_setup" || (profileSwitchable && profileID !== setupPreview.selected_profile_id)) && (
        <Button disabled={busy || !profileID} onClick={() => void confirmSetup()}>
          {t(setupPreview.setup_state === "ready" ? "characters.wizardChangeProfile" : "characters.wizardConfirmProfile")}
        </Button>
      )}
      {setupPreview.setup_state === "ready" && profileSwitchable && (
        <p className="hint">{t("characters.wizardProfileHint")}</p>
      )}
    </>}

    {setupPreview?.supported && setupPreview.setup_state === "needs_anchor" && <>
      <h4>{t("characters.wizardAnchor")}</h4>
      <ol>
        <li>{t("characters.wizardOpenD2R")}</li>
        <li>{t("characters.wizardOpenSelection")}</li>
        <li>{t("characters.wizardSelectCharacter", { character: setupPreview.character.name })}</li>
        <li>{t("characters.wizardConfirmAnchor")}</li>
      </ol>
      <label className="capture-confirmation"><input type="checkbox" checked={captureConfirmed} onChange={(event) => setCaptureConfirmed(event.target.checked)} /> {t("characters.wizardMarked", { character: setupPreview.character.name })}</label>
      <Button disabled={busy || !captureConfirmed} onClick={() => void captureSelectionAnchor()}>{t("characters.wizardSaveAnchor")}</Button>
    </>}

    {setupPreview?.setup_state === "ready" && <>
      {mode !== "settings" && <StateMessage kind="empty" title={t("characters.wizardReadyTitle")}>{t("characters.wizardReadyDetail")}</StateMessage>}
      {selectedProfile && mode !== "settings" && <>
        <p className="character-profile-summary">{t("characters.wizardProfileSummary")}<strong>{presentProfileName(selectedProfile.id, selectedProfile.display_name, t)}</strong></p>
        <h4>{t("characters.requiredSkills")}</h4>
        <RequiredSkillsList skills={selectedProfile.required_skills ?? []} standardAttack={selectedProfile.standard_attack} />
        <h4>{t("characters.bindings")}</h4>
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
          <Button disabled={busy || !profileID} onClick={() => void saveBindings()}>{t("characters.wizardSaveKeys")}</Button>
          {allowDeferBindings && <Button variant="secondary" disabled={busy} onClick={() => setBindingsDeferred(true)}>{t("characters.wizardLater")}</Button>}
        </div>
        {bindingsDeferred && !farmReady && <StateMessage kind="error" title={t("characters.queueLocked")}>{t("characters.wizardDeferred")}</StateMessage>}
        {farmReady && <StateMessage kind="empty" title={t("characters.farmReady")}>{t("characters.farmReadyDetail")}</StateMessage>}
        {!farmReady && (catalogEntry?.farm_ready_reasons?.length ?? 0) > 0 && (
          <StateMessage kind="error" title={t("characters.wizardNotReady")}>
            {(catalogEntry?.farm_ready_reasons ?? []).map((reason) => farmReadyReasonText(reason, t)).join(" ")}
          </StateMessage>
        )}
      </>}
    </>}

    {setupPreview?.setup_state === "blocked" && <StateMessage kind="error" title={t("characters.wizardBlocked")}>{setupPreview.reasons.map((reason) => characterReasonText(reason, catalog, t)).join(" ")}</StateMessage>}
  </div>;
}
