import { useEffect, useMemo, useRef, useState } from "react";
import { createPickitProfile, deletePickitProfile, duplicatePickitProfile, updatePickitAssignment, updatePickitProfile } from "../../api/client";
import {
  getPickitAssignments, getPickitCatalog, getPickitProfiles, importPickit, validatePickitProfile,
  type PickitAssignmentsDTO, type PickitCatalogDTO, type PickitProfileDTO, type PickitRuleDTO,
} from "../../api/generated";
import { Button, Dialog } from "../../app/ui";
import {
  buildCombinedRuleExpression, equipmentTypeLabel, equipmentTypeOptions, socketOperators,
  type SocketOperator,
} from "./pickitRuleBuilder";
import { useTranslation } from "react-i18next";
import { localeForLanguage, resolveSupportedLanguage } from "../../i18n/types";
import { apiErrorCode, presentApiError, type AppTranslator } from "../../i18n/presenters";
import { gameBaseItemName, gameIdentityName, gameSetName } from "../../i18n/game";

interface Props { characters: string[]; selectedCharacter: string; onSelectedCharacterChange?(character: string): void; runs: string[]; locked: boolean; refreshKey: number }
const pickitSlugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
type PickitDialog =
  | { kind: "create"; id: string; name: string }
  | { kind: "duplicate"; id: string; name: string }
  | { kind: "delete" }
  | { kind: "discard"; nextID: string };

