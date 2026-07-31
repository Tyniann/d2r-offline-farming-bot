export interface EquipmentTypeOption {
  label: string;
  codes: readonly string[];
}

export const equipmentTypeOptions: readonly EquipmentTypeOption[] = [
  { label: "Armbrüste", codes: ["xbow"] },
  { label: "Äxte", codes: ["axe"] },
  { label: "Bögen", codes: ["bow"] },
  { label: "Bögen – Amazone", codes: ["abow"] },
  { label: "Dolche", codes: ["knif"] },
  { label: "Hämmer", codes: ["hamm"] },
  { label: "Helme", codes: ["helm"] },
  { label: "Helme – Barbar", codes: ["phlm"] },
  { label: "Helme – Druide", codes: ["pelt"] },
  { label: "Helme – Reife", codes: ["circ"] },
  { label: "Keulen", codes: ["club"] },
  { label: "Klauen – Assassine", codes: ["h2h", "h2h2"] },
  { label: "Körperrüstungen", codes: ["tors"] },
  { label: "Orbs – Zauberin", codes: ["orb"] },
  { label: "Schilde", codes: ["shie"] },
  { label: "Schilde – Hexenmeister (Grimoires)", codes: ["grim"] },
  { label: "Schilde – Nekromant (Köpfe)", codes: ["head"] },
  { label: "Schilde – Paladin", codes: ["ashd"] },
  { label: "Schwerter", codes: ["swor"] },
  { label: "Speere", codes: ["spea"] },
  { label: "Speere – Amazone", codes: ["aspe"] },
  { label: "Stäbe", codes: ["staf"] },
  { label: "Stangenwaffen", codes: ["pole"] },
  { label: "Streitkolben", codes: ["mace"] },
  { label: "Szepter", codes: ["scep"] },
  { label: "Zauberstäbe", codes: ["wand"] },
].sort((left, right) => left.label.localeCompare(right.label, "de"));

export const socketOperators = ["==", "!=", ">", ">=", "<", "<="] as const;
export type SocketOperator = (typeof socketOperators)[number];

interface CombinedRuleInput {
  types: readonly EquipmentTypeOption[];
  tier: "" | "normal" | "exceptional" | "elite";
  socketsOperator: SocketOperator | "";
  sockets: string;
  ethereal: boolean;
}

export interface CombinedRuleResult {
  expression: string;
  errors: {
    types?: string;
    socketsOperator?: string;
    sockets?: string;
  };
}

export function buildCombinedRuleExpression(input: CombinedRuleInput): CombinedRuleResult {
  const errors: CombinedRuleResult["errors"] = {};
  if (input.types.length === 0) errors.types = "Mindestens einen Itemtyp auswählen.";
  if (!input.socketsOperator) errors.socketsOperator = "Sockeloperator auswählen.";

  const sockets = Number(input.sockets);
  if (!Number.isInteger(sockets) || sockets < 1 || sockets > 6 || input.sockets.trim() === "") {
    errors.sockets = "Sockelzahl muss eine ganze Zahl von 1 bis 6 sein.";
  }
  if (Object.keys(errors).length > 0) return { expression: "", errors };

  const typeConditions = [...new Set(input.types.flatMap((option) => option.codes))]
    .map((code) => `[type] == ${JSON.stringify(code)}`);
  const conditions = [
    typeConditions.length === 1 ? typeConditions[0] : `(${typeConditions.join(" || ")})`,
  ];
  if (input.tier) conditions.push(`[tier] == ${JSON.stringify(input.tier)}`);
  conditions.push(`[sockets] ${input.socketsOperator} ${sockets}`);
  if (input.ethereal) conditions.push("[flag] == ethereal");

  return { expression: conditions.join(" && "), errors };
}

export function missingEquipmentTypeCodes(catalogTypes: Iterable<string>): string[] {
  const available = new Set(catalogTypes);
  return [...new Set(equipmentTypeOptions.flatMap((option) => option.codes))]
    .filter((code) => !available.has(code));
}
