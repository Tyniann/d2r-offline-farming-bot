export const supportedLanguages = ["de", "en"] as const;

export type SupportedLanguage = typeof supportedLanguages[number];

export function isSupportedLanguage(value: unknown): value is SupportedLanguage {
  return typeof value === "string" && supportedLanguages.includes(value as SupportedLanguage);
}

export function resolveSupportedLanguage(value: string | null | undefined): SupportedLanguage {
  const language = value?.trim().toLowerCase().replace("_", "-").split("-")[0];
  if (language === "de" || language === "en") return language;
  return "en";
}

export function localeForLanguage(language: SupportedLanguage): "de-DE" | "en-US" {
  return language === "de" ? "de-DE" : "en-US";
}
