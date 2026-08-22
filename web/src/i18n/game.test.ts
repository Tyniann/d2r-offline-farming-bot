import { describe, expect, it } from "vitest";

import { gameAreaName, gameBaseItemName, gameHistoryItemName, gameSkillName } from "./game";

describe("generierte D2R-Namen", () => {
  it("wechselt Gebiets- und Itemnamen nur anhand des aktiven Sprachschlüssels", () => {
    expect(gameAreaName(1, "technical area", "de")).toBe("Lager der Jägerinnen");
    expect(gameAreaName(1, "technical area", "en")).toBe("Rogue Encampment");
    expect(gameBaseItemName("r16", "technical rune", "de")).toBe("Io-Rune");
    expect(gameBaseItemName("r16", "technical rune", "en")).toBe("Io Rune");
  });

  it("löst Produkt-Skills und stabile Historien-Basiscodes auf", () => {
    expect(gameSkillName("bone_spear", "technical skill", "de")).toBe("Knochenspeer");
    expect(gameSkillName("bone_spear", "technical skill", "en")).toBe("Bone Spear");
    expect(gameHistoryItemName({ item_key: "base:r16:normal", item_name: "old name" }, "de")).toBe("Io-Rune");
  });

  it("behält alte technische Namen neutral, wenn kein stabiler Schlüssel auflösbar ist", () => {
    expect(gameHistoryItemName({ item_key: "legacy:unknown", item_name: "Legacy Name" }, "de")).toBe("Legacy Name");
  });
});
