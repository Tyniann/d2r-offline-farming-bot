import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { i18n, initializeI18n } from "../i18n";
import { LanguageSwitcher } from "./LanguageSwitcher";

afterEach(() => {
  cleanup();
  window.d2rDesktop = undefined;
});

describe("LanguageSwitcher", () => {
  beforeEach(async () => initializeI18n("de"));

  it("persistiert Englisch vor dem sichtbaren Wechsel und benötigt keinen Reload", async () => {
    const calls: string[] = [];
    const updateDesktopSettings = vi.fn(async () => {
      calls.push("persist");
      expect(i18n.resolvedLanguage).toBe("de");
      return { schema_version: 3, language: "en" as const, autostart: false, onboarding_completed: true };
    });
    window.d2rDesktop = { updateDesktopSettings } as unknown as D2RDesktopBridge;

    render(<LanguageSwitcher />);
    fireEvent.click(screen.getByRole("button", { name: "Sprache zu Englisch wechseln" }));

    await waitFor(() => expect(screen.getByRole("group", { name: "Language" })).toBeInTheDocument());
    expect(updateDesktopSettings).toHaveBeenCalledWith({ language: "en" });
    expect(calls).toEqual(["persist"]);
    expect(screen.getByRole("button", { name: "Switch language to English" })).toHaveAttribute("aria-pressed", "true");
    expect(document.documentElement.lang).toBe("en");
  });

  it("behält nach einem Persistenzfehler Deutsch aktiv", async () => {
    window.d2rDesktop = {
      updateDesktopSettings: vi.fn().mockRejectedValue(new Error("write failed")),
    } as unknown as D2RDesktopBridge;

    render(<LanguageSwitcher />);
    fireEvent.click(screen.getByRole("button", { name: "Sprache zu Englisch wechseln" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Die Sprache konnte nicht gespeichert werden.");
    expect(i18n.resolvedLanguage).toBe("de");
    expect(screen.getByRole("button", { name: "Sprache zu Deutsch wechseln" })).toHaveAttribute("aria-pressed", "true");
  });

  it("wechselt im Browsermodus nur für die aktuelle Sitzung", async () => {
    render(<LanguageSwitcher />);
    fireEvent.click(screen.getByRole("button", { name: "Sprache zu Englisch wechseln" }));
    await waitFor(() => expect(i18n.resolvedLanguage).toBe("en"));
  });
});
