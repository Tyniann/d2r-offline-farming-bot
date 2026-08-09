import type { OperatorSettingsDTO } from "../../api/generated";

const fieldLabels: Array<{ pattern: RegExp; label: (match: RegExpMatchArray) => string }> = [
  { pattern: /^characters\.([^.]+)\.queue$/, label: (m) => `Run-Reihenfolge (${m[1]})` },
  { pattern: /^characters\.([^.]+)\.last_difficulty$/, label: (m) => `Letzte Schwierigkeit (${m[1]})` },
  { pattern: /^characters\.([^.]+)\.profile_bindings$/, label: (m) => `Tastenbelegung (${m[1]})` },
  { pattern: /^characters\.([^.]+)\.inventory_lock$/, label: (m) => `Inventarschutz (${m[1]})` },
  { pattern: /^budgets\.max_runs$/, label: () => "Maximale Runs" },
  { pattern: /^budgets\.max_duration_ms$/, label: () => "Maximale Dauer" },
  { pattern: /^budgets\.max_consecutive_failures$/, label: () => "Fehler in Folge" },
  { pattern: /^budgets\.max_total_restarts$/, label: () => "Gesamte Restarts" },
  { pattern: /^input\.enabled$/, label: () => "Gameplay-Input" },
  { pattern: /^input\.pause_hotkey$/, label: () => "Pause-Hotkey" },
  { pattern: /^input\.stop_after_run_hotkey$/, label: () => "Stop-nach-Run-Hotkey" },
  { pattern: /^input\.recording_finish_hotkey$/, label: () => "Aufnahme-Hotkey" },
  { pattern: /^input\.emergency_stop_hotkey$/, label: () => "Emergency-Stop-Hotkey" },
  { pattern: /^history\.retention_enabled$/, label: () => "Aufbewahrung aktiv" },
  { pattern: /^history\.retention_days$/, label: () => "Aufbewahrung" },
];

/** cloneSettings kopiert den Operator-Vertrag für Draft-Mutationen. */
export function cloneSettings(settings: OperatorSettingsDTO): OperatorSettingsDTO {
  return JSON.parse(JSON.stringify(settings)) as OperatorSettingsDTO;
}

/** settingsEqual prüft strukturelle Gleichheit von Draft und gespeichertem Stand. */
export function settingsEqual(left: OperatorSettingsDTO, right: OperatorSettingsDTO): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

/** labelChangedField übersetzt Core-Feldpfade in deutsche Bedienertexte. */
export function labelChangedField(path: string): string {
  for (const entry of fieldLabels) {
    const match = path.match(entry.pattern);
    if (match) return entry.label(match);
  }
  return path;
}

/** summarizeChangedFields liefert eine kurze deutsche Zusammenfassung. */
export function summarizeChangedFields(paths: string[]): string {
  if (!paths.length) return "keine";
  const labels = [...new Set(paths.map(labelChangedField))];
  return labels.join(", ");
}

/** collectLocalDiffPaths vergleicht Draft und gespeicherten Stand lokal. */
export function collectLocalDiffPaths(saved: OperatorSettingsDTO, draft: OperatorSettingsDTO): string[] {
  const paths: string[] = [];
  for (const [name, character] of Object.entries(draft.characters)) {
    const baseline = saved.characters[name];
    if (!baseline) {
      paths.push(`characters.${name}.queue`);
      continue;
    }
    if (character.last_difficulty !== baseline.last_difficulty) paths.push(`characters.${name}.last_difficulty`);
    if (JSON.stringify(character.queue) !== JSON.stringify(baseline.queue)) paths.push(`characters.${name}.queue`);
    if (JSON.stringify(character.profile_bindings ?? {}) !== JSON.stringify(baseline.profile_bindings ?? {})) {
      paths.push(`characters.${name}.profile_bindings`);
    }
    if (JSON.stringify(character.inventory_lock ?? null) !== JSON.stringify(baseline.inventory_lock ?? null)) {
      paths.push(`characters.${name}.inventory_lock`);
    }  }
  if (draft.budgets.max_runs !== saved.budgets.max_runs) paths.push("budgets.max_runs");
  if (draft.budgets.max_duration_ms !== saved.budgets.max_duration_ms) paths.push("budgets.max_duration_ms");
  if (draft.budgets.max_consecutive_failures !== saved.budgets.max_consecutive_failures) paths.push("budgets.max_consecutive_failures");
  if (draft.budgets.max_total_restarts !== saved.budgets.max_total_restarts) paths.push("budgets.max_total_restarts");
  if (draft.input.enabled !== saved.input.enabled) paths.push("input.enabled");
  if (draft.input.pause_hotkey !== saved.input.pause_hotkey) paths.push("input.pause_hotkey");
  if (draft.input.stop_after_run_hotkey !== saved.input.stop_after_run_hotkey) paths.push("input.stop_after_run_hotkey");
  if (draft.input.recording_finish_hotkey !== saved.input.recording_finish_hotkey) paths.push("input.recording_finish_hotkey");
  if (draft.input.emergency_stop_hotkey !== saved.input.emergency_stop_hotkey) paths.push("input.emergency_stop_hotkey");
  if (draft.history.retention_enabled !== saved.history.retention_enabled) paths.push("history.retention_enabled");
  if (draft.history.retention_days !== saved.history.retention_days) paths.push("history.retention_days");
  return paths;
}

/** pathChanged prüft, ob ein Feldpfad oder Präfix lokal geändert ist. */
export function pathChanged(paths: string[], prefix: string): boolean {
  return paths.some((path) => path === prefix || path.startsWith(`${prefix}.`) || path.startsWith(prefix));
}

/** msToMinutes wandelt Budget-Millisekunden in ganze Minuten um. */
export function msToMinutes(ms: number): number {
  return Math.max(1, Math.round(ms / 60_000));
}

/** minutesToMs wandelt Minuten zurück in Millisekunden. */
export function minutesToMs(minutes: number): number {
  return Math.max(1, Math.round(minutes)) * 60_000;
}

/** move verschiebt ein Listenelement von from nach to. */
export function move<T>(entries: T[], from: number, to: number): T[] {
  if (to < 0 || to >= entries.length || from === to) return entries;
  const result = [...entries];
  const [entry] = result.splice(from, 1);
  result.splice(to, 0, entry);
  return result;
}
