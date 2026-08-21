import { createContext, type ReactNode, useContext, useEffect, useMemo, useRef, useState } from "react";
import type { CatalogDTO, StatusDTO } from "../api/generated";

export interface AppSelection {
  character: string;
  difficulty: string;
}

export interface AppSelectionContextValue extends AppSelection {
  selectCharacter(character: string, preferredDifficulty?: string): void;
  selectDifficulty(difficulty: string): void;
  confirm(selection: AppSelection): void;
}

const AppSelectionContext = createContext<AppSelectionContextValue | null>(null);

/** useAppSelectionState owns the app-wide selection without treating it as D2R confirmation. */
export function useAppSelectionState(
  catalog: CatalogDTO | null,
  status: StatusDTO | null,
  preference: Partial<AppSelection> | undefined = {},
  onPreferenceChange?: (selection: AppSelection) => void,
): AppSelectionContextValue {
  const [selection, setSelection] = useState<AppSelection>({ character: "", difficulty: "" });
  const initialized = useRef(false);

  useEffect(() => {
    if (!catalog || preference === undefined) return;
    const selectable = catalog.characters.filter((entry) => entry.selectable);
    const characterExists = selectable.some((entry) => entry.name === selection.character);
    const difficultyExists = catalog.difficulties.some((entry) => entry.id === selection.difficulty);
    if (initialized.current && characterExists && difficultyExists) return;

    const confirmedCharacter = status?.selection.character;
    const confirmedDifficulty = status?.selection.difficulty;
    const preferredCharacter = selectable.some((entry) => entry.name === preference.character) ? preference.character : "";
    const preferredDifficulty = catalog.difficulties.some((entry) => entry.id === preference.difficulty) ? preference.difficulty : "";
    const character = preferredCharacter
      || confirmedCharacter
      || (characterExists ? selection.character : selectable[0]?.name ?? "");
    const difficulty = preferredDifficulty
      || (confirmedDifficulty ? confirmedDifficulty : "")
      || (difficultyExists ? selection.difficulty : catalog.default_difficulty);
    const next = { character, difficulty };
    initialized.current = true;
    setSelection(next);
    if (character && difficulty && (preference.character !== character || preference.difficulty !== difficulty)) onPreferenceChange?.(next);
  }, [catalog, onPreferenceChange, preference?.character, preference?.difficulty, selection.character, selection.difficulty, status?.selection.character, status?.selection.difficulty]);

  return useMemo(() => ({
    ...selection,
    selectCharacter(character: string, preferredDifficulty?: string) {
      const next = { character, difficulty: preferredDifficulty || selection.difficulty };
      setSelection(next);
      if (next.character && next.difficulty) onPreferenceChange?.(next);
    },
    selectDifficulty(difficulty: string) {
      const next = { ...selection, difficulty };
      setSelection(next);
      if (next.character && next.difficulty) onPreferenceChange?.(next);
    },
    confirm(next: AppSelection) {
      setSelection(next);
      if (next.character && next.difficulty) onPreferenceChange?.(next);
    },
  }), [onPreferenceChange, selection]);
}

/** AppSelectionProvider publishes the single app-wide character and difficulty selection. */
export function AppSelectionProvider({ value, children }: { value: AppSelectionContextValue; children: ReactNode }) {
  return <AppSelectionContext.Provider value={value}>{children}</AppSelectionContext.Provider>;
}

/** useAppSelection returns the current app selection and its update functions. */
export function useAppSelection(): AppSelectionContextValue {
  const value = useContext(AppSelectionContext);
  if (!value) throw new Error("useAppSelection muss innerhalb von AppSelectionProvider verwendet werden");
  return value;
}
