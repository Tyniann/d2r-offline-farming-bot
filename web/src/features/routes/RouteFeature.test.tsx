import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RouteFeature } from "./RouteFeature";

const mocks = vi.hoisted(() => ({
  library: vi.fn(), candidates: vi.fn(), options: vi.fn(), system: vi.fn(), hotkeys: vi.fn(), workflow: vi.fn(),
  preview: vi.fn(), confirm: vi.fn(), start: vi.fn(), finish: vi.fn(),
}));
vi.mock("../../api/generated", () => ({ getRouteLibrary: mocks.library, getRouteCandidates: mocks.candidates, getRecordingOptions: mocks.options, getSystemRouteStatus: mocks.system, getHotkeyHelp: mocks.hotkeys, getRouteWorkflow: mocks.workflow }));
vi.mock("../../api/client", () => ({ previewRouteMutation: mocks.preview, confirmRouteMutation: mocks.confirm, startRouteWorkflow: mocks.start, finishRouteRecording: mocks.finish }));

describe("RouteFeature", () => {
  afterEach(cleanup);
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.library.mockResolvedValue({ revision: 1, character: "MrBones", routes: [] }); mocks.candidates.mockResolvedValue([]); mocks.options.mockResolvedValue([]);
    mocks.system.mockResolvedValue([{ act: "act2", ready: false, reason: "egress_missing" }, { act: "act3", ready: true }]);
    mocks.hotkeys.mockResolvedValue({ recording_finish: "f9", stop_after_run: "f10", emergency_stop: "f11", pause: "pause" });
    mocks.workflow.mockResolvedValue({ generation: 1, state: "idle" }); mocks.start.mockResolvedValue({ generation: 2, state: "recording" });
		mocks.finish.mockResolvedValue({ workflow_id: "workflow-1", generation: 4, state: "freezing", run_id: "countess", character: "MrBones" });
  });

	it("beendet eine aktive Aufnahme über denselben Core-Finish-Intent wie F9", async () => {
		mocks.workflow.mockResolvedValue({ workflow_id: "workflow-1", generation: 3, state: "recording", run_id: "countess", character: "MrBones" });
		render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
		fireEvent.click(await screen.findByRole("button", { name: "Aufnahme beenden" }));
		await waitFor(() => expect(mocks.finish).toHaveBeenCalledWith("workflow-1", 3));
		expect(await screen.findByText("Routen-Workflow: freezing")).toBeInTheDocument();
	});

  it("zeigt Loading, Empty, Charakterfilter, Archiv und nur fehlenden Egress als Setup", async () => {
    render(<RouteFeature characters={["MrBones", "MrHammer"]} selectedCharacter="MrBones" refreshKey={0} />);
    expect(screen.getByText("Routen werden geladen …")).toBeInTheDocument();
    expect(await screen.findByText("Für diesen Charakter gibt es noch keine Farming-Route.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Egress aufnehmen" })).toBeInTheDocument();
		expect(screen.getByText(/Town Portal.*Portal-Ankunftspunkt/)).toHaveTextContent(/f9/i);
    expect(screen.getByText("ACT3").closest("article")?.querySelector("button")?.textContent).toBe("Playback prüfen");
    fireEvent.change(screen.getByLabelText("Charakter", { selector: "select" }), { target: { value: "MrHammer" } });
    await waitFor(() => expect(mocks.library).toHaveBeenCalledWith("MrHammer", false, expect.any(AbortSignal)));
    fireEvent.click(screen.getByRole("button", { name: "Archiv anzeigen" }));
    await waitFor(() => expect(mocks.library).toHaveBeenCalledWith("MrHammer", true, expect.any(AbortSignal)));
    expect(await screen.findByText("Das Archiv ist leer.")).toBeInTheDocument();
  });

  it("übernimmt die nach dem Mount bestätigte Core-Auswahl in den Kandidatenfilter", async () => {
    mocks.candidates.mockResolvedValue([{ candidate_id: "candidate-late", run_id: "countess", character: "MrBones", difficulty: "nightmare", state: "validated", measured_boss_distance: 15, route_sha256: "a".repeat(64) }]);
    const view = render(<RouteFeature characters={[]} selectedCharacter="" refreshKey={0} />);
    expect(await screen.findByText("Für diesen Charakter gibt es noch keinen aufgenommenen Kandidaten.")).toBeInTheDocument();

    view.rerender(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={1} />);
    expect(await screen.findByRole("button", { name: "Isoliert testen" })).toBeEnabled();
    expect(screen.getByText("Bereit zum Test", { exact: false })).toBeInTheDocument();
  });

  it("zeigt den noch nicht gestarteten Egress-Preflight direkt am betroffenen Akt", async () => {
    mocks.workflow.mockResolvedValue({ workflow_id: "egress-1", generation: 2, state: "preflight", act: "act2", reason: "town_egress_start_unconfirmed" });
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    const act2 = (await screen.findByText("ACT2")).closest("article");
    expect(act2).toHaveTextContent("Core-Status: preflight");
    expect(act2).toHaveTextContent("Aufnahme noch nicht gestartet");
    expect(screen.getByText(/Am Portal-Ankunftspunkt stehen bleiben/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Egress aufnehmen" })).toBeDisabled();
  });

  it("trennt Farming-Preflight und Farming-Aufnahme klar vom Egress-Flow", async () => {
    mocks.options.mockResolvedValue([{ run_id: "countess", display_name: "Countess", instructions_de: "Am Black-Marsh-Wegpunkt starten.", start_waypoint: "black_marsh", allowed_start_area_id: 6, allowed_route_area_ids: [6, 20], terminal_area_id: 25, terminal_max_distance_tiles: 80, available: true }]);
    mocks.workflow.mockResolvedValue({ workflow_id: "record-1", generation: 2, state: "preflight", run_id: "countess", character: "MrBones", reason: "recording_preflight_failed" });
    const view = render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    expect(await screen.findByText(/Am angezeigten Startwegpunkt stehen bleiben/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Aufnahme starten" })).toBeDisabled();

    mocks.workflow.mockResolvedValue({ workflow_id: "record-1", generation: 3, state: "recording", run_id: "countess", character: "MrBones" });
    view.rerender(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={1} />);
    const instruction = await screen.findByText(/Farming-Route bis zur selbst gewählten Kampfposition/);
    expect(instruction.closest(".queue-status")).not.toHaveTextContent("ohne Teleport zum lokalen Wegpunkt");
  });

  it("zeigt Core-Hotkeys und fokussiert den revisionsgebundenen Bestätigungsdialog", async () => {
    mocks.library.mockResolvedValue({ revision: 2, character: "MrBones", routes: [{ route_id: "countess-1", display_name: "Countess", run_id: "countess", character: "mrbones", difficulty: "nightmare", lifecycle_status: "valid", management_status: "active", assigned: true }] });
    mocks.preview.mockResolvedValue({ operation: "archive", route_id: "countess-1", catalog_revision: 2, lifecycle_revision: 3, assignment_revision: 4, confirmation_token: "one-use" });
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    fireEvent.click(await screen.findByText("Hotkey-Hilfe"));
    for (const value of ["f9", "f10", "f11", "pause"]) expect(screen.getByText(value)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Archivieren" }));
    const confirm = await screen.findByRole("button", { name: "Änderung bestätigen" });
    await waitFor(() => expect(confirm).toHaveFocus());
    expect(screen.getByText(/Katalog-, Lifecycle- und Assignment-Revisionen/)).toBeInTheDocument();
    fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("verlangt bei endgültigem Delete die manuell eingegebene exakte Route-ID", async () => {
    const archived = { route_id: "countess-test-artifact", display_name: "Testartefakt", run_id: "countess", character: "mrbones", difficulty: "nightmare", lifecycle_status: "valid", management_status: "archived", assigned: false };
    const preview = { operation: "delete", route_id: archived.route_id, catalog_revision: 2, lifecycle_revision: 3, assignment_revision: 4, confirmation_token: "delete-once" };
    mocks.library.mockResolvedValue({ revision: 2, character: "MrBones", routes: [archived] });
    mocks.preview.mockResolvedValue(preview);
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    fireEvent.click(await screen.findByRole("button", { name: "Archiv anzeigen" }));
    fireEvent.click(await screen.findByRole("button", { name: "Endgültig löschen" }));
    const input = await screen.findByRole("textbox", { name: /Route-ID zur endgültigen Löschung eingeben/ });
    const confirm = screen.getByRole("button", { name: "Änderung bestätigen" });
    expect(input).toHaveFocus();
    expect(confirm).toBeDisabled();
    fireEvent.change(input, { target: { value: archived.route_id } });
    expect(confirm).toBeEnabled();
    fireEvent.click(confirm);
    await waitFor(() => expect(mocks.confirm).toHaveBeenCalledWith(preview, archived.route_id));
  });

  it("zeigt beim Replace den unverändert archivierten Vorgänger vor der einzigen Bestätigung", async () => {
    const candidate = { candidate_id: "candidate-1", run_id: "countess", character: "MrBones", difficulty: "nightmare", state: "test_passed", measured_boss_distance: 12, route_sha256: "a".repeat(64) };
    mocks.candidates.mockResolvedValue([candidate]);
    mocks.preview.mockResolvedValue({ operation: "replace", route_id: "countess-new", candidate_id: candidate.candidate_id, replaced_route_id: "countess-old", catalog_revision: 2, lifecycle_revision: 3, assignment_revision: 4, confirmation_token: "replace-once" });
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    fireEvent.click(await screen.findByRole("button", { name: "Veröffentlichen" }));
    expect(await screen.findByText(/bisher aktive Route/)).toHaveTextContent("countess-old");
    expect(screen.getByText(/unverändert archiviert und bleibt wiederherstellbar/)).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Änderung bestätigen" })).toHaveLength(1);
  });

  it("zeigt Fehlerzustände und veröffentlicht nur test_passed Kandidaten", async () => {
    mocks.library.mockRejectedValue(new Error("Katalog defekt"));
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    expect(await screen.findByRole("alert")).toHaveTextContent("Katalog defekt");
    cleanup();
    mocks.library.mockResolvedValue({ revision: 1, character: "MrBones", routes: [] });
    mocks.candidates.mockResolvedValue([{ candidate_id: "candidate-1", run_id: "countess", character: "MrBones", difficulty: "nightmare", state: "recorded", measured_boss_distance: 3, route_sha256: "a".repeat(64) }]);
    render(<RouteFeature characters={["MrBones"]} selectedCharacter="MrBones" refreshKey={0} />);
    expect(await screen.findByRole("button", { name: "Veröffentlichen" })).toBeDisabled();
  });
});
