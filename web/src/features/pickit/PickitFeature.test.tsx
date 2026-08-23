import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { PickitFeature } from "./PickitFeature";
import { apiError } from "../../test/apiError";
import { i18n } from "../../i18n";

const mocks = vi.hoisted(() => ({
  catalog: vi.fn(), profiles: vi.fn(), assignments: vi.fn(), validate: vi.fn(), importRules: vi.fn(),
  create: vi.fn(), update: vi.fn(), duplicate: vi.fn(), remove: vi.fn(), assign: vi.fn(),
}));

vi.mock("../../api/generated", () => ({
  getPickitCatalog: mocks.catalog,
  getPickitProfiles: mocks.profiles,
  getPickitAssignments: mocks.assignments,
  validatePickitProfile: mocks.validate,
  importPickit: mocks.importRules,
}));
vi.mock("../../api/client", () => ({
  createPickitProfile: mocks.create,
  updatePickitProfile: mocks.update,
  duplicatePickitProfile: mocks.duplicate,
  deletePickitProfile: mocks.remove,
  updatePickitAssignment: mocks.assign,
}));

const baseProfile = {
  schema_version: 1,
  revision: 1,
  id: "base",
  name: "Basis",
  rules: [
    { id: "erste", action: "keep", expression: `[type] == "rune"`, summary: { kind: "runes", params: {} } },
    { id: "zweite", action: "sell", expression: `[quality] == "rare"`, summary: { kind: "quality", params: { qualities: ["rare"] } } },
  ],
};
const secondProfile = {
  ...baseProfile,
  id: "zweites",
  name: "Zweites",
  rules: [{ ...baseProfile.rules[0], summary: { kind: "item_types", params: { types: ["shie", "ashd"] } } }],
};
const thirdProfile = { ...baseProfile, id: "drittes", name: "Drittes", rules: [baseProfile.rules[1]] };
const talItems = ["Adjudication", "Guardianship", "Fine-Spun Cloth", "Horadric Crest", "Lidless Eye"].map((suffix, index) => ({
  kind: "set",
  raw_id: 10 + index,
  key: `Tal Rasha's ${suffix}`,
  display_name: `Tal Rasha's ${suffix}`,
  base_code: `t${index}`,
  set_key: "Tal Rasha's Wrappings",
  set_name: "Tal Rasha's Wrappings",
  spawnable: true,
}));

