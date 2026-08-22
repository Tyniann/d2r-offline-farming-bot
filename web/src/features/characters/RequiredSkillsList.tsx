import type { CharacterSetupRequiredSkillDTO } from "../../api/generated";
import { StatusBadge } from "../../app/ui";
import { useTranslation } from "react-i18next";
import { gameSkillName } from "../../i18n/game";

/** RequiredSkillsList zeigt die Core-geordnete Pflichtskillliste read-only. */
export function RequiredSkillsList({
  skills, standardAttack,
}: {
  skills: CharacterSetupRequiredSkillDTO[];
  standardAttack?: string;
}) {
  const { t, i18n } = useTranslation();
  if (!skills.length) {
    return <p className="hint">{t("characters.requiredEmpty")}</p>;
  }
  return <ul className="required-skills-list" aria-label={t("characters.requiredAria")}>
    {skills.map((skill) => (
      <li key={skill.skill}>
        <strong>{gameSkillName(skill.skill, skill.skill, i18n.resolvedLanguage)}</strong>
        <span>{skill.skill}</span>
        {standardAttack === skill.skill ? <StatusBadge tone="success">{t("characters.standardAttack")}</StatusBadge> : null}
      </li>
    ))}
  </ul>;
}
