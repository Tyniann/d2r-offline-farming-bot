import { StateMessage } from "../../app/ui";
import { SettingsDialogs } from "./SettingsControls";
import { SettingsOverview } from "./SettingsOverview";
import { useSettingsModel, type SettingsFeatureProps } from "./settingsModel";
import "./settings.css";

export type { SettingsFeatureProps } from "./settingsModel";

/**
 * SettingsFeature ist die Einstellungsseite: Core-Dokument (Charakter und Bot, gemeinsam als eine Revision
 * gespeichert), Desktop-Werte (sofort wirksam) und Wartung. Zustand und Aktionen liefert [useSettingsModel],
 * das Layout [SettingsOverview]; Bestätigungsdialoge liegen außerhalb des Fokus-Panels, damit sie es überlagern.
 */
export function SettingsFeature(props: SettingsFeatureProps) {
  const model = useSettingsModel(props);
  const { t } = model;

  if (!model.settings || !model.draft) {
    return model.error
      ? <StateMessage kind="error" title={t("settings.notAvailable")}>{model.error}</StateMessage>
      : <StateMessage kind="loading" title={t("settings.loadingSettings")}>{t("settings.loadingContracts")}</StateMessage>;
  }

  return <div className="settings-page">
    <SettingsOverview model={model} />
    <SettingsDialogs model={model} />
  </div>;
}
