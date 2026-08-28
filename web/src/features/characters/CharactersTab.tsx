import { useEffect, useMemo, useState } from "react";
import { previewCharacterSetup, type CatalogDTO, type CharacterSetupPreviewDTO, type OperatorSettingsDTO, type StatusDTO } from "../../api/generated";
import { StateMessage, StatusBadge } from "../../app/ui";
import { BindingEditor, bindingsFromDTO, bindingsToDTO, emptyBindings, type BindingEditorValue } from "./BindingEditor";
import { CharacterSetupWizard } from "./CharacterSetupWizard";
import { characterStatusLabel, farmReadyReasonText } from "./characterReasonText";
import { InventoryLockEditor, inventoryConfigured, inventoryCowSuitable, type InventoryGrid } from "./InventoryLockEditor";
import { RequiredSkillsList } from "./RequiredSkillsList";
import { pathChanged } from "../settings/settingsDiff";
import { useTranslation } from "react-i18next";
import { presentApiError, presentClassName, presentProfileName } from "../../i18n/presenters";

/** CharactersTab bündelt Profilwechsel, Pflichtskills, Bindings und Inventarschutz. */
export function CharactersTab({
  draft, catalog, selectedCharacter, characterNames, mutable, diffPaths, status,
  onSelectCharacter, onChangeDraft, onSetupChanged,
}: {
  draft: OperatorSettingsDTO;
  catalog: CatalogDTO | null;
  selectedCharacter: string;
  characterNames: string[];
  mutable: boolean;
  diffPaths: string[];
  status: StatusDTO | null;
  onSelectCharacter: (name: string) => void;
  onChangeDraft: (update: (current: OperatorSettingsDTO) => OperatorSettingsDTO) => void;
  onSetupChanged?: () => Promise<void> | void;
}) {
  const { t } = useTranslation();
  const [preview, setPreview] = useState<CharacterSetupPreviewDTO | null>(null);
  const [previewError, setPreviewError] = useState("");
  const [busy, setBusy] = useState(false);

  const characterSettings = draft.characters[selectedCharacter];
  const catalogEntry = catalog?.characters.find((entry) => entry.slug === selectedCharacter || entry.name.toLowerCase() === selectedCharacter);
  const profileID = characterSettings?.combat_profile ?? preview?.selected_profile_id ?? "";
  const selectedProfile = preview?.profiles.find((profile) => profile.id === profileID) ?? preview?.profiles.find((profile) => profile.is_selected) ?? preview?.profiles[0];
  const bindingsChanged = pathChanged(diffPaths, `characters.${selectedCharacter}.profile_bindings`);
  const inventoryChanged = pathChanged(diffPaths, `characters.${selectedCharacter}.inventory_lock`);
  const playersChanged = pathChanged(diffPaths, `characters.${selectedCharacter}.players`);
  const players = characterSettings?.players ?? 1;
  const inventoryGrid = characterSettings?.inventory_lock?.grid ?? null;
  const inventoryIsConfigured = inventoryConfigured(inventoryGrid);
  const queueIncludesCows = (characterSettings?.queue ?? []).includes("cows");
  const cowWarning = inventoryIsConfigured && queueIncludesCows && !inventoryCowSuitable(inventoryGrid!);

  useEffect(() => {
    if (!characterSettings) {
      setPreview(null);
      return;
    }
    const name = catalogEntry?.name ?? selectedCharacter;
    let cancelled = false;
    setBusy(true);
    setPreviewError("");
    void previewCharacterSetup({ character: name }).then((next) => {
      if (!cancelled) setPreview(next);
    }).catch((reason) => {
      if (!cancelled) {
        setPreview(null);
        setPreviewError(presentApiError(reason, t, t("characters.previewFallback")));
      }
    }).finally(() => {
      if (!cancelled) setBusy(false);
    });
    return () => { cancelled = true; };
  }, [selectedCharacter, characterSettings?.combat_profile, catalogEntry?.name, t]);

  const bindingValue = useMemo<BindingEditorValue>(() => {
    if (!characterSettings || !profileID) return emptyBindings();
    return bindingsFromDTO(
      characterSettings.profile_bindings?.[profileID],
      selectedProfile?.belt_layout ?? selectedProfile?.default_belt_layout,
      selectedProfile ? { healing: selectedProfile.default_healing_restock, mana: selectedProfile.default_mana_restock } : undefined,
    );
  }, [characterSettings, profileID, selectedProfile]);

  const updateBindings = (next: BindingEditorValue) => {
    if (!profileID) return;
    onChangeDraft((current) => {
      const character = current.characters[selectedCharacter];
      if (!character) return current;
      const profileBindings = { ...(character.profile_bindings ?? {}) };
      profileBindings[profileID] = bindingsToDTO(next);
      current.characters[selectedCharacter] = { ...character, profile_bindings: profileBindings };
      return current;
    });
  };

  const updateInventory = (grid: InventoryGrid) => {
    onChangeDraft((current) => {
      const character = current.characters[selectedCharacter];
      if (!character) return current;
      current.characters[selectedCharacter] = { ...character, inventory_lock: { grid } };
      return current;
    });
  };

  const updatePlayers = (next: number) => {
    onChangeDraft((current) => {
      const character = current.characters[selectedCharacter];
      if (!character) return current;
      current.characters[selectedCharacter] = { ...character, players: next };
      return current;
    });
  };

  return <div className="settings-tab-body settings-scope-characters">
    <p className="settings-scope-line">{t("characters.scope")}</p>

    <section>
      <div className="section-heading">
        <div><h2>{t("characters.title")}</h2><p>{t("characters.subtitle")}</p></div>
      </div>
      {characterNames.length === 0
        ? <StateMessage kind="empty" title={t("characters.emptyTitle")}>{t("characters.emptyDetail")}</StateMessage>
        : <ul className="character-loadout-list" aria-label={t("characters.listAria")}>
          {characterNames.map((name) => {
            const entry = catalog?.characters.find((item) => item.slug === name || item.name.toLowerCase() === name);
            const active = name === selectedCharacter;
            return <li key={name}>
              <button type="button" className={active ? "active" : undefined} onClick={() => onSelectCharacter(name)} aria-current={active ? "true" : undefined}>
                <strong>{entry?.name ?? name}</strong>
                <span>{characterStatusLabel(entry, catalog, t)}</span>
              </button>
            </li>;
          })}
        </ul>}
    </section>

    {characterSettings && <section className={(bindingsChanged || inventoryChanged || playersChanged) ? "settings-field-changed" : undefined}>
      <div className="section-heading">
        <div>
          <h2>{catalogEntry?.name ?? selectedCharacter}</h2>
          <p>
            {t("characters.classProfile", { className: presentClassName(preview?.character.character_class || characterSettings.character_class || "", t), profile: selectedProfile ? presentProfileName(selectedProfile.id, selectedProfile.display_name, t) : characterSettings.combat_profile || t("characters.profileUnset") })}
          </p>
        </div>
        <div className="inline-actions">
          {(bindingsChanged || inventoryChanged || playersChanged) && <StatusBadge tone="warning">{t("characters.changed")}</StatusBadge>}
          {catalogEntry && <StatusBadge tone={catalogEntry.farm_ready ? "success" : "warning"}>{characterStatusLabel(catalogEntry, catalog, t)}</StatusBadge>}
        </div>
      </div>

      {previewError && <StateMessage kind="error" title={t("characters.previewFailed")}>{previewError}</StateMessage>}
      {busy && !preview && <StateMessage kind="loading" title={t("characters.loading")} />}

      {status && catalog && <CharacterSetupWizard
        character={catalogEntry?.name ?? selectedCharacter}
        catalog={catalog}
        status={status}
        mode="settings"
        allowDeferBindings={false}
        showReload={false}
        onChanged={onSetupChanged}
      />}

      <h3>{t("characters.playersTitle")}</h3>
      <p className="hint">{t("characters.playersHint")}</p>
      <select aria-label={t("characters.playersAria")} value={players} disabled={!mutable} onChange={(event) => updatePlayers(Number(event.target.value))}>
        {[1, 2, 3, 4, 5, 6, 7, 8].map((value) => <option key={value} value={value}>{value}</option>)}
      </select>

      {selectedProfile && <>
        <h3>{t("characters.requiredSkills")}</h3>
        <RequiredSkillsList skills={selectedProfile.required_skills ?? []} standardAttack={selectedProfile.standard_attack} />
        <h3>{t("characters.bindings")}</h3>
        <p className="hint">{t("characters.bindingsHint")}</p>
        {!profileID
          ? <StateMessage kind="empty" title={t("characters.noProfileTitle")}>{t("characters.noProfileDetail")}</StateMessage>
          : <BindingEditor
            requiredSkills={selectedProfile.required_skills ?? []}
            optionalSkillPairs={selectedProfile.optional_skill_pairs ?? []}
            standardAttack={selectedProfile.standard_attack}
            requiresMercenary={selectedProfile.requires_mercenary}
            bindingsReady={selectedProfile.bindings_ready}
            bindingReasons={selectedProfile.binding_reasons ?? []}
            value={bindingValue}
            mutable={mutable && !!profileID}
            onChange={updateBindings}
          />}
      </>}

      <h3>{t("characters.inventory")}</h3>
      <InventoryLockEditor
        value={inventoryGrid}
        configured={inventoryIsConfigured}
        mutable={mutable}
        onChange={updateInventory}
      />
      {cowWarning && <StateMessage kind="error" title={t("characters.cowWarningTitle")}>{t("characters.cowWarningDetail")}</StateMessage>}

      {catalogEntry && !catalogEntry.farm_ready && (catalogEntry.farm_ready_reasons?.length ?? 0) > 0 && (
        <StateMessage kind="error" title={t("characters.queueLocked")}>
          {(catalogEntry.farm_ready_reasons ?? []).map((reason) => farmReadyReasonText(reason, t)).join(" ")}
        </StateMessage>
      )}
      {catalogEntry?.farm_ready && <StateMessage kind="empty" title={t("characters.farmReady")}>{t("characters.farmReadyDetail")}</StateMessage>}
    </section>}
  </div>;
}
