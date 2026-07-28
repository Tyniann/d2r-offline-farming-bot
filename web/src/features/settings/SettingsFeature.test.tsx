import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { OperatorSettingsChangeDTO, OperatorSettingsDTO } from "../../api/generated";
import { SettingsFeature } from "./SettingsFeature";

const mocks = vi.hoisted(() => ({
  get: vi.fn(), preview: vi.fn(), previewReset: vi.fn(), save: vi.fn(), restore: vi.fn(),
  previewDelete: vi.fn(), confirmDelete: vi.fn(),
  createDiagnostic: vi.fn(),
  getDesktop: vi.fn(), updateDesktop: vi.fn(), restartCore: vi.fn(),
  getUpdate: vi.fn(), checkUpdate: vi.fn(), openRelease: vi.fn(), revealDiagnostic: vi.fn(),
}));

vi.mock("../../api/generated", () => ({
  getOperatorSettings: mocks.get,
  previewOperatorSettings: mocks.preview,
  previewResetOperatorSettings: mocks.previewReset,
}));
vi.mock("../../api/client", () => ({
  saveOperatorSettings: mocks.save, restoreOperatorSettings: mocks.restore,
  previewHistoryDeleteAll: mocks.previewDelete, confirmHistoryDeleteAll: mocks.confirmDelete,
  createDiagnosticBundle: mocks.createDiagnostic,
}));

const operator: OperatorSettingsDTO = {
  schema_version: 2,
  revision: 4,
  characters: { mrbones: { character_class: "necromancer", combat_profile: "necro_bone_spear", last_difficulty: "nightmare", queue: ["countess", "mephisto"] } },
  budgets: { max_runs: 20, max_duration_ms: 7_200_000, max_consecutive_failures: 3, max_total_restarts: 4 },
  input: { enabled: false, pause_hotkey: "pause", stop_after_run_hotkey: "f10", recording_finish_hotkey: "f9", emergency_stop_hotkey: "f11" },
  history: { retention_enabled: true, retention_days: 60 },
};

