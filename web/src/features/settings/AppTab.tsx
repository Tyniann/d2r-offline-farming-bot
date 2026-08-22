import { useState } from "react";
import { RefreshCw } from "lucide-react";
import { Button, StateMessage, StatusBadge } from "../../app/ui";
import { useTranslation } from "react-i18next";
import type { AppTranslator } from "../../i18n/presenters";

/** AppTab verwaltet den Desktop-Store mit Sofortspeichern und Versionsprüfung. */
export function AppTab({
  desktop, updateStatus, busy, onOpenOnboarding, onToggleAutostart, onCheckUpdates, onOpenRelease,
}: {
  desktop: DesktopSettingsView | null;
  updateStatus: DesktopUpdateStatus | null;
  busy: boolean;
  onOpenOnboarding?: () => void;
  onToggleAutostart: (value: boolean) => Promise<void>;
  onCheckUpdates: () => void;
  onOpenRelease: () => void;
}) {
  const { t } = useTranslation();
  const [savedFlash, setSavedFlash] = useState(false);

  const toggleAutostart = async (value: boolean) => {
    await onToggleAutostart(value);
    setSavedFlash(true);
    window.setTimeout(() => setSavedFlash(false), 1800);
  };

  return <div className="settings-tab-body settings-scope-app">
    <p className="settings-scope-line">{t("settings.appScope")}</p>

    <section>
      <div className="section-heading">
        <div><h2>{t("settings.appStartTitle")}</h2><p>{t("settings.appStartDetail")}</p></div>
        <StatusBadge tone={desktop ? "success" : "neutral"}>{t(desktop ? "settings.local" : "settings.bridgeMissing")}</StatusBadge>
      </div>
      {desktop ? <div className="settings-form-width settings-grid">
        <label className="check">
          <input type="checkbox" checked={desktop.autostart} disabled={busy || !window.d2rDesktop} onChange={(event) => void toggleAutostart(event.target.checked)} />
          {t("settings.autostart")}
          {savedFlash && <span className="settings-inline-saved" role="status">{t("settings.saved")}</span>}
        </label>
        <p className="hint">{t("settings.onboardingStatus", { status: t(desktop.onboarding_completed ? "settings.completed" : "settings.stillOpen") })}</p>
        <p className="hint">{t("settings.autostartHint")}</p>
        {onOpenOnboarding && <Button variant="secondary" onClick={onOpenOnboarding}>{t("settings.openOnboarding")}</Button>}
      </div> : <StateMessage kind="empty" title={t("settings.bridgeInactive")}>{t("settings.bridgeInactiveDetail")}</StateMessage>}
    </section>

    <section>
      <div className="section-heading">
        <div><h2>{t("settings.version")}</h2><p>{t("settings.versionDetail")}</p></div>
        <StatusBadge tone={updateStatus?.status === "available" ? "warning" : updateStatus?.status === "up_to_date" ? "success" : "neutral"}>{updateLabel(updateStatus, t)}</StatusBadge>
      </div>
      <p>{updateDescription(updateStatus, t)}</p>
      <div className="inline-actions">
        <Button variant="secondary" onClick={onCheckUpdates} disabled={busy || !window.d2rDesktop?.checkForUpdates}><RefreshCw aria-hidden="true" size={16} /> {t("settings.checkAgain")}</Button>
        {updateStatus?.status === "available" && <Button onClick={onOpenRelease}>{t("settings.openRelease")}</Button>}
      </div>
      <p className="hint">{t("settings.updateHint")}</p>
    </section>
  </div>;
}

function updateLabel(status: DesktopUpdateStatus | null, t: AppTranslator): string {
  if (!status) return t("settings.desktopBridgeMissing");
  if (status.status === "checking") return t("settings.checking");
  if (status.status === "available") return t("settings.newVersion");
  if (status.status === "up_to_date") return t("settings.upToDate");
  return t("settings.unavailable");
}

function updateDescription(status: DesktopUpdateStatus | null, t: AppTranslator): string {
  if (!status) return t("settings.updateDesktopOnly");
  if (status.status === "checking") return t("settings.updateChecking", { current: status.current_version });
  if (status.status === "available") return t("settings.updateAvailable", { current: status.current_version, latest: status.latest_version });
  if (status.status === "up_to_date") return t("settings.updateCurrent", { current: status.current_version });
  return t("settings.updateUnavailable", { current: status.current_version });
}
