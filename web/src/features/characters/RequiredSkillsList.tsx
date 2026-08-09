import type { CharacterSetupRequiredSkillDTO } from "../../api/generated";
import { StatusBadge } from "../../app/ui";

/** RequiredSkillsList zeigt die Core-geordnete Pflichtskillliste read-only. */
export function RequiredSkillsList({
  skills, standardAttack,
}: {
  skills: CharacterSetupRequiredSkillDTO[];
  standardAttack?: string;
}) {
  if (!skills.length) {
    return <p className="hint">Der Core hat für dieses Profil noch keine Pflichtskills geliefert.</p>;
  }
  return <ul className="required-skills-list" aria-label="Pflichtskills">
    {skills.map((skill) => (
      <li key={skill.skill}>
        <strong>{skill.display_name}</strong>
        <span>{skill.skill}</span>
        {standardAttack === skill.skill ? <StatusBadge tone="success">Standardangriff</StatusBadge> : null}
      </li>
    ))}
  </ul>;
}
