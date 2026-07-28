import { afterEach, expect, it, vi } from "vitest";
import {
  captureCharacterSelection,
  confirmCharacterSetup,
  previewCharacterSetup,
  reloadCharacters,
} from "./generated";

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("schützt nur Confirm und Capture mit dem Control-Token", async () => {
  const response = new Response(JSON.stringify({ schema_version: 1 }), { status: 200, headers: { "Content-Type": "application/json" } });
  const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => response.clone());
  vi.stubGlobal("fetch", fetchMock);

  await reloadCharacters();
  await previewCharacterSetup({ character: "MrBones" });
  await confirmCharacterSetup({
    command_id: "setup-1", character: "MrBones", expected_catalog_revision: 1,
    expected_operator_settings_revision: 1, expected_pickit_assignment_revision: 1, expected_generation: 0,
  }, "control");
  await captureCharacterSelection({
    command_id: "capture-1", character: "MrBones", expected_catalog_revision: 1, expected_generation: 0,
  }, "control");

  expect(fetchMock.mock.calls[0][1]?.headers).not.toHaveProperty("X-D2RBot-Control-Token");
  expect(fetchMock.mock.calls[1][1]?.headers).not.toHaveProperty("X-D2RBot-Control-Token");
  expect(fetchMock.mock.calls[2][1]?.headers).toMatchObject({ "X-D2RBot-Control-Token": "control" });
  expect(fetchMock.mock.calls[3][1]?.headers).toMatchObject({ "X-D2RBot-Control-Token": "control" });
});
