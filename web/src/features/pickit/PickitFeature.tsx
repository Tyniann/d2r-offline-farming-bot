import { useEffect, useMemo, useRef, useState } from "react";
import { createPickitProfile, deletePickitProfile, duplicatePickitProfile, updatePickitAssignment, updatePickitProfile } from "../../api/client";
import {
  getPickitAssignments, getPickitCatalog, getPickitProfiles, importPickit, validatePickitProfile,
  type PickitAssignmentsDTO, type PickitCatalogDTO, type PickitProfileDTO, type PickitRuleDTO,
} from "../../api/generated";
import { Button, Dialog } from "../../app/ui";

interface Props { characters: string[]; selectedCharacter: string; runs: string[]; locked: boolean; refreshKey: number }
const actionLabels: Record<string, string> = { keep: "Behalten", sell: "Identifizieren / verkaufen" };
const pickitSlugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
type PickitDialog =
  | { kind: "create"; id: string; name: string }
  | { kind: "duplicate"; id: string; name: string }
  | { kind: "delete" }
  | { kind: "discard"; nextID: string };

export function PickitFeature({ characters, selectedCharacter, runs, locked, refreshKey }: Props) {
  const [catalog, setCatalog] = useState<PickitCatalogDTO | null>(null);
  const [profiles, setProfiles] = useState<PickitProfileDTO[]>([]);
  const [assignments, setAssignments] = useState<PickitAssignmentsDTO | null>(null);
  const [selectedID, setSelectedID] = useState("");
  const [draft, setDraft] = useState<PickitProfileDTO | null>(null);
  const [saved, setSaved] = useState("");
  const [query, setQuery] = useState("");
  const [newAction, setNewAction] = useState("keep");
  const [ethereal, setEthereal] = useState(false);
  const [quality, setQuality] = useState("unique");
  const [tier, setTier] = useState("elite");
  const [advanced, setAdvanced] = useState(false);
  const [importText, setImportText] = useState("");
  const [assignmentCharacter, setAssignmentCharacter] = useState(selectedCharacter);
  const [assignmentRun, setAssignmentRun] = useState(runs[0] ?? "");
  const [assignmentProfiles, setAssignmentProfiles] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [dialog, setDialog] = useState<PickitDialog | null>(null);
  const [dialogError, setDialogError] = useState("");
  const dialogInputRef = useRef<HTMLInputElement>(null);

  const dirty = draft ? JSON.stringify(draft) !== saved : false;
  const load = async (signal?: AbortSignal) => {
    const [nextCatalog, nextProfiles, nextAssignments] = await Promise.all([getPickitCatalog(signal), getPickitProfiles(signal), getPickitAssignments(signal)]);
    setCatalog(nextCatalog); setProfiles(nextProfiles.profiles); setAssignments(nextAssignments);
    const nextID = nextProfiles.profiles.some((profile) => profile.id === selectedID) ? selectedID : nextProfiles.profiles[0]?.id ?? "";
    selectProfile(nextProfiles.profiles, nextID); setError("");
  };
  useEffect(() => { const controller = new AbortController(); void load(controller.signal).catch((reason: unknown) => !controller.signal.aborted && setError(message(reason))); return () => controller.abort(); }, [refreshKey]);
  useEffect(() => { if (selectedCharacter) setAssignmentCharacter(selectedCharacter); }, [selectedCharacter]);
  useEffect(() => { if (!assignmentRun && runs[0]) setAssignmentRun(runs[0]); }, [assignmentRun, runs]);
  useEffect(() => {
    if (!assignments || !assignmentCharacter || !assignmentRun) return;
    const values = assignments.assignments as Record<string, Record<string, string[]>>;
    const key = Object.keys(values).find((candidate) => candidate.toLowerCase() === assignmentCharacter.toLowerCase());
    setAssignmentProfiles(key ? [...(values[key]?.[assignmentRun] ?? [])] : []);
  }, [assignments, assignmentCharacter, assignmentRun]);
  useEffect(() => { const protect = (event: BeforeUnloadEvent) => { if (dirty) event.preventDefault(); }; window.addEventListener("beforeunload", protect); return () => window.removeEventListener("beforeunload", protect); }, [dirty]);

  const normalizedQuery = query.trim().toLowerCase();
  const matchingBases = useMemo(() => catalog?.bases.filter((entry) => normalizedQuery && `${entry.name} ${entry.code}`.toLowerCase().includes(normalizedQuery)).slice(0, 8) ?? [], [catalog, normalizedQuery]);
  const matchingIdentities = useMemo(() => catalog?.identities.filter((entry) => normalizedQuery && `${entry.display_name} ${entry.set_name ?? ""}`.toLowerCase().includes(normalizedQuery)).slice(0, 12) ?? [], [catalog, normalizedQuery]);
  const matchingSets = useMemo(() => {
    const groups = new Map<string, typeof matchingIdentities>();
    for (const entry of catalog?.identities ?? []) if (entry.kind === "set" && entry.set_name && entry.set_name.toLowerCase().includes(normalizedQuery)) groups.set(entry.set_name, [...(groups.get(entry.set_name) ?? []), entry]);
    return [...groups.entries()].filter(([, entries]) => entries.length > 1).slice(0, 5);
  }, [catalog, normalizedQuery]);

  function selectProfile(source: PickitProfileDTO[], id: string) { const profile = source.find((entry) => entry.id === id); setSelectedID(id); setDraft(profile ? clone(profile) : null); setSaved(profile ? JSON.stringify(profile) : ""); setNotice(""); }
  function chooseProfile(id: string) {
    if (dirty) {
      setDialogError("");
      setDialog({ kind: "discard", nextID: id });
      return;
    }
    selectProfile(profiles, id);
  }
  function openCreateDialog() { setDialogError(""); setDialog({ kind: "create", id: "mein-profil", name: "Mein Profil" }); }
  function openDuplicateDialog() {
    if (!draft) return;
    setDialogError("");
    setDialog({ kind: "duplicate", id: `${draft.id}-kopie`, name: `${draft.name} – Kopie` });
  }
  function openDeleteDialog() { if (!draft) return; setDialogError(""); setDialog({ kind: "delete" }); }
  function closeDialog() { if (busy) return; setDialog(null); setDialogError(""); }
  function addRules(rules: Array<Omit<PickitRuleDTO, "id">>) { if (!draft) return; const existing = new Set(draft.rules.map((rule) => rule.id)); const next = rules.map((rule, index) => ({ ...rule, id: uniqueRuleID(existing, `regel-${draft.rules.length + index + 1}`) })); setDraft({ ...draft, rules: [...draft.rules, ...next] }); }
  function addSet(name: string, entries: PickitCatalogDTO["identities"]) { addRules(entries.map((entry) => ({ action: newAction, expression: `[setitem] == ${JSON.stringify(entry.key)}` }))); setNotice(`${name} wurde sichtbar in ${entries.length} Einzelregeln expandiert.`); }
  function addIdentity(entry: PickitCatalogDTO["identities"][number]) { addRules([{ action: newAction, expression: `[${entry.kind}item] == ${JSON.stringify(entry.key)}` }]); }
  function addBase(code: string) { addRules([{ action: newAction, expression: `[name] == ${JSON.stringify(code)}${ethereal ? " && [ethereal] == true" : ""}` }]); }
  function updateRule(index: number, replacement: PickitRuleDTO) { if (!draft) return; const rules = [...draft.rules]; rules[index] = replacement; setDraft({ ...draft, rules }); }
  function moveRule(index: number, delta: number) { if (!draft) return; const target = index + delta; if (target < 0 || target >= draft.rules.length) return; const rules = [...draft.rules]; [rules[index], rules[target]] = [rules[target], rules[index]]; setDraft({ ...draft, rules }); }

  async function validateDraft() { if (!draft) return; setBusy(true); setError(""); try { const result = await validatePickitProfile({ profile: draft }); setDraft(result.profile); setNotice("Entwurf ist gültig und wurde kanonisiert."); } catch (reason) { setError(message(reason)); } finally { setBusy(false); } }
  async function saveDraft() { if (!draft) return; setBusy(true); setError(""); try { const result = await updatePickitProfile(draft.id, { expected_revision: draft.revision, profile: draft }); setProfiles((current) => current.map((profile) => profile.id === result.id ? result : profile)); setDraft(clone(result)); setSaved(JSON.stringify(result)); setNotice("Profil gespeichert. Die Änderung gilt ab der nächsten validierten Run-Grenze."); } catch (reason) { setError(message(reason)); } finally { setBusy(false); } }
  async function submitCreateProfile() {
    if (!dialog || dialog.kind !== "create") return;
    const id = dialog.id.trim();
    const name = dialog.name.trim();
    if (!pickitSlugPattern.test(id)) {
      setDialogError("Die Profil-ID muss ein Kleinbuchstaben-Slug sein, z. B. mein-profil.");
      return;
    }
    if (!name) {
      setDialogError("Der Anzeigename darf nicht leer sein.");
      return;
    }
    setBusy(true); setError(""); setDialogError("");
    try {
      const result = await createPickitProfile({ profile: { schema_version: 1, revision: 1, id, name, rules: [{ id: "regel-1", action: "keep", expression: `[type] == "rune"` }] } });
      const next = [...profiles, result];
      setProfiles(next);
      selectProfile(next, result.id);
      setDialog(null);
      setNotice(`Profil ${result.name} angelegt.`);
    } catch (reason) {
      setDialogError(message(reason));
    } finally {
      setBusy(false);
    }
  }
  async function submitDuplicateProfile() {
    if (!draft || !dialog || dialog.kind !== "duplicate") return;
    const target_id = dialog.id.trim();
    const target_name = dialog.name.trim();
    if (!pickitSlugPattern.test(target_id)) {
      setDialogError("Die Profil-ID muss ein Kleinbuchstaben-Slug sein, z. B. basis-kopie.");
      return;
    }
    if (!target_name) {
      setDialogError("Der Anzeigename darf nicht leer sein.");
      return;
    }
    setBusy(true); setError(""); setDialogError("");
    try {
      const result = await duplicatePickitProfile(draft.id, { target_id, target_name });
      const next = [...profiles, result];
      setProfiles(next);
      selectProfile(next, result.id);
      setDialog(null);
      setNotice(`Profil ${result.name} angelegt.`);
    } catch (reason) {
      setDialogError(message(reason));
    } finally {
      setBusy(false);
    }
  }
  async function submitDeleteProfile() {
    if (!draft) return;
    setBusy(true); setError(""); setDialogError("");
    try {
      await deletePickitProfile(draft.id, { expected_revision: draft.revision });
      const next = profiles.filter((profile) => profile.id !== draft.id);
      setProfiles(next);
      selectProfile(next, next[0]?.id ?? "");
      setDialog(null);
      setNotice("Profil gelöscht.");
    } catch (reason) {
      setDialog(null);
      setError(message(reason));
    } finally {
      setBusy(false);
    }
  }
  function confirmDiscard() {
    if (!dialog || dialog.kind !== "discard") return;
    const nextID = dialog.nextID;
    setDialog(null);
    selectProfile(profiles, nextID);
  }
  async function pasteImport() { if (!importText.trim()) { setError("Für den Import ist NIP-Text erforderlich."); return; } setBusy(true); try { const result = await importPickit({ text: importText, action: newAction }); addRules(result.rules.map((rule) => ({ action: rule.action, expression: rule.expression }))); setNotice(result.warnings.join(" ")); } catch (reason) { setError(message(reason)); } finally { setBusy(false); } }
  async function saveAssignment() { if (!assignments || !assignmentCharacter || !assignmentRun) return; if (!assignmentProfiles.length) { setError("Mindestens ein Profil muss zugeordnet sein."); return; } setBusy(true); try { const result = await updatePickitAssignment({ character: assignmentCharacter, run_id: assignmentRun, profile_ids: assignmentProfiles, expected_revision: assignments.revision }); setAssignments(result); setNotice("Zuordnung gespeichert; sie wird vor dem nächsten Run neu validiert."); } catch (reason) { setError(message(reason)); } finally { setBusy(false); } }

  return <section aria-labelledby="pickit-title">
    <h2 id="pickit-title">Pickit-Profile</h2>
    <p>Globale Profile werden in fester Reihenfolge pro Charakter und Run ausgewertet. Das erste Match entscheidet.</p>
    {error && <p role="alert">{error} {error.toLowerCase().includes("revision") && <button type="button" className="secondary" onClick={() => void load()}>Aktuellen Stand laden</button>}</p>}
    {notice && <p role="status">{notice}</p>}
    {!catalog && !error && <p>Pickit-Katalog wird geladen …</p>}
    {catalog && <div className="pickit-layout">
      <aside className="pickit-library"><div className="section-heading"><h3>Profilbibliothek</h3><button type="button" onClick={openCreateDialog} disabled={locked || busy}>Neu</button></div>
        {profiles.length === 0 ? <p>Noch keine Profile vorhanden.</p> : <ul>{profiles.map((profile) => <li key={profile.id}><button type="button" className={profile.id === selectedID ? "active" : "secondary"} onClick={() => chooseProfile(profile.id)}>{profile.name}<small>{profile.id} · Revision {profile.revision}</small></button></li>)}</ul>}
      </aside>
      <div className="pickit-editor">
        {!draft ? <p>Profil auswählen oder neu anlegen.</p> : <>
          <div className="section-heading"><div><h3>{draft.name}</h3><p>{dirty ? "Ungespeicherte Änderungen" : `Gespeichert · Revision ${draft.revision}`}</p></div><div className="inline-actions"><button type="button" className="secondary" disabled={busy} onClick={openDuplicateDialog}>Duplizieren</button><button type="button" className="danger" disabled={locked || busy} onClick={openDeleteDialog}>Löschen</button></div></div>
          <label>Profilname<input value={draft.name} disabled={locked} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label>
          <fieldset><legend>Neue Regel</legend><div className="guided-rule"><label>Suche nach Set, Item oder Basis<input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="z. B. Tal Rasha oder Thresher" /></label><label>Aktion<select value={newAction} onChange={(event) => setNewAction(event.target.value)}><option value="keep">Behalten</option><option value="sell">Identifizieren / verkaufen</option></select></label><label className="check"><input type="checkbox" checked={ethereal} onChange={(event) => setEthereal(event.target.checked)} /> Nur ätherisch</label></div>
            <div className="quick-rules"><button type="button" className="secondary" onClick={() => addRules([{ action: newAction, expression: `[type] == "rune"` }])}>Alle Runen</button><button type="button" className="secondary" onClick={() => addRules([{ action: newAction, expression: `[name] == "pk1"` }])}>Schlüssel des Terrors</button><button type="button" className="secondary" onClick={() => addRules([["gzv", "gpv"], ["gly", "gpy"], ["glb", "gpb"], ["glg", "gpg"], ["glr", "gpr"], ["glw", "gpw"], ["skl", "skz"]].map((codes) => ({ action: newAction, expression: `[name] == "${codes[0]}" || [name] == "${codes[1]}"` })))}>Makellose/perfekte Gems (7 Regeln)</button><label>Qualität<select value={quality} onChange={(event) => setQuality(event.target.value)}>{catalog.qualities.map((entry) => <option key={entry}>{entry}</option>)}</select></label><button type="button" className="secondary" onClick={() => addRules([{ action: newAction, expression: `[quality] == "${quality}"` }])}>Qualität hinzufügen</button><label>Tier<select value={tier} onChange={(event) => setTier(event.target.value)}><option value="normal">normal</option><option value="exceptional">exceptional</option><option value="elite">elite</option></select></label><button type="button" className="secondary" onClick={() => addRules([{ action: newAction, expression: `[tier] == "${tier}"` }])}>Tier hinzufügen</button></div>
            {normalizedQuery && <div className="search-results" aria-label="Katalogtreffer">{matchingSets.map(([name, entries]) => <button type="button" key={name} onClick={() => addSet(name, entries)}>Ganzes Set {name} hinzufügen ({entries.length})</button>)}{matchingIdentities.map((entry) => <button type="button" className="secondary" key={`${entry.kind}-${entry.raw_id}`} onClick={() => addIdentity(entry)}>{entry.display_name} <small>{entry.kind}</small></button>)}{matchingBases.map((entry) => <button type="button" className="secondary" key={entry.txt_file_no} onClick={() => addBase(entry.code)}>{entry.name} <small>{entry.code} · {entry.base_tier}</small></button>)}</div>}
          </fieldset>
          <h4>Regelreihenfolge</h4><ol className="rule-list">{draft.rules.map((rule, index) => <li key={rule.id}><div><strong>{index + 1}. {actionLabels[rule.action] ?? rule.action}</strong><code>{rule.expression}</code></div><div className="inline-actions"><button type="button" className="secondary" aria-label={`Regel ${index + 1} nach oben`} disabled={locked || index === 0} onClick={() => moveRule(index, -1)}>↑</button><button type="button" className="secondary" aria-label={`Regel ${index + 1} nach unten`} disabled={locked || index === draft.rules.length - 1} onClick={() => moveRule(index, 1)}>↓</button><button type="button" className="secondary" disabled={locked} onClick={() => setDraft({ ...draft, rules: draft.rules.filter((_, item) => item !== index) })}>Entfernen</button></div>{advanced && <label>Ausdruck<input value={rule.expression} onChange={(event) => updateRule(index, { ...rule, expression: event.target.value })} /></label>}</li>)}</ol>
          <button type="button" className="secondary" onClick={() => setAdvanced((value) => !value)}>{advanced ? "Erweitertes Ausdrucksfeld schließen" : "Erweitertes Ausdrucksfeld"}</button>
          {advanced && <div className="import-box"><label>NIP-Text<textarea value={importText} onChange={(event) => setImportText(event.target.value)} /></label><label>Datei importieren<input type="file" accept=".nip,.txt,text/plain" onChange={(event) => { const file = event.target.files?.[0]; if (file) void file.text().then(setImportText); }} /></label><button type="button" className="secondary" onClick={() => void pasteImport()} disabled={busy}>Als Entwurf importieren</button></div>}
          <div className="editor-actions"><button type="button" className="secondary" onClick={() => void validateDraft()} disabled={busy}>Entwurf prüfen</button><button type="button" onClick={() => void saveDraft()} disabled={locked || busy || !dirty || draft.rules.length === 0}>{busy ? "Core prüft …" : "Profil speichern"}</button></div>
        </>}
      </div>
    </div>}
    {assignments && <div className="assignment-editor"><h3>Zuordnung</h3><div className="selection-grid"><label>Charakter<select value={assignmentCharacter} onChange={(event) => setAssignmentCharacter(event.target.value)}>{characters.map((character) => <option key={character}>{character}</option>)}</select></label><label>Run<select value={assignmentRun} onChange={(event) => setAssignmentRun(event.target.value)}>{runs.map((run) => <option key={run}>{run}</option>)}</select></label></div><p>Reihenfolge durch Auswahl; das erste passende Profil gewinnt.</p><ol>{assignmentProfiles.map((id, index) => <li key={id}>{index + 1}. {profiles.find((profile) => profile.id === id)?.name ?? id}<button type="button" className="secondary" onClick={() => setAssignmentProfiles((current) => current.filter((entry) => entry !== id))}>Entfernen</button></li>)}</ol><label>Profil hinzufügen<select value="" onChange={(event) => { const id = event.target.value; if (id && !assignmentProfiles.includes(id)) setAssignmentProfiles((current) => [...current, id]); }}><option value="">Bitte wählen</option>{profiles.filter((profile) => !assignmentProfiles.includes(profile.id)).map((profile) => <option key={profile.id} value={profile.id}>{profile.name}</option>)}</select></label><button type="button" onClick={() => void saveAssignment()} disabled={locked || busy}>Zuordnung speichern</button></div>}
    {(dialog?.kind === "create" || dialog?.kind === "duplicate") && <Dialog title={dialog.kind === "create" ? "Neues Pickit-Profil" : "Profil duplizieren"} onClose={closeDialog} initialFocusRef={dialogInputRef}>
      <p>{dialog.kind === "create" ? "Die Profil-ID ist unveränderlich und muss ein Kleinbuchstaben-Slug sein." : `Kopie von ${draft?.name ?? ""} anlegen. Die neue Profil-ID ist unveränderlich.`}</p>
      <label>Profil-ID<input ref={dialogInputRef} value={dialog.id} disabled={busy} onChange={(event) => { setDialogError(""); setDialog({ ...dialog, id: event.target.value }); }} placeholder="mein-profil" /></label>
      <label>Anzeigename<input value={dialog.name} disabled={busy} onChange={(event) => { setDialogError(""); setDialog({ ...dialog, name: event.target.value }); }} placeholder="Mein Profil" /></label>
      {dialogError && <p role="alert">{dialogError}</p>}
      <div className="modal-actions">
        <Button variant="secondary" onClick={closeDialog} disabled={busy}>Abbrechen</Button>
        <Button onClick={() => void (dialog.kind === "create" ? submitCreateProfile() : submitDuplicateProfile())} disabled={busy}>{busy ? "Core prüft …" : dialog.kind === "create" ? "Profil anlegen" : "Kopie anlegen"}</Button>
      </div>
    </Dialog>}
    {dialog?.kind === "delete" && draft && <Dialog title="Profil löschen?" onClose={closeDialog}>
      <p>Profil <strong>{draft.name}</strong> (<code>{draft.id}</code>) wird dauerhaft entfernt. Zugeordnete Profile lehnt der Core ab.</p>
      {dialogError && <p role="alert">{dialogError}</p>}
      <div className="modal-actions">
        <Button variant="secondary" onClick={closeDialog} disabled={busy}>Abbrechen</Button>
        <Button variant="danger" onClick={() => void submitDeleteProfile()} disabled={busy}>{busy ? "Löschen …" : "Endgültig löschen"}</Button>
      </div>
    </Dialog>}
    {dialog?.kind === "discard" && <Dialog title="Ungespeicherte Änderungen verwerfen?" onClose={closeDialog}>
      <p>Der aktuelle Entwurf geht verloren, wenn du das Profil wechselst.</p>
      <div className="modal-actions">
        <Button variant="secondary" onClick={closeDialog}>Abbrechen</Button>
        <Button variant="danger" onClick={confirmDiscard}>Verwerfen</Button>
      </div>
    </Dialog>}
  </section>;
}

function clone<T>(value: T): T { return JSON.parse(JSON.stringify(value)) as T; }
function message(reason: unknown): string { return reason instanceof Error ? reason.message : "Pickit-Vorgang fehlgeschlagen"; }
function uniqueRuleID(existing: Set<string>, suggestion: string): string { let id = suggestion, suffix = 2; while (existing.has(id)) id = `${suggestion}-${suffix++}`; existing.add(id); return id; }
