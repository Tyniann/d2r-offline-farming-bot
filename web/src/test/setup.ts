import "@testing-library/jest-dom/vitest";
import { configure } from "@testing-library/react";

// Vollständige Suite läuft parallel in mehreren jsdom-Workern; asynchrone Core-
// Projektionen dürfen dabei langsamer als der Bibliotheksdefault erscheinen.
configure({ asyncUtilTimeout: 10000 });
