import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RouteFeature } from "./RouteFeature";

const mocks = vi.hoisted(() => ({
  library: vi.fn(), candidates: vi.fn(), options: vi.fn(), hotkeys: vi.fn(), workflow: vi.fn(),
  preview: vi.fn(), confirm: vi.fn(), start: vi.fn(), finish: vi.fn(),
}));

vi.mock("../../api/generated", () => ({
  getRouteLibrary: mocks.library, getRouteCandidates: mocks.candidates, getRecordingOptions: mocks.options,
  getHotkeyHelp: mocks.hotkeys, getRouteWorkflow: mocks.workflow,
}));
vi.mock("../../api/client", () => ({
  previewRouteMutation: mocks.preview, confirmRouteMutation: mocks.confirm,
  startRouteWorkflow: mocks.start, finishRouteRecording: mocks.finish,
}));

const recordingOptions = [
  { run_id: "countess", display_name: "Countess", instructions_de: "Gräfin-Anleitung vollständig.", start_waypoint: "black_marsh", allowed_start_area_id: 6, allowed_route_area_ids: [6, 20], terminal_area_id: 25, terminal_max_distance_tiles: 80, available: true, prerequisites: [{ id: "waypoint", ready: true }] },
  { run_id: "mephisto", display_name: "Mephisto", instructions_de: "Mephisto-Anleitung vollständig.", start_waypoint: "durance_of_hate_level_2", allowed_start_area_id: 101, allowed_route_area_ids: [101], terminal_area_id: 102, terminal_max_distance_tiles: 80, available: true, prerequisites: [] },
  { run_id: "lower-kurast", display_name: "Lower Kurast", instructions_de: "Starte am Wegpunkt Unteres Kurast und ende an den Lagerfeuer-Hütten.", start_waypoint: "lower_kurast", allowed_start_area_id: 79, allowed_route_area_ids: [79], terminal_area_id: 79, terminal_max_distance_tiles: 80, available: true, prerequisites: [{ id: "waypoint", ready: true }] },
  { run_id: "cows", route_role: "leg_acquisition", display_name: "Kuhlevel", instructions_de: "Wirt-Anleitung vollständig.", start_waypoint: "stony_field", allowed_start_area_id: 4, allowed_route_area_ids: [4, 38], terminal_area_id: 38, terminal_max_distance_tiles: 20, available: true, prerequisites: [{ id: "town_portal", ready: true }] },
  { run_id: "cows", route_role: "cow_sweep", display_name: "Kuhlevel", instructions_de: "Cow-Anleitung vollständig.", start_kind: "object_portal_arrival", start_waypoint: "", allowed_start_area_id: 39, allowed_route_area_ids: [39], terminal_area_id: 39, terminal_max_distance_tiles: 0, available: true, prerequisites: [{ id: "teleport", ready: true }] },
];

