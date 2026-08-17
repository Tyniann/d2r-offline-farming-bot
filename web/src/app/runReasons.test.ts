import { describe, expect, it } from "vitest";
import { isRunStartable, queueStartErrorText, runAvailabilityText, selectionErrorText } from "./runReasons";

describe("runReasons", () => {
  it("unterscheidet Klassenkonflikt und fehlende Strategy", () => {
    expect(runAvailabilityText("unavailable", ["profile_class_mismatch"]).detail).toContain("anderen Klasse");
    expect(runAvailabilityText("unavailable", ["profile_run_strategy_unavailable"], "paladin").detail).toBe("Dieser Run ist für Paladin noch nicht freigegeben.");
    expect(runAvailabilityText("unavailable", ["character_profile_run_incompatible"]).detail).toContain("anderen Klasse");
  });

  it("übersetzt Kuh-Set- und Lifecycle-Reasons", () => {
    expect(runAvailabilityText("unavailable", ["leg_acquisition_route_missing"]).detail).toContain("Wirt-Route");
    expect(runAvailabilityText("unavailable", ["cow_sweep_route_stale"]).detail).toContain("Cow-Route");
    expect(runAvailabilityText("unavailable", ["route_lifecycle_unavailable"]).detail).toContain("nicht mehr verwendbar");
  });

  it("erlaubt Start nur bei verfügbaren Runs", () => {
    expect(isRunStartable("available")).toBe(true);
    expect(isRunStartable("runtime_validation_required")).toBe(true);
    expect(isRunStartable("unavailable")).toBe(false);
  });

  it("zeigt keine Core-Rohcodes im Queue-Fehler", () => {
    expect(queueStartErrorText('queue_entry_unavailable: queue[0]="mephisto": profile_class_mismatch')).not.toMatch(/queue_entry_unavailable|profile_class_mismatch/);
    expect(queueStartErrorText("queue_context_mismatch")).toBe("Die Queue gehört nicht zur bestätigten Auswahl.");
    expect(queueStartErrorText("game_start_failed")).toContain("Rogue Encampment");
    expect(queueStartErrorText("start queue game: session game expected in_game, got menu")).toContain("Rogue Encampment");
  });

  it("übersetzt Charakterauswahl-Timeout ohne Rohcodes", () => {
    expect(selectionErrorText("character selection timeout: d2r window has no usable client area")).toBe("Das D2R-Fenster hat keine nutzbare Fläche. Stelle Fenster-Modus 1280 × 720 ein und lass das Fenster sichtbar, nicht minimiert.");
    expect(selectionErrorText("Die Charakterauswahl konnte nicht sicher bestätigt werden: character selection timeout")).toBe("Der Charakterbildschirm wurde nicht sicher erkannt. D2R muss auf dem Offline-Charakterbildschirm bei 1280 × 720 stehen, und der gewünschte Save muss sichtbar markiert sein.");
    expect(selectionErrorText("character selection timeout")).not.toMatch(/timeout/);
  });
});
