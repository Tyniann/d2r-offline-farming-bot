import de from "./generated/game.de.json";
import en from "./generated/game.en.json";
import { resolveSupportedLanguage } from "./types";

type GameNames = typeof de;

function catalog(language: string | null | undefined): GameNames {
  return resolveSupportedLanguage(language) === "de" ? de : en;
}

/** gameAreaName resolves an area ID from generated CASC data and keeps a diagnostic fallback. */
export function gameAreaName(areaID: number, fallback: string, language: string | null | undefined): string {
  return catalog(language).areas[String(areaID) as keyof GameNames["areas"]] ?? fallback;
}

/** gameSkillName resolves a canonical skill key from generated CASC data. */
export function gameSkillName(skillKey: string, fallback: string, language: string | null | undefined): string {
  return catalog(language).skills[skillKey as keyof GameNames["skills"]] ?? (fallback || skillKey);
}

/** gameBaseItemName resolves a stable base code from generated CASC data. */
export function gameBaseItemName(baseCode: string, fallback: string, language: string | null | undefined): string {
  return catalog(language).items[baseCode as keyof GameNames["items"]] ?? (fallback || baseCode);
}

/** gameIdentityName resolves a stable unique or set identity key from generated CASC data. */
export function gameIdentityName(identityKey: string, fallback: string, language: string | null | undefined): string {
  const names = catalog(language).item_identities;
  const direct = names[identityKey as keyof typeof names];
  if (direct) return direct;
  const normalized = normalizeIdentityKey(identityKey);
  const match = Object.entries(names).find(([key]) => normalizeIdentityKey(key) === normalized);
  return match?.[1] ?? (fallback || identityKey);
}

/** gameSetName resolves a stable set key from generated CASC data. */
export function gameSetName(setKey: string, fallback: string, language: string | null | undefined): string {
  const names = catalog(language).item_sets;
  return names[setKey as keyof typeof names] ?? (fallback || setKey);
}

/** gameHistoryItemName prefers stable telemetry keys and retains old technical names as a neutral fallback. */
export function gameHistoryItemName(item: { item_key?: string; item_name?: string; base_code?: string; identity_key?: string }, language: string | null | undefined): string {
  const fallback = item.item_name || item.item_key || "–";
  if (item.identity_key) return gameIdentityName(item.identity_key, fallback, language);
  if (item.base_code) return gameBaseItemName(item.base_code, fallback, language);
  const baseCode = item.item_key?.match(/^base:([^:]+):/)?.[1];
  if (baseCode) return gameBaseItemName(baseCode, fallback, language);
  const identityKey = item.item_key?.match(/^(?:unique|set):(.+)$/)?.[1];
  return identityKey ? gameIdentityName(identityKey, fallback, language) : fallback;
}

function normalizeIdentityKey(value: string): string {
  return value.toLocaleLowerCase("en-US").replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}
