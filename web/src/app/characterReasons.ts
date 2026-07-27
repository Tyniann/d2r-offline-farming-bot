import type { CatalogDTO, CharacterCatalogEntry } from "../api/generated";

const classLabels: Record<string, string> = {
  amazon: "Amazone",
  assassin: "Assassine",
  barbarian: "Barbar",
  druid: "Druide",
  necromancer: "Totenbeschwörer",
  paladin: "Paladin",
  sorceress: "Zauberin",
};

export function supportedCharacterClasses(catalog: CatalogDTO): string {
  const classes = [...new Set(catalog.profiles.map((profile) => classLabels[profile.character_class] ?? profile.character_class))];
  return classes.length ? classes.join(", ") : "keine";
}

export function characterReasonText(reason: string, catalog: CatalogDTO): string {
  switch (reason) {
    case "character_save_missing":
      return "Lokaler Offline-Spielstand fehlt.";
    case "character_unconfigured":
      return `Kein unterstütztes Kampfprofil zugeordnet. Derzeit unterstützt: ${supportedCharacterClasses(catalog)}.`;
    case "character_anchor_missing":
      return "Automatische Auswahl dieses Charakters in D2R ist noch nicht eingerichtet.";
    default:
      return reason;
  }
}

export function characterAvailabilityText(entry: CharacterCatalogEntry, catalog: CatalogDTO): string {
  return (entry.reasons ?? []).map((reason) => characterReasonText(reason, catalog)).join(" ");
}
