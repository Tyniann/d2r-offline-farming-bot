import { app, BrowserWindow, dialog, ipcMain, Menu, nativeImage, Notification, screen, session, shell, Tray } from "electron";
import type { MessageBoxOptions, MessageBoxReturnValue } from "electron";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { access, lstat } from "node:fs/promises";
import { constants } from "node:fs";
import { tmpdir } from "node:os";
import { fileURLToPath, pathToFileURL } from "node:url";
import { dirname, isAbsolute, join, resolve } from "node:path";
import { DesktopCoreController } from "./core-controller.js";
import { provisionDataRoot } from "./core-process.js";
import { isAllowedIPCSender, isAllowedNavigation, secureWebPreferences, type CoreHandshake, type DesktopCoreReason } from "./core-contract.js";
import { DesktopSettingsStore, desktopSettingsDefaults, parseDesktopSettingsUpdate, type DesktopSettings } from "./desktop-settings.js";
import { desktopLifecyclePolicy, desktopNotificationSpec, notificationForTransition, shouldShowDesktopNotification, type DesktopNotificationKind, type StableAppTarget } from "./desktop-lifecycle.js";
import { clampWindowBounds } from "./desktop-window.js";
import { portalMarkPath } from "./portal-icon.js";
import { checkLatestRelease, githubReleasesURL, type DesktopUpdateStatus } from "./update-check.js";
import { carryOnboardingStep } from "./onboarding-resume.js";

const moduleDirectory = dirname(fileURLToPath(import.meta.url));
const argumentsMap = parseArguments(process.argv.slice(1));
const dataRoot = resolveDataRoot(argumentsMap.get("data-root"));
app.setName("D2R Offline Farming Bot");
// Chromiums Profil ist nur entbehrlicher Desktop-Laufzeitzustand. Es darf den
// kanonischen Datenroot insbesondere vor dessen atomarer Core-Veröffentlichung
// weder anlegen noch als vermeintlich bestehenden Importstand verunreinigen.
app.setPath("userData", resolveDesktopRuntimeProfile(dataRoot));

let mainWindow: BrowserWindow | undefined;
let tray: Tray | undefined;
let coreOrigin = "";
let recoveryURL = "";
let provisioningURL = "";
let provisioningActive = false;
let provisioningPending = false;
let selectedImportRoot: string | undefined;
let allowQuit = false;
let quitting = false;
let exitPromptOpen = false;
let restartCorePending = false;
let boundsTimer: ReturnType<typeof setTimeout> | undefined;
let desktopSettings = desktopSettingsDefaults();
const desktopSettingsStore = new DesktopSettingsStore(join(dataRoot, "desktop-settings.json"));
let desktopSettingsSave = Promise.resolve(desktopSettings);
let updateStatus: DesktopUpdateStatus = { status: "unavailable", current_version: app.getVersion(), reason: "update_check_unavailable" };
let updateCheckPending: Promise<DesktopUpdateStatus> | undefined;
const portalIcon = nativeImage.createFromPath(portalMarkPath()).resize({ width: 32, height: 32 });

const gotSingleInstanceLock = app.requestSingleInstanceLock({ dataRoot });
if (!gotSingleInstanceLock) {
  app.quit();
} else {
  void startDesktop();
}

async function startDesktop(): Promise<void> {
  app.on("second-instance", () => showMainWindow());
  app.on("before-quit", (event) => {
    if (allowQuit) return;
    event.preventDefault();
    if (!quitting) void requestQuit(false);
  });

  await app.whenReady();
  const loaded = await desktopSettingsStore.load();
  desktopSettings = loaded.settings;
  desktopSettingsSave = Promise.resolve(desktopSettings);
  applyAutostart(desktopSettings.autostart);
  registerPermissionDeny();
  registerDesktopIPC();
  if (app.isPackaged) void runUpdateCheck();
  provisioningActive = await needsProvisioning();
  if (provisioningActive) {
    await loadProvisioningWindow();
  } else {
    ensureTray();
    await controller.start();
  }
}

