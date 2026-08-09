import { useRef } from "react";

const rows = 4;
const cols = 10;

export type InventoryCell = 0 | 1;
export type InventoryGrid = InventoryCell[][];

/** allLockedGrid liefert das effektive 4×10-Raster für unbestätigte Inventare. */
export function allLockedGrid(): InventoryGrid {
  return Array.from({ length: rows }, () => Array.from({ length: cols }, () => 1 as InventoryCell));
}

/** cloneGrid kopiert ein Inventarraster defensiv. */
export function cloneGrid(grid: InventoryGrid): InventoryGrid {
  return grid.map((row) => [...row]);
}

/** inventoryConfigured meldet, ob der Draft ein gespeichertes/bestätigtes Raster trägt. */
export function inventoryConfigured(grid: number[][] | null | undefined): grid is InventoryGrid {
  return Array.isArray(grid) && grid.length === rows && grid.every((row) => row.length === cols && row.every((cell) => cell === 0 || cell === 1));
}

/** inventoryCowSuitable spiegelt die Core-Heuristik für die Charaktere-Warnung. */
export function inventoryCowSuitable(grid: InventoryGrid): boolean {
  if (!inventoryConfigured(grid)) return false;
  const locked = grid.map((row) => row.map((cell) => cell === 1));
  let hasLocked2x2 = false;
  for (let row = 0; row < 3 && !hasLocked2x2; row += 1) {
    for (let col = 0; col < 9; col += 1) {
      if (locked[row][col] && locked[row][col + 1] && locked[row + 1][col] && locked[row + 1][col + 1]) {
        hasLocked2x2 = true;
        break;
      }
    }
  }
  if (!hasLocked2x2) return false;
  return canPlace(locked, 1, 3) && canPlace(locked, 1, 2);
}

function canPlace(locked: boolean[][], width: number, height: number): boolean {
  for (let row = 0; row <= rows - height; row += 1) {
    for (let col = 0; col <= cols - width; col += 1) {
      let fits = true;
      for (let dy = 0; dy < height && fits; dy += 1) {
        for (let dx = 0; dx < width; dx += 1) {
          if (locked[row + dy][col + dx]) {
            fits = false;
            break;
          }
        }
      }
      if (fits) return true;
    }
  }
  return false;
}

function normalizeGrid(value: number[][] | null | undefined): InventoryGrid {
  if (inventoryConfigured(value)) return cloneGrid(value);
  return allLockedGrid();
}

/** InventoryLockEditor ist der originale 4×10-Schutzeditor ohne Free-Preset. */
export function InventoryLockEditor({
  value, configured, mutable, onChange,
}: {
  value: number[][] | null | undefined;
  configured: boolean;
  mutable: boolean;
  onChange: (next: InventoryGrid) => void;
}) {
  const dragTarget = useRef<InventoryCell | null>(null);
  const grid = normalizeGrid(value);

  const commit = (next: InventoryGrid) => onChange(cloneGrid(next));

  const setCell = (row: number, col: number, locked: InventoryCell) => {
    if (!mutable) return;
    const next = normalizeGrid(value);
    next[row][col] = locked;
    commit(next);
  };

  const lockAll = () => {
    if (!mutable) return;
    commit(allLockedGrid());
  };

  const onPointerDown = (row: number, col: number) => {
    if (!mutable) return;
    const nextValue: InventoryCell = grid[row][col] === 1 ? 0 : 1;
    dragTarget.current = nextValue;
    setCell(row, col, nextValue);
  };

  const onPointerEnter = (row: number, col: number) => {
    if (!mutable || dragTarget.current === null) return;
    if (grid[row][col] === dragTarget.current) return;
    setCell(row, col, dragTarget.current);
  };

  const endDrag = () => {
    dragTarget.current = null;
  };

  const moveFocus = (row: number, col: number, key: string) => {
    let nextRow = row;
    let nextCol = col;
    if (key === "ArrowRight") nextCol = Math.min(cols - 1, col + 1);
    if (key === "ArrowLeft") nextCol = Math.max(0, col - 1);
    if (key === "ArrowDown") nextRow = Math.min(rows - 1, row + 1);
    if (key === "ArrowUp") nextRow = Math.max(0, row - 1);
    const target = document.getElementById(`inventory-cell-${nextRow}-${nextCol}`);
    target?.focus();
  };

  return <div className="inventory-lock-editor">
    <p className="hint">4 Zeilen × 10 Spalten. Geschützte Felder bleiben für den Bot gesperrt; freie Felder stehen für Beute zur Verfügung.</p>
    {!configured && <p className="inventory-unconfirmed" role="status">Noch nicht bestätigt</p>}
    <div className="inventory-lock-legend" aria-hidden="true">
      <span className="inventory-legend-locked">Geschützt</span>
      <span className="inventory-legend-free">Für Beute verfügbar</span>
    </div>
    <div
      className="inventory-lock-grid"
      role="grid"
      aria-label="Inventarschutz 4 Zeilen mal 10 Spalten"
      onPointerUp={endDrag}
      onPointerCancel={endDrag}
      onPointerLeave={endDrag}
    >
      {grid.map((rowValues, row) => <div key={row} className="inventory-lock-row" role="row">
        {rowValues.map((cell, col) => {
          const locked = cell === 1;
          return <button
            key={`${row}-${col}`}
            id={`inventory-cell-${row}-${col}`}
            type="button"
            role="gridcell"
            className={locked ? "inventory-cell locked" : "inventory-cell free"}
            aria-label={locked ? `Zeile ${row + 1} Spalte ${col + 1} geschützt` : `Zeile ${row + 1} Spalte ${col + 1} für Beute verfügbar`}
            aria-pressed={locked}
            disabled={!mutable}
            onPointerDown={(event) => {
              event.preventDefault();
              onPointerDown(row, col);
            }}
            onPointerEnter={() => onPointerEnter(row, col)}
            onKeyDown={(event) => {
              if (event.key === " " || event.key === "Enter") {
                event.preventDefault();
                setCell(row, col, locked ? 0 : 1);
                return;
              }
              if (event.key.startsWith("Arrow")) {
                event.preventDefault();
                moveFocus(row, col, event.key);
              }
            }}
          >
            {locked ? <span className="inventory-lock-mark" aria-hidden="true" /> : null}
          </button>;
        })}
      </div>)}
    </div>
    <div className="inline-actions">
      <button type="button" disabled={!mutable} onClick={lockAll}>Alle schützen</button>
    </div>
  </div>;
}
