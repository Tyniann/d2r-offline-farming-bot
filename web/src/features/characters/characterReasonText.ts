import type { CatalogDTO, CharacterCatalogEntry } from "../../api/generated";
import { characterReasonText } from "../../app/characterReasons";

/** farmReadyReasonText erklärt Core-Farm-Readiness ohne Rohcodes. */
export function farmReadyReasonText(reason: string): string {
  switch (reason) {
    case "profile_bindings_incomplete":
      return "Für dieses Kampfprofil fehlen Tastenbelegungen.";
    case "character_inventory_unconfigured":
      return "Der Inventarschutz wurde noch nicht bestätigt.";
    default:
      return "Dieser Charakter ist noch nicht farmbereit.";
  }
}

/** characterStatusLabel fasst selectable und farm_ready für die Charaktere-Liste zusammen. */
export function characterStatusLabel(entry: CharacterCatalogEntry | undefined, catalog: CatalogDTO | null): string {
  if (!entry) return "Unbekannt";
  if (!entry.selectable) {
    if ((entry.reasons ?? []).includes("character_class_unsupported")) return "Nicht unterstützt";
    if ((entry.reasons ?? []).includes("character_profile_missing") || (entry.reasons ?? []).includes("character_anchor_missing")) {
      return "Einrichtung fehlt";
    }
    if (catalog) {
      const detail = (entry.reasons ?? []).map((reason) => characterReasonText(reason, catalog)).join(" ");
      return detail || "Nicht verfügbar";
    }
    return "Nicht verfügbar";
  }
  if (!entry.farm_ready) {
    const reasons = entry.farm_ready_reasons ?? [];
    if (reasons.includes("profile_bindings_incomplete")) return "Tasten fehlen";
    if (reasons.includes("character_inventory_unconfigured")) return "Inventar fehlt";
    return "Noch nicht farmbereit";
  }
  return "Farmbereit";
}