const controller = new DesktopCoreController(coreLaunchOptions(), {
  onReady: async (handshake) => {
    coreOrigin = new URL(handshake.base_url).origin;
    installCSP(coreOrigin);
    await loadCoreWindow(handshake);
  },
  onRecoveryRequired: async (reason, restartCount) => loadRecoveryWindow(reason, restartCount),
  onStatusChanged: (previous, current) => {
    rebuildTray();
    const kind = notificationForTransition(previous, current);
    if (kind) showDesktopNotification(kind);
  },
});

function coreLaunchOptions() {
  const fakeCore = argumentsMap.get("fake-core");
  const timeout = Number(argumentsMap.get("handshake-timeout-ms") ?? "5000");
  if (fakeCore) {
    return {
      executable: process.execPath,
      executableArgs: [resolve(fakeCore), `--mode=${argumentsMap.get("fake-core-mode") ?? "valid"}`],
      dataRoot,
      handshakeTimeoutMs: Number.isFinite(timeout) && timeout > 0 ? timeout : 5_000,
      environment: { ...process.env, ELECTRON_RUN_AS_NODE: "1" },
    };
  }
  return {
    executable: join(process.resourcesPath, "core", "d2rbot.exe"),
    dataRoot,
    handshakeTimeoutMs: 5_000,
    environment: process.env,
  };
}

async function loadCoreWindow(handshake: CoreHandshake): Promise<void> {
  const window = ensureWindow();
  const currentURL = window.webContents.getURL();
  await window.loadURL(carryOnboardingStep(currentURL, handshake.bootstrap_url, 8));
  window.show();
}

async function loadRecoveryWindow(reason: DesktopCoreReason, restartCount: number): Promise<void> {
  const window = ensureWindow();
  const recoveryFile = join(moduleDirectory, "recovery.html");
  recoveryURL = pathToFileURL(recoveryFile).toString();
  await window.loadFile(recoveryFile, { query: { reason, restarts: String(restartCount) } });
  window.show();
}

async function loadProvisioningWindow(): Promise<void> {
  const provisioningFile = join(moduleDirectory, "ui", "index.html");
  provisioningURL = pathToFileURL(provisioningFile).toString();
  const window = ensureWindow();
  await window.loadFile(provisioningFile);
  window.show();
}

function ensureWindow(): BrowserWindow {
  if (mainWindow && !mainWindow.isDestroyed()) return mainWindow;
  const preload = join(moduleDirectory, "preload.cjs");
  const bounds = clampWindowBounds(desktopSettings.window_bounds, screen.getAllDisplays().map((display) => display.workArea));
  mainWindow = new BrowserWindow({
    ...bounds,
    minWidth: 1100,
    minHeight: 700,
    show: false,
    autoHideMenuBar: true,
    icon: portalMarkPath(),
    webPreferences: secureWebPreferences(preload),
  });
  mainWindow.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
  mainWindow.webContents.on("will-attach-webview", (event) => event.preventDefault());
  mainWindow.webContents.on("will-navigate", (event, target) => {
    if (target !== provisioningURL && !isAllowedNavigation(target, coreOrigin)) event.preventDefault();
  });
  mainWindow.webContents.on("will-redirect", (event, target) => {
    if (target !== provisioningURL && !isAllowedNavigation(target, coreOrigin)) event.preventDefault();
  });
  mainWindow.on("move", scheduleBoundsSave);
  mainWindow.on("resize", scheduleBoundsSave);
  mainWindow.on("close", (event) => {
    if (allowQuit) return;
    event.preventDefault();
    const policy = desktopLifecyclePolicy(controller.statusSnapshot);
    if (policy.closeAction === "hide") {
      mainWindow?.hide();
      return;
    }
    void requestQuit(true);
  });
  mainWindow.on("closed", () => { mainWindow = undefined; });
  return mainWindow;
}

function showMainWindow(target?: StableAppTarget): void {
  if (!mainWindow || mainWindow.isDestroyed()) return;
  if (mainWindow.isMinimized()) mainWindow.restore();
  mainWindow.show();
  mainWindow.focus();
  if (target) mainWindow.webContents.send("desktop:navigate", target);
}