describe("PickitFeature", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.catalog.mockResolvedValue({
      schema_version: 1,
      catalog_version: "3.2",
      bases: [{ txt_file_no: 255, code: "7s8", name: "Thresher", type: "pole", base_tier: "elite" }],
      identities: talItems,
      actions: ["keep", "sell", "ignore"],
      qualities: [],
      speed_categories: [],
    });
    mocks.profiles.mockResolvedValue({ profiles: [baseProfile], assignment_revision: 1 });
    mocks.assignments.mockResolvedValue({ schema_version: 1, revision: 1, assignments: { MrBones: { countess: ["base"], summoner: ["base"], nihlathak: ["base"] } } });
    mocks.validate.mockImplementation(async ({ profile }) => ({ valid: true, profile }));
    mocks.importRules.mockResolvedValue({ rules: [{ id: "importiert", action: "keep", expression: `[name] == "rin"`, summary: { kind: "item_codes", params: { codes: ["rin"] } } }], warnings: [] });
    mocks.create.mockImplementation(async ({ profile }) => ({ ...profile, revision: 1 }));
    mocks.update.mockImplementation(async (_id, request) => ({ ...request.profile, revision: request.profile.revision + 1 }));
    mocks.duplicate.mockResolvedValue({ ...baseProfile, id: "basis-kopie", name: "Basis Kopie" });
    mocks.remove.mockResolvedValue({ assignments: { schema_version: 1, revision: 2, assignments: {} }, removed: [] });
    mocks.assign.mockResolvedValue({ schema_version: 1, revision: 2, assignments: { MrBones: { countess: ["base"] } } });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("lädt Profile, sprachneutrale Summaries und Zuordnungen aus den APIs", async () => {
    await renderLoaded();
    expect(mocks.catalog).toHaveBeenCalledTimes(1);
    expect(mocks.profiles).toHaveBeenCalledTimes(1);
    expect(mocks.assignments).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Alle Runen")).toBeInTheDocument();
    expect(screen.getByText("Qualität: Selten")).toBeInTheDocument();
    expect(screen.getAllByText("Bei 3 Routen verwendet")).not.toHaveLength(0);
  });

  it("expandiert ein gefundenes Set in sichtbare Einzelregeln", async () => {
    await renderLoaded();
    openBuilder();
    fireEvent.change(screen.getByPlaceholderText("z. B. Tal Rasha oder Thresher"), { target: { value: "Tal Rasha" } });
    fireEvent.click(screen.getByRole("button", { name: /Ganzes Set Tal Rashas Hüllen hinzufügen \(5\)/ }));
    expect(screen.getByRole("status")).toHaveTextContent("5 Einzelregeln");
    openExpressionEditor();
    expect(screen.getAllByText(/\[setitem\]/)).toHaveLength(5);
  });

  it("behält die durchsuchbare Itemtyp-Mehrfachauswahl und erzeugt eine kombinierte Regel", async () => {
    await renderLoaded();
    openBuilder();
    const picker = screen.getByRole("button", { name: "Itemtypen 1 ausgewählt" });
    fireEvent.click(picker);
    const search = screen.getByRole("textbox", { name: "Typen durchsuchen" });
    fireEvent.change(search, { target: { value: "Paladin" } });
    expect(screen.getByText("1 Treffer · 1 ausgewählt")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Schilde – Paladin"));
    fireEvent.click(screen.getByRole("button", { name: "Auswahl schließen" }));
    fireEvent.click(screen.getByRole("button", { name: "Regel hinzufügen" }));
    expect(screen.getByText("Sockelfilter: 4")).toBeInTheDocument();
    openExpressionEditor();
    expect(screen.getByText(`([type] == "shie" || [type] == "ashd") && [tier] == "elite" && [sockets] == 4`)).toBeInTheDocument();
  });

  it("macht das letzte Entfernen rückgängig und schützt einen Dirty-Profilwechsel", async () => {
    mocks.profiles.mockResolvedValue({ profiles: [baseProfile, secondProfile], assignment_revision: 1 });
    await renderLoaded();
    fireEvent.click(screen.getByRole("button", { name: "Alle Runen entfernen" }));
    expect(screen.queryByText("Alle Runen")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Rückgängig" }));
    expect(screen.getByText("Alle Runen")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Profilname"), { target: { value: "Geändert" } });
    fireEvent.click(screen.getByRole("button", { name: /Zweites1 Regel/ }));
    const discardDialog = screen.getByRole("dialog", { name: "Ungespeicherte Änderungen verwerfen?" });
    fireEvent.click(within(discardDialog).getByRole("button", { name: "Verwerfen" }));
    expect(await screen.findByDisplayValue("Zweites")).toBeInTheDocument();
    expect(screen.getByText("Schilde, Schilde – Paladin")).toBeInTheDocument();
  });

  it("legt ein Profil an, speichert es und dupliziert das gespeicherte Profil", async () => {
    mocks.duplicate.mockImplementation(async (_id, request) => ({ ...baseProfile, id: request.target_id, name: request.target_name }));
    await renderLoaded();
    fireEvent.click(screen.getByRole("button", { name: "Profil anlegen" }));
    const createDialog = screen.getByRole("dialog", { name: "Neues Profil" });
    fireEvent.change(within(createDialog).getByLabelText("Profilname"), { target: { value: "Mein Profil" } });
    fireEvent.click(within(createDialog).getByRole("button", { name: "Profil anlegen" }));
    openQuickRule("Alle Runen");
    fireEvent.click(screen.getByRole("button", { name: "Profil speichern" }));
    await waitFor(() => expect(mocks.create).toHaveBeenCalledWith({ profile: expect.objectContaining({ id: "mein-profil", name: "Mein Profil" }) }));
    fireEvent.click(screen.getByRole("button", { name: "Duplizieren" }));
    const duplicateDialog = screen.getByRole("dialog", { name: "Profil duplizieren" });
    fireEvent.change(within(duplicateDialog).getByLabelText("Profilname"), { target: { value: "Mein Profil Kopie" } });
    fireEvent.click(within(duplicateDialog).getByRole("button", { name: "Kopie anlegen" }));
    await waitFor(() => expect(mocks.duplicate).toHaveBeenCalledWith("mein-profil", { target_id: "mein-profil-kopie", target_name: "Mein Profil Kopie" }));
    expect(await screen.findByDisplayValue("Mein Profil Kopie")).toBeInTheDocument();
  });

  it("speichert und verwirft Profiländerungen gegen den letzten Serverstand", async () => {
    await renderLoaded();
    fireEvent.change(screen.getByLabelText("Profilname"), { target: { value: "Gespeichert" } });
    fireEvent.click(screen.getByRole("button", { name: "Profil speichern" }));
    await waitFor(() => expect(mocks.update).toHaveBeenCalledWith("base", expect.objectContaining({ expected_revision: 1 })));
    expect(await screen.findByDisplayValue("Gespeichert")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Profilname"), { target: { value: "Nicht behalten" } });
    fireEvent.click(screen.getByRole("button", { name: "Änderungen verwerfen" }));
    expect(screen.getByDisplayValue("Gespeichert")).toBeInTheDocument();
  });

  it("bearbeitet manuellen Pickit-Text, validiert und importiert Regeln", async () => {
    await renderLoaded();
    openExpressionEditor();
    fireEvent.change(screen.getByRole("textbox", { name: "Pickit-Text für Regel 1: Alle Runen" }), { target: { value: `[type] == "gem"` } });
    expect(screen.getByText("Manuell bearbeitete Regel")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Entwurf prüfen" }));
    await waitFor(() => expect(mocks.validate).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: "Regeln importieren" }));
    fireEvent.change(screen.getByLabelText("NIP-Text"), { target: { value: `[name] == "rin"` } });
    fireEvent.click(screen.getByRole("button", { name: "Als Entwurf importieren" }));
    await waitFor(() => expect(mocks.importRules).toHaveBeenCalledWith({ text: `[name] == "rin"`, action: "keep" }));
  });

  it("ordnet Profile pro Route und speichert die Reihenfolge", async () => {
    mocks.profiles.mockResolvedValue({ profiles: [baseProfile, secondProfile, thirdProfile], assignment_revision: 1 });
    mocks.assignments.mockResolvedValue({ schema_version: 1, revision: 1, assignments: { MrBones: { countess: ["base", "zweites"] } } });
    await renderLoaded();
    fireEvent.click(screen.getByRole("tab", { name: "Zuordnung" }));
    const chain = document.querySelectorAll(".pickit-assignment-chain li");
    fireEvent.dragStart(chain[1]);
    fireEvent.dragOver(chain[0]);
    fireEvent.drop(chain[0]);
    fireEvent.click(screen.getByRole("button", { name: "Zuordnung speichern" }));
    await waitFor(() => expect(mocks.assign).toHaveBeenCalledWith({ character: "MrBones", run_id: "countess", profile_ids: ["zweites", "base"], expected_revision: 1 }));
    expect(screen.getByRole("tab", { name: "Beschwörer" })).toBeInTheDocument();
    expect(screen.queryByText(/Run 1 von/)).not.toBeInTheDocument();
  });

  it("löscht ein verwendetes Profil erst nach Bestätigung samt allen Zuordnungen", async () => {
    mocks.remove.mockResolvedValue({
      assignments: { schema_version: 1, revision: 2, assignments: {} },
      removed: [
        { character: "MrBones", route_id: "countess" },
        { character: "MrBones", route_id: "summoner" },
        { character: "MrBones", route_id: "nihlathak" },
      ],
    });
    await renderLoaded();
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
    expect(screen.getByRole("dialog", { name: "Basis löschen?" })).toHaveTextContent("3 Zuordnungen werden entfernt");
    fireEvent.click(screen.getByRole("button", { name: "Profil und Zuordnungen löschen" }));
    await waitFor(() => expect(mocks.remove).toHaveBeenCalledWith("base", { expected_revision: 1, expected_assignment_revision: 1, remove_assignments: true }));
    expect(await screen.findByText("Profil auswählen oder neu anlegen.")).toBeInTheDocument();
  });

  it("löscht ein unzugeordnetes Profil ohne Dialog und bietet bei Revisionkonflikten Neuladen an", async () => {
    mocks.assignments.mockResolvedValue({ schema_version: 1, revision: 4, assignments: {} });
    mocks.remove.mockRejectedValueOnce(apiError("revision_conflict"));
    await renderLoaded();
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
    expect(screen.queryByRole("dialog", { name: "Basis löschen?" })).not.toBeInTheDocument();
    await waitFor(() => expect(mocks.remove).toHaveBeenCalledWith("base", { expected_revision: 1, expected_assignment_revision: 4, remove_assignments: false }));
    expect(await screen.findByRole("button", { name: "Aktuellen Stand laden" })).toBeInTheDocument();
  });

  it("bedient beide Tab-Leisten mit Pfeiltasten und verknüpft Tabs mit ihren Bereichen", async () => {
    await renderLoaded();
    const profilesTab = screen.getByRole("tab", { name: "Profile" });
    profilesTab.focus();
    fireEvent.keyDown(profilesTab, { key: "ArrowRight" });
    const assignmentsTab = screen.getByRole("tab", { name: "Zuordnung" });
    expect(assignmentsTab).toHaveFocus();
    expect(assignmentsTab).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tabpanel", { name: "Zuordnung" })).toHaveAttribute("id", assignmentsTab.getAttribute("aria-controls"));

    const countessTab = screen.getByRole("tab", { name: "Gräfin" });
    countessTab.focus();
    fireEvent.keyDown(countessTab, { key: "End" });
    expect(screen.getByRole("tab", { name: "Beschwörer" })).toHaveFocus();
    expect(screen.getByRole("tabpanel", { name: "Beschwörer" })).toBeInTheDocument();
  });

  it("schließt aufklappbare Werkzeuge per Tastatur und gibt den Fokus zurück", async () => {
    await renderLoaded();
    openBuilder();
    const typePicker = screen.getByRole("button", { name: "Itemtypen 1 ausgewählt" });
    fireEvent.click(typePicker);
    const typeSearch = screen.getByRole("textbox", { name: "Typen durchsuchen" });
    fireEvent.keyDown(typeSearch, { key: "Escape" });
    expect(typePicker).toHaveFocus();
    expect(typePicker).toHaveAttribute("aria-expanded", "false");

    const closeBuilderButton = screen.getAllByRole("button", { name: "Regel-Builder schließen" })
      .find((button) => button.classList.contains("pickit-builder-close"));
    expect(closeBuilderButton).toBeDefined();
    fireEvent.click(closeBuilderButton!);
    const builderToggle = screen.getByRole("button", { name: "Regel hinzufügen" });
    await waitFor(() => expect(builderToggle).toHaveFocus());
    expect(builderToggle).toHaveAttribute("aria-expanded", "false");

    const usageButton = screen.getByRole("button", { name: "Bei 3 Routen verwendet" });
    fireEvent.click(usageButton);
    fireEvent.keyDown(usageButton, { key: "Escape" });
    expect(usageButton).toHaveFocus();
    expect(usageButton).toHaveAttribute("aria-expanded", "false");
  });

  it("ordnet zugewiesene Profile ohne Maus neu", async () => {
    mocks.profiles.mockResolvedValue({ profiles: [baseProfile, secondProfile], assignment_revision: 1 });
    mocks.assignments.mockResolvedValue({ schema_version: 1, revision: 1, assignments: { MrBones: { countess: ["base", "zweites"] } } });
    await renderLoaded();
    fireEvent.click(screen.getByRole("tab", { name: "Zuordnung" }));
    const firstProfile = screen.getByRole("listitem", { name: "Basis, Position 1 von 2" });
    firstProfile.focus();
    fireEvent.keyDown(firstProfile, { key: "ArrowDown", altKey: true });
    expect(await screen.findByRole("listitem", { name: "Basis, Position 2 von 2" })).toHaveFocus();
    fireEvent.click(screen.getByRole("button", { name: "Zuordnung speichern" }));
    await waitFor(() => expect(mocks.assign).toHaveBeenCalledWith(expect.objectContaining({ profile_ids: ["zweites", "base"] })));
  });

  it("verwendet auf Englisch dieselbe semantische Prototypstruktur", async () => {
    await i18n.changeLanguage("en");
    await renderLoaded();
    expect(screen.getByRole("tab", { name: "Profiles" })).toHaveAttribute("aria-controls");
    expect(screen.getByRole("tabpanel", { name: "Profiles" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Add rule" }));
    expect(screen.getByRole("heading", { name: "What are you looking for?" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Item types 1 selected" })).toHaveAttribute("aria-controls", "equipment-type-options");
  });
});

async function renderLoaded() {
  const rendered = renderFeature();
  await screen.findByDisplayValue("Basis");
  return rendered;
}

function renderFeature() {
  return render(<PickitFeature characters={["MrBones"]} selectedCharacter="MrBones" runs={["countess", "mephisto", "nihlathak", "summoner"]} locked={false} refreshKey={0} />);
}

function openBuilder() {
  fireEvent.click(screen.getByRole("button", { name: "Regel hinzufügen" }));
}

function openQuickRule(name: string) {
  openBuilder();
  fireEvent.click(screen.getByRole("button", { name }));
}

function openExpressionEditor() {
  fireEvent.click(screen.getByRole("button", { name: "Erweitert" }));
  fireEvent.click(screen.getByText("Pickit-Text bearbeiten"));
}
