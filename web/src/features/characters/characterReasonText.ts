import type { CatalogDTO, CharacterCatalogEntry } from "../../api/generated";
import { characterReasonText } from "../../app/characterReasons";
import type { AppTranslator } from "../../i18n/presenters";

/** farmReadyReasonText erklärt Core-Farm-Readiness ohne Rohcodes. */
export function farmReadyReasonText(reason: string, t: AppTranslator): string {
  switch (reason) {
    case "profile_bindings_incomplete":
      return t("characters.reasonBindings");
    case "character_inventory_unconfigured":
      return t("characters.reasonInventory");
    default:
      return t("characters.reasonNotReady");
  }
}

/** characterStatusLabel fasst selectable und farm_ready für die Charaktere-Liste zusammen. */
export function characterStatusLabel(entry: CharacterCatalogEntry | undefined, catalog: CatalogDTO | null, t: AppTranslator): string {
  if (!entry) return t("characters.statusUnknown");
  if (!entry.selectable) {
    if ((entry.reasons ?? []).includes("character_class_unsupported")) return t("characters.statusUnsupported");
    if ((entry.reasons ?? []).includes("character_profile_missing") || (entry.reasons ?? []).includes("character_anchor_missing")) {
      return t("characters.statusSetupMissing");
    }
    if (catalog) {
      const detail = (entry.reasons ?? []).map((reason) => characterReasonText(reason, catalog, t)).join(" ");
      return detail || t("characters.statusUnavailable");
    }
    return t("characters.statusUnavailable");
  }
  if (!entry.farm_ready) {
    const reasons = entry.farm_ready_reasons ?? [];
    if (reasons.includes("profile_bindings_incomplete")) return t("characters.statusKeysMissing");
    if (reasons.includes("character_inventory_unconfigured")) return t("characters.statusInventoryMissing");
    return t("characters.statusNotReady");
  }
  return t("characters.statusReady");
}
