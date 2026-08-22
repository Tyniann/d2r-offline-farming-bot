import { beforeEach, describe, expect, it } from "vitest";

import { formatBytes, formatDate, formatDuration, formatNumber, formatPercent } from "./format";
import { changeAppLanguage, initializeI18n } from "./index";
import { localeForLanguage, resolveSupportedLanguage } from "./types";

describe("Locale-Auflösung", () => {
  it.each([
    ["de-AT", "de"],
    ["de-DE", "de"],
    ["en-GB", "en"],
    ["en-US", "en"],
    ["fr-FR", "en"],
  ] as const)("reduziert %s auf %s", (input, expected) => {
    expect(resolveSupportedLanguage(input)).toBe(expected);
  });

  it("ordnet nur die zwei Produkt-Locale-Codes zu", () => {
    expect(localeForLanguage("de")).toBe("de-DE");
    expect(localeForLanguage("en")).toBe("en-US");
  });
});

describe("lokale Formatierung", () => {
  beforeEach(async () => initializeI18n("de"));

  it("formatiert Datum, Zahl, Prozent, Byte und Dauer auf Deutsch", () => {
    expect(formatDate("2026-08-22T12:00:00Z", { dateStyle: "medium", timeZone: "UTC" })).toBe("22.08.2026");
    expect(formatNumber(1234.5)).toBe("1.234,5");
    expect(formatPercent(0.125)).toBe("12,5 %");
    expect(formatBytes(1536)).toBe("1,5 KB");
    expect(formatDuration(65_000)).toBe("1 Min. 5 Sek.");
  });

  it("formatiert Datum, Zahl, Prozent, Byte und Dauer auf Englisch", async () => {
    await changeAppLanguage("en");
    expect(formatDate("2026-08-22T12:00:00Z", { dateStyle: "medium", timeZone: "UTC" })).toBe("Aug 22, 2026");
    expect(formatNumber(1234.5)).toBe("1,234.5");
    expect(formatPercent(0.125)).toBe("12.5%");
    expect(formatBytes(1536)).toBe("1.5 KB");
    expect(formatDuration(65_000)).toBe("1 min 5 sec");
  });
});
