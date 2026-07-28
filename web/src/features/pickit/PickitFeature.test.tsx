import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { PickitFeature } from "./PickitFeature";

const mocks = vi.hoisted(() => ({
  catalog: vi.fn(), profiles: vi.fn(), assignments: vi.fn(), validate: vi.fn(), importRules: vi.fn(),
  create: vi.fn(), update: vi.fn(), duplicate: vi.fn(), remove: vi.fn(), assign: vi.fn(),
}));
vi.mock("../../api/generated", () => ({ getPickitCatalog: mocks.catalog, getPickitProfiles: mocks.profiles, getPickitAssignments: mocks.assignments, validatePickitProfile: mocks.validate, importPickit: mocks.importRules }));
vi.mock("../../api/client", () => ({ createPickitProfile: mocks.create, updatePickitProfile: mocks.update, duplicatePickitProfile: mocks.duplicate, deletePickitProfile: mocks.remove, updatePickitAssignment: mocks.assign }));

const baseProfile = { schema_version: 1, revision: 1, id: "base", name: "Basis", rules: [
  { id: "erste", action: "keep", expression: `[type] == "rune"` },
  { id: "zweite", action: "sell", expression: `[quality] == "rare"` },
] };
const talItems = ["Adjudication", "Guardianship", "Fine-Spun Cloth", "Horadric Crest", "Lidless Eye"].map((suffix, index) => ({ kind: "set", raw_id: 10 + index, key: `Tal Rasha's ${suffix}`, display_name: `Tal Rasha's ${suffix}`, base_code: `t${index}`, set_key: "Tal Rasha's Wrappings", set_name: "Tal Rasha's Wrappings", spawnable: true }));

describe("PickitFeature", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.catalog.mockResolvedValue({ schema_version: 1, catalog_version: "3.2", bases: [{ txt_file_no: 255, code: "7s8", name: "Thresher", type: "pole", base_tier: "elite" }], identities: talItems, actions: ["keep", "sell", "ignore"], qualities: [], speed_categories: [] });
    mocks.profiles.mockResolvedValue({ profiles: [baseProfile], assignment_revision: 1 });
    mocks.assignments.mockResolvedValue({ schema_version: 1, revision: 1, assignments: { MrBones: { countess: ["base"], summoner: ["base"], nihlathak: ["base"] } } });
    mocks.validate.mockImplementation(async ({ profile }) => ({ valid: true, profile }));
    mocks.update.mockImplementation(async (_id, request) => ({ ...request.profile, revision: request.profile.revision + 1 }));
    mocks.duplicate.mockResolvedValue({ ...baseProfile, id: "base-kopie", name: "Basis Kopie" });
    mocks.remove.mockResolvedValue(undefined); mocks.assign.mockResolvedValue({ schema_version: 1, revision: 2, assignments: { MrBones: { countess: ["base"] } } });
  });
  afterEach(() => { cleanup(); vi.restoreAllMocks(); });

  it("expandiert Tal Rasha sichtbar in fünf Einzelregeln", async () => {
    renderFeature();
    const search = await screen.findByPlaceholderText("z. B. Tal Rasha oder Thresher");
    fireEvent.change(search, { target: { value: "Tal Rasha" } });
    fireEvent.click(screen.getByRole("button", { name: /Ganzes Set Tal Rasha's Wrappings hinzufügen \(5\)/ }));
    expect(screen.getByRole("status")).toHaveTextContent("5 Einzelregeln");
    expect(screen.getAllByText(/\[setitem\]/)).toHaveLength(5);
  });

  it("erzeugt für ätherischen Thresher genau eine kombinierte Regel und ordnet Regeln um", async () => {
    renderFeature(); await screen.findByRole("heading", { name: "Basis" });
    fireEvent.change(screen.getByPlaceholderText("z. B. Tal Rasha oder Thresher"), { target: { value: "Thresher" } });
    fireEvent.click(screen.getByLabelText("Nur ätherisch")); fireEvent.click(screen.getByRole("button", { name: /Thresher.*7s8/ }));
    expect(screen.getByText(`[name] == "7s8" && [ethereal] == true`)).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Entscheidungsvorschau" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Regel 2 nach oben" }));
    const rules = screen.getAllByRole("listitem").filter((entry) => entry.closest(".rule-list"));
    expect(rules[0]).toHaveTextContent("rare");
  });

  it("dupliziert Profile und zeigt den Delete-Block des Core", async () => {
    const prompt = vi.spyOn(window, "prompt").mockReturnValueOnce("base-kopie").mockReturnValueOnce("Basis Kopie");
    vi.spyOn(window, "confirm").mockReturnValue(true); mocks.remove.mockRejectedValueOnce(new Error("Profil ist noch zugeordnet."));
    renderFeature(); await screen.findByRole("heading", { name: "Basis" });
    fireEvent.click(screen.getByRole("button", { name: "Duplizieren" }));
    await waitFor(() => expect(mocks.duplicate).toHaveBeenCalledWith("base", { target_id: "base-kopie", target_name: "Basis Kopie" }));
    expect(prompt).toHaveBeenCalledTimes(2);
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("zugeordnet");
  });

  it("zeigt Importfehler, stale Save und leere Assignment-Validierung verständlich", async () => {
    mocks.importRules.mockRejectedValueOnce(new Error("Import Zeile 2 ist ungültig"));
    renderFeature(); await screen.findByRole("heading", { name: "Basis" });
    fireEvent.click(screen.getByRole("button", { name: "Erweitertes Ausdrucksfeld" }));
    fireEvent.change(screen.getByLabelText("NIP-Text"), { target: { value: "[kaputt]" } }); fireEvent.click(screen.getByRole("button", { name: "Als Entwurf importieren" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Zeile 2");
    mocks.update.mockRejectedValueOnce(new Error("revision conflict: aktueller Stand ist Revision 2"));
    fireEvent.change(screen.getByLabelText("Profilname"), { target: { value: "Geändert" } }); fireEvent.click(screen.getByRole("button", { name: "Profil speichern" }));
    expect(await screen.findByRole("button", { name: "Aktuellen Stand laden" })).toBeInTheDocument();
    const removeButtons = screen.getAllByRole("button", { name: "Entfernen" });
    fireEvent.click(removeButtons[removeButtons.length - 1]);
    fireEvent.click(screen.getByRole("button", { name: "Zuordnung speichern" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Mindestens ein Profil");
  });

  it("bietet Pickit-Zuordnungen für alle Katalog-Runs an", async () => {
    renderFeature();
    const runSelect = await screen.findByRole("combobox", { name: "Run" });
    expect(runSelect).toHaveTextContent("summoner");
    expect(runSelect).toHaveTextContent("nihlathak");
  });
});

function renderFeature() { return render(<PickitFeature characters={["MrBones"]} selectedCharacter="MrBones" runs={["countess", "mephisto", "nihlathak", "summoner"]} locked={false} refreshKey={0} />); }
