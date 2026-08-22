import { describe, expect, it } from "vitest";

import de from "./locales/de.json";
import en from "./locales/en.json";

type CatalogLeaf = { path: string; type: string };

function catalogLeaves(value: unknown, prefix = ""): CatalogLeaf[] {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return [{ path: prefix, type: Array.isArray(value) ? "array" : typeof value }];
  }
  return Object.entries(value).flatMap(([key, child]) => catalogLeaves(child, prefix ? `${prefix}.${key}` : key));
}

function catalogDifference(left: unknown, right: unknown): CatalogLeaf[] {
  const rightLeaves = new Map(catalogLeaves(right).map((leaf) => [leaf.path, leaf.type]));
  return catalogLeaves(left).filter((leaf) => rightLeaves.get(leaf.path) !== leaf.type);
}

describe("Sprachkatalogvertrag", () => {
  it("hält Blattpfade und Blatttypen in Deutsch und Englisch identisch", () => {
    expect(catalogDifference(de, en)).toEqual([]);
    expect(catalogDifference(en, de)).toEqual([]);
  });

  it("erkennt fehlende, zusätzliche und typfalsche Blätter", () => {
    expect(catalogDifference({ common: { save: "Speichern" } }, { common: {} })).toEqual([
      { path: "common.save", type: "string" },
    ]);
    expect(catalogDifference({ common: {} }, { common: { save: "Save" } })).toEqual([]);
    expect(catalogDifference({ common: { save: "Speichern" } }, { common: { save: 1 } })).toEqual([
      { path: "common.save", type: "string" },
    ]);
  });
});
