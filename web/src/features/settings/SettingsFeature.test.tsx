import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { CatalogDTO, OperatorSettingsChangeDTO, OperatorSettingsDTO } from "../../api/generated";
import { changeAppLanguage } from "../../i18n";
import { apiError } from "../../test/apiError";
import { SettingsFeature } from "./SettingsFeature";

const mocks = vi.hoisted(() => ({
  get: vi.fn(), preview: vi.fn(), previewReset: vi.fn(), save: vi.fn(), restore: vi.fn(),
  previewDelete: vi.fn(), confirmDelete: vi.fn(),
  createDiagnostic: vi.fn(),
  previewCharacterSetup: vi.fn(),
  getRunAvailabilities: vi.fn(),
  getDesktop: vi.fn(), updateDesktop: vi.fn(), restartCore: vi.fn(),
  getUpdate: vi.fn(), checkUpdate: vi.fn(), openRelease: vi.fn(), revealDiagnostic: vi.fn(),
}));

vi.mock("../../api/generated", () => ({
  getOperatorSettings: mocks.get,
  previewOperatorSettings: mocks.preview,
  previewResetOperatorSettings: mocks.previewReset,
  previewCharacterSetup: mocks.previewCharacterSetup,
  getRunAvailabilities: mocks.getRunAvailabilities,
  reloadCharacters: vi.fn(),
}));
vi.mock("../../api/client", () => ({
  saveOperatorSettings: mocks.save, restoreOperatorSettings: mocks.restore,
  previewHistoryDeleteAll: mocks.previewDelete, confirmHistoryDeleteAll: mocks.confirmDelete,
  createDiagnosticBundle: mocks.createDiagnostic,
  confirmCharacterSetup: vi.fn(), captureCharacterSelection: vi.fn(),
}));

const operator: OperatorSettingsDTO = {
  schema_version: 3,
  revision: 4,
  characters: {
    mrbones: {
      character_class: "necromancer", combat_profile: "necro_bone_spear", last_difficulty: "nightmare", queue: ["countess", "mephisto"],
      profile_bindings: {
        necro_bone_spear: {
          skills: { teleport: "f7", town_portal: "f6", amplify_damage: "f1", corpse_explosion: "f2", bone_prison: "f3", bone_armor: "f5", bone_spear: "f8" },
          belt: { slot_1: "1", slot_2: "2", slot_3: "3", slot_4: "4" },
        },
      },
    },
  },
  budgets: { max_runs: 20, max_duration_ms: 7_200_000, max_consecutive_failures: 3, max_total_restarts: 4 },
  input: { enabled: false, pause_hotkey: "pause", stop_after_run_hotkey: "f10", recording_finish_hotkey: "f9", emergency_stop_hotkey: "f11" },
  history: { retention_enabled: true, retention_days: 60 },
};

const catalog: CatalogDTO = {
  schema_version: 1,
  revision: 1,
  default_difficulty: "nightmare",
  characters: [
    { name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true },
    { name: "Holyhammer", slug: "holyhammer", selectable: true, farm_ready: false, farm_ready_reasons: ["profile_bindings_incomplete"] },
    { name: "Frostia", slug: "frostia", selectable: false, farm_ready: false, expected_class: "sorceress", reasons: ["character_class_unsupported"] },
  ],
  difficulties: [],
  profiles: [{ id: "necro_bone_spear", character_class: "necromancer" }, { id: "paladin_hammerdin", character_class: "paladin" }],
  runs: [],
};

