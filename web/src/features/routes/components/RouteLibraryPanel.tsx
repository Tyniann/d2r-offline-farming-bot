import { MoreHorizontal, Route as RouteIcon } from "lucide-react";
import type { RecordingOptionDTO, RouteEntryDTO } from "../../../api/generated";
import { difficultyLabel, roleLabel, routeStatus, routeStatusTone, runLabel, runOrder } from "../routePresentation";

interface Props {
  routes: RouteEntryDTO[] | null;
  options: RecordingOptionDTO[];
  archive: boolean;
  locked: boolean;
  onArchiveChange(archive: boolean): void;
  onRecord(runID: string, role?: string): void;
  onMutate(operation: string, routeID: string): void;
}

function routeIcon(runID: string): string {
  return runLabel(runID).slice(0, 1).toUpperCase();
}

export function RouteLibraryPanel({ routes, options, archive, locked, onArchiveChange, onRecord, onMutate }: Props) {
  const availableRuns = new Set([...options.map((entry) => entry.run_id), ...(routes ?? []).map((entry) => entry.run_id)]);
  const orderedRuns = [...runOrder.filter((runID) => availableRuns.has(runID)), ...[...availableRuns].filter((runID) => !runOrder.includes(runID as typeof runOrder[number]))];
  const visibleRows = archive ? (routes ?? []).map((route) => ({ runID: route.run_id, routes: [route] })) : orderedRuns.map((runID) => ({ runID, routes: (routes ?? []).filter((route) => route.run_id === runID) }));

  return <div className="route-panel" aria-labelledby="route-library-title">
    <div className="route-panel-heading">
      <div><h3 id="route-library-title">Meine Routen</h3><p>{archive ? "Archivierte Routen verwalten und bei Bedarf wiederherstellen." : "Eingerichtete Laufwege dieses Charakters auf einen Blick."}</p></div>
      <label className="route-view-filter">Ansicht
        <select value={archive ? "archive" : "active"} onChange={(event) => onArchiveChange(event.target.value === "archive")}>
          <option value="active">Aktive Routen</option><option value="archive">Archiv</option>
        </select>
      </label>
    </div>
    {routes === null && <p className="route-empty">Routen werden geladen …</p>}
    {routes !== null && visibleRows.length === 0 && <p className="route-empty">{archive ? "Das Archiv ist leer." : "Für diesen Charakter gibt es noch keine Farming-Route."}</p>}
    {routes !== null && visibleRows.length > 0 && <div className="route-library-list">
      {visibleRows.map(({ runID, routes: groupedRoutes }, index) => {
        const primary = groupedRoutes.find((route) => route.assigned) ?? groupedRoutes[0];
        const isCows = runID === "cows";
        const status = isCows ? (groupedRoutes.some((route) => route.route_role === "leg_acquisition") && groupedRoutes.some((route) => route.route_role === "cow_sweep") ? "Vollständig" : "Unvollständig") : routeStatus(primary);
        const key = archive ? primary?.route_id ?? `${runID}-${index}` : runID;
        return <article className={`route-library-row${isCows && !archive ? " route-library-row-wide" : ""}`} key={key}>
          <div className="route-run-icon" aria-hidden="true">{routeIcon(runID)}</div>
          <div className="route-row-main">
            <strong>{runLabel(runID)}{archive && primary?.route_role ? ` · ${roleLabel(primary.route_role)}` : ""}</strong>
            <span>{primary ? difficultyLabel(primary.difficulty) : "Noch nicht eingerichtet"}{isCows && !archive ? " · Zwei zusammengehörige Routen" : " · Standardroute"}</span>
          </div>
          <span className={`route-status ${routeStatusTone(status)}`}>{status}</span>
          {!primary && !archive && <button type="button" className="secondary route-row-action" disabled={locked} onClick={() => onRecord(runID)}>Jetzt aufnehmen</button>}
          {primary && (!isCows || archive) && <details className="route-action-menu">
            <summary aria-label={`Weitere Aktionen für ${runLabel(runID)}`}><MoreHorizontal aria-hidden="true" size={20} /></summary>
            <div>
              {archive ? <>
                <button type="button" disabled={locked} onClick={() => onMutate("restore", primary.route_id)}>Wiederherstellen</button>
                <button type="button" className="danger" disabled={locked} onClick={() => onMutate("delete", primary.route_id)}>Endgültig löschen</button>
              </> : <button type="button" disabled={locked} onClick={() => onMutate("archive", primary.route_id)}>Archivieren</button>}
            </div>
          </details>}
          {isCows && !archive && <div className="route-subroutes">
            {["leg_acquisition", "cow_sweep"].map((role, roleIndex) => {
              const route = groupedRoutes.find((entry) => entry.route_role === role);
              const subStatus = routeStatus(route);
              return <div key={role}><span>{roleIndex + 1} · {roleLabel(role)}</span><span>{subStatus}</span>{!route ? <button type="button" className="route-text-button" disabled={locked} onClick={() => onRecord(runID, role)}>Jetzt aufnehmen</button> : <details className="route-subroute-menu"><summary aria-label={`Weitere Aktionen für ${roleLabel(role)}`}><MoreHorizontal aria-hidden="true" size={18}/></summary><div><button type="button" disabled={locked} onClick={() => onMutate("archive", route.route_id)}>Archivieren</button></div></details>}</div>;
            })}
          </div>}
        </article>;
      })}
    </div>}
    {!archive && routes !== null && visibleRows.length === 0 && options.length > 0 && <button type="button" className="secondary" onClick={() => onRecord(options[0].run_id)}><RouteIcon aria-hidden="true" size={18} /> Erste Route aufnehmen</button>}
  </div>;
}