describe("RouteFeature Redesign", () => {
  afterEach(cleanup);
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.library.mockResolvedValue({ revision: 1, character: "MrBones", routes: [] });
    mocks.candidates.mockResolvedValue([]);
    mocks.options.mockResolvedValue(recordingOptions);
    mocks.hotkeys.mockResolvedValue({ recording_finish: "F9", stop_after_run: "F10", emergency_stop: "F11", pause: "Pause" });
    mocks.workflow.mockResolvedValue({ workflow_id: "", generation: 1, state: "idle", run_id: "", character: "MrBones" });
    mocks.start.mockResolvedValue({ workflow_id: "workflow-1", generation: 2, state: "recording", run_id: "countess", character: "MrBones" });
    mocks.finish.mockResolvedValue({ workflow_id: "workflow-1", generation: 3, state: "freezing", run_id: "countess", character: "MrBones" });
  });

  it("zeigt drei aufgabenorientierte Bereiche und behält den Charakterkontext", async () => {
    const onSelectedCharacterChange = vi.fn();
    const view = render(<RouteFeature characters={["MrBones", "MrHammer"]} selectedCharacter="MrBones" onSelectedCharacterChange={onSelectedCharacterChange} refreshKey={0} />);
    expect(await screen.findByRole("heading", { name: "Routen" })).toBeInTheDocument();
    for (const label of ["Meine Routen", "Route aufnehmen", "Entwürfe"]) expect(screen.getByRole("button", { name: new RegExp(label) })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Charakter"), { target: { value: "MrHammer" } });
    expect(onSelectedCharacterChange).toHaveBeenCalledWith("MrHammer");
    view.rerender(<RouteFeature characters={["MrBones", "MrHammer"]} selectedCharacter="MrHammer" onSelectedCharacterChange={onSelectedCharacterChange} refreshKey={0} />);
    await waitFor(() => expect(mocks.library).toHaveBeenCalledWith("MrHammer", false, expect.any(AbortSignal)));
    expect(mocks.options).toHaveBeenCalledWith("MrHammer", expect.any(AbortSignal));
    expect(screen.getByRole("button", { name: "Meine Routen" })).toHaveAttribute("aria-pressed", "true");
  });

  it("verwendet kanonische Run-Titel und blendet IDs sowie Rohcodes aus", async () => {
    mocks.library.mockResolvedValue({ revision: 2, character: "MrBones", routes: [{ route_id: "countess-deadbeef", display_name: "Countess deadbeef", run_id: "countess", character: "MrBones", difficulty: "nightmare", lifecycle_status: "runtime_validation_required", management_status: "active", assigned: true }] });
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    expect(await screen.findByText("Gräfin")).toBeInTheDocument();
    expect(screen.getByText("Alptraum · Standardroute")).toBeInTheDocument();
    expect(screen.queryByText(/countess-deadbeef|runtime_validation_required|Countess deadbeef/)).not.toBeInTheDocument();
  });

  it("rendert im Leerlauf weder Workflowstatus noch Generation", async () => {
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    await screen.findByText("Gräfin");
    expect(screen.queryByText(/Workflow|Generation 1|Bereit ·/)).not.toBeInTheDocument();
  });

  it("zeigt nur die Anleitung des ausgewählten Runs und Kuhhinweise ausschließlich beim Kuhlevel", async () => {
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    fireEvent.click(await screen.findByRole("button", { name: "Route aufnehmen" }));
    expect(screen.getByText("Gräfin-Anleitung vollständig.")).toBeInTheDocument();
    expect(screen.queryByText("Mephisto-Anleitung vollständig.")).not.toBeInTheDocument();
    expect(screen.queryByText("Vorbereitung für das Kuhlevel")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Kuhlevel/ }));
    expect(screen.getByText("Wirt-Anleitung vollständig.")).toBeInTheDocument();
    expect(screen.getByText("Vorbereitung für das Kuhlevel")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "2 Cow-Route" }));
    expect(screen.getByText("Cow-Anleitung vollständig.")).toBeInTheDocument();
    expect(screen.queryByText("Wirt-Anleitung vollständig.")).not.toBeInTheDocument();
  });

  it("zeigt vor der Aufnahme den Routenzweck und den Weg zu Pickit", async () => {
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    fireEvent.click(await screen.findByRole("button", { name: "Route aufnehmen" }));
    const countessPurpose = screen.getByRole("note", { name: "Geeignet für Gräfin" });
    expect(countessPurpose).toHaveTextContent("Schlüssel des Terrors");
    expect(countessPurpose).toHaveTextContent("Runen");
    expect(within(countessPurpose).getByRole("link", { name: "Pickit konfigurieren" })).toHaveAttribute("href", "#pickit");

    fireEvent.click(screen.getByRole("button", { name: /^Mephisto/ }));
    const mephistoPurpose = screen.getByRole("note", { name: "Geeignet für Mephisto" });
    expect(mephistoPurpose).toHaveTextContent("Set-Items");
    expect(mephistoPurpose).toHaveTextContent("Unique-Items");
    expect(screen.queryByText("Schlüssel des Terrors")).not.toBeInTheDocument();
  });

  it("zeigt Unteres Kurast mit Lagerfeuer-Ziel und schließt das Aufnahme-Overlay per Button", async () => {
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    fireEvent.click(await screen.findByRole("button", { name: "Route aufnehmen" }));
    fireEvent.click(screen.getByRole("button", { name: /Unteres Kurast/ }));
    expect(screen.getByText("Starte am Wegpunkt Unteres Kurast und ende an den Lagerfeuer-Hütten.")).toBeInTheDocument();
    expect(screen.getByText("Lagerfeuer-Hütten")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Unteres Kurast aufnehmen" })).toBeInTheDocument();
    expect(screen.queryByText(/Würfel|Neuwürfeln|gute Karte/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Lagerfeuer vergrößern" }));
    const dialog = await screen.findByRole("dialog", { name: "Lagerfeuer" });
    expect(dialog).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Schließen" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Lagerfeuer" })).not.toBeInTheDocument());
  });

  it("gruppiert beide Kuhlevel-Routen und hält F9 sowie F11 an der Aufnahmeaktion", async () => {
    mocks.library.mockResolvedValue({ revision: 2, character: "MrBones", routes: [
      { route_id: "leg-1", display_name: "Leg hash", run_id: "cows", route_role: "leg_acquisition", character: "MrBones", difficulty: "hell", lifecycle_status: "valid", management_status: "active", assigned: true },
      { route_id: "cow-1", display_name: "Cow hash", run_id: "cows", route_role: "cow_sweep", character: "MrBones", difficulty: "hell", lifecycle_status: "valid", management_status: "active", assigned: true },
    ] });
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    const cowRow = (await screen.findByText(/Zwei zusammengehörige Routen/)).closest("article");
    expect(cowRow).toHaveTextContent("1 · Wirt-Route"); expect(cowRow).toHaveTextContent("2 · Cow-Route"); expect(cowRow).toHaveTextContent("Vollständig");
    fireEvent.click(screen.getByRole("button", { name: "Route aufnehmen" }));
    expect(screen.getByText("F9")).toBeInTheDocument(); expect(screen.getByText("F11")).toBeInTheDocument();
    expect(screen.queryByText("Hotkey-Hilfe")).not.toBeInTheDocument();
  });

  it.each([
    { run_id: "countess", button: "Gräfin aufnehmen" },
    { run_id: "mephisto", button: "Mephisto aufnehmen" },
  ])("lässt nach fehlgeschlagener $button-Aufnahme erneut starten", async ({ run_id, button }) => {
    mocks.workflow.mockResolvedValue({ workflow_id: "workflow-1", generation: 4, state: "failed_safe", run_id, character: "MrBones", reason: "recording_terminal_area_mismatch" });
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    fireEvent.click(await screen.findByRole("button", { name: "Route aufnehmen" }));
    expect(await screen.findByText("Vorgang abgebrochen")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: button }));
    await waitFor(() => expect(mocks.start).toHaveBeenCalledWith("record", 4, expect.objectContaining({ runId: run_id, character: "MrBones" })));
  });

  it("lässt nach fehlgeschlagener Aufnahme einen anderen Run starten", async () => {
    mocks.workflow.mockResolvedValue({ workflow_id: "workflow-1", generation: 4, state: "failed_safe", run_id: "countess", character: "MrBones", reason: "recording_terminal_area_mismatch" });
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    fireEvent.click(await screen.findByRole("button", { name: "Route aufnehmen" }));
    fireEvent.click(await screen.findByRole("button", { name: /^Mephisto/ }));
    fireEvent.click(screen.getByRole("button", { name: "Mephisto aufnehmen" }));
    await waitFor(() => expect(mocks.start).toHaveBeenCalledWith("record", 4, expect.objectContaining({ runId: "mephisto" })));
  });

  it("lässt nach Notabbruch denselben Run erneut starten", async () => {
    mocks.workflow.mockResolvedValue({ workflow_id: "workflow-2", generation: 5, state: "emergency_cancelled", run_id: "countess", character: "MrBones" });
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    fireEvent.click(await screen.findByRole("button", { name: "Route aufnehmen" }));
    expect(await screen.findByText("Notabbruch ausgeführt")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Gräfin aufnehmen" }));
    await waitFor(() => expect(mocks.start).toHaveBeenCalledWith("record", 5, expect.objectContaining({ runId: "countess" })));
  });

  it("beendet eine aktive Aufnahme über denselben Core-Intent wie F9", async () => {
    mocks.workflow.mockResolvedValue({ workflow_id: "workflow-1", generation: 3, state: "recording", run_id: "countess", character: "MrBones" });
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    fireEvent.click(await screen.findByRole("button", { name: "Aufnahme beenden" }));
    await waitFor(() => expect(mocks.finish).toHaveBeenCalledWith("workflow-1", 3));
    expect(await screen.findByText("Aufnahme wird geprüft")).toBeInTheDocument();
  });

  it("zeigt Entwürfe ohne Kandidaten-ID und Hash, sendet intern aber die richtige ID", async () => {
    const candidate = { candidate_id: "candidate-secret", run_id: "countess", character: "MrBones", difficulty: "hell", state: "validated", measured_boss_distance: 25, route_sha256: "a".repeat(64), created_at: "2026-08-13T09:42:00Z" };
    mocks.candidates.mockResolvedValue([candidate]);
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    fireEvent.click(await screen.findByRole("button", { name: /Entwürfe 1/ }));
    expect(screen.getAllByText("Gräfin").length).toBeGreaterThan(0); expect(screen.getByText("Bereit zum Test")).toBeInTheDocument();
    expect(document.body).not.toHaveTextContent("candidate-secret"); expect(document.body).not.toHaveTextContent("aaaaaaaaaaaa");
    fireEvent.click(screen.getByRole("button", { name: "Testen" }));
    await waitFor(() => expect(mocks.start).toHaveBeenCalledWith("test", 1, { candidateId: "candidate-secret" }));
  });

  it("löscht einen Entwurf über eine lesbare Preview-Bestätigung und aktualisiert den Badge", async () => {
    const candidate = { candidate_id: "candidate-delete", run_id: "countess", character: "MrBones", difficulty: "hell", state: "failed", measured_boss_distance: 25, route_sha256: "b".repeat(64), created_at: "2026-08-13T09:42:00Z", reason: "route_test_terminal_mismatch" };
    mocks.candidates.mockResolvedValue([candidate]);
    const preview = { operation: "delete_candidate", route_id: "", candidate_id: candidate.candidate_id, catalog_revision: 2, lifecycle_revision: 3, assignment_revision: 4, confirmation_token: "delete-draft" };
    mocks.preview.mockResolvedValue(preview);
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    fireEvent.click(await screen.findByRole("button", { name: /Entwürfe 1/ }));
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("Entwurf löschen?"); expect(dialog).toHaveTextContent("Gräfin"); expect(dialog).not.toHaveTextContent("candidate-delete");
    mocks.candidates.mockResolvedValue([]);
    fireEvent.click(within(dialog).getByRole("button", { name: "Entwurf löschen" }));
    await waitFor(() => expect(mocks.confirm).toHaveBeenCalledWith(preview, ""));
    await waitFor(() => expect(screen.getByRole("button", { name: "Entwürfe" })).toBeInTheDocument());
  });

  it("lädt und rendert keine System-Egress-Daten", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    await screen.findByText("Gräfin");
    expect(screen.queryByText(/Egress|Portal→Wegpunkt|Playback prüfen/)).not.toBeInTheDocument();
    expect(fetchSpy).not.toHaveBeenCalled();
    fetchSpy.mockRestore();
  });

  it("ersetzt unbekannte Backendfehler durch eine neutrale deutsche Meldung", async () => {
    mocks.library.mockRejectedValue(new Error("route_catalog_internal_failure"));
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    expect(await screen.findByRole("alert")).toHaveTextContent("Routen konnten nicht geladen werden.");
    expect(document.body).not.toHaveTextContent("route_catalog_internal_failure");
  });

  it("bietet nach einer Onboarding-Übergabe den Rückweg an", async () => {
    const onReturn = vi.fn();
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} onReturnToOnboarding={onReturn} />);
    fireEvent.click(await screen.findByRole("button", { name: "Zurück zur Einrichtung" }));
    expect(onReturn).toHaveBeenCalledOnce();
  });
});