function registerDesktopIPC(): void {
  ipcMain.handle("desktop:get-provisioning-state", (event) => {
    validateSender(event.senderFrame?.url ?? "");
    return { required: provisioningActive, import_selected: selectedImportRoot !== undefined, import_label: selectedImportRoot ? "Ausgewählter bestehender Datenroot" : "" };
  });
  ipcMain.handle("desktop:choose-import-root", async (event) => {
    validateSender(event.senderFrame?.url ?? "");
    if (!provisioningActive || provisioningPending) throw new Error("Die Importauswahl ist derzeit nicht verfügbar.");
    const result = await dialog.showOpenDialog(mainWindow!, {
      title: "Bestehenden D2R-Offline-Farming-Bot-Datenroot auswählen",
      properties: ["openDirectory", "dontAddToRecent"],
    });
    if (result.canceled || result.filePaths.length !== 1) return { selected: false, label: "" };
    selectedImportRoot = resolve(result.filePaths[0]);
    return { selected: true, label: "Bestehender Datenroot ausgewählt" };
  });
  ipcMain.handle("desktop:provision", async (event, request: unknown) => {
    validateSender(event.senderFrame?.url ?? "");
    if (!provisioningActive || provisioningPending) throw new Error("Die Datenanlage läuft bereits oder ist abgeschlossen.");
    const mode = parseProvisioningMode(request);
    if (mode === "import" && !selectedImportRoot) throw new Error("Wähle zuerst einen bestehenden Datenroot aus.");
    provisioningPending = true;
    try {
      // Der Go-Core importiert ausschließlich seine fachlichen Daten. Electron
      // liest aus einer bestehenden Installation nur seinen eigenen, streng
      // validierten Abschlusswert; Autostart und Fensterbounds werden nicht
      // ungefragt auf den neuen Root übertragen.
      const importedOnboardingCompleted = mode === "import"
        ? (await new DesktopSettingsStore(join(selectedImportRoot!, "desktop-settings.json")).load()).settings.onboarding_completed
        : false;
      const result = await provisionDataRoot(coreLaunchOptions(), mode === "new"
        ? { mode, defaultsRoot: resolveDefaultsRoot() }
        : { mode, importRoot: selectedImportRoot });
      // Der Main-Prozess hat den Zielroot vor der atomaren Core-Veröffentlichung
      // absichtlich als nicht vorhanden gelesen. Nach einem Import muss deshalb
      // auch der Electron-eigene Abschlusszustand neu aus dem veröffentlichten
      // Root übernommen werden, bevor der produktive Renderer startet.
      const loadedAfterProvisioning = await desktopSettingsStore.load();
      desktopSettings = loadedAfterProvisioning.settings;
      if (importedOnboardingCompleted) {
        desktopSettings = await desktopSettingsStore.save({ ...desktopSettings, onboarding_completed: true });
      }
      desktopSettingsSave = Promise.resolve(desktopSettings);
      applyAutostart(desktopSettings.autostart);
      provisioningActive = false;
      selectedImportRoot = undefined;
      ensureTray();
      await controller.start();
      return result;
    } finally {
      provisioningPending = false;
    }
  });
  ipcMain.handle("desktop:get-app-info", (event) => {
    validateSender(event.senderFrame?.url ?? "");
    return {
      app_version: app.getVersion(),
      platform: process.platform,
      core_connected: controller.connected,
      core_state: controller.lastState,
      core_restart_count: controller.restartCount,
      lifecycle_state: desktopLifecyclePolicy(controller.statusSnapshot).state,
    };
  });
  ipcMain.handle("desktop:get-update-status", (event) => {
    validateSender(event.senderFrame?.url ?? "");
    return updateStatus;
  });
  ipcMain.handle("desktop:check-for-updates", async (event) => {
    validateSender(event.senderFrame?.url ?? "");
    return runUpdateCheck();
  });
  ipcMain.handle("desktop:open-release-page", async (event) => {
    validateSender(event.senderFrame?.url ?? "");
    // Die URL stammt nie aus der Netzwerkantwort und bleibt damit eine feste Produktgrenze.
    await shell.openExternal(githubReleasesURL);
  });
  ipcMain.handle("desktop:reveal-diagnostic-bundle", async (event, filename: unknown) => {
    validateSender(event.senderFrame?.url ?? "");
    if (typeof filename !== "string" || !/^diagnose-\d{8}T\d{6}Z-[a-f0-9]{8}\.zip$/.test(filename)) {
      throw new Error("Das Diagnosepaket hat keinen gültigen lokalen Dateinamen.");
    }
    const path = join(dataRoot, "diagnostics", filename);
    const info = await lstat(path);
    if (!info.isFile() || info.isSymbolicLink()) {
      throw new Error("Das Diagnosepaket ist keine reguläre lokale Datei.");
    }
    shell.showItemInFolder(path);
  });
  ipcMain.handle("desktop:show-window", (event) => {
    validateSender(event.senderFrame?.url ?? "");
    showMainWindow();
  });
  ipcMain.handle("desktop:get-settings", (event) => {
    validateSender(event.senderFrame?.url ?? "");
    return desktopSettingsView(desktopSettings);
  });
  ipcMain.handle("desktop:update-settings", async (event, request: unknown) => {
    validateSender(event.senderFrame?.url ?? "");
    const update = parseDesktopSettingsUpdate(request);
    desktopSettings = await persistDesktopSettings(update);
    applyAutostart(desktopSettings.autostart);
    return desktopSettingsView(desktopSettings);
  });
  ipcMain.handle("desktop:restart-core", async (event) => {
    validateSender(event.senderFrame?.url ?? "");
    if (restartCorePending) throw new Error("Der kontrollierte Core-Neustart läuft bereits.");
    restartCorePending = true;
    try {
      await controller.restart();
    } finally {
      restartCorePending = false;
    }
  });
  ipcMain.handle("desktop:restart-as-administrator", async (event) => {
    validateSender(event.senderFrame?.url ?? "");
    if (!controller.privilegeMismatch) {
      throw new Error("Ein Privilegienproblem wurde vom Core nicht nachgewiesen.");
    }
    if (!desktopLifecyclePolicy(controller.statusSnapshot).canQuit) {
      throw new Error("Der Administrator-Neustart ist während einer aktiven Session gesperrt.");
    }
    await controller.shutdown();
    app.releaseSingleInstanceLock();
    try {
      await launchElevatedDesktop();
    } catch (error) {
      app.requestSingleInstanceLock({ dataRoot });
      await controller.start();
      throw error;
    }
    allowQuit = true;
    app.quit();
  });
}

