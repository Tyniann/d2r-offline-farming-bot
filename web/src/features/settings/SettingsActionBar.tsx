import { Save } from "lucide-react";
import { Button } from "../../app/ui";
import { useTranslation } from "react-i18next";

/** SettingsActionBar ist die sticky Commit-Leiste für das Core-Farming-Dokument. */
export function SettingsActionBar({
  dirty, locked, revision, summary, busy, collapsed, onDiscard, onSave, onShowFarming,
}: {
  dirty: boolean;
  locked: boolean;
  revision: number;
  summary: string;
  busy: boolean;
  collapsed?: boolean;
  onDiscard: () => void;
  onSave: () => void;
  onShowFarming?: () => void;
}) {
  const { t } = useTranslation();
  if (collapsed) {
    return <div className={`settings-actionbar${dirty ? " dirty" : ""}`} role="status">
      <div><strong>{t("settings.unsavedChanges")}</strong><p>{summary}</p></div>
      {onShowFarming && <Button variant="secondary" onClick={onShowFarming}>{t("settings.view")}</Button>}
    </div>;
  }

  if (locked) {
    return <div className="settings-actionbar locked" role="status">
      <div><strong>{t("settings.lockedSession")}</strong><p>{t("settings.saveWhenIdle")}</p></div>
      <Button disabled><Save aria-hidden="true" size={16} /> {t("settings.save")}</Button>
    </div>;
  }

  const changeCount = dirty ? summary.split(", ").filter(Boolean).length : 0;
  return <div className={`settings-actionbar${dirty ? " dirty" : ""}`} role="status">
    <div>
      {dirty
        ? <strong>{t("settings.changeCount", { count: changeCount, summary })}</strong>
        : <><strong>{t("settings.noOpenChanges")}</strong><p>{t("settings.revision", { revision })}</p></>}
    </div>
    <div className="inline-actions">
      {dirty && <Button variant="secondary" onClick={onDiscard} disabled={busy}>{t("settings.discard")}</Button>}
      <Button onClick={onSave} disabled={!dirty || busy}><Save aria-hidden="true" size={16} /> {t(busy ? "settings.coreChecking" : "settings.save")}</Button>
    </div>
  </div>;
}
