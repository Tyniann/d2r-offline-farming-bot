import type { CatalogDTO, CharacterCatalogEntry } from "../api/generated";
import { presentClassName, type AppTranslator } from "../i18n/presenters";

export function characterClassLabel(characterClass: string, t: AppTranslator) {
  return presentClassName(characterClass, t);
}

export function supportedCharacterClasses(catalog: CatalogDTO, t: AppTranslator): string {
  const classes = [...new Set(catalog.profiles.map((profile) => presentClassName(profile.character_class, t)))];
  return classes.length ? classes.join(", ") : t("characters.noSupportedClasses");
}

export function characterReasonText(reason: string, catalog: CatalogDTO, t: AppTranslator): string {
  switch (reason) {
    case "character_save_missing":
      return t("characters.reasons.saveMissing");
    case "character_save_unreadable":
      return t("characters.reasons.saveUnreadable");
    case "character_save_header_invalid":
      return t("characters.reasons.saveHeaderInvalid");
    case "character_save_version_unsupported":
      return t("characters.reasons.saveVersionUnsupported");
    case "character_save_name_mismatch":
      return t("characters.reasons.saveNameMismatch");
    case "character_save_name_conflict":
      return t("characters.reasons.saveNameConflict");
    case "character_class_unknown":
      return t("characters.reasons.classUnknown");
    case "character_class_unsupported":
      return t("characters.reasons.classUnsupported", { classes: supportedCharacterClasses(catalog, t) });
    case "character_profile_missing":
      return t("characters.reasons.profileMissing");
    case "character_profile_incompatible":
      return t("characters.reasons.profileIncompatible");
    case "character_anchor_missing":
      return t("characters.reasons.anchorMissing");
    case "profile_bindings_incomplete":
      return t("characters.reasons.bindingsIncomplete");
    default:
      return t("characters.reasons.unavailable");
  }
}

export function characterAvailabilityText(entry: CharacterCatalogEntry, catalog: CatalogDTO, t: AppTranslator): string {
  return (entry.reasons ?? []).map((reason) => characterReasonText(reason, catalog, t)).join(" ");
}