function ensureTray(): void {
  if (tray && !tray.isDestroyed()) return;
  tray = new Tray(portalIcon);
  tray.setToolTip("D2R Offline Farming Bot");
  tray.on("click", () => showMainWindow());
  rebuildTray();
}

function rebuildTray(): void {
  if (!tray || tray.isDestroyed()) return;
  const policy = desktopLifecyclePolicy(controller.statusSnapshot);
  tray.setToolTip(`D2R Offline Farming Bot – ${policy.label}`);
  tray.setContextMenu(Menu.buildFromTemplate([
    { label: "Öffnen", click: () => showMainWindow() },
    { label: `Status: ${policy.label}`, enabled: false },
    { type: "separator" },
    { label: "Pause nach aktuellem Run", enabled: policy.canPauseAfterRun, click: () => void performTrayIntent("pause_after_run") },
    { label: "Stop nach aktuellem Run", enabled: policy.canStopAfterRun, click: () => void performTrayIntent("stop_after_run") },
    { label: "Emergency Stop", enabled: policy.canEmergencyStop, click: () => void performTrayIntent("emergency_stop") },
    { type: "separator" },
    { label: "Beenden", enabled: policy.canQuit, click: () => void requestQuit(false) },
  ]));
}

async function performTrayIntent(intent: "pause_after_run" | "stop_after_run" | "emergency_stop"): Promise<void> {
  try {
    await controller.sendSessionIntent(intent);
  } catch (error) {
    await showDesktopDialog({ type: "error", title: "Befehl nicht ausgeführt", message: errorMessage(error), buttons: ["OK"] });
  }
}

function showDesktopNotification(kind: DesktopNotificationKind): void {
  if (!shouldShowDesktopNotification(mainWindow?.isFocused() === true, Notification.isSupported())) return;
  const spec = desktopNotificationSpec(kind);
  const notification = new Notification({ title: spec.title, body: spec.body, icon: portalIcon });
  notification.on("click", () => showMainWindow(spec.target));
  notification.show();
}

