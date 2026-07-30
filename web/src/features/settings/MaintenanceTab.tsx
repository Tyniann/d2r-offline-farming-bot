import { FolderOpen } from "lucide-react";
import type { LiveEvent, OperatorSettingsDTO } from "../../api/generated";
import { Button, StateMessage, StatusBadge } from "../../app/ui";
import type { SettingsRun } from "./settingsTypes";

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
  return <div className="settings-tab-body settings-scope-maintenance">
    <p className="settings-scope-line">Sofortaktionen. Es gibt nichts zu speichern.</p>

    <section>
      <div className="section-heading">
        <div><h2>Lokales Diagnosepaket</h2><p>Der Go-Core redigiert Tokens und absolute Benutzerpfade. Nichts verlässt den Rechner.</p></div>
        <StatusBadge tone="neutral">kein Upload</StatusBadge>
      </div>
      <p>Spielstände, Speicherabbilder und Screenshots sind immer ausgeschlossen.</p>
      <div className="settings-grid two-columns settings-form-width">
        <label className="check"><input type="checkbox" checked={includeTelemetry} onChange={(event) => onIncludeTelemetry(event.target.checked)} /> Vollständige Telemetrie ausdrücklich beilegen</label>
        <label className="check"><input type="checkbox" checked={includeRoutes} onChange={(event) => onIncludeRoutes(event.target.checked)} /> Routenkoordinaten ausdrücklich beilegen</label>
      </div>
      <Button onClick={onBuildDiagnostic} disabled={busy}><FolderOpen aria-hidden="true" size={16} /> Redigiertes ZIP lokal erstellen</Button>
    </section>

    <section>
      <div className="section-heading">
        <div><h2>Live-Ereignisse</h2><p>Begrenzte Diagnoseprojektion der letzten Core-Zustandsänderungen.</p></div>
        <StatusBadge tone="neutral">maximal 40</StatusBadge>
      </div>
      {events.length === 0
        ? <StateMessage kind="empty" title="Noch keine Zustandsänderung">Der Rohfeed bleibt eine begrenzte Diagnoseprojektion.</StateMessage>
        : <ol className="event-feed">{events.map((event) => <li key={event.sequence}><time>{new Date(event.timestamp).toLocaleTimeString("de-DE")}</time><strong>{event.event}</strong><span>{event.area || event.step || event.reason || "Core-Aktualisierung"}</span></li>)}</ol>}
    </section>

    <section>
      <div className="section-heading"><div><h2>Effektive Werte (nur Anzeige)</h2><p>Vom Core gelesene Projektion – nicht editierbar.</p></div></div>
      <details className="effective-settings"><summary>Effektive Operatorwerte</summary><p>Datei: <code>configs/operator-settings.local.yaml</code></p><pre>{JSON.stringify(settings, null, 2)}</pre></details>
      <details className="effective-settings"><summary>Effektive Route-Combat-Werte (read-only)</summary><p>Vom Core nach Defaults und Validierung projiziert; <code>enabled: false</code> deaktiviert das Interleave ohne Änderung der Route.</p><pre>{JSON.stringify(Object.fromEntries(runs.map((run) => [run.id, run.routeCombat ?? null])), null, 2)}</pre></details>
    </section>

    <section>
      <div className="danger-zone">
        <h2>Gefahrenzone</h2>
        <h3>Gesamte Historie löschen</h3>
        <p>Die Vorschau umfasst alle direkten JSONL-Kategorien. Aktive Writer bleiben auch nach der zweiten Bestätigung geschützt. Es gibt weder Papierkorb noch Telemetriebackup.</p>
        <Button variant="danger" onClick={onPreviewDeleteHistory} disabled={!mutable || busy}>Löschvorschau erstellen</Button>
      </div>
    </section>
  </div>;
}
