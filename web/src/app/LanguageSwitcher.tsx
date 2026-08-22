import { useState } from "react";
import { useTranslation } from "react-i18next";

import { changeAppLanguage } from "../i18n";
import { isSupportedLanguage, supportedLanguages, type SupportedLanguage } from "../i18n/types";

export function LanguageSwitcher() {
  const { t, i18n } = useTranslation();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const activeLanguage: SupportedLanguage = isSupportedLanguage(i18n.resolvedLanguage) ? i18n.resolvedLanguage : "de";

  async function selectLanguage(language: SupportedLanguage): Promise<void> {
    if (pending || language === activeLanguage) return;
    setPending(true);
    setError("");
    try {
      const saved = window.d2rDesktop?.updateDesktopSettings
        ? await window.d2rDesktop.updateDesktopSettings({ language })
        : { language };
      await changeAppLanguage(saved.language);
    } catch {
      setError(t("language.saveFailed"));
    } finally {
      setPending(false);
    }
  }

  return <div className="language-switcher-shell">
    <div className="language-switcher" role="group" aria-label={t("language.label")}>
      {supportedLanguages.map((language) => {
        const languageName = t(language === "de" ? "language.german" : "language.english");
        return <button
          key={language}
          type="button"
          lang={language}
          aria-pressed={activeLanguage === language}
          aria-label={t("language.switchTo", { language: languageName })}
          disabled={pending}
          onClick={() => void selectLanguage(language)}
        >{language.toUpperCase()}</button>;
      })}
    </div>
    {error && <small className="language-switcher-error" role="alert">{error}</small>}
  </div>;
}
