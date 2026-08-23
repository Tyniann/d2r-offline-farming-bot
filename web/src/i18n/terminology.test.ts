import { describe, expect, it } from "vitest";

import de from "./locales/de.json";
import en from "./locales/en.json";

describe("sichtbares Fachvokabular", () => {
  it.each([
    ["de", de],
    ["en", en],
  ] as const)("verwendet in %s kein alleinstehendes Run oder runs", (_language, translations) => {
    const violations = collectStrings(translations)
      .filter(({ value }) => /\bruns?\b/i.test(value.replace(/\{\{[^}]+\}\}/g, "")))
      .map(({ path, value }) => `${path}: ${value}`);

    expect(violations).toEqual([]);
  });
});

function collectStrings(value: unknown, path = ""): Array<{ path: string; value: string }> {
  if (typeof value === "string") return [{ path, value }];
  if (!value || typeof value !== "object") return [];
  return Object.entries(value).flatMap(([key, child]) => collectStrings(child, path ? `${path}.${key}` : key));
}
