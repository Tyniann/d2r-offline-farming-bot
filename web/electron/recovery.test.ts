// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import { renderRecovery } from "./recovery.js";

describe("Recovery-Dokument", () => {
  it.each([
    ["de", "Core-Wiederherstellung erforderlich", "Der lokale Core hat nicht rechtzeitig geantwortet."],
    ["en", "Core recovery required", "The local Core did not respond in time."],
  ] as const)("zeigt dieselbe Reason-ID auf %s mit bereits aufgelösten Texten", (language, title, body) => {
    document.documentElement.innerHTML = '<head><title>D2R Offline Farming Bot</title></head><body><h1 id="title"></h1><p id="reason"></p></body>';
    const query = new URLSearchParams({ language, reason: "core_handshake_timeout", restarts: "1", title, body });

    renderRecovery(document, `?${query}`);

    expect(document.documentElement.lang).toBe(language);
    expect(document.body.dataset.reason).toBe("core_handshake_timeout");
    expect(document.body.dataset.restarts).toBe("1");
    expect(document.title).toBe(title);
    expect(document.querySelector("#title")?.textContent).toBe(title);
    expect(document.querySelector("#reason")?.textContent).toBe(body);
  });

  it("behält bei fehlenden Parametern die neutralen leeren Zielknoten", () => {
    document.documentElement.innerHTML = '<head><title>D2R Offline Farming Bot</title></head><body><h1 id="title"></h1><p id="reason"></p></body>';
    renderRecovery(document, "");
    expect(document.querySelector("#title")?.textContent).toBe("");
    expect(document.querySelector("#reason")?.textContent).toBe("");
  });
});
