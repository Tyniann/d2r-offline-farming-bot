import "@testing-library/jest-dom/vitest";
import { configure } from "@testing-library/react";
import { beforeEach } from "vitest";

import { initializeI18n } from "../i18n";

// Vollständige Suite läuft parallel in mehreren jsdom-Workern; asynchrone Core-
// Projektionen dürfen dabei langsamer als der Bibliotheksdefault erscheinen.
configure({ asyncUtilTimeout: 10000 });

beforeEach(async () => {
  await initializeI18n("de");
});