describe("SettingsFeature", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.get.mockResolvedValue(operator);
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

  it("bindet Operator- und Desktopwerte, Vorschau und kontrollierten Restart", async () => {
    const changed = change({ ...operator, revision: 5, input: { ...operator.input, pause_hotkey: "f8" } }, ["input.pause_hotkey"], true);
    mocks.preview.mockResolvedValue(changed);
    mocks.save.mockResolvedValue(changed);
    renderFeature();

    fireEvent.change(await screen.findByLabelText("Pause"), { target: { value: "f8" } });
    fireEvent.click(screen.getByRole("button", { name: "Änderungen prüfen" }));
    expect(await screen.findByRole("dialog", { name: "Änderungen speichern?" })).toHaveTextContent("input.pause_hotkey");
    expect(mocks.preview).toHaveBeenCalledWith(expect.objectContaining({ expected_revision: 4, expected_generation: 12, settings: expect.objectContaining({ revision: 4 }) }));
    expect(mocks.preview.mock.calls[0][0].settings.characters.mrbones).toMatchObject({
      character_class: "necromancer",
      combat_profile: "necro_bone_spear",
    });
    const save = screen.getByRole("button", { name: "Revisionsgebunden speichern" });
    await waitFor(() => expect(save).toBeEnabled());
    fireEvent.click(save);
    await waitFor(() => expect(mocks.save).toHaveBeenCalledWith(expect.objectContaining({ expected_revision: 4, expected_generation: 12 })));
    expect(await screen.findByText("Core-Neustart erforderlich", {}, { timeout: 3000 })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Core kontrolliert neu starten" }));
    await waitFor(() => expect(mocks.restartCore).toHaveBeenCalledOnce());

    fireEvent.click(screen.getByLabelText("App mit Windows starten"));
    fireEvent.click(screen.getByRole("button", { name: "Desktop-Einstellungen speichern" }));
    await waitFor(() => expect(mocks.updateDesktop).toHaveBeenCalledWith({ autostart: true, onboarding_completed: false }));
  });

  it("zeigt Resetvorschau und einen stale Revision-Konflikt persistent", async () => {
    mocks.previewReset.mockResolvedValue(change({ ...operator, revision: 5 }, ["budgets.max_runs"], false));
    mocks.restore.mockRejectedValue(new Error("Die Einstellungen wurden zwischenzeitlich geändert."));
    renderFeature();
    fireEvent.click(await screen.findByRole("button", { name: "Sichere Defaults vorschauen" }));
    expect(await screen.findByRole("dialog", { name: "Sichere Defaults anwenden?" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Defaults anwenden" }));
    expect(await screen.findByText("Revision ist veraltet")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Aktuellen Stand laden" })).toBeInTheDocument();
  });

  it("zeigt Sperrhinweis und gesperrte Felder außerhalb des inaktiven Corezustands", async () => {
    render(<SettingsFeature generation={12} coreState="idle_in_game" characters={["mrbones"]} runs={[{ id: "countess", label: "Countess" }]} events={[]} />);
    expect(await screen.findByText("Diese Einstellungen sind gesperrt")).toBeInTheDocument();
    expect(screen.getByText(/Die Felder und Speicheraktionen sind derzeit nicht änderbar/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Änderungen prüfen" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Sichere Defaults vorschauen" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Löschvorschau erstellen" })).toBeDisabled();
    expect(screen.getByLabelText("Maximale Runs")).toBeDisabled();
  });

  it("zeigt keinen Sperrhinweis und lässt Felder im inaktiven Zustand zu", async () => {
    renderFeature();
    expect(await screen.findByLabelText("Maximale Runs")).toBeEnabled();
    expect(screen.queryByText("Diese Einstellungen sind gesperrt")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Änderungen prüfen" })).toBeEnabled();
  });

  it("löscht Historie nur über metadatengebundene zweite Bestätigung", async () => {
    mocks.previewDelete.mockResolvedValue({ confirmation_token: "one-use", index_generation: 7, candidate_files: 4, candidate_bytes: 1234, protected_files: 1, categories: { schema3_run: 2, legacy: 2 } });
    mocks.confirmDelete.mockResolvedValue({ deleted_files: 4, deleted_bytes: 1234, protected_files: 1, diagnostics: [] });
    renderFeature();
    fireEvent.click(await screen.findByRole("button", { name: "Löschvorschau erstellen" }));
    const dialog = await screen.findByRole("dialog", { name: "Gesamte Historie dauerhaft löschen?" });
    expect(dialog).toHaveTextContent("4 direkte Datei(en)");
    expect(dialog).toHaveTextContent("schema3_run: 2");
    fireEvent.click(screen.getByRole("button", { name: "Alles endgültig löschen" }));
    await waitFor(() => expect(mocks.confirmDelete).toHaveBeenCalledWith({
      expected_generation: 12, confirmation_token: "one-use", index_generation: 7, candidate_files: 4, candidate_bytes: 1234,
    }));
    expect(await screen.findByText(/4 Historiendatei/)).toHaveTextContent("1 aktive Datei(en) blieben geschützt");
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
    fireEvent.click(await screen.findByRole("button", { name: "Redigiertes ZIP lokal erstellen" }));
    await waitFor(() => expect(mocks.createDiagnostic).toHaveBeenCalledWith({ include_telemetry: false, include_routes: false }));
    expect(mocks.revealDiagnostic).toHaveBeenCalledWith("diagnose-20260726T120000Z-aabbccdd.zip");
    expect(await screen.findByText(/Diagnosepaket diagnose-/)).toBeInTheDocument();
  });
});

function renderFeature() {
  return render(<SettingsFeature generation={12} coreState="idle" characters={["mrbones"]} runs={[{ id: "countess", label: "Countess" }, { id: "mephisto", label: "Mephisto" }]} events={[]} />);
}

function change(settings: OperatorSettingsDTO, fields: string[], restart: boolean): OperatorSettingsChangeDTO {
  return { schema_version: 1, generation: 12, settings, changed_fields: fields, restart_required: restart, reason_code: restart ? "config_restart_required" : undefined };
}
