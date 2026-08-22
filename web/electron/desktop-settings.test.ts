// @vitest-environment node

import { mkdtemp, readFile, rm, stat, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { afterEach, describe, expect, it } from "vitest";
import { DesktopSettingsStore, desktopSettingsDefaults, parseDesktopSettingsUpdate, resolveDesktopLanguage } from "./desktop-settings.js";

const roots: string[] = [];

afterEach(async () => {
  await Promise.all(roots.splice(0).map((root) => rm(root, { recursive: true, force: true })));
});

describe("DesktopSettingsStore", () => {
  it("liefert bei einer fehlenden Datei sichere Defaults", async () => {
    const store = await newStore();
    await expect(store.load()).resolves.toEqual({ settings: desktopSettingsDefaults(), recovered: false });
  });

  it("schreibt atomar und liest exakt den persistierten Vertrag erneut", async () => {
    const { store, path } = await newStoreWithPath();
    const saved = await store.save({
      schema_version: 3,
      language: "en",
      window_bounds: { x: -1200, y: 40, width: 1280, height: 800 },
      autostart: true,
      onboarding_completed: true,
      selected_character: "MrHammer",
      selected_difficulty: "hell",
    });
    expect(saved.window_bounds).toEqual({ x: -1200, y: 40, width: 1280, height: 800 });
    expect(saved).toMatchObject({ language: "en", selected_character: "MrHammer", selected_difficulty: "hell" });
    await expect(store.load()).resolves.toEqual({ settings: saved, recovered: false });
    expect((await readFile(path, "utf8")).endsWith("\n")).toBe(true);
  });

  it.each([
    ["unbekanntes Feld", { schema_version: 1, autostart: true, onboarding_completed: true, input_enabled: true }],
    ["unbekanntes Schema", { schema_version: 4, language: "de", autostart: true, onboarding_completed: true }],
    ["zu kleine Bounds", { schema_version: 1, autostart: true, onboarding_completed: true, window_bounds: { x: 0, y: 0, width: 200, height: 100 } }],
    ["zusätzliches Bounds-Feld", { schema_version: 1, autostart: true, onboarding_completed: true, window_bounds: { x: 0, y: 0, width: 1280, height: 800, display: 1 } }],
  ])("setzt %s vollständig fail-closed zurück", async (_name, value) => {
    const { store, path } = await newStoreWithPath();
    await writeFile(path, JSON.stringify(value), "utf8");
    await expect(store.load()).resolves.toEqual({ settings: desktopSettingsDefaults(), recovered: true });
  });

  it.each([1, 2] as const)("migriert Schema %s ohne Recovery mit deutscher Bestandssprache", async (schemaVersion) => {
    const { store, path } = await newStoreWithPath();
    await writeFile(path, JSON.stringify({ schema_version: schemaVersion, autostart: true, onboarding_completed: true }), "utf8");
    await expect(store.load()).resolves.toEqual({
      settings: { schema_version: 3, language: "de", autostart: true, onboarding_completed: true },
      recovered: false,
    });
  });

  it("verwirft ungültige gespeicherte Auswahlwerte vollständig", async () => {
    const { store, path } = await newStoreWithPath();
    await writeFile(path, JSON.stringify({ schema_version: 2, autostart: true, onboarding_completed: true, selected_character: " MrBones" }), "utf8");
    await expect(store.load()).resolves.toEqual({ settings: desktopSettingsDefaults(), recovered: true });
  });

  it("validiert IPC-Teilaktualisierungen feldgenau", () => {
    expect(parseDesktopSettingsUpdate({ selected_character: "MrBones", selected_difficulty: "nightmare" })).toEqual({ selected_character: "MrBones", selected_difficulty: "nightmare" });
    expect(parseDesktopSettingsUpdate({ language: "en" })).toEqual({ language: "en" });
    expect(parseDesktopSettingsUpdate({ autostart: true })).toEqual({ autostart: true });
    expect(() => parseDesktopSettingsUpdate({})).toThrow("keine Änderung");
    expect(() => parseDesktopSettingsUpdate({ selected_character: undefined })).toThrow("ungültig");
    expect(() => parseDesktopSettingsUpdate({ selected_character: "MrBones", input_enabled: true })).toThrow("Unbekanntes");
    expect(() => parseDesktopSettingsUpdate({ language: "fr" })).toThrow("de oder en");
  });

  it("behält bei einem Fehler vor dem Replace die alte Datei bytegleich", async () => {
    const { path } = await newStoreWithPath();
    const original = '{"schema_version":1,"autostart":false,"onboarding_completed":true}\n';
    await writeFile(path, original, "utf8");
    const store = new DesktopSettingsStore(path, { beforeReplace: () => { throw new Error("simulierter Replace-Fehler"); } });
    await expect(store.save({ schema_version: 3, language: "de", autostart: true, onboarding_completed: true })).rejects.toThrow("simulierter Replace-Fehler");
    await expect(readFile(path, "utf8")).resolves.toBe(original);
  });

  it("verweigert relative Speicherpfade", () => {
    expect(() => new DesktopSettingsStore("desktop-settings.json")).toThrow("absolut");
  });

  it("leitet frische Defaults aus der Systemsprache ab, ohne den Zielroot anzulegen", async () => {
    const root = await mkdtemp(join(tmpdir(), "d2rbot-desktop-settings-default-"));
    roots.push(root);
    const targetRoot = join(root, "not-yet-provisioned");
    const store = new DesktopSettingsStore(join(targetRoot, "desktop-settings.json"), {}, "en");

    await expect(store.load()).resolves.toEqual({ settings: desktopSettingsDefaults("en"), recovered: false });
    await expect(stat(targetRoot)).rejects.toMatchObject({ code: "ENOENT" });
  });

  it("lädt die atomar gespeicherte Sprache mit einer neuen Store-Instanz", async () => {
    const { store, path } = await newStoreWithPath();
    await store.save({ schema_version: 3, language: "en", autostart: false, onboarding_completed: true });
    await expect(new DesktopSettingsStore(path).load()).resolves.toEqual({
      settings: { schema_version: 3, language: "en", autostart: false, onboarding_completed: true },
      recovered: false,
    });
  });

  it.each([["de-AT", "de"], ["de_DE", "de"], ["en-GB", "en"], ["fr-FR", "en"]] as const)("löst die Systemsprache %s als %s auf", (input, expected) => {
    expect(resolveDesktopLanguage(input)).toBe(expected);
  });
});

async function newStore(): Promise<DesktopSettingsStore> {
  return (await newStoreWithPath()).store;
}

async function newStoreWithPath(): Promise<{ store: DesktopSettingsStore; path: string }> {
  const root = await mkdtemp(join(tmpdir(), "d2rbot-desktop-settings-"));
  roots.push(root);
  const path = join(root, "desktop-settings.json");
  return { store: new DesktopSettingsStore(path), path };
}
