import i18next, { createInstance, type i18n as I18nInstance } from "i18next";
import { initReactI18next } from "react-i18next";

import de from "./locales/de.json";
import en from "./locales/en.json";
import { supportedLanguages, type SupportedLanguage } from "./types";

export const defaultNS = "translation" as const;
export const resources = {
  de: { translation: de },
  en: { translation: en },
} as const;

export const i18n = createInstance();

export async function initializeI18n(
  language: SupportedLanguage,
  instance: I18nInstance = i18n,
): Promise<void> {
  if (instance.isInitialized) {
    await instance.changeLanguage(language);
    return;
  }

  const failOnMissingKey = import.meta.env.MODE === "test";
  instance.use(initReactI18next);
  await instance.init({
    resources,
    defaultNS,
    ns: [defaultNS],
    lng: language,
    fallbackLng: "de",
    supportedLngs: [...supportedLanguages],
    load: "languageOnly",
    initAsync: false,
    returnNull: false,
    saveMissing: failOnMissingKey,
    missingKeyHandler: failOnMissingKey
      ? (_languages, _namespace, key) => {
          throw new Error(`Fehlender Übersetzungsschlüssel: ${key}`);
        }
      : undefined,
    interpolation: { escapeValue: false },
    react: { useSuspense: false },
  });
  synchronizeDocument(instance);
}

export async function changeAppLanguage(language: SupportedLanguage): Promise<void> {
  await i18n.changeLanguage(language);
  synchronizeDocument(i18n);
}

function synchronizeDocument(instance: I18nInstance): void {
  if (typeof document === "undefined") return;
  document.documentElement.lang = instance.resolvedLanguage ?? "de";
  document.title = instance.t("app.title");
}

export { i18next };
