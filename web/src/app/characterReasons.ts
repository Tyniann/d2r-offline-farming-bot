import type { CatalogDTO, CharacterCatalogEntry } from "../api/generated";

const classLabels: Record<string, string> = {
  amazon: "Amazone",
  assassin: "Assassine",
  barbarian: "Barbar",
  druid: "Druide",
  necromancer: "Totenbeschwörer",
  paladin: "Paladin",
  sorceress: "Zauberin",
  warlock: "Hexenmeister",
};

export function characterClassLabel(characterClass: string) {
  return classLabels[characterClass] ?? "";
}

export function supportedCharacterClasses(catalog: CatalogDTO): string {
  const classes = [...new Set(catalog.profiles.map((profile) => classLabels[profile.character_class] ?? profile.character_class))];
  return classes.length ? classes.join(", ") : "keine";
}

export function characterReasonText(reason: string, catalog: CatalogDTO): string {
  switch (reason) {
    case "character_save_missing":
      return "Lokaler Offline-Spielstand fehlt.";
    case "character_save_unreadable":
      return "Der lokale Offline-Spielstand kann nicht sicher gelesen werden.";
    case "character_save_header_invalid":
      return "Der lokale Offline-Spielstand hat keinen gültigen D2R-Kopf.";
    case "character_save_version_unsupported":
      return "Diese Spielstandsversion wird noch nicht unterstützt.";
    case "character_save_name_mismatch":
      return "Dateiname und Charaktername im Spielstand stimmen nicht überein.";
    case "character_save_name_conflict":
      return "Mehrere Spielstände verwenden denselben Charakternamen.";
    case "character_class_unknown":
      return "Die Charakterklasse dieses Spielstands ist unbekannt.";
    case "character_class_unsupported":
      return `Für diese Klasse gibt es noch kein freigegebenes Kampfprofil. Derzeit unterstützt: ${supportedCharacterClasses(catalog)}.`;
    case "character_profile_missing":
      return "Das feste Kampfprofil für diesen Charakter muss noch bestätigt werden.";
    case "character_profile_incompatible":
      return "Das gespeicherte Kampfprofil passt nicht zur Charakterklasse.";
    case "character_anchor_missing":
      return "Das Auswahlbild für diesen Charakter fehlt noch.";
    case "profile_bindings_incomplete":
      return "Für dieses Kampfprofil fehlen Tastenbelegungen.";
    default:
      return "Dieser Charakter ist derzeit nicht verfügbar.";
  }
}

export function characterAvailabilityText(entry: CharacterCatalogEntry, catalog: CatalogDTO): string {
  return (entry.reasons ?? []).map((reason) => characterReasonText(reason, catalog)).join(" ");
}