async function requestQuit(confirmInactive: boolean): Promise<void> {
  if (quitting) return;
  const policy = desktopLifecyclePolicy(controller.statusSnapshot);
  if (!policy.canQuit) {
    showMainWindow("dashboard");
    await showDesktopDialog({
      type: "warning",
      title: "Session zuerst beenden",
      message: "Die App beendet keinen aktiven Core blind.",
      detail: "Nutze zuerst „Stop nach aktuellem Run“ oder den getrennten Emergency Stop. Währenddessen bleibt X→Tray aktiv.",
      buttons: ["OK"],
    });
    return;
  }
  if (confirmInactive) {
    if (exitPromptOpen) return;
    exitPromptOpen = true;
    const answer = await showDesktopDialog({ type: "question", title: "App beenden?", message: "D2R Offline Farming Bot vollständig beenden?", detail: "Die inaktive Core-Instanz wird kontrolliert geschlossen.", buttons: ["Beenden", "Abbrechen"], defaultId: 1, cancelId: 1, noLink: true });
    exitPromptOpen = false;
    if (answer.response !== 0) return;
  }
  quitting = true;
  try {
    await controller.shutdown();
    allowQuit = true;
    app.quit();
  } catch (error) {
    quitting = false;
    await showDesktopDialog({ type: "error", title: "Core konnte nicht beendet werden", message: errorMessage(error), buttons: ["OK"] });
  }
}

function scheduleBoundsSave(): void {
  // Im Pre-Core-Modus muss der Zielroot bis zur atomaren Veröffentlichung
  // vollständig abwesend bleiben. Erst danach darf Electron seine eigene
  // desktop-settings.json ergänzen.
  if (provisioningActive) return;
  if (boundsTimer !== undefined) clearTimeout(boundsTimer);
  boundsTimer = setTimeout(() => {
    boundsTimer = undefined;
    const window = mainWindow;
    if (!window || window.isDestroyed() || window.isMinimized() || window.isMaximized() || window.isFullScreen()) return;
    void persistDesktopSettings({ window_bounds: window.getNormalBounds() }).then((saved) => { desktopSettings = saved; }).catch(() => undefined);
  }, 250);
}

function persistDesktopSettings(update: Partial<DesktopSettings>): Promise<DesktopSettings> {
  desktopSettingsSave = desktopSettingsSave.catch(() => desktopSettings).then((current) => desktopSettingsStore.save({ ...current, ...update }));
  return desktopSettingsSave;
}

function desktopSettingsView(settings: DesktopSettings) {
  return {
    schema_version: settings.schema_version,
    autostart: settings.autostart,
    onboarding_completed: settings.onboarding_completed,
    ...(settings.selected_character ? { selected_character: settings.selected_character } : {}),
    ...(settings.selected_difficulty ? { selected_difficulty: settings.selected_difficulty } : {}),
  };
}

function applyAutostart(enabled: boolean): void {
  if (!app.isPackaged) return;
  app.setLoginItemSettings({ openAtLogin: enabled, path: process.execPath, args: ["--data-root", dataRoot] });
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Unbekannter Desktopfehler.";
}

function showDesktopDialog(options: MessageBoxOptions): Promise<MessageBoxReturnValue> {
  const window = mainWindow;
  return window && !window.isDestroyed() ? dialog.showMessageBox(window, options) : dialog.showMessageBox(options);
}

async function launchElevatedDesktop(): Promise<void> {
  if (process.platform !== "win32") throw new Error("Der Administrator-Neustart ist nur unter Windows verfügbar.");
  const args = app.isPackaged
    ? ["--data-root", dataRoot]
    : [fileURLToPath(import.meta.url), "--data-root", dataRoot];
  const literals = args.map(powerShellLiteral).join(",");
  const command = `$p=Start-Process -FilePath ${powerShellLiteral(process.execPath)} -ArgumentList @(${literals}) -Verb RunAs -PassThru; if($null -eq $p){exit 1}`;
  const encoded = Buffer.from(command, "utf16le").toString("base64");
  await new Promise<void>((resolveLaunch, rejectLaunch) => {
    const child = spawn("powershell.exe", ["-NoProfile", "-NonInteractive", "-EncodedCommand", encoded], { windowsHide: true, stdio: "ignore" });
    child.once("error", rejectLaunch);
    child.once("exit", (code) => code === 0 ? resolveLaunch() : rejectLaunch(new Error("Der Administrator-Neustart wurde abgebrochen oder ist fehlgeschlagen.")));
  });
}

