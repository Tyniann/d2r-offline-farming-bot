import type { AppTranslator } from "../../i18n/presenters";

const equipmentTypeTranslationKeys = {
  crossbows: "pickit.equipmentTypes.crossbows",
  axes: "pickit.equipmentTypes.axes",
  bows: "pickit.equipmentTypes.bows",
  amazonBows: "pickit.equipmentTypes.amazonBows",
  daggers: "pickit.equipmentTypes.daggers",
  hammers: "pickit.equipmentTypes.hammers",
  helms: "pickit.equipmentTypes.helms",
  barbarianHelms: "pickit.equipmentTypes.barbarianHelms",
  druidHelms: "pickit.equipmentTypes.druidHelms",
  circlets: "pickit.equipmentTypes.circlets",
  clubs: "pickit.equipmentTypes.clubs",
  assassinClaws: "pickit.equipmentTypes.assassinClaws",
  bodyArmor: "pickit.equipmentTypes.bodyArmor",
  sorceressOrbs: "pickit.equipmentTypes.sorceressOrbs",
  shields: "pickit.equipmentTypes.shields",
  warlockGrimoires: "pickit.equipmentTypes.warlockGrimoires",
  necromancerHeads: "pickit.equipmentTypes.necromancerHeads",
  paladinShields: "pickit.equipmentTypes.paladinShields",
  swords: "pickit.equipmentTypes.swords",
  spears: "pickit.equipmentTypes.spears",
  amazonSpears: "pickit.equipmentTypes.amazonSpears",
  staves: "pickit.equipmentTypes.staves",
  polearms: "pickit.equipmentTypes.polearms",
  maces: "pickit.equipmentTypes.maces",
  scepters: "pickit.equipmentTypes.scepters",
  wands: "pickit.equipmentTypes.wands",
} as const;

export type EquipmentTypeID = keyof typeof equipmentTypeTranslationKeys;

export interface EquipmentTypeOption {
  id: EquipmentTypeID;
  codes: readonly string[];
}

export const equipmentTypeOptions: readonly EquipmentTypeOption[] = [
  { id: "crossbows", codes: ["xbow"] },
  { id: "axes", codes: ["axe"] },
  { id: "bows", codes: ["bow"] },
  { id: "amazonBows", codes: ["abow"] },
  { id: "daggers", codes: ["knif"] },
  { id: "hammers", codes: ["hamm"] },
  { id: "helms", codes: ["helm"] },
  { id: "barbarianHelms", codes: ["phlm"] },
  { id: "druidHelms", codes: ["pelt"] },
  { id: "circlets", codes: ["circ"] },
  { id: "clubs", codes: ["club"] },
  { id: "assassinClaws", codes: ["h2h", "h2h2"] },
  { id: "bodyArmor", codes: ["tors"] },
  { id: "sorceressOrbs", codes: ["orb"] },
  { id: "shields", codes: ["shie"] },
  { id: "warlockGrimoires", codes: ["grim"] },
  { id: "necromancerHeads", codes: ["head"] },
  { id: "paladinShields", codes: ["ashd"] },
  { id: "swords", codes: ["swor"] },
  { id: "spears", codes: ["spea"] },
  { id: "amazonSpears", codes: ["aspe"] },
  { id: "staves", codes: ["staf"] },
  { id: "polearms", codes: ["pole"] },
  { id: "maces", codes: ["mace"] },
  { id: "scepters", codes: ["scep"] },
  { id: "wands", codes: ["wand"] },
];

export const socketOperators = ["==", "!=", ">", ">=", "<", "<="] as const;
export type SocketOperator = (typeof socketOperators)[number];

export function equipmentTypeLabel(id: EquipmentTypeID, t: AppTranslator): string {
  return t(equipmentTypeTranslationKeys[id]);
}

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
  if (input.types.length === 0) errors.types = i18n.t("pickit.typeRequired");

  // Empty operator and count omit the socket clause ("any"). Count 0 is
  // unsocketed (`[sockets] == 0`); 1–6 are exact totals.
  const omitSockets = !input.socketsOperator && input.sockets.trim() === "";
  let sockets = 0;
  if (!omitSockets) {
    if (!input.socketsOperator) errors.socketsOperator = i18n.t("pickit.socketOperatorRequired");
    sockets = Number(input.sockets);
    if (!Number.isInteger(sockets) || sockets < 0 || sockets > 6 || input.sockets.trim() === "") {
      errors.sockets = i18n.t("pickit.socketCountInvalid");
    }
  }
  if (Object.keys(errors).length > 0) return { expression: "", errors };

  const typeConditions = [...new Set(input.types.flatMap((option) => option.codes))]
    .map((code) => `[type] == ${JSON.stringify(code)}`);
  const conditions = [
    typeConditions.length === 1 ? typeConditions[0] : `(${typeConditions.join(" || ")})`,
  ];
  if (input.tier) conditions.push(`[tier] == ${JSON.stringify(input.tier)}`);
  if (!omitSockets) conditions.push(`[sockets] ${input.socketsOperator} ${sockets}`);
  if (input.ethereal) conditions.push("[flag] == ethereal");

  return { expression: conditions.join(" && "), errors };
}

export function missingEquipmentTypeCodes(catalogTypes: Iterable<string>): string[] {
  const available = new Set(catalogTypes);
  return [...new Set(equipmentTypeOptions.flatMap((option) => option.codes))]
    .filter((code) => !available.has(code));
}
import { i18n } from "../../i18n";
