import { beforeEach, describe, expect, it } from "vitest";
import { ApiError } from "../api/generated";
import { changeAppLanguage, i18n } from "../i18n";
import { presentApiError } from "../i18n/presenters";
import { isRunStartable, queueStartErrorText, runAvailabilityText, selectionErrorText } from "./runReasons";

describe("runReasons", () => {
  beforeEach(() => changeAppLanguage("de"));

  it("unterscheidet Klassenkonflikt und fehlende Strategy", () => {
    expect(runAvailabilityText("unavailable", ["profile_class_mismatch"], "", i18n.t).detail).toContain("anderen Klasse");
    expect(runAvailabilityText("unavailable", ["profile_run_strategy_unavailable"], "paladin", i18n.t).detail).toBe("Für dieses Kampfprofil ist diese Route noch nicht freigegeben.");
    expect(runAvailabilityText("unavailable", ["character_profile_run_incompatible"], "", i18n.t).detail).toContain("anderen Klasse");
  });

  it("übersetzt Kuh-Set- und Lifecycle-Reasons", () => {
    expect(runAvailabilityText("unavailable", ["leg_acquisition_route_missing"], "", i18n.t).detail).toContain("Wirt-Route");
    expect(runAvailabilityText("unavailable", ["cow_sweep_route_stale"], "", i18n.t).detail).toContain("Cow-Route");
    expect(runAvailabilityText("unavailable", ["route_lifecycle_unavailable"], "", i18n.t).detail).toContain("nicht mehr verwendbar");
  });

  it("erlaubt Start nur bei verfügbaren Runs", () => {
    expect(isRunStartable("available")).toBe(true);
    expect(isRunStartable("runtime_validation_required")).toBe(true);
    expect(isRunStartable("unavailable")).toBe(false);
  });

  it("präsentiert strukturierte API-Fehler ohne Satzfragment-Parsing", () => {
    expect(queueStartErrorText(new ApiError("queue_entry_unavailable", { run_id: "mephisto" }, "req-1", 409), i18n.t)).toBe("Eine Route in der Reihenfolge ist für diesen Charakter nicht startfähig.");
    expect(queueStartErrorText(new ApiError("queue_context_mismatch", {}, "req-2", 409), i18n.t)).toBe("Die Queue gehört nicht zur bestätigten Auswahl.");
    expect(queueStartErrorText(new ApiError("unsupported_resolution", { expected_width: 1280, expected_height: 720, actual_width: 1920, actual_height: 1080 }, "req-3", 409), i18n.t)).toBe(
      "D2R läuft in 1920 × 1080. Stelle Fenster-Modus 1280 × 720 ein.",
    );
    expect(queueStartErrorText(new Error("queue_entry_unavailable"), i18n.t)).toBe("Session-Befehl fehlgeschlagen");
  });

  it("präsentiert bekannte Codes in Deutsch und Englisch und unbekannte Codes mit sichtbarem Fallback", async () => {
    const error = new ApiError("character_selection_unconfirmed", {}, "req-4", 409);
    expect(selectionErrorText(error, i18n.t)).toBe("Die Charakterauswahl konnte nicht sicher bestätigt werden.");
    expect(presentApiError(new ApiError("future_code", {}, "req-5", 409), i18n.t, "Fallback")).toBe("Unbekannter Fehler (future_code).");

    await changeAppLanguage("en");
    expect(selectionErrorText(error, i18n.t)).toBe("The character selection could not be confirmed safely.");
    expect(presentApiError(new ApiError("future_code", {}, "req-5", 409), i18n.t, "Fallback")).toBe("Unknown error (future_code).");
  });
});
