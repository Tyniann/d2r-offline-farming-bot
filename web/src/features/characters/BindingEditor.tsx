import type {
  CharacterSetupOptionalSkillPairDTO,
  CharacterSetupRequiredSkillDTO,
  OperatorBeltBindingsDTO,
  OperatorBeltLayoutDTO,
  OperatorProfileBindingsDTO,
} from "../../api/generated";
import { StatusBadge } from "../../app/ui";

const skillKeys = ["f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8"] as const;
const beltKeys = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "0", "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z", ",", ".", "-", "]"] as const;
const potionKinds = [
  { value: "healing", label: "Heiltrank" },
  { value: "mana", label: "Manatrank" },
  { value: "rejuvenation", label: "Regeneration" },
] as const;

export type BindingEditorValue = {
  skills: Record<string, string>;
  belt: OperatorBeltBindingsDTO;
  belt_layout: OperatorBeltLayoutDTO;
};

const defaultBeltLayout = (): OperatorBeltLayoutDTO => ({
  slot_1: "healing",
  slot_2: "mana",
  slot_3: "mana",
  slot_4: "rejuvenation",
});

/** emptyBindings liefert einen leeren Binding-Draft mit Standard-Trankspalten. */
export function emptyBindings(): BindingEditorValue {
  return { skills: {}, belt: {}, belt_layout: defaultBeltLayout() };
}

/** bindingsFromDTO normalisiert optionale Core-Bindings für den Editor. */
export function bindingsFromDTO(
  value?: OperatorProfileBindingsDTO | null,
  layoutFallback?: OperatorBeltLayoutDTO | null,
): BindingEditorValue {
  const fallback = layoutFallback && layoutComplete(layoutFallback) ? layoutFallback : defaultBeltLayout();
  return {
    skills: { ...(value?.skills ?? {}) },
    belt: {
      slot_1: value?.belt?.slot_1 ?? "",
      slot_2: value?.belt?.slot_2 ?? "",
      slot_3: value?.belt?.slot_3 ?? "",
      slot_4: value?.belt?.slot_4 ?? "",
    },
    belt_layout: {
      slot_1: value?.belt_layout?.slot_1 || fallback.slot_1 || "healing",
      slot_2: value?.belt_layout?.slot_2 || fallback.slot_2 || "mana",
      slot_3: value?.belt_layout?.slot_3 || fallback.slot_3 || "mana",
      slot_4: value?.belt_layout?.slot_4 || fallback.slot_4 || "rejuvenation",
    },
  };
}

/** bindingsToDTO entfernt leere Skill-/Gürteleinträge vor dem Speichern. */
export function bindingsToDTO(value: BindingEditorValue): OperatorProfileBindingsDTO {
  const skills: Record<string, string> = {};
  for (const [skill, key] of Object.entries(value.skills)) {
    const trimmed = key.trim();
    if (trimmed) skills[skill] = trimmed;
  }
  const belt: OperatorBeltBindingsDTO = {};
  if (value.belt.slot_1?.trim()) belt.slot_1 = value.belt.slot_1.trim();
  if (value.belt.slot_2?.trim()) belt.slot_2 = value.belt.slot_2.trim();
  if (value.belt.slot_3?.trim()) belt.slot_3 = value.belt.slot_3.trim();
  if (value.belt.slot_4?.trim()) belt.slot_4 = value.belt.slot_4.trim();
  const belt_layout: OperatorBeltLayoutDTO = {
    slot_1: value.belt_layout.slot_1,
    slot_2: value.belt_layout.slot_2,
    slot_3: value.belt_layout.slot_3,
    slot_4: value.belt_layout.slot_4,
  };
  return { skills, belt, belt_layout };
}

