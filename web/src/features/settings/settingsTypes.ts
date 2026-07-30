import type { RouteCombatConfigDTO } from "../../api/generated";

/** SettingsRun beschreibt einen Katalog-Run für den Queue-Editor. */
export type SettingsRun = {
  id: string;
  label: string;
  status?: string;
  reasons?: string[];
  routeCombat?: RouteCombatConfigDTO;
};

/** SettingsTab benennt die drei Scope-Bereiche der Einstellungen. */
export type SettingsTab = "farming" | "app" | "maintenance";
