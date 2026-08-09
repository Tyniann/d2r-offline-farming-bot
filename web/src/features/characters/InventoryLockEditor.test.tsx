import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { InventoryLockEditor, allLockedGrid, inventoryCowSuitable } from "./InventoryLockEditor";

describe("InventoryLockEditor", () => {
  afterEach(() => cleanup());

  it("zeigt unbestätigtes Raster als geschützt und materialisiert beim Speichern-Vorgänger", () => {
    const onChange = vi.fn();
    render(<InventoryLockEditor value={null} configured={false} mutable onChange={onChange} />);
    expect(screen.getByText("Noch nicht bestätigt")).toBeInTheDocument();
    expect(screen.getByRole("grid", { name: /inventarschutz/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /freigeben|free/i })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Alle schützen" }));
    expect(onChange).toHaveBeenCalledWith(allLockedGrid());
  });

  it("toggled Zellen per Tastatur und Pointer", () => {
    const onChange = vi.fn();
    const grid = allLockedGrid();
    render(<InventoryLockEditor value={grid} configured mutable onChange={onChange} />);
    const cell = screen.getByRole("gridcell", { name: /zeile 1 spalte 5 geschützt/i });
    fireEvent.keyDown(cell, { key: " " });
    expect(onChange).toHaveBeenCalled();
    const next = onChange.mock.calls.at(-1)?.[0] as number[][];
    expect(next[0][4]).toBe(0);
  });

  it("erkennt Cow-geeignete und ungeeignete Raster", () => {
    const suitable = allLockedGrid();
    for (let row = 0; row < 4; row += 1) {
      for (let col = 4; col < 10; col += 1) suitable[row][col] = 0;
    }
    expect(inventoryCowSuitable(suitable)).toBe(true);
    expect(inventoryCowSuitable(allLockedGrid())).toBe(false);
  });
});
