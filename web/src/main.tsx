import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./app/App";
import { initializeI18n } from "./i18n";
import { resolveSupportedLanguage } from "./i18n/types";

async function startRenderer(): Promise<void> {
  let language = resolveSupportedLanguage(navigator.language);
  if (window.d2rDesktop?.getDesktopSettings) {
    try {
      language = (await window.d2rDesktop.getDesktopSettings()).language;
    } catch {
      // Der Browser-Locale bleibt der sitzungsweite Fallback. Ein fehlender
      // Desktop-Read darf den ersten sichtbaren Render nicht blockieren.
    }
  }
  await initializeI18n(language);
  createRoot(document.getElementById("root")!).render(<StrictMode><App /></StrictMode>);
}

void startRenderer();
