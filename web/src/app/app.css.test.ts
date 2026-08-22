import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const css = readFileSync(resolve(process.cwd(), "src/app/app.css"), "utf8");

describe("App-Designvertrag", () => {
  it("definiert die vereinbarten Farb-, Fokus-, Spacing- und Zustandstokens", () => {
    for (const token of ["--ember", "--gold", "--crimson", "--surface", "--text", "--focus", "--space", "--success", "--warning", "--danger"]) {
      expect(css).toContain(token);
    }
  });

  it("hält Grundflächen fast schwarz und verwendet Feuerfarben nur als Akzent", () => {
    expect(css).toContain("--surface-0: #060507");
    expect(css).toContain("--surface-1: #0c090d");
    expect(css).toContain("--surface-2: #130f14");
    expect(css).toContain("--surface-3: #1b151c");
    expect(css).toMatch(/body\s*\{[^}]*radial-gradient\([^)]*#1d0a06[^)]*\)[^}]*linear-gradient\([^)]*#060507/s);
  });

  it("enthält kleine Desktopgrößen und den reduzierten Bewegungsmodus", () => {
    expect(css).toMatch(/@media\s*\(max-width:\s*1000px\)/);
    expect(css).toMatch(/@media\s*\(max-width:\s*620px\)/);
    expect(css).toMatch(/prefers-reduced-motion:\s*reduce/);
  });

  it("isoliert Onboarding-Schritte und einfache Listen von der globalen Dreispaltenregel", () => {
    expect(css).toMatch(/\.onboarding-progress li\s*\{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\)/s);
    expect(css).toMatch(/\.onboarding-panel\s*>\s*ul:not\(\.character-availability\):not\(\.readiness-list\)\s*>\s*li\s*\{[^}]*display:\s*list-item/s);
    expect(css).toMatch(/body\s*\{[^}]*overflow-x:\s*hidden/s);
  });
});
