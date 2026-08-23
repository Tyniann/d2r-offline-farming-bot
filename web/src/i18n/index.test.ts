import type { TFunction } from "i18next";
import { describe, expect, it } from "vitest";

import { changeAppLanguage, i18n, initializeI18n } from "./index";

describe("i18n-Grundmodul", () => {
  it("rendert denselben Schlüssel auf Deutsch und Englisch", async () => {
    await initializeI18n("de");
    expect(i18n.t("common.save")).toBe("Speichern");
    expect(i18n.t("common.runsCompleted", { count: 1 })).toBe("1 Routenausführung abgeschlossen");

    await changeAppLanguage("en");
    expect(i18n.t("common.save")).toBe("Save");
    expect(i18n.t("common.runsCompleted", { count: 2 })).toBe("2 route executions completed");
  });

  it("bricht im Testbetrieb bei unbekannten Schlüsseln ab", async () => {
    await initializeI18n("de");
    expect(() => i18n.t("missing.key" as never)).toThrow("Fehlender Übersetzungsschlüssel: missing.key");
  });

  it("typisiert bekannte Schlüssel und Interpolationswerte", () => {
    const compileContract = (t: TFunction<"translation">) => {
      t("common.save");
      t("common.runsCompleted", { count: 2 });
      // @ts-expect-error Unbekannte Katalogschlüssel müssen den Typecheck brechen.
      t("common.unknown");
    };
    expect(compileContract).toBeTypeOf("function");
  });
});