function powerShellLiteral(value: string): string {
  return `'${value.replaceAll("'", "''")}'`;
}

function validateSender(senderURL: string): void {
  if (!isAllowedIPCSender(senderURL, coreOrigin, recoveryURL, provisioningURL)) {
    throw new Error("Unerlaubter Desktop-IPC-Sender.");
  }
}

async function runUpdateCheck(): Promise<DesktopUpdateStatus> {
  if (updateCheckPending) return updateCheckPending;
  updateStatus = { status: "checking", current_version: app.getVersion() };
  mainWindow?.webContents.send("desktop:update-status", updateStatus);
  updateCheckPending = checkLatestRelease(app.getVersion())
    .then((result) => {
      updateStatus = result;
      mainWindow?.webContents.send("desktop:update-status", result);
      return result;
    })
    .finally(() => { updateCheckPending = undefined; });
  return updateCheckPending;
}

async function needsProvisioning(): Promise<boolean> {
  try {
    await access(join(dataRoot, "configs", "config.yaml"), constants.R_OK);
    return false;
  } catch (error) {
    if (typeof error === "object" && error !== null && "code" in error && (error as { code?: unknown }).code === "ENOENT") return true;
    throw error;
  }
}

function resolveDefaultsRoot(): string {
  return resolve(argumentsMap.get("defaults-root") ?? join(process.resourcesPath, "defaults"));
}

function parseProvisioningMode(value: unknown): "new" | "import" {
  if (typeof value !== "object" || value === null || Array.isArray(value)) throw new Error("Provisionierung benötigt ein Objekt.");
  const record = value as Record<string, unknown>;
  if (Object.keys(record).join(",") !== "mode" || (record.mode !== "new" && record.mode !== "import")) {
    throw new Error("Unerlaubter Provisionierungsmodus.");
  }
  return record.mode;
}

function registerPermissionDeny(): void {
  session.defaultSession.setPermissionRequestHandler((_contents, _permission, callback) => callback(false));
  session.defaultSession.setPermissionCheckHandler(() => false);
}

function installCSP(origin: string): void {
  session.defaultSession.webRequest.onHeadersReceived((details, callback) => {
    if (!details.url.startsWith(`${origin}/`)) {
      callback({ responseHeaders: details.responseHeaders });
      return;
    }
    callback({
      responseHeaders: {
        ...details.responseHeaders,
        "Content-Security-Policy": ["default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'"],
      },
    });
  });
}

function resolveDataRoot(argument: string | undefined): string {
  if (argument) {
    if (!isAbsolute(argument)) throw new Error("Der Desktop-Datenroot muss absolut sein.");
    return resolve(argument);
  }
  const localAppData = process.env.LOCALAPPDATA;
  if (!localAppData || !isAbsolute(localAppData)) throw new Error("LOCALAPPDATA ist nicht verfügbar.");
  return join(localAppData, "D2ROfflineFarmingBot");
}

function resolveDesktopRuntimeProfile(root: string): string {
  const identity = createHash("sha256").update(process.platform === "win32" ? root.toLowerCase() : root).digest("hex").slice(0, 24);
  return join(tmpdir(), "D2ROfflineFarmingBot", `electron-profile-${identity}`);
}

function parseArguments(args: string[]): Map<string, string> {
  const result = new Map<string, string>();
  for (let index = 0; index < args.length; index++) {
    const raw = args[index];
    if (!raw.startsWith("--")) continue;
    const equals = raw.indexOf("=");
    if (equals >= 0) result.set(raw.slice(2, equals), raw.slice(equals + 1));
    else if (args[index + 1] && !args[index + 1].startsWith("--")) result.set(raw.slice(2), args[++index]);
    else result.set(raw.slice(2), "true");
  }
  return result;
}
