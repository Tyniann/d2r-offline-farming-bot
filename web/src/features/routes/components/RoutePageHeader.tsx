import type { ChangeEvent } from "react";
import { useTranslation } from "react-i18next";

export type RouteArea = "library" | "recording" | "drafts";

interface Props {
  characters: string[];
  character: string;
  area: RouteArea;
  draftCount: number;
  onCharacterChange(character: string): void;
  onAreaChange(area: RouteArea): void;
}

export function RoutePageHeader({ characters, character, area, draftCount, onCharacterChange, onAreaChange }: Props) {
  const { t } = useTranslation();
  const changeCharacter = (event: ChangeEvent<HTMLSelectElement>) => onCharacterChange(event.target.value);
  return <>
    <div className="route-page-header">
      <div>
        <h1 id="routes-title">{t("routes.title")}</h1>
        <p>{t("routes.description")}</p>
      </div>
      <label className="route-character-select">{t("routes.character")}
        <select value={character} onChange={changeCharacter} autoFocus={!character}>
          {!character && <option value="">{t("routes.chooseCharacter")}</option>}
          {characters.map((name) => <option key={name} value={name}>{name}</option>)}
        </select>
      </label>
    </div>
    <nav className="route-tabs" aria-label={t("routes.areasAria")}>
      <button type="button" className={area === "library" ? "active" : ""} aria-pressed={area === "library"} onClick={() => onAreaChange("library")}>{t("routes.libraryTab")}</button>
      <button type="button" className={area === "recording" ? "active" : ""} aria-pressed={area === "recording"} onClick={() => onAreaChange("recording")}>{t("routes.recordingTab")}</button>
      <button type="button" className={area === "drafts" ? "active" : ""} aria-pressed={area === "drafts"} onClick={() => onAreaChange("drafts")}>{t("routes.draftsTab")} {draftCount > 0 && <span className="route-tab-badge">{draftCount}</span>}</button>
    </nav>
  </>;
}
