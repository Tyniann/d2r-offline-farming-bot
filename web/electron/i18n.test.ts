// @vitest-environment node

import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { beforeAll, describe, expect, it } from "vitest";
import { createDesktopTranslator, desktopDialogText, desktopNotificationText, desktopRecoveryText, desktopTrayText, loadDesktopTranslators, type DesktopTranslator } from "./i18n.js";

let translators: Record<"de" | "en", DesktopTranslator>;

beforeAll(async () => {
  const moduleDirectory = dirname(fileURLToPath(import.meta.url));
  translators = await loadDesktopTranslators(join(moduleDirectory, "..", "src", "i18n", "locales"));
});

describe("Desktop-i18n", () => {
  it("übersetzt nur den Desktop-Namespace und interpoliert primitive Werte", () => {
    const translator = createDesktopTranslator({ desktop: { greeting: "Hello {{name}}: {{count}}" } });
    expect(translator.t("desktop.greeting", { name: "Deckard", count: 2 })).toBe("Hello Deckard: 2");
    expect(() => translator.t("app.title")).toThrow("outside the desktop namespace");
    expect(() => translator.t("desktop.missing")).toThrow("missing");
  });

  it.each([
    ["de", "Öffnen", "Status: Session läuft", "Beenden"],
    ["en", "Open", "Status: Session running", "Quit"],
  ] as const)("liefert Traylabels auf %s", (language, open, status, quit) => {
    expect(desktopTrayText(translators[language], "running")).toMatchObject({ open, status, quit });
  });

  it.each([
    ["de", "Session abgeschlossen", "Neue Version verfügbar"],
    ["en", "Session complete", "New version available"],
  ] as const)("liefert Notificationtitel auf %s", (language, completed, update) => {
    expect(desktopNotificationText(translators[language], "session_completed").title).toBe(completed);
    expect(desktopNotificationText(translators[language], "update_available").title).toBe(update);
  });

  it.each([
    ["de", ["Beenden", "Abbrechen"]],
    ["en", ["Quit", "Cancel"]],
  ] as const)("liefert Dialogbuttons auf %s", (language, buttons) => {
    expect(desktopDialogText(translators[language], "confirm_quit").buttons).toEqual(buttons);
  });

  it("löst dieselbe Recovery-Reason in beiden Sprachen auf", () => {
    expect(desktopRecoveryText(translators.de, "core_handshake_timeout", 1)).toEqual({
      title: "Core-Wiederherstellung erforderlich",
      body: "Der lokale Core hat nicht rechtzeitig geantwortet. Automatische Neustarts: 1. Es wurden keine weiteren Bot-Aktionen gestartet.",
    });
    expect(desktopRecoveryText(translators.en, "core_handshake_timeout", 1)).toEqual({
      title: "Core recovery required",
      body: "The local Core did not respond in time. Automatic restarts: 1. No further bot actions were started.",
    });
  });
});
