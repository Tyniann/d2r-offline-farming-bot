import { randomUUID } from "node:crypto";
import { mkdir, open, readFile, rename, rm } from "node:fs/promises";
import { dirname, isAbsolute } from "node:path";

export const DESKTOP_SETTINGS_SCHEMA_VERSION = 2;

export interface WindowBounds {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface DesktopSettings {
  schema_version: typeof DESKTOP_SETTINGS_SCHEMA_VERSION;
  window_bounds?: WindowBounds;
  autostart: boolean;
  onboarding_completed: boolean;
  selected_character?: string;
  selected_difficulty?: string;
}

export type DesktopSettingsUpdate = Partial<Pick<DesktopSettings, "autostart" | "onboarding_completed" | "selected_character" | "selected_difficulty">>;

export interface LoadedDesktopSettings {
  settings: DesktopSettings;
  recovered: boolean;
}

export interface DesktopSettingsStoreHooks {
  beforeReplace?: () => void | Promise<void>;
}

const DEFAULT_SETTINGS: Readonly<DesktopSettings> = Object.freeze({
  schema_version: DESKTOP_SETTINGS_SCHEMA_VERSION,
  autostart: false,
  onboarding_completed: false,
});

// DesktopSettingsStore gehört ausschließlich dem Electron-Main-Prozess. Fach- und
// Safetywerte werden absichtlich nicht in diesen kleinen Desktopvertrag aufgenommen.
export class DesktopSettingsStore {
  readonly #path: string;
  readonly #hooks: DesktopSettingsStoreHooks;

  constructor(path: string, hooks: DesktopSettingsStoreHooks = {}) {
    if (!isAbsolute(path)) {
      throw new Error("Der Pfad für Desktop-Einstellungen muss absolut sein.");
    }
    this.#path = path;
    this.#hooks = hooks;
  }

