import type { ChangeEvent } from "react";

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
  const changeCharacter = (event: ChangeEvent<HTMLSelectElement>) => onCharacterChange(event.target.value);
  return <>
    <div className="route-page-header">
      <div>
        <h1 id="routes-title">Routen</h1>
        <p>Verwalte und erstelle die Laufwege für deine Farming-Runs.</p>
      </div>
      <label className="route-character-select">Charakter
        <select value={character} onChange={changeCharacter} autoFocus={!character}>
          {!character && <option value="">Charakter wählen</option>}
          {characters.map((name) => <option key={name} value={name}>{name}</option>)}
        </select>
      </label>
    </div>
    <nav className="route-tabs" aria-label="Routenbereiche">
      <button type="button" className={area === "library" ? "active" : ""} aria-pressed={area === "library"} onClick={() => onAreaChange("library")}>Meine Routen</button>
      <button type="button" className={area === "recording" ? "active" : ""} aria-pressed={area === "recording"} onClick={() => onAreaChange("recording")}>Route aufnehmen</button>
      <button type="button" className={area === "drafts" ? "active" : ""} aria-pressed={area === "drafts"} onClick={() => onAreaChange("drafts")}>Entwürfe {draftCount > 0 && <span className="route-tab-badge">{draftCount}</span>}</button>
    </nav>
  </>;
}
