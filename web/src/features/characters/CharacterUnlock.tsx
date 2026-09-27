import { useState } from "react";
import { useTranslation } from "react-i18next";
import { reloadCharacters, type CatalogDTO, type StatusDTO } from "../../api/generated";
import { Button, Dialog, StateMessage } from "../../app/ui";
import { characterAvailabilityText } from "../../app/characterReasons";
import { presentApiError } from "../../i18n/presenters";
import { CharacterSetupWizard } from "./CharacterSetupWizard";

/** CharacterUnlock hält die Freischaltung getrennt von der aktiven Farming-Auswahl. */
export function CharacterUnlock({ catalog, status, lockedReason, onChanged, onConfigure }: {
  catalog: CatalogDTO; status: StatusDTO; lockedReason: string;
  onChanged(): Promise<void>; onConfigure(character: string): void;
}) {
  const { t } = useTranslation();
  const [chosen, setChosen] = useState("");
  const [editing, setEditing] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  // Selectable beschreibt Profil und Auswahlbild. Fehlende Tasten oder
  // Inventarschutz stehen separat in farm_ready und verschieben niemanden.
  const pending = catalog.characters.filter((entry) => !entry.selectable);
  const selected = pending.find((entry) => entry.name === chosen) ?? pending[0];
  const unsupported = selected?.reasons?.some((reason) => reason === "character_class_unsupported" || reason === "character_class_unknown");
  const resume = selected?.reasons?.includes("character_anchor_missing") && !selected.reasons.includes("character_profile_missing");
  const unlocked = catalog.characters.find((entry) => entry.name === editing)?.selectable;

  async function reload() {
    setBusy(true);
    setError("");
    try {
      await reloadCharacters();
      await onChanged();
    } catch (reason) {
      setError(presentApiError(reason, t, t("characters.wizardLoadFailed")));
    } finally { setBusy(false); }
  }

  return <>
    <section className="sidebar-context" aria-label={t("characters.unlockTitle")}>
      {pending.length > 0 && <>
        <strong>{t("characters.unlockTitle")}</strong>
        <label>{t("characters.unlockCharacter")}<select value={selected?.name ?? ""} disabled={busy || !!lockedReason} onChange={(event) => setChosen(event.target.value)}>
          {pending.map((entry) => <option key={entry.slug} value={entry.name}>{entry.name}</option>)}
        </select></label>
        {selected && <small>{characterAvailabilityText(selected, catalog, t)}</small>}
        <Button disabled={busy || !!lockedReason || !selected || unsupported} onClick={() => setEditing(selected!.name)}>{t(resume ? "characters.unlockResume" : "characters.unlockStart")}</Button>
      </>}
      <Button variant="secondary" disabled={busy || !!lockedReason} onClick={() => void reload()}>{t("characters.unlockSearch")}</Button>
      {lockedReason && <small>{lockedReason}</small>}
      {error && <small role="alert">{error}</small>}
    </section>
    {editing && <Dialog title={t("characters.unlockTitle")} onClose={() => { if (!busy) setEditing(""); }}>
      {lockedReason
        ? <StateMessage kind="error" title={t("characters.unlockPaused")}>{lockedReason}</StateMessage>
        : <CharacterSetupWizard character={editing} catalog={catalog} status={status} mode="unlock" allowDeferBindings={false} onBusyChange={setBusy} onChanged={onChanged} />}
      <div className="modal-actions">
        {unlocked && <Button disabled={busy || !!lockedReason} onClick={() => { onConfigure(editing); setEditing(""); }}>{t("characters.unlockSettings")}</Button>}
        <Button variant="secondary" disabled={busy} onClick={() => setEditing("")}>{t("characters.unlockClose")}</Button>
      </div>
    </Dialog>}
  </>;
}