export function PickitFeature({ characters, selectedCharacter: assignmentCharacter, onSelectedCharacterChange, runs, locked, refreshKey }: Props) {
  const { t, i18n } = useTranslation();
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
  const [typePickerOpen, setTypePickerOpen] = useState(false);
  const [typeQuery, setTypeQuery] = useState("");
  const [selectedTypes, setSelectedTypes] = useState<string[]>([]);
  const [socketTier, setSocketTier] = useState<"" | "normal" | "exceptional" | "elite">("");
  const [socketOperator, setSocketOperator] = useState<SocketOperator | "">("");
  const [socketCount, setSocketCount] = useState("");
  const [socketEthereal, setSocketEthereal] = useState(false);
  const [builderErrors, setBuilderErrors] = useState<ReturnType<typeof buildCombinedRuleExpression>["errors"]>({});
  const [advanced, setAdvanced] = useState(false);
  const [importText, setImportText] = useState("");
  const [assignmentRun, setAssignmentRun] = useState(runs[0] ?? "");
  const [assignmentProfiles, setAssignmentProfiles] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [errorCode, setErrorCode] = useState("");
  const [notice, setNotice] = useState("");
  const [dialog, setDialog] = useState<PickitDialog | null>(null);
  const [dialogError, setDialogError] = useState("");
  const dialogInputRef = useRef<HTMLInputElement>(null);
  const typePickerButtonRef = useRef<HTMLButtonElement>(null);

  const dirty = draft ? JSON.stringify(draft) !== saved : false;
  const load = async (signal?: AbortSignal) => {
    const [nextCatalog, nextProfiles, nextAssignments] = await Promise.all([getPickitCatalog(signal), getPickitProfiles(signal), getPickitAssignments(signal)]);
    setCatalog(nextCatalog); setProfiles(nextProfiles.profiles); setAssignments(nextAssignments);
    const nextID = nextProfiles.profiles.some((profile) => profile.id === selectedID) ? selectedID : nextProfiles.profiles[0]?.id ?? "";
    selectProfile(nextProfiles.profiles, nextID); setError(""); setErrorCode("");
  };
  const reportError = (reason: unknown, fallback = t("pickit.operationFailed")) => { setError(message(reason, t, fallback)); setErrorCode(apiErrorCode(reason) ?? ""); };
  useEffect(() => { const controller = new AbortController(); void load(controller.signal).catch((reason: unknown) => !controller.signal.aborted && reportError(reason)); return () => controller.abort(); }, [refreshKey, t]);
  useEffect(() => { if (!assignmentRun && runs[0]) setAssignmentRun(runs[0]); }, [assignmentRun, runs]);
  useEffect(() => {
    if (!assignments || !assignmentCharacter || !assignmentRun) return;
    const values = assignments.assignments as Record<string, Record<string, string[]>>;
    const key = Object.keys(values).find((candidate) => candidate.toLowerCase() === assignmentCharacter.toLowerCase());
    setAssignmentProfiles(key ? [...(values[key]?.[assignmentRun] ?? [])] : []);
  }, [assignments, assignmentCharacter, assignmentRun]);
  useEffect(() => { const protect = (event: BeforeUnloadEvent) => { if (dirty) event.preventDefault(); }; window.addEventListener("beforeunload", protect); return () => window.removeEventListener("beforeunload", protect); }, [dirty]);

  const normalizedQuery = query.trim().toLowerCase();
  const gameLanguage = i18n.resolvedLanguage;
  const matchingBases = useMemo(() => catalog?.bases.filter((entry) => normalizedQuery && `${gameBaseItemName(entry.code, entry.name, gameLanguage)} ${entry.code}`.toLowerCase().includes(normalizedQuery)).slice(0, 8) ?? [], [catalog, gameLanguage, normalizedQuery]);
  const matchingIdentities = useMemo(() => catalog?.identities.filter((entry) => normalizedQuery && `${gameIdentityName(entry.key, entry.display_name, gameLanguage)} ${gameSetName(entry.set_key ?? "", entry.set_name ?? "", gameLanguage)}`.toLowerCase().includes(normalizedQuery)).slice(0, 12) ?? [], [catalog, gameLanguage, normalizedQuery]);
  const matchingSets = useMemo(() => {
    const groups = new Map<string, typeof matchingIdentities>();
    for (const entry of catalog?.identities ?? []) if (entry.kind === "set" && entry.set_key && gameSetName(entry.set_key, entry.set_name ?? "", gameLanguage).toLowerCase().includes(normalizedQuery)) groups.set(entry.set_key, [...(groups.get(entry.set_key) ?? []), entry]);
    return [...groups.entries()].filter(([, entries]) => entries.length > 1).slice(0, 5);
  }, [catalog, gameLanguage, normalizedQuery]);
  const activeLocale = localeForLanguage(resolveSupportedLanguage(i18n.resolvedLanguage));
  const normalizedTypeQuery = typeQuery.trim().toLocaleLowerCase(activeLocale);
  const matchingTypeOptions = useMemo(
    () => equipmentTypeOptions.filter((option) => equipmentTypeLabel(option.id, t).toLocaleLowerCase(activeLocale).includes(normalizedTypeQuery)),
    [activeLocale, normalizedTypeQuery, t],
  );
  const selectedTypeOptions = useMemo(
    () => equipmentTypeOptions.filter((option) => selectedTypes.includes(option.id)),
    [selectedTypes],
  );

  function selectProfile(source: PickitProfileDTO[], id: string) { const profile = source.find((entry) => entry.id === id); setSelectedID(id); setDraft(profile ? clone(profile) : null); setSaved(profile ? JSON.stringify(profile) : ""); setNotice(""); }
  function chooseProfile(id: string) {
    if (dirty) {
      setDialogError("");
      setDialog({ kind: "discard", nextID: id });
      return;
    }
    selectProfile(profiles, id);
  }
  function openCreateDialog() { setDialogError(""); setDialog({ kind: "create", id: t("pickit.defaultProfileId"), name: t("pickit.defaultProfileName") }); }
  function openDuplicateDialog() {
    if (!draft) return;
    setDialogError("");
    setDialog({ kind: "duplicate", id: `${draft.id}-${t("pickit.copyIdSuffix")}`, name: `${draft.name} – ${t("pickit.copySuffix")}` });
  }
  function openDeleteDialog() { if (!draft) return; setDialogError(""); setDialog({ kind: "delete" }); }
  function closeDialog() { if (busy) return; setDialog(null); setDialogError(""); }
  function addRules(rules: Array<Omit<PickitRuleDTO, "id">>) { if (!draft) return; const existing = new Set(draft.rules.map((rule) => rule.id)); const next = rules.map((rule, index) => ({ ...rule, id: uniqueRuleID(existing, `regel-${draft.rules.length + index + 1}`) })); setDraft({ ...draft, rules: [...draft.rules, ...next] }); }
  function addSet(name: string, entries: PickitCatalogDTO["identities"]) { addRules(entries.map((entry) => ({ action: newAction, expression: `[setitem] == ${JSON.stringify(entry.key)}` }))); setNotice(t("pickit.setExpanded", { name, count: entries.length })); }
  function addIdentity(entry: PickitCatalogDTO["identities"][number]) { addRules([{ action: newAction, expression: `[${entry.kind}item] == ${JSON.stringify(entry.key)}` }]); }
  function addBase(code: string) { addRules([{ action: newAction, expression: `[name] == ${JSON.stringify(code)}${ethereal ? " && [flag] == ethereal" : ""}` }]); }
  function toggleEquipmentType(id: string) {
    setSelectedTypes((current) => current.includes(id) ? current.filter((entry) => entry !== id) : [...current, id]);
    setBuilderErrors((current) => ({ ...current, types: undefined }));
  }
  function closeTypePicker() {
    setTypePickerOpen(false);
    typePickerButtonRef.current?.focus();
  }
  function addCombinedRule() {
    const result = buildCombinedRuleExpression({
      types: selectedTypeOptions,
      tier: socketTier,
      socketsOperator: socketOperator,
      sockets: socketCount,
      ethereal: socketEthereal,
    });
    setBuilderErrors(result.errors);
    if (!result.expression) return;
    addRules([{ action: newAction, expression: result.expression }]);
    setNotice(t("pickit.combinedAdded"));
  }
  function updateRule(index: number, replacement: PickitRuleDTO) { if (!draft) return; const rules = [...draft.rules]; rules[index] = replacement; setDraft({ ...draft, rules }); }
  function moveRule(index: number, delta: number) { if (!draft) return; const target = index + delta; if (target < 0 || target >= draft.rules.length) return; const rules = [...draft.rules]; [rules[index], rules[target]] = [rules[target], rules[index]]; setDraft({ ...draft, rules }); }

  async function validateDraft() { if (!draft) return; setBusy(true); setError(""); setErrorCode(""); try { const result = await validatePickitProfile({ profile: draft }); setDraft(result.profile); setNotice(t("pickit.validDraft")); } catch (reason) { reportError(reason); } finally { setBusy(false); } }
  async function saveDraft() { if (!draft) return; setBusy(true); setError(""); setErrorCode(""); try { const result = await updatePickitProfile(draft.id, { expected_revision: draft.revision, profile: draft }); setProfiles((current) => current.map((profile) => profile.id === result.id ? result : profile)); setDraft(clone(result)); setSaved(JSON.stringify(result)); setNotice(t("pickit.profileSaved")); } catch (reason) { reportError(reason); } finally { setBusy(false); } }
  async function submitCreateProfile() {
    if (!dialog || dialog.kind !== "create") return;
    const id = dialog.id.trim();
    const name = dialog.name.trim();
    if (!pickitSlugPattern.test(id)) {
      setDialogError(t("pickit.invalidSlug", { example: t("pickit.defaultProfileId") }));
      return;
    }
    if (!name) {
      setDialogError(t("pickit.emptyName"));
      return;
    }
    setBusy(true); setError(""); setDialogError("");
    try {
      const result = await createPickitProfile({ profile: { schema_version: 1, revision: 1, id, name, rules: [{ id: "regel-1", action: "keep", expression: `[type] == "rune"` }] } });
      const next = [...profiles, result];
      setProfiles(next);
      selectProfile(next, result.id);
      setDialog(null);
      setNotice(t("pickit.profileCreated", { name: result.name }));
    } catch (reason) {
      setDialogError(message(reason, t, t("pickit.operationFailed")));
    } finally {
      setBusy(false);
    }
  }
  async function submitDuplicateProfile() {
    if (!draft || !dialog || dialog.kind !== "duplicate") return;
    const target_id = dialog.id.trim();
    const target_name = dialog.name.trim();
    if (!pickitSlugPattern.test(target_id)) {
      setDialogError(t("pickit.invalidSlug", { example: t("pickit.copyIdExample") }));
      return;
    }
    if (!target_name) {
      setDialogError(t("pickit.emptyName"));
      return;
    }
    setBusy(true); setError(""); setDialogError("");
    try {
      const result = await duplicatePickitProfile(draft.id, { target_id, target_name });
      const next = [...profiles, result];
      setProfiles(next);
      selectProfile(next, result.id);
      setDialog(null);
      setNotice(t("pickit.profileCreated", { name: result.name }));
    } catch (reason) {
      setDialogError(message(reason, t, t("pickit.operationFailed")));
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
      setNotice(t("pickit.profileDeleted"));
    } catch (reason) {
      setDialog(null);
      reportError(reason);
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
  async function pasteImport() { if (!importText.trim()) { setError(t("pickit.importRequired")); setErrorCode(""); return; } setBusy(true); try { const result = await importPickit({ text: importText, action: newAction }); addRules(result.rules.map((rule) => ({ action: rule.action, expression: rule.expression }))); setNotice(result.warnings.join(" ")); } catch (reason) { reportError(reason); } finally { setBusy(false); } }
  async function saveAssignment() { if (!assignments || !assignmentCharacter || !assignmentRun) return; if (!assignmentProfiles.length) { setError(t("pickit.assignmentRequired")); setErrorCode(""); return; } setBusy(true); try { const result = await updatePickitAssignment({ character: assignmentCharacter, run_id: assignmentRun, profile_ids: assignmentProfiles, expected_revision: assignments.revision }); setAssignments(result); setNotice(t("pickit.assignmentSaved")); } catch (reason) { reportError(reason); } finally { setBusy(false); } }

  return <section aria-labelledby="pickit-title">
    <h2 id="pickit-title">{t("pickit.title")}</h2>
    <p>{t("pickit.description")}</p>
    {error && <p role="alert">{error} {["revision_conflict", "state_changed"].includes(errorCode) && <button type="button" className="secondary" onClick={() => void load()}>{t("pickit.reload")}</button>}</p>}
    {notice && <p role="status">{notice}</p>}
    {!catalog && !error && <p>{t("pickit.loading")}</p>}
    {catalog && <div className="pickit-layout">
      <aside className="pickit-library"><div className="section-heading"><h3>{t("pickit.library")}</h3><button type="button" onClick={openCreateDialog} disabled={locked || busy}>{t("pickit.new")}</button></div>
        {profiles.length === 0 ? <p>{t("pickit.noProfiles")}</p> : <ul>{profiles.map((profile) => <li key={profile.id}><button type="button" className={profile.id === selectedID ? "active" : "secondary"} onClick={() => chooseProfile(profile.id)}>{profile.name}<small>{t("pickit.revision", { id: profile.id, revision: profile.revision })}</small></button></li>)}</ul>}
      </aside>
      <div className="pickit-editor">
        {!draft ? <p>{t("pickit.selectOrCreate")}</p> : <>
          <div className="section-heading"><div><h3>{draft.name}</h3><p>{dirty ? t("pickit.unsaved") : t("pickit.savedRevision", { revision: draft.revision })}</p></div><div className="inline-actions"><button type="button" className="secondary" disabled={busy} onClick={openDuplicateDialog}>{t("pickit.duplicate")}</button><button type="button" className="danger" disabled={locked || busy} onClick={openDeleteDialog}>{t("pickit.delete")}</button></div></div>
          <label>{t("pickit.profileName")}<input value={draft.name} disabled={locked} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label>
          <fieldset><legend>{t("pickit.newRule")}</legend><div className="guided-rule"><label>{t("pickit.search")}<input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t("pickit.searchPlaceholder")} /></label><label>{t("pickit.action")}<select value={newAction} onChange={(event) => setNewAction(event.target.value)}><option value="keep">{t("pickit.keep")}</option><option value="sell">{t("pickit.sell")}</option></select></label><label className="check"><input type="checkbox" checked={ethereal} onChange={(event) => setEthereal(event.target.checked)} /> {t("pickit.etherealOnly")}</label></div>
            <div className="quick-rules"><button type="button" className="secondary" onClick={() => addRules([{ action: newAction, expression: `[type] == "rune"` }])}>{t("pickit.allRunes")}</button><button type="button" className="secondary" onClick={() => addRules([{ action: newAction, expression: `[name] == "pk1"` }])}>{t("pickit.terrorKey")}</button><button type="button" className="secondary" onClick={() => addRules([["gzv", "gpv"], ["gly", "gpy"], ["glb", "gpb"], ["glg", "gpg"], ["glr", "gpr"], ["glw", "gpw"], ["skl", "skz"]].map((codes) => ({ action: newAction, expression: `[name] == "${codes[0]}" || [name] == "${codes[1]}"` })))}>{t("pickit.gems")}</button><label>{t("pickit.quality")}<select value={quality} onChange={(event) => setQuality(event.target.value)}>{catalog.qualities.map((entry) => <option key={entry}>{pickitQualityLabel(entry, t)}</option>)}</select></label><button type="button" className="secondary" onClick={() => addRules([{ action: newAction, expression: `[quality] == "${quality}"` }])}>{t("pickit.addQuality")}</button><label>{t("pickit.tier")}<select value={tier} onChange={(event) => setTier(event.target.value)}><option value="normal">{t("pickit.normal")}</option><option value="exceptional">{t("pickit.exceptional")}</option><option value="elite">{t("pickit.elite")}</option></select></label><button type="button" className="secondary" onClick={() => addRules([{ action: newAction, expression: `[tier] == "${tier}"` }])}>{t("pickit.addTier")}</button></div>
            {normalizedQuery && <div className="search-results" aria-label={t("pickit.catalogMatches")}>{matchingSets.map(([setKey, entries]) => { const name = gameSetName(setKey, entries[0]?.set_name ?? "", gameLanguage); return <button type="button" key={setKey} onClick={() => addSet(name, entries)}>{t("pickit.addSet", { name, count: entries.length })}</button>; })}{matchingIdentities.map((entry) => <button type="button" className="secondary" key={`${entry.kind}-${entry.raw_id}`} onClick={() => addIdentity(entry)}>{gameIdentityName(entry.key, entry.display_name, gameLanguage)} <small>{entry.kind}</small></button>)}{matchingBases.map((entry) => <button type="button" className="secondary" key={entry.txt_file_no} onClick={() => addBase(entry.code)}>{gameBaseItemName(entry.code, entry.name, gameLanguage)} <small>{entry.code} · {entry.base_tier}</small></button>)}</div>}
            <fieldset className="combined-rule-builder">
              <legend>{t("pickit.combinedRule")}</legend>
              <div className="combined-rule-grid">
                <div className="type-picker">
                  <span id="equipment-types-label">{t("pickit.itemTypes")}</span>
                  <button
                    ref={typePickerButtonRef}
                    type="button"
                    className="secondary"
                    aria-labelledby="equipment-types-label equipment-types-selection"
                    aria-expanded={typePickerOpen}
                    aria-controls="equipment-type-options"
                    onClick={() => setTypePickerOpen((value) => !value)}
                  >
                    <span id="equipment-types-selection">{selectedTypes.length === 0 ? t("pickit.chooseTypes") : t("pickit.selectedTypes", { count: selectedTypes.length })}</span>
                  </button>
                  {builderErrors.types && <small role="alert">{builderErrors.types}</small>}
                  {typePickerOpen && <div id="equipment-type-options" className="type-picker-panel" onKeyDown={(event) => { if (event.key === "Escape") closeTypePicker(); }}>
                    <label>{t("pickit.searchTypes")}<input value={typeQuery} onChange={(event) => setTypeQuery(event.target.value)} autoFocus /></label>
                    <p role="status">{t("pickit.typeMatches", { matches: matchingTypeOptions.length, selected: selectedTypes.length })}</p>
                    <div className="type-option-list">
                      {matchingTypeOptions.map((option) => <label className="check" key={option.id}><input type="checkbox" checked={selectedTypes.includes(option.id)} onChange={() => toggleEquipmentType(option.id)} /> {equipmentTypeLabel(option.id, t)}</label>)}
                    </div>
                    <button type="button" className="secondary" onClick={closeTypePicker}>{t("pickit.closeSelection")}</button>
                  </div>}
                </div>
                <label>{t("pickit.tier")}<select value={socketTier} onChange={(event) => setSocketTier(event.target.value as typeof socketTier)}><option value="">{t("pickit.any")}</option><option value="normal">{t("pickit.normal")}</option><option value="exceptional">{t("pickit.exceptional")}</option><option value="elite">{t("pickit.elite")}</option></select></label>
                <label>{t("pickit.socketOperator")}<select value={socketOperator} aria-invalid={Boolean(builderErrors.socketsOperator)} onChange={(event) => { setSocketOperator(event.target.value as SocketOperator | ""); setBuilderErrors((current) => ({ ...current, socketsOperator: undefined })); }}><option value="">{t("pickit.choose")}</option>{socketOperators.map((operator) => <option key={operator} value={operator}>{operator}</option>)}</select>{builderErrors.socketsOperator && <small role="alert">{builderErrors.socketsOperator}</small>}</label>
                <label>{t("pickit.socketCount")}<input type="number" min="1" max="6" step="1" value={socketCount} aria-invalid={Boolean(builderErrors.sockets)} onChange={(event) => { setSocketCount(event.target.value); setBuilderErrors((current) => ({ ...current, sockets: undefined })); }} />{builderErrors.sockets && <small role="alert">{builderErrors.sockets}</small>}</label>
                <label className="check"><input type="checkbox" checked={socketEthereal} onChange={(event) => setSocketEthereal(event.target.checked)} /> {t("pickit.ethereal")}</label>
                <button type="button" onClick={addCombinedRule} disabled={locked}>{t("pickit.addCombined")}</button>
              </div>
            </fieldset>
          </fieldset>
          <h4>{t("pickit.ruleOrder")}</h4><ol className="rule-list">{draft.rules.map((rule, index) => <li key={rule.id}><div><strong>{index + 1}. {rule.action === "keep" ? t("pickit.keep") : rule.action === "sell" ? t("pickit.sell") : rule.action}</strong><code>{rule.expression}</code></div><div className="inline-actions"><button type="button" className="secondary" aria-label={t("pickit.moveUp", { number: index + 1 })} disabled={locked || index === 0} onClick={() => moveRule(index, -1)}>↑</button><button type="button" className="secondary" aria-label={t("pickit.moveDown", { number: index + 1 })} disabled={locked || index === draft.rules.length - 1} onClick={() => moveRule(index, 1)}>↓</button><button type="button" className="secondary" disabled={locked} onClick={() => setDraft({ ...draft, rules: draft.rules.filter((_, item) => item !== index) })}>{t("pickit.remove")}</button></div>{advanced && <label>{t("pickit.expression")}<input value={rule.expression} onChange={(event) => updateRule(index, { ...rule, expression: event.target.value })} /></label>}</li>)}</ol>
          <button type="button" className="secondary" onClick={() => setAdvanced((value) => !value)}>{t(advanced ? "pickit.closeAdvanced" : "pickit.openAdvanced")}</button>
          {advanced && <div className="import-box"><label>{t("pickit.nipText")}<textarea value={importText} onChange={(event) => setImportText(event.target.value)} /></label><label>{t("pickit.importFile")}<input type="file" accept=".nip,.txt,text/plain" onChange={(event) => { const file = event.target.files?.[0]; if (file) void file.text().then(setImportText); }} /></label><button type="button" className="secondary" onClick={() => void pasteImport()} disabled={busy}>{t("pickit.importDraft")}</button></div>}
          <div className="editor-actions"><button type="button" className="secondary" onClick={() => void validateDraft()} disabled={busy}>{t("pickit.validateDraft")}</button><button type="button" onClick={() => void saveDraft()} disabled={locked || busy || !dirty || draft.rules.length === 0}>{t(busy ? "pickit.coreChecking" : "pickit.saveProfile")}</button></div>
        </>}
      </div>
    </div>}
    {assignments && <div className="assignment-editor"><h3>{t("pickit.assignment")}</h3><div className="selection-grid"><label>{t("pickit.character")}<select value={assignmentCharacter} onChange={(event) => onSelectedCharacterChange?.(event.target.value)}>{characters.map((character) => <option key={character}>{character}</option>)}</select></label><label>{t("pickit.run")}<select value={assignmentRun} onChange={(event) => setAssignmentRun(event.target.value)}>{runs.map((run) => <option key={run}>{run}</option>)}</select></label></div><p>{t("pickit.assignmentOrder")}</p><ol>{assignmentProfiles.map((id, index) => <li key={id}>{index + 1}. {profiles.find((profile) => profile.id === id)?.name ?? id}<button type="button" className="secondary" onClick={() => setAssignmentProfiles((current) => current.filter((entry) => entry !== id))}>{t("pickit.remove")}</button></li>)}</ol><label>{t("pickit.addProfile")}<select value="" onChange={(event) => { const id = event.target.value; if (id && !assignmentProfiles.includes(id)) setAssignmentProfiles((current) => [...current, id]); }}><option value="">{t("pickit.choose")}</option>{profiles.filter((profile) => !assignmentProfiles.includes(profile.id)).map((profile) => <option key={profile.id} value={profile.id}>{profile.name}</option>)}</select></label><button type="button" onClick={() => void saveAssignment()} disabled={locked || busy}>{t("pickit.saveAssignment")}</button></div>}
    {(dialog?.kind === "create" || dialog?.kind === "duplicate") && <Dialog title={t(dialog.kind === "create" ? "pickit.createTitle" : "pickit.duplicateTitle")} onClose={closeDialog} initialFocusRef={dialogInputRef}>
      <p>{dialog.kind === "create" ? t("pickit.createDetail") : t("pickit.duplicateDetail", { name: draft?.name ?? "" })}</p>
      <label>{t("pickit.profileId")}<input ref={dialogInputRef} value={dialog.id} disabled={busy} onChange={(event) => { setDialogError(""); setDialog({ ...dialog, id: event.target.value }); }} placeholder={t("pickit.defaultProfileId")} /></label>
      <label>{t("pickit.displayName")}<input value={dialog.name} disabled={busy} onChange={(event) => { setDialogError(""); setDialog({ ...dialog, name: event.target.value }); }} placeholder={t("pickit.defaultProfileName")} /></label>
      {dialogError && <p role="alert">{dialogError}</p>}
      <div className="modal-actions">
        <Button variant="secondary" onClick={closeDialog} disabled={busy}>{t("common.cancel")}</Button>
        <Button onClick={() => void (dialog.kind === "create" ? submitCreateProfile() : submitDuplicateProfile())} disabled={busy}>{t(busy ? "pickit.coreChecking" : dialog.kind === "create" ? "pickit.create" : "pickit.createCopy")}</Button>
      </div>
    </Dialog>}
    {dialog?.kind === "delete" && draft && <Dialog title={t("pickit.deleteTitle")} onClose={closeDialog}>
      <p>{t("pickit.deleteDetail", { name: draft.name, id: draft.id })}</p>
      {dialogError && <p role="alert">{dialogError}</p>}
      <div className="modal-actions">
        <Button variant="secondary" onClick={closeDialog} disabled={busy}>{t("common.cancel")}</Button>
        <Button variant="danger" onClick={() => void submitDeleteProfile()} disabled={busy}>{t(busy ? "pickit.deleting" : "pickit.deletePermanently")}</Button>
      </div>
    </Dialog>}
    {dialog?.kind === "discard" && <Dialog title={t("pickit.discardTitle")} onClose={closeDialog}>
      <p>{t("pickit.discardDetail")}</p>
      <div className="modal-actions">
        <Button variant="secondary" onClick={closeDialog}>{t("common.cancel")}</Button>
        <Button variant="danger" onClick={confirmDiscard}>{t("pickit.discard")}</Button>
      </div>
    </Dialog>}
  </section>;
}

function clone<T>(value: T): T { return JSON.parse(JSON.stringify(value)) as T; }
function message(reason: unknown, t: AppTranslator, fallback: string): string { return presentApiError(reason, t, fallback); }
function uniqueRuleID(existing: Set<string>, suggestion: string): string { let id = suggestion, suffix = 2; while (existing.has(id)) id = `${suggestion}-${suffix++}`; existing.add(id); return id; }
function pickitQualityLabel(quality: string, t: AppTranslator): string {
  const keys = { low_quality: "pickit.qualities.lowQuality", normal: "pickit.qualities.normal", superior: "pickit.qualities.superior", magic: "pickit.qualities.magic", set: "pickit.qualities.set", rare: "pickit.qualities.rare", unique: "pickit.qualities.unique", crafted: "pickit.qualities.crafted" } as const;
  const key = keys[quality as keyof typeof keys];
  return key ? t(key) : quality;
}
