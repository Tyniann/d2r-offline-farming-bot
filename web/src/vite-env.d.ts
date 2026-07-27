/// <reference types="vite/client" />

interface DesktopSettingsView {
  schema_version: number;
  autostart: boolean;
  onboarding_completed: boolean;
}

type DesktopUpdateStatus =
  | { status: "checking"; current_version: string }
  | { status: "up_to_date"; current_version: string; latest_version: string }
  | { status: "available"; current_version: string; latest_version: string }
  | { status: "unavailable"; current_version: string; reason: "update_check_unavailable" | "update_response_invalid" };

interface D2RDesktopBridge {
  getProvisioningState(): Promise<{ required: boolean; import_selected: boolean; import_label: string }>;
  chooseImportRoot(): Promise<{ selected: boolean; label: string }>;
  provision(request: { mode: "new" | "import" }): Promise<{ schema_version: 1; status: "published" | "existing"; diagnostic_count: number }>;
  getAppInfo(): Promise<{ app_version: string; platform: string; core_connected: boolean; core_state?: string; core_restart_count: number; lifecycle_state: string }>;
  getUpdateStatus?(): Promise<DesktopUpdateStatus>;
  checkForUpdates?(): Promise<DesktopUpdateStatus>;
  openReleasePage?(): Promise<void>;
  revealDiagnosticBundle?(filename: string): Promise<void>;
  getDesktopSettings(): Promise<DesktopSettingsView>;
  updateDesktopSettings(request: { autostart: boolean; onboarding_completed: boolean }): Promise<DesktopSettingsView>;
  showWindow(): Promise<void>;
  restartCore(): Promise<void>;
  restartAsAdministrator(): Promise<void>;
  onNavigate(callback: (target: string) => void): () => void;
  onUpdateStatus?(callback: (status: DesktopUpdateStatus) => void): () => void;
}

interface Window {
  d2rDesktop?: D2RDesktopBridge;
}