/** BindingEditor belegt profilautorisierte Pflicht- und optionale Skills sowie den Gürtel. */
export function BindingEditor({
  requiredSkills, optionalSkillPairs = [], standardAttack, requiresMercenary = false,
  bindingsReady, bindingReasons = [], value, mutable, onChange,
}: {
  requiredSkills: CharacterSetupRequiredSkillDTO[];
  optionalSkillPairs?: CharacterSetupOptionalSkillPairDTO[];
  standardAttack?: string;
  requiresMercenary?: boolean;
  bindingsReady?: boolean;
  bindingReasons?: string[];
  value: BindingEditorValue;
  mutable: boolean;
  onChange: (next: BindingEditorValue) => void;
}) {
  const collisions = collectCollisions(value);

  return <div className="binding-editor">
    {bindingsReady !== undefined && <div className="binding-readiness" role="status" aria-label="Core-Status der Tastenbelegung">
      <StatusBadge tone={bindingsReady ? "success" : "warning"}>
        {bindingsReady ? "Core: Tasten vollständig" : "Core: Tasten fehlen"}
      </StatusBadge>
      {!bindingsReady && bindingReasons.length > 0
        ? <span>{bindingReasons.map((reason) => bindingReasonText(reason, optionalSkillPairs.length > 0)).join(" ")}</span>
        : null}
    </div>}
    <p className="hint">Der Mausslot ist durch das Kampfprofil festgelegt. Du wählst nur die F-Taste.</p>
    <div className="binding-skill-grid" role="group" aria-label="Skilltasten">
      {requiredSkills.map((skill) => {
        const selected = value.skills[skill.skill] ?? "";
        const collision = selected ? collisions[selected] : undefined;
        return <label key={skill.skill} className={collision ? "binding-collision" : undefined}>
          <span className="binding-skill-label">
            <strong>{skill.display_name}</strong>
            <StatusBadge tone="neutral">{skillSlotLabel(skill.slot)}</StatusBadge>
            {standardAttack === skill.skill ? <StatusBadge tone="success">Standardangriff</StatusBadge> : null}
          </span>
          <select
            value={selected}
            disabled={!mutable}
            aria-label={`${skill.display_name} Taste`}
            aria-invalid={!!collision}
            onChange={(event) => onChange({
              ...value,
              skills: { ...value.skills, [skill.skill]: event.target.value },
            })}
          >
            <option value="">Nicht belegt</option>
            {skillKeys.map((key) => <option key={key} value={key}>{key.toUpperCase()}</option>)}
          </select>
          {collision ? <small role="alert">{collision}</small> : null}
        </label>;
      })}
    </div>

    {optionalSkillPairs.map((pair, pairIndex) => <section className="binding-optional-pair" key={pair.skills.map((skill) => skill.skill).join("-") || pairIndex}>
      <div className="binding-optional-heading">
        <div>
          <h4>Optional: Call to Arms</h4>
          <p>Call to Arms ist optional. Wenn du Battle Command und Battle Orders belegst, muss CTA im zweiten Waffenset liegen. Ein Holy-Shield-Schild darf ebenfalls dort ausgerüstet sein. Der Bot prüft Waffenset und Skillauswahl, aber nicht das Runenwort oder die Söldnerausrüstung.</p>
        </div>
        <StatusBadge tone="neutral">Waffenset II · beide oder keine</StatusBadge>
      </div>
      <div className="binding-skill-grid" role="group" aria-label="Optionale Call-to-Arms-Tasten">
        {pair.skills.map((skill) => {
          const selected = value.skills[skill.skill] ?? "";
          const collision = selected ? collisions[selected] : undefined;
          return <label key={skill.skill} className={collision ? "binding-collision" : undefined}>
            <span className="binding-skill-label">
              <strong>{skill.display_name}</strong>
              <StatusBadge tone="neutral">Optional · {skillSlotLabel(skill.slot)}</StatusBadge>
            </span>
            <select
              value={selected}
              disabled={!mutable}
              aria-label={`${skill.display_name} Taste`}
              aria-invalid={!!collision}
              onChange={(event) => onChange({
                ...value,
                skills: { ...value.skills, [skill.skill]: event.target.value },
              })}
            >
              <option value="">Nicht belegt</option>
              {skillKeys.map((key) => <option key={key} value={key}>{key.toUpperCase()}</option>)}
            </select>
            {collision ? <small role="alert">{collision}</small> : null}
          </label>;
        })}
      </div>
    </section>)}

    {requiresMercenary && <p className="binding-mercenary-note">Für Hammerdin muss ein lebender Söldner verfügbar sein. Seine Ausrüstung wird nicht geprüft.</p>}

    <h4>Gürtel</h4>
    <p className="hint">Pro Spalte Taste und Tranktyp festlegen. Der Bot hebt und benutzt nur die hier zugeordneten Tränke.</p>
    <div className="binding-belt-grid" role="group" aria-label="Gürtelbelegung">
      {([1, 2, 3, 4] as const).map((slot) => {
        const field = `slot_${slot}` as keyof OperatorBeltBindingsDTO;
        const selected = value.belt[field] ?? "";
        const potion = value.belt_layout[field] ?? "";
        const collision = selected ? collisions[selected] : undefined;
        return <div key={slot} className={collision ? "binding-belt-slot binding-collision" : "binding-belt-slot"}>
          <span className="binding-belt-slot-title">Slot {slot}</span>
          <label>
            <span>Taste</span>
            <select
              value={selected}
              disabled={!mutable}
              aria-label={`Gürtel Slot ${slot} Taste`}
              aria-invalid={!!collision}
              onChange={(event) => onChange({
                ...value,
                belt: { ...value.belt, [field]: event.target.value },
              })}
            >
              <option value="">Nicht belegt</option>
              {beltKeys.map((key) => <option key={key} value={key}>{key}</option>)}
            </select>
          </label>
          <label>
            <span>Trank</span>
            <select
              value={potion}
              disabled={!mutable}
              aria-label={`Gürtel Slot ${slot} Trank`}
              onChange={(event) => onChange({
                ...value,
                belt_layout: { ...value.belt_layout, [field]: event.target.value as OperatorBeltLayoutDTO[typeof field] },
              })}
            >
              {potionKinds.map((kind) => <option key={kind.value} value={kind.value}>{kind.label}</option>)}
            </select>
          </label>
          {collision ? <small role="alert">{collision}</small> : null}
        </div>;
      })}
    </div>
  </div>;
}

