import { FolderOpen } from "lucide-react";
import type { LiveEvent, OperatorSettingsDTO } from "../../api/generated";
import { Button, StateMessage, StatusBadge } from "../../app/ui";
import type { SettingsRun } from "./settingsTypes";
import { useTranslation } from "react-i18next";
import { formatDate } from "../../i18n/format";

/** MaintenanceTab bündelt Sofortaktionen, Diagnose und read-only Projektionen. */
export function MaintenanceTab({
  settings, runs, events, mutable, busy, includeTelemetry, includeRoutes,
  onIncludeTelemetry, onIncludeRoutes, onBuildDiagnostic, onPreviewDeleteHistory,
}: {
  settings: OperatorSettingsDTO;
  runs: SettingsRun[];
  events: LiveEvent[];
  mutable: boolean;
  busy: boolean;
  includeTelemetry: boolean;
  includeRoutes: boolean;
  onIncludeTelemetry: (value: boolean) => void;
  onIncludeRoutes: (value: boolean) => void;
  onBuildDiagnostic: () => void;
  onPreviewDeleteHistory: () => void;
}) {
  const { t } = useTranslation();
  return <div className="settings-tab-body settings-scope-maintenance">
    <p className="settings-scope-line">{t("settings.maintenanceScope")}</p>

    <section>
      <div className="section-heading">
        <div><h2>{t("settings.diagnosticTitle")}</h2><p>{t("settings.diagnosticDetail")}</p></div>
        <StatusBadge tone="neutral">{t("settings.noUpload")}</StatusBadge>
      </div>
      <p>{t("settings.diagnosticExcludes")}</p>
      <div className="settings-grid two-columns settings-form-width">
        <label className="check"><input type="checkbox" checked={includeTelemetry} onChange={(event) => onIncludeTelemetry(event.target.checked)} /> {t("settings.includeTelemetry")}</label>
        <label className="check"><input type="checkbox" checked={includeRoutes} onChange={(event) => onIncludeRoutes(event.target.checked)} /> {t("settings.includeRoutes")}</label>
      </div>
      <Button onClick={onBuildDiagnostic} disabled={busy}><FolderOpen aria-hidden="true" size={16} /> {t("settings.buildDiagnostic")}</Button>
    </section>

    <section>
      <div className="section-heading">
        <div><h2>{t("settings.liveEvents")}</h2><p>{t("settings.liveEventsDetail")}</p></div>
        <StatusBadge tone="neutral">{t("settings.max40")}</StatusBadge>
      </div>
      {events.length === 0
        ? <StateMessage kind="empty" title={t("settings.noEvents")}>{t("settings.noEventsDetail")}</StateMessage>
        : <ol className="event-feed">{events.map((event) => <li key={event.sequence}><time>{formatDate(event.timestamp, { timeStyle: "medium" })}</time><strong>{event.event}</strong><span>{event.area || event.step || event.reason || t("settings.coreUpdate")}</span></li>)}</ol>}
    </section>

    <section>
      <div className="section-heading"><div><h2>{t("settings.effectiveTitle")}</h2><p>{t("settings.effectiveDetail")}</p></div></div>
      <details className="effective-settings"><summary>{t("settings.effectiveOperator")}</summary><p>{t("settings.file")} <code>configs/operator-settings.local.yaml</code></p><pre>{JSON.stringify(settings, null, 2)}</pre></details>
      <details className="effective-settings"><summary>{t("settings.effectiveRouteCombat")}</summary><p>{t("settings.routeCombatDetail")}</p><pre>{JSON.stringify(Object.fromEntries(runs.map((run) => [run.id, run.routeCombat ?? null])), null, 2)}</pre></details>
    </section>

    <section>
      <div className="danger-zone">
        <h2>{t("settings.dangerZone")}</h2>
        <h3>{t("settings.deleteHistory")}</h3>
        <p>{t("settings.deleteHistoryDetail")}</p>
        <Button variant="danger" onClick={onPreviewDeleteHistory} disabled={!mutable || busy}>{t("settings.previewDelete")}</Button>
      </div>
    </section>
  </div>;
}
