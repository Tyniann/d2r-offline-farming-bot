import { presentDifficultyName, presentRunName, type AppTranslator } from "../../i18n/presenters";

/** dashboardRunName resolves a stable run ID through the active application catalog. */
export function dashboardRunName(runID: string, t: AppTranslator): string { return presentRunName(runID.toLowerCase(), t); }

/** dashboardDifficultyName resolves a stable difficulty ID through the active application catalog. */
export function dashboardDifficultyName(difficulty: string, t: AppTranslator): string { return presentDifficultyName(difficulty.toLowerCase(), t); }