function layoutComplete(layout: OperatorBeltLayoutDTO): boolean {
  return Boolean(layout.slot_1 && layout.slot_2 && layout.slot_3 && layout.slot_4);
}

function skillSlotLabel(slot: CharacterSetupRequiredSkillDTO["slot"]): string {
  if (slot === "left") return "LMB";
  if (slot === "right") return "RMB";
  return "Mausslot";
}

function bindingReasonText(reason: string, hasOptionalPair: boolean): string {
  if (reason === "profile_bindings_incomplete" && hasOptionalPair) return "Pflichtskills, Gürtel oder das optionale CTA-Paar sind noch nicht vollständig gültig.";
  if (reason === "profile_bindings_incomplete") return "Pflichtskills oder Gürtel sind noch nicht vollständig gültig.";
  return "Die Tastenbelegung ist laut Core noch nicht gültig.";
}

function collectCollisions(value: BindingEditorValue): Record<string, string> {
  const owners = new Map<string, string>();
  const collisions: Record<string, string> = {};
  const mark = (rawKey: string | undefined, owner: string) => {
    const key = (rawKey ?? "").trim();
    if (!key) return;
    const prior = owners.get(key);
    if (prior && prior !== owner) {
      collisions[key] = `Taste ${key.toUpperCase()} ist doppelt belegt (${prior} und ${owner}).`;
      return;
    }
    owners.set(key, owner);
  };
  for (const [skill, key] of Object.entries(value.skills)) mark(key, skill);
  mark(value.belt.slot_1, "Gürtel 1");
  mark(value.belt.slot_2, "Gürtel 2");
  mark(value.belt.slot_3, "Gürtel 3");
  mark(value.belt.slot_4, "Gürtel 4");
  return collisions;
}
