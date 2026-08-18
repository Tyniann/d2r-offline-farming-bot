import { useEffect, useMemo, useState } from "react";
import { previewCharacterSetup, type CatalogDTO, type CharacterSetupPreviewDTO, type OperatorSettingsDTO, type StatusDTO } from "../../api/generated";
import { StateMessage, StatusBadge } from "../../app/ui";
import { BindingEditor, bindingsFromDTO, bindingsToDTO, emptyBindings, type BindingEditorValue } from "./BindingEditor";
import { CharacterSetupWizard } from "./CharacterSetupWizard";
import { characterStatusLabel, farmReadyReasonText } from "./characterReasonText";
import { InventoryLockEditor, inventoryConfigured, inventoryCowSuitable, type InventoryGrid } from "./InventoryLockEditor";
import { RequiredSkillsList } from "./RequiredSkillsList";
import { pathChanged } from "../settings/settingsDiff";

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
  const [preview, setPreview] = useState<CharacterSetupPreviewDTO | null>(null);
  const [previewError, setPreviewError] = useState("");
  const [busy, setBusy] = useState(false);

  const characterSettings = draft.characters[selectedCharacter];
  const catalogEntry = catalog?.characters.find((entry) => entry.slug === selectedCharacter || entry.name.toLowerCase() === selectedCharacter);
  const profileID = characterSettings?.combat_profile ?? preview?.selected_profile_id ?? "";
  const selectedProfile = preview?.profiles.find((profile) => profile.id === profileID) ?? preview?.profiles.find((profile) => profile.is_selected) ?? preview?.profiles[0];
  const bindingsChanged = pathChanged(diffPaths, `characters.${selectedCharacter}.profile_bindings`);
  const inventoryChanged = pathChanged(diffPaths, `characters.${selectedCharacter}.inventory_lock`);
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
        setPreviewError(reason instanceof Error ? reason.message : "Charaktervorschau fehlgeschlagen.");
      }
    }).finally(() => {
      if (!cancelled) setBusy(false);
    });
    return () => { cancelled = true; };
  }, [selectedCharacter, characterSettings?.combat_profile, catalogEntry?.name]);

  const bindingValue = useMemo<BindingEditorValue>(() => {
    if (!characterSettings || !profileID) return emptyBindings();
    return bindingsFromDTO(
      characterSettings.profile_bindings?.[profileID],
      selectedProfile?.belt_layout ?? selectedProfile?.default_belt_layout,
    );
  }, [characterSettings, profileID, selectedProfile?.belt_layout, selectedProfile?.default_belt_layout]);

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

  return <div className="settings-tab-body settings-scope-characters">
    <p className="settings-scope-line">Tastenbelegung und Inventarschutz gehören zum Charakter. Speichern nutzt dieselbe Core-Revision wie Farming.</p>

    <section>
      <div className="section-heading">
        <div><h2>Charaktere</h2><p>Alle erkannten Spielstände mit Core-Readiness.</p></div>
      </div>
      {characterNames.length === 0
        ? <StateMessage kind="empty" title="Keine Charaktere">Der Core hat noch keine Operatorwerte geliefert.</StateMessage>
        : <ul className="character-loadout-list" aria-label="Charakterliste">
          {characterNames.map((name) => {
            const entry = catalog?.characters.find((item) => item.slug === name || item.name.toLowerCase() === name);
            const active = name === selectedCharacter;
            return <li key={name}>
              <button type="button" className={active ? "active" : undefined} onClick={() => onSelectCharacter(name)} aria-current={active ? "true" : undefined}>
                <strong>{entry?.name ?? name}</strong>
                <span>{characterStatusLabel(entry, catalog)}</span>
              </button>
            </li>;
          })}
        </ul>}
    </section>

    {characterSettings && <section className={(bindingsChanged || inventoryChanged) ? "settings-field-changed" : undefined}>
      <div className="section-heading">
        <div>
          <h2>{catalogEntry?.name ?? selectedCharacter}</h2>
          <p>
            Klasse {preview?.character.class_display_name || characterSettings.character_class || "–"}
            {" · "}Profil {selectedProfile?.display_name || characterSettings.combat_profile || "noch nicht gesetzt"}
          </p>
        </div>
        <div className="inline-actions">
          {(bindingsChanged || inventoryChanged) && <StatusBadge tone="warning">Geändert</StatusBadge>}
          {catalogEntry && <StatusBadge tone={catalogEntry.farm_ready ? "success" : "warning"}>{characterStatusLabel(catalogEntry, catalog)}</StatusBadge>}
        </div>
      </div>

      {previewError && <StateMessage kind="error" title="Vorschau fehlgeschlagen">{previewError}</StateMessage>}
      {busy && !preview && <StateMessage kind="loading" title="Charakterdaten werden geladen" />}

      {status && catalog && <CharacterSetupWizard
        character={catalogEntry?.name ?? selectedCharacter}
        catalog={catalog}
        status={status}
        mode="settings"
        allowDeferBindings={false}
        showReload={false}
        onChanged={onSetupChanged}
      />}

      {selectedProfile && <>
        <h3>Pflichtskills</h3>
        <RequiredSkillsList skills={selectedProfile.required_skills ?? []} standardAttack={selectedProfile.standard_attack} />
        <h3>Tastenbelegung</h3>
        <p className="hint">Skills, Gürteltasten und welche Trankart in welcher Spalte liegt.</p>
        {!profileID
          ? <StateMessage kind="empty" title="Kein Kampfprofil">Richte den Charakter zuerst ein, bevor Tasten gespeichert werden.</StateMessage>
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

      <h3>Inventarschutz</h3>
      <InventoryLockEditor
        value={inventoryGrid}
        configured={inventoryIsConfigured}
        mutable={mutable}
        onChange={updateInventory}
      />
      {cowWarning && <StateMessage kind="error" title="Inventarlayout für Cow-Runs ungeeignet">Countess und andere Runs bleiben möglich. Für Cow-Runs fehlt gleichzeitig geschützter 2×2-Platz und freier Platz für Bein sowie Stadtportalbuch.</StateMessage>}

      {catalogEntry && !catalogEntry.farm_ready && (catalogEntry.farm_ready_reasons?.length ?? 0) > 0 && (
        <StateMessage kind="error" title="Queue bleibt gesperrt">
          {(catalogEntry.farm_ready_reasons ?? []).map((reason) => farmReadyReasonText(reason)).join(" ")}
        </StateMessage>
      )}
      {catalogEntry?.farm_ready && <StateMessage kind="empty" title="Charakter farmbereit">Die Queue darf starten, sobald Route und übrige Voraussetzungen bereit sind.</StateMessage>}
    </section>}
  </div>;
}
