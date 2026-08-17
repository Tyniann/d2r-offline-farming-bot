import { useState, type DragEvent } from "react";
import { Button, StateMessage } from "../../app/ui";
import { isRunStartable, runAvailabilityText } from "../../app/runReasons";
import { move } from "./settingsDiff";
import type { SettingsRun } from "./settingsTypes";

/** QueueEditor zeigt die Run-Reihenfolge als Zwei-Spalten-Hero mit Drag & Drop. */
export function QueueEditor({
  queue, runs, mutable, changed, characterClass = "", onChange,
}: {
  queue: string[];
  runs: SettingsRun[];
  mutable: boolean;
  changed: boolean;
  characterClass?: string;
  onChange: (queue: string[]) => void;
}) {
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [dropHighlight, setDropHighlight] = useState(false);
  const available = runs.filter((run) => !queue.includes(run.id));

  const reorder = (from: number, to: number) => {
    if (!mutable || from === to) return;
    onChange(move(queue, from, to));
  };

  const insertCatalogRun = (runID: string, at: number) => {
    if (!mutable || queue.includes(runID)) return;
    const run = runs.find((entry) => entry.id === runID);
    if (!run || !isRunStartable(run.status ?? "")) return;
    const next = [...queue];
    next.splice(Math.max(0, Math.min(at, next.length)), 0, runID);
    onChange(next);
  };

  const parsePayload = (event: DragEvent) => {
    const raw = event.dataTransfer.getData("text/plain");
    if (raw.startsWith("add:")) return { kind: "add" as const, id: raw.slice(4) };
    if (raw.startsWith("reorder:")) return { kind: "reorder" as const, index: Number(raw.slice(8)) };
    return null;
  };

  const allowDrop = (event: DragEvent) => {
    if (!mutable) return;
    event.preventDefault();
    // Catalog advertises copy; queue reorder advertises move. Chromium cancels
    // the drop when dropEffect is incompatible with effectAllowed — do not
    // hard-code copy for both (that broke in-list reorder after catalog DnD).
    event.dataTransfer.dropEffect = event.dataTransfer.effectAllowed === "copy" ? "copy" : "move";
    setDropHighlight(true);
  };

  const dropAt = (event: DragEvent, at: number) => {
    event.preventDefault();
    event.stopPropagation();
    const payload = parsePayload(event);
    setDragIndex(null);
    setDropHighlight(false);
    if (!payload) return;
    if (payload.kind === "add") {
      insertCatalogRun(payload.id, at);
      return;
    }
    if (Number.isFinite(payload.index)) reorder(payload.index, Math.min(at, queue.length - 1));
  };

  return <div className={`settings-queue-editor${changed ? " settings-field-changed" : ""}`}>
    <div className="section-heading">
      <div><h2>Run-Reihenfolge</h2><p>Die Runs laufen pro Spiel in dieser Reihenfolge. Ziehe Katalog-Runs von rechts nach links oder sortiere die aktive Liste.</p></div>
      {changed && <span className="status-badge status-warning">Geändert</span>}
    </div>
    <div className="settings-queue-panes">
      <div
        className={`settings-queue-pane${dropHighlight ? " drop-target" : ""}`}
        data-testid="queue-drop-pane"
        onDragOver={allowDrop}
        onDragLeave={() => setDropHighlight(false)}
        onDrop={(event) => dropAt(event, queue.length)}
      >
        <h3>Deine Reihenfolge</h3>
        {queue.length === 0
          ? <StateMessage kind="empty" title="Noch keine Runs in der Reihenfolge">Ziehe rechts einen Run hierher oder klicke auf „+“.</StateMessage>
          : <ol className="queue-list settings-queue">
            {queue.map((runID, index) => {
              const label = runs.find((run) => run.id === runID)?.label ?? runID;
              return <li
                key={runID}
                draggable={mutable}
                onDragStart={(event) => {
                  setDragIndex(index);
                  event.dataTransfer.setData("text/plain", `reorder:${index}`);
                  event.dataTransfer.effectAllowed = "move";
                }}
                onDragOver={allowDrop}
                onDrop={(event) => dropAt(event, index)}
                onDragEnd={() => { setDragIndex(null); setDropHighlight(false); }}
                className={dragIndex === index ? "dragging" : undefined}
              >
                <span className="drag-handle" aria-hidden="true">⠿</span>
                <span>{index + 1}</span>
                <strong>{label}</strong>
                <div className="queue-actions">
                  <Button variant="secondary" aria-label={`${label} nach oben`} disabled={!mutable || index === 0} onClick={() => reorder(index, index - 1)}>↑</Button>
                  <Button variant="secondary" aria-label={`${label} nach unten`} disabled={!mutable || index === queue.length - 1} onClick={() => reorder(index, index + 1)}>↓</Button>
                  <Button variant="secondary" disabled={!mutable} onClick={() => onChange(queue.filter((entry) => entry !== runID))}>Entfernen</Button>
                </div>
              </li>;
            })}
          </ol>}
      </div>
      <div className="settings-queue-pane">
        <h3>Verfügbare Runs</h3>
        {available.length === 0
          ? <p className="hint">Alle Katalog-Runs sind bereits in der Reihenfolge.</p>
          : <ul className="settings-run-catalog">
            {available.map((run) => {
              const availability = runAvailabilityText(run.status ?? "", run.reasons ?? [], characterClass);
              const startable = isRunStartable(run.status ?? "");
              return <li
                key={run.id}
                draggable={mutable && startable}
                onDragStart={(event) => {
                  if (!startable) {
                    event.preventDefault();
                    return;
                  }
                  event.dataTransfer.setData("text/plain", `add:${run.id}`);
                  event.dataTransfer.effectAllowed = "copy";
                }}
                className={mutable && startable ? "catalog-draggable" : undefined}
                title={startable ? (mutable ? "In die aktive Reihenfolge ziehen" : undefined) : availability.detail}
              >
                <span className="drag-handle" aria-hidden="true">⠿</span>
                <Button
                  variant="secondary"
                  disabled={!mutable || !startable}
                  onClick={() => onChange([...queue, run.id])}
                  onDragStart={(event) => event.preventDefault()}
                >+ {run.label}</Button>
                <small title={availability.detail}>{availability.title}</small>
              </li>;
            })}
          </ul>}
      </div>
    </div>
  </div>;
}
