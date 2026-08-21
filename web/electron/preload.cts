import { contextBridge, ipcRenderer } from "electron";

export interface DesktopAppInfo {
  app_version: string;
  platform: string;
  core_connected: boolean;
  core_state?: string;
  core_restart_count: number;
  lifecycle_state: string;
}

export interface DesktopSettingsView {
  schema_version: number;
  autostart: boolean;
  onboarding_completed: boolean;
  selected_character?: string;
  selected_difficulty?: string;
}

export interface DesktopSettingsUpdate {
  autostart?: boolean;
  onboarding_completed?: boolean;
  selected_character?: string;
  selected_difficulty?: string;
}

export interface DesktopProvisioningState {
  required: boolean;
  import_selected: boolean;
  import_label: string;
}

export type DesktopUpdateStatus =
  | { status: "checking"; current_version: string }
  | { status: "up_to_date"; current_version: string; latest_version: string }
  | { status: "available"; current_version: string; latest_version: string }
  | { status: "unavailable"; current_version: string; reason: "update_check_unavailable" | "update_response_invalid" };

const allowedTargets = new Set(["dashboard", "routes", "pickit", "history", "settings"]);

const bridge = Object.freeze({
  getProvisioningState: (): Promise<DesktopProvisioningState> => ipcRenderer.invoke("desktop:get-provisioning-state") as Promise<DesktopProvisioningState>,
  chooseImportRoot: (): Promise<{ selected: boolean; label: string }> => ipcRenderer.invoke("desktop:choose-import-root") as Promise<{ selected: boolean; label: string }>,
  provision: (request: { mode: "new" | "import" }): Promise<{ schema_version: 1; status: "published" | "existing"; diagnostic_count: number }> => ipcRenderer.invoke("desktop:provision", request) as Promise<{ schema_version: 1; status: "published" | "existing"; diagnostic_count: number }>,
  getAppInfo: (): Promise<DesktopAppInfo> => ipcRenderer.invoke("desktop:get-app-info") as Promise<DesktopAppInfo>,
  getUpdateStatus: (): Promise<DesktopUpdateStatus> => ipcRenderer.invoke("desktop:get-update-status") as Promise<DesktopUpdateStatus>,
  checkForUpdates: (): Promise<DesktopUpdateStatus> => ipcRenderer.invoke("desktop:check-for-updates") as Promise<DesktopUpdateStatus>,
  openReleasePage: (): Promise<void> => ipcRenderer.invoke("desktop:open-release-page") as Promise<void>,
  revealDiagnosticBundle: (filename: string): Promise<void> => ipcRenderer.invoke("desktop:reveal-diagnostic-bundle", filename) as Promise<void>,
  getDesktopSettings: (): Promise<DesktopSettingsView> => ipcRenderer.invoke("desktop:get-settings") as Promise<DesktopSettingsView>,
  updateDesktopSettings: (request: DesktopSettingsUpdate): Promise<DesktopSettingsView> => ipcRenderer.invoke("desktop:update-settings", request) as Promise<DesktopSettingsView>,
  showWindow: (): Promise<void> => ipcRenderer.invoke("desktop:show-window") as Promise<void>,
  restartCore: (): Promise<void> => ipcRenderer.invoke("desktop:restart-core") as Promise<void>,
  restartAsAdministrator: (): Promise<void> => ipcRenderer.invoke("desktop:restart-as-administrator") as Promise<void>,
  onNavigate: (callback: (target: string) => void): (() => void) => {
    const listener = (_event: Electron.IpcRendererEvent, target: unknown) => {
      if (typeof target === "string" && allowedTargets.has(target)) callback(target);
    };
    ipcRenderer.on("desktop:navigate", listener);
    return () => ipcRenderer.removeListener("desktop:navigate", listener);
  },
  onUpdateStatus: (callback: (status: DesktopUpdateStatus) => void): (() => void) => {
    const listener = (_event: Electron.IpcRendererEvent, status: DesktopUpdateStatus) => callback(status);
    ipcRenderer.on("desktop:update-status", listener);
    return () => ipcRenderer.removeListener("desktop:update-status", listener);
  },
});

contextBridge.exposeInMainWorld("d2rDesktop", bridge);