describe("SettingsFeature", () => {
  beforeEach(async () => {
    await changeAppLanguage("de");
    vi.clearAllMocks();
    mocks.get.mockResolvedValue(operator);
    mocks.getRunAvailabilities.mockResolvedValue({
      schema_version: 1, character: "MrBones", difficulty: "nightmare",
      runs: [
        { run_id: "countess", display_name: "Countess", status: "available" },
        { run_id: "mephisto", display_name: "Mephisto", status: "available" },
        { run_id: "nihlathak", display_name: "Nihlathak", status: "runtime_validation_required" },
        { run_id: "summoner", display_name: "Summoner", status: "available" },
      ],
    });
    mocks.previewCharacterSetup.mockResolvedValue({
      schema_version: 1,
      catalog_revision: 1,
      operator_settings_revision: 4,
      pickit_assignment_revision: 1,
      character: { name: "MrBones", slug: "mrbones", character_class: "necromancer", class_display_name: "Totenbeschwörer" },
      supported: true,
      profiles: [{
        id: "necro_bone_spear", display_name: "Knochen-Speer", is_default: true, is_selected: true, standard_attack: "bone_spear", requires_mercenary: false, bindings_ready: true,
        required_skills: [
          { skill: "teleport", skill_id: 54, display_name: "Teleport", slot: "right" },
          { skill: "town_portal", skill_id: 359, display_name: "Stadtportal", slot: "right" },
          { skill: "bone_spear", skill_id: 84, display_name: "Knochen-Speer", slot: "right" },
          { skill: "amplify_damage", skill_id: 66, display_name: "Verstärkter Schaden", slot: "right" },
          { skill: "corpse_explosion", skill_id: 74, display_name: "Kadaverexplosion", slot: "right" },
          { skill: "bone_armor", skill_id: 68, display_name: "Knochenrüstung", slot: "right" },
          { skill: "bone_prison", skill_id: 88, display_name: "Knochengefängnis", slot: "right" },
        ],
        supported_runs: ["countess"],
        default_belt_layout: { slot_1: "healing", slot_2: "mana", slot_3: "mana", slot_4: "rejuvenation" },
        belt_layout: { slot_1: "healing", slot_2: "mana", slot_3: "mana", slot_4: "rejuvenation" },
        default_healing_restock: 2,
        default_mana_restock: 4,
      }],
      selected_profile_id: "necro_bone_spear",
      default_profile_id: "necro_bone_spear",
      pickit_defaults: [],
      anchor_state: "ready",
      setup_state: "ready",
      reasons: [],
    });
    mocks.getDesktop.mockResolvedValue({ schema_version: 1, autostart: false, onboarding_completed: false });
    mocks.updateDesktop.mockImplementation(async (value) => ({ schema_version: 1, ...value }));
    mocks.getUpdate.mockResolvedValue({ status: "up_to_date", current_version: "1.2.3", latest_version: "1.2.3" });
    mocks.checkUpdate.mockResolvedValue({ status: "available", current_version: "1.2.3", latest_version: "1.3.0" });
    window.d2rDesktop = {
      getProvisioningState: vi.fn().mockResolvedValue({ required: false, import_selected: false, import_label: "" }), chooseImportRoot: vi.fn(), provision: vi.fn(),
      getAppInfo: vi.fn(), getDesktopSettings: mocks.getDesktop, updateDesktopSettings: mocks.updateDesktop,
      showWindow: vi.fn(), restartCore: mocks.restartCore, restartAsAdministrator: vi.fn(), onNavigate: vi.fn(() => vi.fn()),
      getUpdateStatus: mocks.getUpdate, checkForUpdates: mocks.checkUpdate, openReleasePage: mocks.openRelease,
      revealDiagnosticBundle: mocks.revealDiagnostic, onUpdateStatus: vi.fn(() => vi.fn()),
    };
  });
  afterEach(() => { cleanup(); delete window.d2rDesktop; });

  it("rendert die Übersicht mit drei Flächen auf Englisch", async () => {
    await changeAppLanguage("en");
    render(<SettingsFeature generation={12} coreState="idle" characters={["mrbones"]} runs={defaultRuns()} events={[]} />);

    expect(await screen.findByRole("group", { name: "Character list" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Bot" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "System" })).toBeInTheDocument();
    for (const name of [/Route order/, /Key bindings/, /Inventory protection/, /Maintenance/]) {
      expect(screen.getByRole("button", { name })).toBeInTheDocument();
    }
  });

  it("speichert Änderungen über Speichern und meldet Dirty-State", async () => {
    const changed = change({ ...operator, revision: 5, input: { ...operator.input, pause_hotkey: "f8" } }, ["input.pause_hotkey"], true);
    const onSettingsApplied = vi.fn();
    const onDirtyChange = vi.fn();
    mocks.preview.mockResolvedValue(changed);
    mocks.save.mockImplementation(async () => ({
      schema_version: 1,
      generation: 12,
      settings: { ...operator, revision: 5, input: { ...operator.input, pause_hotkey: "f8" } },
      changed_fields: ["input.pause_hotkey"],
      restart_required: true,
      reason_code: "config_restart_required",
    }));
    render(<SettingsFeature generation={12} coreState="idle" characters={["mrbones"]} runs={defaultRuns()} events={[]} onSettingsApplied={onSettingsApplied} onDirtyChange={onDirtyChange} />);

    fireEvent.change(await screen.findByLabelText("Pause"), { target: { value: "f8" } });
    await waitFor(() => expect(onDirtyChange).toHaveBeenCalledWith(true));
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(await screen.findByRole("dialog", { name: "Änderungen speichern?" })).toHaveTextContent("Pause-Hotkey");
    expect(mocks.preview).toHaveBeenCalledWith(expect.objectContaining({ expected_revision: 4, expected_generation: 12, settings: expect.objectContaining({ revision: 4 }) }));
    expect(mocks.preview.mock.calls[0][0].settings.characters.mrbones).toMatchObject({
      character_class: "necromancer",
      combat_profile: "necro_bone_spear",
    });
    fireEvent.click(screen.getByRole("button", { name: "Jetzt speichern" }));
    await waitFor(() => expect(mocks.save).toHaveBeenCalledWith(expect.objectContaining({ expected_revision: 4, expected_generation: 12 })));
    expect(onSettingsApplied).toHaveBeenCalledOnce();
    const saved = await mocks.save.mock.results.at(-1)?.value;
    expect(saved?.restart_required).toBe(true);
    await waitFor(() => expect(screen.getByText("Core-Neustart erforderlich")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Core kontrolliert neu starten" }));
    await waitFor(() => expect(mocks.restartCore).toHaveBeenCalledOnce());
  });

  it("speichert Autostart sofort ohne eigenen Speichern-Button", async () => {
    renderFeature();
    fireEvent.click(await screen.findByLabelText("App mit Windows starten"));
    await waitFor(() => expect(mocks.updateDesktop).toHaveBeenCalledWith({ autostart: true, onboarding_completed: false }));
    expect(await screen.findByText("Gespeichert ✓")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Speichern" })).not.toBeInTheDocument();
  });

  it("zeigt Resetvorschau aus dem Wartungs-Panel und einen stale Revision-Konflikt persistent", async () => {
    mocks.previewReset.mockResolvedValue(change({ ...operator, revision: 5 }, ["budgets.max_runs"], false));
    mocks.restore.mockRejectedValue(apiError("config_revision_conflict"));
    renderFeature();
    await openPanel(/Wartung/);
    fireEvent.click(screen.getByRole("button", { name: "Auf sichere Standardwerte zurücksetzen" }));
    expect(await screen.findByRole("dialog", { name: "Sichere Standardwerte anwenden?" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Standardwerte anwenden" }));
    expect(await screen.findByText("Revision ist veraltet")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Aktuellen Stand laden" })).toBeInTheDocument();
  });

  it("zeigt Sperrhinweis und gesperrte Felder außerhalb des inaktiven Corezustands", async () => {
    render(<SettingsFeature generation={12} coreState="idle_in_game" characters={["mrbones"]} runs={[{ id: "countess", label: "Countess" }]} events={[]} />);
    expect(await screen.findByText("Diese Einstellungen sind gesperrt")).toBeInTheDocument();
    expect(screen.getAllByText(/Gesperrt, solange eine Session läuft/).length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
    expect(screen.getByLabelText("Maximale Routenausführungen")).toBeDisabled();
    await openPanel(/Wartung/);
    expect(screen.getByRole("button", { name: "Löschvorschau erstellen" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Auf sichere Standardwerte zurücksetzen" })).toBeDisabled();
  });

  it("zeigt im inaktiven Zustand keine Sperre und keine Speichern-Leiste", async () => {
    renderFeature();
    expect(await screen.findByLabelText("Maximale Routenausführungen")).toBeEnabled();
    expect(screen.queryByText("Diese Einstellungen sind gesperrt")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Speichern" })).not.toBeInTheDocument();
    expect(screen.getByText(/Keine offenen Änderungen/)).toHaveTextContent("Revision 4");
  });

  it("öffnet Fokus-Panels als modale Dialoge, sperrt das Scrollen und gibt den Fokus zurück", async () => {
    renderFeature();
    const line = await screen.findByRole("button", { name: /Routenreihenfolge/ });
    line.focus();
    fireEvent.click(line);
    const dialog = await screen.findByRole("dialog", { name: "Routenreihenfolge" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(document.body.style.overflow).toBe("hidden");
    expect(dialog).toHaveFocus();
    fireEvent.keyDown(dialog, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Routenreihenfolge" })).not.toBeInTheDocument());
    expect(document.body.style.overflow).toBe("");
    expect(line).toHaveFocus();

    fireEvent.click(screen.getByRole("button", { name: /Inventarschutz/ }));
    const inventory = await screen.findByRole("dialog", { name: "Inventarschutz" });
    expect(within(inventory).getByRole("grid")).toBeInTheDocument();
    fireEvent.mouseDown(inventory.parentElement as HTMLElement);
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Inventarschutz" })).not.toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /Wartung/ }));
    const maintenance = await screen.findByRole("dialog", { name: "Wartung" });
    fireEvent.click(within(maintenance).getByRole("button", { name: "Fertig" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Wartung" })).not.toBeInTheDocument());
  });

  it("zeigt Tastenbelegung im Fokus-Panel mit F1–F8 und Gürtel 1–4", async () => {
    renderFeature();
    await openPanel(/Tastenbelegung/);
    const dialog = screen.getByRole("dialog", { name: "Tastenbelegung" });
    expect(await within(dialog).findByLabelText("Verstärkter Schaden Taste")).toHaveValue("f1");
    expect(within(dialog).getByLabelText("Kadaverexplosion Taste")).toHaveValue("f2");
    expect(within(dialog).getByLabelText("Knochengefängnis Taste")).toHaveValue("f3");
    expect(within(dialog).getByLabelText("Knochenrüstung Taste")).toHaveValue("f5");
    expect(within(dialog).getByLabelText("Schriftrolle des Stadtportals Taste")).toHaveValue("f6");
    expect(within(dialog).getByLabelText("Teleport Taste")).toHaveValue("f7");
    expect(within(dialog).getByLabelText("Knochenspeer Taste")).toHaveValue("f8");
    expect(within(dialog).getByLabelText("Gürtel Slot 1 Taste")).toHaveValue("1");
    expect(within(dialog).getByLabelText("Gürtel Slot 4 Trank")).toHaveValue("rejuvenation");
    expect(mocks.previewCharacterSetup).toHaveBeenCalledWith({ character: "mrbones" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Schließen" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Tastenbelegung" })).not.toBeInTheDocument());
  });

  it("zeigt nicht unterstützte Charaktere sichtbar, aber nicht auswählbar", async () => {
    const onSelectedCharacterChange = vi.fn();
    mocks.get.mockResolvedValue({
      ...operator,
      characters: {
        ...operator.characters,
        holyhammer: { character_class: "paladin", combat_profile: "paladin_hammerdin", last_difficulty: "hell", queue: ["countess"] },
      },
    });
    render(<SettingsFeature generation={12} coreState="idle" characters={["mrbones", "holyhammer", "frostia"]} selectedCharacter="mrbones" onSelectedCharacterChange={onSelectedCharacterChange} catalog={catalog} runs={defaultRuns()} events={[]} />);
    const list = await screen.findByRole("group", { name: "Charakterliste" });
    expect(within(list).getByRole("button", { name: /MrBones/ })).toHaveAttribute("aria-pressed", "true");
    expect(within(list).getByRole("button", { name: /Frostia/ })).toBeDisabled();
    expect(within(list).getByRole("button", { name: /Frostia/ })).toHaveTextContent("Nicht unterstützt");
    fireEvent.click(within(list).getByRole("button", { name: /Holyhammer/ }));
    expect(onSelectedCharacterChange).toHaveBeenCalledWith("holyhammer");
    expect(screen.getByRole("combobox", { name: "Spieleranzahl" })).toHaveValue("1");
    expect(screen.getByRole("combobox", { name: "Letzte Schwierigkeit" })).toHaveValue("nightmare");
  });

  it("löscht Historie nur über metadatengebundene zweite Bestätigung", async () => {
    mocks.previewDelete.mockResolvedValue({ confirmation_token: "one-use", index_generation: 7, candidate_files: 4, candidate_bytes: 1234, protected_files: 1, categories: { schema3_run: 2, legacy: 2 } });
    mocks.confirmDelete.mockResolvedValue({ deleted_files: 4, deleted_bytes: 1234, protected_files: 1, diagnostics: [] });
    renderFeature();
    await openPanel(/Wartung/);
    fireEvent.click(screen.getByRole("button", { name: "Löschvorschau erstellen" }));
    const dialog = await screen.findByRole("dialog", { name: "Gesamte Historie dauerhaft löschen?" });
    expect(dialog).toHaveTextContent("4 direkte Datei(en)");
    expect(dialog).toHaveTextContent("schema3_run: 2");
    fireEvent.click(screen.getByRole("button", { name: "Alles endgültig löschen" }));
    await waitFor(() => expect(mocks.confirmDelete).toHaveBeenCalledWith({
      expected_generation: 12, confirmation_token: "one-use", index_generation: 7, candidate_files: 4, candidate_bytes: 1234,
    }));
    expect(await screen.findByText(/4 Historiendatei/)).toHaveTextContent("1 aktive Datei(en) blieben geschützt");
  });

  it("öffnet nach dem Löschen erneut eine Vorschau mit null Dateien", async () => {
    const onHistoryDeleted = vi.fn();
    mocks.previewDelete
      .mockResolvedValueOnce({ confirmation_token: "first", index_generation: 7, candidate_files: 2, candidate_bytes: 800, protected_files: 0, categories: { schema3_run: 2 } })
      .mockResolvedValueOnce({ confirmation_token: "empty", index_generation: 8, candidate_files: 0, candidate_bytes: 0, protected_files: 0, categories: {} });
    mocks.confirmDelete.mockResolvedValue({ deleted_files: 2, deleted_bytes: 800, protected_files: 0, diagnostics: [] });
    render(<SettingsFeature generation={12} coreState="idle" characters={["mrbones"]} runs={defaultRuns()} events={[]} onHistoryDeleted={onHistoryDeleted} />);

    await openPanel(/Wartung/);
    fireEvent.click(screen.getByRole("button", { name: "Löschvorschau erstellen" }));
    expect(await screen.findByRole("dialog", { name: "Gesamte Historie dauerhaft löschen?" })).toHaveTextContent("2 direkte Datei(en)");
    fireEvent.click(screen.getByRole("button", { name: "Alles endgültig löschen" }));
    await waitFor(() => expect(onHistoryDeleted).toHaveBeenCalledOnce());

    fireEvent.click(screen.getByRole("button", { name: "Löschvorschau erstellen" }));
    const emptyDialog = await screen.findByRole("dialog", { name: "Gesamte Historie dauerhaft löschen?" });
    expect(emptyDialog).toHaveTextContent("0 direkte Datei(en)");
    expect(within(emptyDialog).getByRole("button", { name: "Alles endgültig löschen" })).toBeDisabled();
    expect(mocks.previewDelete).toHaveBeenCalledTimes(2);
  });

  it("zeigt Versionsstatus, manuellen Retry und ausschließlich die feste Release-Aktion", async () => {
    renderFeature();
    expect(await screen.findByText("Installiert: 1.2.3. Kein neueres stabiles Release gefunden.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Erneut prüfen" }));
    expect(await screen.findByText("Installiert: 1.2.3. Veröffentlicht: 1.3.0.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Feste Release-Seite öffnen" }));
    expect(mocks.openRelease).toHaveBeenCalledOnce();
  });

  it("erstellt Diagnose standardmäßig ohne sensitive Opt-ins und zeigt nur den lokalen Dateinamen", async () => {
    mocks.createDiagnostic.mockResolvedValue({ filename: "diagnose-20260726T120000Z-aabbccdd.zip", bytes: 2048, included_telemetry: false, included_routes: false });
    renderFeature();
    await openPanel(/Wartung/);
    fireEvent.click(screen.getByRole("button", { name: "Redigiertes ZIP lokal erstellen" }));
    await waitFor(() => expect(mocks.createDiagnostic).toHaveBeenCalledWith({ include_telemetry: false, include_routes: false }));
    expect(mocks.revealDiagnostic).toHaveBeenCalledWith("diagnose-20260726T120000Z-aabbccdd.zip");
    expect(await screen.findByText(/Diagnosepaket diagnose-/)).toBeInTheDocument();
  });

  it("ergänzt Queue-Runs per Klick und per Drag aus dem Katalog", async () => {
    renderFeature();
    await openPanel(/Routenreihenfolge/);
    const heading = await screen.findByRole("heading", { name: "Verfügbare Routen" });
    const pane = heading.closest(".settings-queue-pane");
    expect(pane).toBeTruthy();
    await waitFor(() => expect(within(pane as HTMLElement).getByRole("button", { name: "+ Beschwörer" })).toBeEnabled());
    expect(within(pane as HTMLElement).getByRole("button", { name: "+ Nihlathak" })).toBeEnabled();
    fireEvent.click(within(pane as HTMLElement).getByRole("button", { name: "+ Beschwörer" }));
    const dropPane = screen.getByTestId("queue-drop-pane");
    expect(within(dropPane).getByText("Beschwörer")).toBeInTheDocument();

    const nihlathakRow = within(pane as HTMLElement).getByRole("button", { name: "+ Nihlathak" }).closest("li");
    expect(nihlathakRow).toBeTruthy();
    fireEvent.dragStart(nihlathakRow as HTMLElement, { dataTransfer: { setData: vi.fn(), effectAllowed: "copy" } });
    // jsdom dataTransfer is limited; exercise drop path via a synthetic transfer.
    const transfer = { getData: (type: string) => type === "text/plain" ? "add:nihlathak" : "", dropEffect: "copy", effectAllowed: "copy", types: ["text/plain"] };
    fireEvent.dragOver(dropPane, { dataTransfer: transfer });
    expect(transfer.dropEffect).toBe("copy");
    fireEvent.drop(dropPane, { dataTransfer: transfer });
    expect(within(dropPane).getByText("Nihlathak")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });

  it("nimmt nicht startfähige Katalog-Runs nicht in die Reihenfolge auf", async () => {
    mocks.getRunAvailabilities.mockResolvedValue({
      schema_version: 1, character: "MrBones", difficulty: "nightmare",
      runs: [
        { run_id: "countess", display_name: "Countess", status: "available" },
        { run_id: "mephisto", display_name: "Mephisto", status: "available" },
        { run_id: "summoner", display_name: "Summoner", status: "unavailable", reasons: ["profile_run_strategy_unavailable"] },
      ],
    });
    renderFeature();
    await openPanel(/Routenreihenfolge/);
    const dropPane = await screen.findByTestId("queue-drop-pane");
    const transfer = { getData: (type: string) => type === "text/plain" ? "add:summoner" : "", dropEffect: "copy", effectAllowed: "copy", types: ["text/plain"] };
    fireEvent.drop(dropPane, { dataTransfer: transfer });
    expect(screen.queryByRole("button", { name: "+ Beschwörer" })).toBeDisabled();
    expect(within(dropPane).queryByText("Beschwörer")).not.toBeInTheDocument();
  });

  it("zeigt Katalog-Namen ohne Live-Availability wenn der Run-Fetch fehlschlägt", async () => {
    mocks.getRunAvailabilities.mockRejectedValue(new Error("offline"));
    renderFeature();
    await openPanel(/Routenreihenfolge/);
    const heading = await screen.findByRole("heading", { name: "Verfügbare Routen" });
    const pane = heading.closest(".settings-queue-pane");
    expect(within(pane as HTMLElement).getByRole("button", { name: "+ Beschwörer" })).toBeDisabled();
    expect(within(pane as HTMLElement).getByRole("button", { name: "+ Nihlathak" })).toBeDisabled();
  });

  it("sortiert die aktive Run-Reihenfolge per Drag mit move-dropEffect", async () => {
    renderFeature();
    await openPanel(/Routenreihenfolge/);
    const dropPane = await screen.findByTestId("queue-drop-pane");
    const rows = within(dropPane).getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(within(rows[0]).getByText("Gräfin")).toBeInTheDocument();
    expect(within(rows[1]).getByText("Mephisto")).toBeInTheDocument();

    const transfer = {
      getData: (type: string) => type === "text/plain" ? "reorder:0" : "",
      setData: vi.fn(),
      dropEffect: "none",
      effectAllowed: "move",
      types: ["text/plain"],
    };
    fireEvent.dragStart(rows[0], { dataTransfer: transfer });
    fireEvent.dragOver(rows[1], { dataTransfer: transfer });
    expect(transfer.dropEffect).toBe("move");
    fireEvent.drop(rows[1], { dataTransfer: transfer });

    const reordered = within(dropPane).getAllByRole("listitem");
    expect(within(reordered[0]).getByText("Mephisto")).toBeInTheDocument();
    expect(within(reordered[1]).getByText("Gräfin")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });

  it("zeigt die vom Core aufgelösten Route-Combat-Werte read-only unter Wartung", async () => {
    render(<SettingsFeature generation={12} coreState="idle" characters={["mrbones"]} runs={[{
      id: "summoner", label: "Summoner", routeCombat: {
        enabled: true, immediate_radius_tiles: 18, corridor_width_tiles: 7, landing_radius_tiles: 10,
        attack_distance_tiles: 30, no_progress_timeout_ms: 12000, teleport_mana_reserve_percent: 20,
        resume_mana_percent: 35, emergency_mana_percent: 10, mana_recovery_timeout_ms: 5000,
      },
    }]} events={[]} />);
    await openPanel(/Wartung/);
    const summary = await screen.findByText("Effektive Route-Combat-Werte (read-only)");
    fireEvent.click(summary);
    expect(summary.parentElement).toHaveTextContent('"no_progress_timeout_ms": 12000');
    expect(summary.parentElement).toHaveTextContent('"enabled": true');
  });

  it("verwirft lokale Änderungen ohne Core-Aufruf", async () => {
    renderFeature();
    fireEvent.change(await screen.findByLabelText("Pause"), { target: { value: "f8" } });
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Verwerfen" }));
    expect(screen.getByLabelText("Pause")).toHaveValue("pause");
    expect(screen.queryByRole("button", { name: "Speichern" })).not.toBeInTheDocument();
    expect(mocks.preview).not.toHaveBeenCalled();
  });
});

function defaultRuns() {
  return [
    { id: "countess", label: "Countess", status: "available" },
    { id: "mephisto", label: "Mephisto", status: "available" },
    { id: "nihlathak", label: "Nihlathak", status: "runtime_validation_required" },
    { id: "summoner", label: "Summoner", status: "available" },
  ];
}

function renderFeature() {
  return render(<SettingsFeature generation={12} coreState="idle" characters={["mrbones"]} runs={defaultRuns()} events={[]} />);
}

async function openPanel(name: RegExp) {
  fireEvent.click(await screen.findByRole("button", { name }));
  return screen.findByRole("dialog");
}

function change(settings: OperatorSettingsDTO, fields: string[], restart: boolean): OperatorSettingsChangeDTO {
  return { schema_version: 1, generation: 12, settings, changed_fields: fields, restart_required: restart, reason_code: restart ? "config_restart_required" : undefined };
}
