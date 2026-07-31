import { describe, expect, it } from "vitest";
import {
  buildCombinedRuleExpression,
  equipmentTypeOptions,
  missingEquipmentTypeCodes,
  socketOperators,
} from "./pickitRuleBuilder";

const option = (label: string) => {
  const result = equipmentTypeOptions.find((entry) => entry.label === label);
  if (!result) throw new Error(`Testoption fehlt: ${label}`);
  return result;
};

describe("buildCombinedRuleExpression", () => {
  it("baut einen einzelnen Typ ohne Klammer", () => {
    expect(buildCombinedRuleExpression({
      types: [option("Stangenwaffen")],
      tier: "",
      socketsOperator: ">=",
      sockets: "1",
      ethereal: false,
    })).toEqual({ expression: `[type] == "pole" && [sockets] >= 1`, errors: {} });
  });

  it("baut mehrere Typen in fester Reihenfolge", () => {
    expect(buildCombinedRuleExpression({
      types: [option("Schilde"), option("Schilde – Paladin")],
      tier: "elite",
      socketsOperator: "==",
      sockets: "4",
      ethereal: true,
    }).expression).toBe(`([type] == "shie" || [type] == "ashd") && [tier] == "elite" && [sockets] == 4 && [flag] == ethereal`);
  });

  it("expandiert Assassinenklauen sichtbar", () => {
    expect(buildCombinedRuleExpression({
      types: [option("Klauen – Assassine")],
      tier: "exceptional",
      socketsOperator: "<=",
      sockets: "6",
      ethereal: false,
    }).expression).toBe(`([type] == "h2h" || [type] == "h2h2") && [tier] == "exceptional" && [sockets] <= 6`);
  });

  it.each(socketOperators)("unterstützt den Operator %s", (socketsOperator) => {
    expect(buildCombinedRuleExpression({
      types: [option("Helme")],
      tier: "normal",
      socketsOperator,
      sockets: "2",
      ethereal: false,
    }).errors).toEqual({});
  });

  it.each(["", "0", "7", "1.5", "keine"])("lehnt die Sockelzahl %j ab", (sockets) => {
    expect(buildCombinedRuleExpression({
      types: [option("Helme")],
      tier: "",
      socketsOperator: "==",
      sockets,
      ethereal: false,
    }).errors.sockets).toBe("Sockelzahl muss eine ganze Zahl von 1 bis 6 sein.");
  });

  it("liefert deutsche Pflichtfeldfehler", () => {
    expect(buildCombinedRuleExpression({
      types: [],
      tier: "",
      socketsOperator: "",
      sockets: "",
      ethereal: false,
    })).toEqual({
      expression: "",
      errors: {
        types: "Mindestens einen Itemtyp auswählen.",
        socketsOperator: "Sockeloperator auswählen.",
        sockets: "Sockelzahl muss eine ganze Zahl von 1 bis 6 sein.",
      },
    });
  });
});

describe("equipmentTypeOptions", () => {
  it("verwendet nur Typcodes, die der API-Katalog liefert", () => {
    const catalogTypes = [
      "abow", "ashd", "aspe", "axe", "bow", "circ", "club", "grim", "h2h", "h2h2",
      "hamm", "head", "helm", "knif", "mace", "orb", "pelt", "phlm", "pole",
      "scep", "shie", "spea", "staf", "swor", "tors", "wand", "xbow",
    ];
    const apiCatalog = {
      bases: catalogTypes.map((type, txt_file_no) => ({ txt_file_no, code: `base-${type}`, name: type, type, base_tier: "normal" })),
    };
    expect(missingEquipmentTypeCodes(apiCatalog.bases.map((base) => base.type))).toEqual([]);
  });

  it("ist deutsch alphabetisch sortiert", () => {
    const labels = equipmentTypeOptions.map((entry) => entry.label);
    expect(labels).toEqual([...labels].sort((left, right) => left.localeCompare(right, "de")));
  });

  it("ordnet Grimoires dem Hexenmeister und Köpfe dem Nekromanten zu", () => {
    expect(option("Schilde – Hexenmeister (Grimoires)").codes).toEqual(["grim"]);
    expect(option("Schilde – Nekromant (Köpfe)").codes).toEqual(["head"]);
    expect(equipmentTypeOptions.some((entry) => entry.label.includes("Nekromant") && entry.codes.includes("grim"))).toBe(false);
  });
});
