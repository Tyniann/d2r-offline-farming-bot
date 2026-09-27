import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { CatalogDTO, StatusDTO } from "../../api/generated";
import { CharacterUnlock } from "./CharacterUnlock";

const reload = vi.hoisted(() => vi.fn());
vi.mock("../../api/generated", () => ({ reloadCharacters: reload }));
vi.mock("./CharacterSetupWizard", () => ({ CharacterSetupWizard: ({ character, onBusyChange }: { character: string; onBusyChange(busy: boolean): void }) =>
  <div>Setup für {character}<button onClick={() => onBusyChange(true)}>Speichern simulieren</button><button onClick={() => onBusyChange(false)}>Speichern fertig</button></div> }));

const catalog: CatalogDTO = { schema_version: 1, revision: 1, default_difficulty: "hell", difficulties: [{ id: "hell" }], runs: [], profiles: [{ id: "sorceress_blizzard", character_class: "sorceress" }], characters: [
  { name: "Hammer", slug: "hammer", selectable: true, farm_ready: false, farm_ready_reasons: ["profile_bindings_missing"] },
  { name: "Frost", slug: "frost", selectable: false, farm_ready: false, expected_class: "sorceress", reasons: ["character_profile_missing", "character_anchor_missing"] },
  { name: "Amazon", slug: "amazon", selectable: false, farm_ready: false, expected_class: "amazon", reasons: ["character_class_unsupported"] },
] };
// Dieser Dialog reicht Status unverändert an den separat getesteten Wizard weiter.
const status = { generation: 1 } as StatusDTO;

describe("CharacterUnlock", () => {
  afterEach(cleanup);
  beforeEach(() => { vi.clearAllMocks(); reload.mockResolvedValue({}); });

  it("öffnet erst per Button und behält den Wizard nach der Freischaltung", () => {
    const onConfigure = vi.fn();
    const props = { catalog, status, lockedReason: "", onChanged: vi.fn(), onConfigure };
    const { rerender } = render(<CharacterUnlock {...props} />);
    expect(screen.queryByRole("option", { name: "Hammer" })).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Einrichtung starten" }));
    expect(screen.getByText("Setup für Frost")).toBeInTheDocument();
    const updated = { ...catalog, characters: catalog.characters.map((entry) => entry.name === "Frost" ? { ...entry, selectable: true, reasons: [] } : entry) };
    rerender(<CharacterUnlock {...props} catalog={updated} />);
    expect(screen.getByText("Setup für Frost")).toBeInTheDocument();
    expect(onConfigure).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Charaktereinstellungen öffnen" }));
    expect(onConfigure).toHaveBeenCalledWith("Frost");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("setzt eine begonnene Einrichtung fort und sperrt nicht unterstützte Klassen", () => {
    const partial = { ...catalog, characters: catalog.characters.map((entry) => entry.name === "Frost" ? { ...entry, reasons: ["character_anchor_missing"] } : entry) };
    render(<CharacterUnlock catalog={partial} status={status} lockedReason="" onChanged={vi.fn()} onConfigure={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Einrichtung fortsetzen" })).toBeEnabled();
    fireEvent.change(screen.getByLabelText("Neuer Charakter"), { target: { value: "Amazon" } });
    expect(screen.getByRole("button", { name: "Einrichtung starten" })).toBeDisabled();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it.each(["Beende zuerst den Run.", "Speichere zuerst deine Änderungen."])("beachtet die Sperre: %s", (lockedReason) => {
    render(<CharacterUnlock catalog={catalog} status={status} lockedReason={lockedReason} onChanged={vi.fn()} onConfigure={vi.fn()} />);
    expect(screen.getByLabelText("Neuer Charakter")).toBeDisabled();
    expect(screen.getByRole("button", { name: "Einrichtung starten" })).toBeDisabled();
    expect(screen.getByText(lockedReason)).toBeInTheDocument();
  });

  it("bietet ohne neue Charaktere nur die erneute Suche an", async () => {
    const changed = vi.fn();
    render(<CharacterUnlock catalog={{ ...catalog, characters: [catalog.characters[0]] }} status={status} lockedReason="" onChanged={changed} onConfigure={vi.fn()} />);
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Charaktere neu suchen" }));
    await waitFor(() => expect(changed).toHaveBeenCalledOnce());
    expect(reload).toHaveBeenCalledOnce();
  });

  it("schließt nicht während einer laufenden Speicheraktion", () => {
    render(<CharacterUnlock catalog={catalog} status={status} lockedReason="" onChanged={vi.fn()} onConfigure={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Einrichtung starten" }));
    fireEvent.click(screen.getByText("Speichern simulieren"));
    fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Speichern fertig"));
    fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
