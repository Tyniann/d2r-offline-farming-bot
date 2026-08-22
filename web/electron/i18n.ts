import { readFile } from "node:fs/promises";
import { join } from "node:path";
import type { DesktopCoreReason } from "./core-contract.js";
import type { DesktopLanguage } from "./desktop-settings.js";
import type { DesktopLifecycleState, DesktopNotificationKind } from "./desktop-lifecycle.js";

export type DesktopTranslationParameter = string | number | boolean;
export type DesktopTranslationParameters = Readonly<Record<string, DesktopTranslationParameter>>;

export interface DesktopTranslator {
  t(key: string, parameters?: DesktopTranslationParameters): string;
}

export interface DesktopTrayText {
  tooltip: string;
  open: string;
  status: string;
  pauseAfterRun: string;
  stopAfterRun: string;
  emergencyStop: string;
  quit: string;
}

export interface DesktopNotificationText {
  title: string;
  body: string;
}

export type DesktopDialogKind = "command_failed" | "active_session" | "confirm_quit" | "shutdown_failed";

export interface DesktopDialogText {
  title: string;
  message: string;
  detail?: string;
  buttons: string[];
}

export interface DesktopRecoveryText {
  title: string;
  body: string;
}

const requiredDesktopKeys = [
  "desktop.provisioning.selectedImportRoot",
  "desktop.provisioning.chooseImportRoot",
  "desktop.provisioning.importRootSelected",
  "desktop.lifecycle.idle",
  "desktop.lifecycle.running",
  "desktop.lifecycle.pause_pending",
  "desktop.lifecycle.paused",
  "desktop.lifecycle.stop_pending",
  "desktop.lifecycle.cancelling",
  "desktop.lifecycle.error",
  "desktop.lifecycle.core_down",
  "desktop.tray.tooltip",
  "desktop.tray.open",
  "desktop.tray.status",
  "desktop.tray.pauseAfterRun",
  "desktop.tray.stopAfterRun",
  "desktop.tray.emergencyStop",
  "desktop.tray.quit",
  "desktop.notifications.session_completed.title",
  "desktop.notifications.session_completed.body",
  "desktop.notifications.terminal_error.title",
  "desktop.notifications.terminal_error.body",
  "desktop.notifications.pause_reached.title",
  "desktop.notifications.pause_reached.body",
  "desktop.notifications.update_available.title",
  "desktop.notifications.update_available.body",
  "desktop.dialogs.ok",
  "desktop.dialogs.cancel",
  "desktop.dialogs.quit",
  "desktop.dialogs.command_failed.title",
  "desktop.dialogs.command_failed.message",
  "desktop.dialogs.active_session.title",
  "desktop.dialogs.active_session.message",
  "desktop.dialogs.active_session.detail",
  "desktop.dialogs.confirm_quit.title",
  "desktop.dialogs.confirm_quit.message",
  "desktop.dialogs.confirm_quit.detail",
  "desktop.dialogs.shutdown_failed.title",
  "desktop.dialogs.shutdown_failed.message",
  "desktop.recovery.title",
  "desktop.recovery.body",
  "desktop.recovery.reasons.core_start_failed",
  "desktop.recovery.reasons.core_handshake_timeout",
  "desktop.recovery.reasons.core_handshake_invalid",
  "desktop.recovery.reasons.core_exited",
  "desktop.recovery.reasons.core_recovery_required",
  "desktop.recovery.reasons.core_shutdown_failed",
] as const;

export async function loadDesktopTranslators(directory: string): Promise<Record<DesktopLanguage, DesktopTranslator>> {
  const [de, en] = await Promise.all([
    loadDesktopTranslator(join(directory, "de.json")),
    loadDesktopTranslator(join(directory, "en.json")),
  ]);
  return { de, en };
}

export function createDesktopTranslator(catalog: unknown, requiredKeys: readonly string[] = []): DesktopTranslator {
  if (!isRecord(catalog) || !isRecord(catalog.desktop)) {
    throw new Error("Desktop translation catalog must contain a desktop object.");
  }
  const translator: DesktopTranslator = {
    t(key, parameters = {}) {
      if (!key.startsWith("desktop.")) throw new Error(`Desktop translation key is outside the desktop namespace: ${key}.`);
      const value = resolveCatalogValue(catalog, key);
      if (typeof value !== "string") throw new Error(`Desktop translation is missing or not a string: ${key}.`);
      return interpolate(value, parameters);
    },
  };
  for (const key of requiredKeys) translator.t(key);
  return Object.freeze(translator);
}

export function desktopTrayText(translator: DesktopTranslator, state: DesktopLifecycleState): DesktopTrayText {
  const lifecycle = translator.t(`desktop.lifecycle.${state}`);
  return {
    tooltip: translator.t("desktop.tray.tooltip", { state: lifecycle }),
    open: translator.t("desktop.tray.open"),
    status: translator.t("desktop.tray.status", { state: lifecycle }),
    pauseAfterRun: translator.t("desktop.tray.pauseAfterRun"),
    stopAfterRun: translator.t("desktop.tray.stopAfterRun"),
    emergencyStop: translator.t("desktop.tray.emergencyStop"),
    quit: translator.t("desktop.tray.quit"),
  };
}

export function desktopNotificationText(translator: DesktopTranslator, kind: DesktopNotificationKind): DesktopNotificationText {
  return {
    title: translator.t(`desktop.notifications.${kind}.title`),
    body: translator.t(`desktop.notifications.${kind}.body`),
  };
}

export function desktopDialogText(translator: DesktopTranslator, kind: DesktopDialogKind): DesktopDialogText {
  const prefix = `desktop.dialogs.${kind}`;
  const buttons = kind === "confirm_quit"
    ? [translator.t("desktop.dialogs.quit"), translator.t("desktop.dialogs.cancel")]
    : [translator.t("desktop.dialogs.ok")];
  return {
    title: translator.t(`${prefix}.title`),
    message: translator.t(`${prefix}.message`),
    ...(kind === "active_session" || kind === "confirm_quit" ? { detail: translator.t(`${prefix}.detail`) } : {}),
    buttons,
  };
}

export function desktopRecoveryText(translator: DesktopTranslator, reason: DesktopCoreReason, restartCount: number): DesktopRecoveryText {
  return {
    title: translator.t("desktop.recovery.title"),
    body: translator.t("desktop.recovery.body", {
      reason: translator.t(`desktop.recovery.reasons.${reason}`),
      restarts: restartCount,
    }),
  };
}

async function loadDesktopTranslator(path: string): Promise<DesktopTranslator> {
  const raw = JSON.parse(await readFile(path, "utf8")) as unknown;
  return createDesktopTranslator(raw, requiredDesktopKeys);
}

function resolveCatalogValue(catalog: Record<string, unknown>, key: string): unknown {
  return key.split(".").reduce<unknown>((value, segment) => isRecord(value) ? value[segment] : undefined, catalog);
}

function interpolate(template: string, parameters: DesktopTranslationParameters): string {
  return template.replace(/{{\s*([A-Za-z0-9_]+)\s*}}/g, (match, key: string) => {
    const value = parameters[key];
    return value === undefined ? match : String(value);
  });
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
