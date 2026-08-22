import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { BindingEditor, bindingsFromDTO, bindingsToDTO, emptyBindings } from "./BindingEditor";

describe("BindingEditor", () => {
  afterEach(() => cleanup());

  it("zeigt Pflichtskills, Standardangriff und erkennt doppelte Tasten", () => {
    const onChange = vi.fn();
    const value = bindingsFromDTO({
      skills: { teleport: "f7", town_portal: "f7" },
      belt: { slot_1: "1", slot_2: "2", slot_3: "3", slot_4: "4" },
    });
    render(<BindingEditor
      requiredSkills={[
        { skill: "teleport", skill_id: 54, slot: "right" },
        { skill: "town_portal", skill_id: 359, slot: "right" },
        { skill: "bone_spear", skill_id: 84, slot: "right" },
      ]}
      standardAttack="bone_spear"
      value={value}
      mutable
      onChange={onChange}
    />);
    expect(screen.getByText("Standardangriff")).toBeInTheDocument();
    expect(screen.getAllByText(/Taste F7 ist doppelt belegt/).length).toBeGreaterThan(0);
    fireEvent.change(screen.getByLabelText("Knochenspeer Taste"), { target: { value: "f8" } });
    expect(onChange).toHaveBeenCalledWith({
      skills: { teleport: "f7", town_portal: "f7", bone_spear: "f8" },
      belt: { slot_1: "1", slot_2: "2", slot_3: "3", slot_4: "4" },
      belt_layout: { slot_1: "healing", slot_2: "mana", slot_3: "mana", slot_4: "rejuvenation" },
    });
  });

  it("serialisiert Tasten und Trankspalten", () => {
    expect(bindingsToDTO({
      skills: { teleport: "f7", bone_spear: "" },
      belt: { slot_1: "1", slot_2: "", slot_3: "3", slot_4: "4" },
      belt_layout: { slot_1: "healing", slot_2: "healing", slot_3: "rejuvenation", slot_4: "rejuvenation" },
    })).toEqual({
      skills: { teleport: "f7" },
      belt: { slot_1: "1", slot_3: "3", slot_4: "4" },
      belt_layout: { slot_1: "healing", slot_2: "healing", slot_3: "rejuvenation", slot_4: "rejuvenation" },
    });
    expect(emptyBindings()).toEqual({
      skills: {},
      belt: {},
      belt_layout: { slot_1: "healing", slot_2: "mana", slot_3: "mana", slot_4: "rejuvenation" },
    });
  });

  it("ändert den Tranktyp eines Gürtelslots", () => {
    const onChange = vi.fn();
    render(<BindingEditor
      requiredSkills={[{ skill: "teleport", skill_id: 54, slot: "right" }]}
      value={bindingsFromDTO({ skills: { teleport: "f7" }, belt: { slot_1: "1", slot_2: "2", slot_3: "3", slot_4: "4" } })}
      mutable
      onChange={onChange}
    />);
    fireEvent.change(screen.getByLabelText("Gürtel Slot 2 Trank"), { target: { value: "healing" } });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({
      belt_layout: { slot_1: "healing", slot_2: "healing", slot_3: "mana", slot_4: "rejuvenation" },
    }));
  });

  it("zeigt den Hammerdin-Slotvertrag und bedient das optionale CTA-Paar per Tastatur", async () => {
    const onChange = vi.fn();
    render(<BindingEditor
      requiredSkills={[
        { skill: "blessed_hammer", skill_id: 112, slot: "left" },
        { skill: "concentration", skill_id: 113, slot: "right" },
      ]}
      optionalSkillPairs={[{ skills: [
        { skill: "battle_command", skill_id: 155, slot: "right" },
        { skill: "battle_orders", skill_id: 149, slot: "right" },
      ] }]}
      standardAttack="blessed_hammer"
      requiresMercenary
      bindingsReady={false}
      bindingReasons={["profile_bindings_incomplete"]}
      value={bindingsFromDTO({ skills: {}, belt: {} })}
      mutable
      onChange={onChange}
    />);

    expect(screen.getByText("Core: Tasten fehlen")).toBeInTheDocument();
    expect(screen.getByText(/CTA im zweiten Waffenset/)).toBeInTheDocument();
    expect(screen.getByText(/Holy-Shield-Schild/)).toBeInTheDocument();
    expect(screen.getByText(/nicht das Runenwort oder die Söldnerausrüstung/)).toBeInTheDocument();
    expect(screen.getByText("Waffenset II · beide oder keine")).toBeInTheDocument();
    expect(screen.getByText(/lebender Söldner/)).toBeInTheDocument();
    expect(screen.getByText("LMB")).toBeInTheDocument();
    expect(screen.getAllByText(/RMB/)).toHaveLength(3);

    const battleCommand = screen.getByLabelText("Kampfaufruf Taste");
    battleCommand.focus();
    expect(battleCommand).toHaveFocus();
    fireEvent.change(battleCommand, { target: { value: "f4" } });
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    expect(onChange).toHaveBeenCalledWith({
      skills: { battle_command: "f4" },
      belt: { slot_1: "", slot_2: "", slot_3: "", slot_4: "" },
      belt_layout: { slot_1: "healing", slot_2: "mana", slot_3: "mana", slot_4: "rejuvenation" },
    });
  });
});