  async load(): Promise<LoadedDesktopSettings> {
    try {
      const raw = JSON.parse(await readFile(this.#path, "utf8")) as unknown;
      return { settings: parseDesktopSettings(raw), recovered: false };
    } catch (error) {
      if (isMissingFile(error)) {
        return { settings: desktopSettingsDefaults(), recovered: false };
      }
      // Beschädigte, unbekannte oder unzulässige Inhalte werden nie teilweise
      // übernommen. Insbesondere bleibt Autostart nach einer Recovery deaktiviert.
      return { settings: desktopSettingsDefaults(), recovered: true };
    }
  }

  async save(value: DesktopSettings): Promise<DesktopSettings> {
    const settings = parseDesktopSettings(value);
    const directory = dirname(this.#path);
    const temporaryPath = `${this.#path}.${randomUUID()}.tmp`;
    await mkdir(directory, { recursive: true });

    let handle;
    try {
      handle = await open(temporaryPath, "wx", 0o600);
      await handle.writeFile(`${JSON.stringify(settings, null, 2)}\n`, "utf8");
      await handle.sync();
      await handle.close();
      handle = undefined;
      await this.#hooks.beforeReplace?.();
      await rename(temporaryPath, this.#path);

      // Der Re-Read stellt sicher, dass nur der tatsächlich persistierte Vertrag
      // als effektiver Desktopzustand an den Aufrufer zurückgegeben wird.
      return parseDesktopSettings(JSON.parse(await readFile(this.#path, "utf8")) as unknown);
    } finally {
      await handle?.close().catch(() => undefined);
      await rm(temporaryPath, { force: true }).catch(() => undefined);
    }
  }
}

export function desktopSettingsDefaults(): DesktopSettings {
  return { ...DEFAULT_SETTINGS };
}

export function parseDesktopSettings(value: unknown): DesktopSettings {
  if (!isRecord(value)) {
    throw new Error("Desktop-Einstellungen müssen ein JSON-Objekt sein.");
  }
  if (value.schema_version === 1) {
    requireExactKeys(
      value,
      ["schema_version", "window_bounds", "autostart", "onboarding_completed"],
      ["schema_version", "autostart", "onboarding_completed"],
    );
    return parseKnownDesktopSettings(value);
  }
  requireExactKeys(
    value,
    ["schema_version", "window_bounds", "autostart", "onboarding_completed", "selected_character", "selected_difficulty"],
    ["schema_version", "autostart", "onboarding_completed"],
  );
  if (value.schema_version !== DESKTOP_SETTINGS_SCHEMA_VERSION) {
    throw new Error("Unbekannte Desktop-Einstellungsversion.");
  }
  return parseKnownDesktopSettings(value);
}

export function parseDesktopSettingsUpdate(value: unknown): DesktopSettingsUpdate {
  if (!isRecord(value)) throw new Error("Desktop-Einstellungen müssen ein Objekt sein.");
  requireExactKeys(value, ["autostart", "onboarding_completed", "selected_character", "selected_difficulty"], []);
  if (Object.keys(value).length === 0) throw new Error("Desktop-Einstellungen enthalten keine Änderung.");
  if ("autostart" in value && typeof value.autostart !== "boolean") throw new Error("Autostart muss boolesch sein.");
  if ("onboarding_completed" in value && typeof value.onboarding_completed !== "boolean") throw new Error("Onboarding muss boolesch sein.");

  const update: DesktopSettingsUpdate = {};
  if ("autostart" in value) update.autostart = value.autostart as boolean;
  if ("onboarding_completed" in value) update.onboarding_completed = value.onboarding_completed as boolean;
  if ("selected_character" in value) update.selected_character = parsePreference(value.selected_character, "Charakter", 128);
  if ("selected_difficulty" in value) update.selected_difficulty = parsePreference(value.selected_difficulty, "Schwierigkeit", 32);
  return update;
}

function parseKnownDesktopSettings(value: Record<string, unknown>): DesktopSettings {
  if (typeof value.autostart !== "boolean" || typeof value.onboarding_completed !== "boolean") throw new Error("Desktop-Schalter müssen boolesch sein.");

  const settings: DesktopSettings = {
    schema_version: DESKTOP_SETTINGS_SCHEMA_VERSION,
    autostart: value.autostart,
    onboarding_completed: value.onboarding_completed,
  };
  if (value.window_bounds !== undefined) {
    settings.window_bounds = parseWindowBounds(value.window_bounds);
  }
  if (value.selected_character !== undefined) settings.selected_character = parsePreference(value.selected_character, "Charakter", 128);
  if (value.selected_difficulty !== undefined) settings.selected_difficulty = parsePreference(value.selected_difficulty, "Schwierigkeit", 32);
  return settings;
}

function parsePreference(value: unknown, label: string, maxLength: number): string {
  if (typeof value !== "string" || value.length === 0 || value.length > maxLength || value.trim() !== value || /[\u0000-\u001f\u007f]/.test(value)) {
    throw new Error(`Gespeicherte ${label}-Auswahl ist ungültig.`);
  }
  return value;
}

function parseWindowBounds(value: unknown): WindowBounds {
  if (!isRecord(value)) {
    throw new Error("Fensterbounds müssen ein JSON-Objekt sein.");
  }
  requireExactKeys(value, ["x", "y", "width", "height"], ["x", "y", "width", "height"]);
  const numbers = [value.x, value.y, value.width, value.height];
  if (!numbers.every(Number.isSafeInteger)) {
    throw new Error("Fensterbounds müssen sichere Ganzzahlen sein.");
  }
  const { x, y, width, height } = value as Record<"x" | "y" | "width" | "height", number>;
  if (width < 800 || height < 600 || width > 16_384 || height > 16_384) {
    throw new Error("Fensterbounds liegen außerhalb der sicheren Größenlimits.");
  }
  return { x, y, width, height };
}

function requireExactKeys(value: Record<string, unknown>, allowed: readonly string[], required: readonly string[]): void {
  const allowedKeys = new Set(allowed);
  const unknown = Object.keys(value).find((key) => !allowedKeys.has(key));
  if (unknown !== undefined) {
    throw new Error(`Unbekanntes Desktop-Einstellungsfeld: ${unknown}.`);
  }
  for (const key of required) {
    if (!(key in value)) {
      throw new Error(`Desktop-Einstellungsfeld fehlt: ${key}.`);
    }
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isMissingFile(error: unknown): boolean {
  return isRecord(error) && error.code === "ENOENT";
}
