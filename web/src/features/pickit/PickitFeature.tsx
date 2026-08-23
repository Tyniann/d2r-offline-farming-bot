import { useEffect, useId, useMemo, useRef, useState } from "react";
import {
  ArrowDown, ArrowUp, Check, ChevronDown, CircleHelp, Copy, GripVertical, Layers3,
  ListFilter, MoreHorizontal, Plus, Search, SlidersHorizontal, Sparkles, Trash2, Undo2, Users, WandSparkles, X,
} from "lucide-react";
import { createPickitProfile, deletePickitProfile, duplicatePickitProfile, updatePickitAssignment, updatePickitProfile } from "../../api/client";
import {
  getPickitAssignments, getPickitCatalog, getPickitProfiles, importPickit, validatePickitProfile,
  type PickitAssignmentsDTO, type PickitCatalogDTO, type PickitProfileDTO, type PickitRuleDTO,
} from "../../api/generated";
import { Button, Dialog } from "../../app/ui";
import {
  buildCombinedRuleExpression, equipmentTypeLabel, equipmentTypeOptions,
  type SocketOperator,
} from "./pickitRuleBuilder";
import { useTranslation } from "react-i18next";
import { localeForLanguage, resolveSupportedLanguage } from "../../i18n/types";
import { apiErrorCode, presentApiError, presentRunName, type AppTranslator } from "../../i18n/presenters";
import { gameBaseItemName, gameIdentityName, gameSetName } from "../../i18n/game";
import "./PickitFeature.css";

export interface PickitFeatureProps { characters: string[]; selectedCharacter: string; onSelectedCharacterChange?(character: string): void; runs: string[]; locked: boolean; refreshKey: number }
const pickitSlugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
type PickitDialog =
  | { kind: "create"; id: string; name: string }
  | { kind: "duplicate"; id: string; name: string }
  | { kind: "delete" }
  | { kind: "discard"; nextID: string };
type PickitView = "profiles" | "assignments";

export function PickitFeature({ selectedCharacter: assignmentCharacter, runs, locked, refreshKey }: PickitFeatureProps) {
  const { t, i18n } = useTranslation();
  const [catalog, setCatalog] = useState<PickitCatalogDTO | null>(null);
  const [profiles, setProfiles] = useState<PickitProfileDTO[]>([]);
  const [assignments, setAssignments] = useState<PickitAssignmentsDTO | null>(null);
  const [selectedID, setSelectedID] = useState("");
  const [draft, setDraft] = useState<PickitProfileDTO | null>(null);
  const [saved, setSaved] = useState("");
  const [query, setQuery] = useState("");
  const newAction = "keep";
  const [typePickerOpen, setTypePickerOpen] = useState(false);
  const [typeQuery, setTypeQuery] = useState("");
  const [selectedTypes, setSelectedTypes] = useState<string[]>(["shields"]);
  const [socketTier, setSocketTier] = useState<"" | "normal" | "exceptional" | "elite">("elite");
  const [socketOperator, setSocketOperator] = useState<SocketOperator | "">("==");
  const [socketCount, setSocketCount] = useState("4");
  const [socketEthereal, setSocketEthereal] = useState(false);
  const [builderErrors, setBuilderErrors] = useState<ReturnType<typeof buildCombinedRuleExpression>["errors"]>({});
  const [advanced, setAdvanced] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [importText, setImportText] = useState("");
  const [assignmentRun, setAssignmentRun] = useState(runs[0] ?? "");
  const [assignmentProfiles, setAssignmentProfiles] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [errorCode, setErrorCode] = useState("");
  const [notice, setNotice] = useState("");
  const [dialog, setDialog] = useState<PickitDialog | null>(null);
  const [dialogError, setDialogError] = useState("");
  const [view, setView] = useState<PickitView>("profiles");
  const [builderOpen, setBuilderOpen] = useState(false);
  const [usageOpen, setUsageOpen] = useState(false);
  const [isNew, setIsNew] = useState(false);
  const [removedRule, setRemovedRule] = useState<{ rule: PickitRuleDTO; index: number } | null>(null);
  const [draggedRule, setDraggedRule] = useState<number | null>(null);
  const [draggedAssignment, setDraggedAssignment] = useState<number | null>(null);
  const dialogInputRef = useRef<HTMLInputElement>(null);
  const typePickerButtonRef = useRef<HTMLButtonElement>(null);
  const builderToggleRef = useRef<HTMLButtonElement>(null);
  const usageButtonRef = useRef<HTMLButtonElement>(null);
  const profilesTabID = useId();
  const assignmentsTabID = useId();
  const profilesPanelID = useId();
  const assignmentsPanelID = useId();
  const builderID = useId();
  const usagePopoverID = useId();
  const advancedPanelID = useId();
  const importPanelID = useId();
  const assignmentKeyboardHintID = useId();

  const dirty = draft ? JSON.stringify(draft) !== saved : false;
  const load = async (signal?: AbortSignal) => {
    const [nextCatalog, nextProfiles, nextAssignments] = await Promise.all([getPickitCatalog(signal), getPickitProfiles(signal), getPickitAssignments(signal)]);
    setCatalog(nextCatalog); setProfiles(nextProfiles.profiles); setAssignments(nextAssignments);
    const nextID = nextProfiles.profiles.some((profile) => profile.id === selectedID) ? selectedID : nextProfiles.profiles[0]?.id ?? "";
    selectProfile(nextProfiles.profiles, nextID); setError(""); setErrorCode("");
  };
  const reportError = (reason: unknown, fallback = t("pickit.operationFailed")) => { setError(message(reason, t, fallback)); setErrorCode(apiErrorCode(reason) ?? ""); };
  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal).catch((reason: unknown) => !controller.signal.aborted && reportError(reason));
    return () => controller.abort();
  }, [refreshKey]);
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

  function selectProfile(source: PickitProfileDTO[], id: string) { const profile = source.find((entry) => entry.id === id); setSelectedID(id); setDraft(profile ? clone(profile) : null); setSaved(profile ? JSON.stringify(profile) : ""); setIsNew(false); setRemovedRule(null); setNotice(""); }
  function chooseProfile(id: string) {
    if (dirty) {
      setDialogError("");
      setDialog({ kind: "discard", nextID: id });
      return;
    }
    selectProfile(profiles, id);
  }
  function openCreateDialog() { setDialogError(""); setDialog({ kind: "create", id: "", name: t("pickit.defaultProfileName") }); }
  function openDuplicateDialog() {
    if (!draft) return;
    setDialogError("");
    setDialog({ kind: "duplicate", id: `${draft.id}-${t("pickit.copyIdSuffix")}`, name: `${draft.name} – ${t("pickit.copySuffix")}` });
  }
  function requestDeleteProfile() {
    if (!draft) return;
    if (profileUsages(assignments, draft.id).length > 0) {
      setDialogError("");
      setDialog({ kind: "delete" });
      return;
    }
    void submitDeleteProfile();
  }
  function closeDialog() { if (busy) return; setDialog(null); setDialogError(""); }
  function addRules(rules: Array<Omit<PickitRuleDTO, "id">>) { if (!draft) return; const existing = new Set(draft.rules.map((rule) => rule.id)); const next = rules.map((rule, index) => ({ ...rule, id: uniqueRuleID(existing, `regel-${draft.rules.length + index + 1}`) })); setDraft({ ...draft, rules: [...draft.rules, ...next] }); setRemovedRule(null); setNotice(t("pickit.workspace.notices.ruleAdded")); }
  function addSet(name: string, entries: PickitCatalogDTO["identities"]) { addRules(entries.map((entry) => ({ action: newAction, expression: `[setitem] == ${JSON.stringify(entry.key)}`, summary: { kind: "set_item", params: { set_key: entry.key } } }))); setNotice(t("pickit.setExpanded", { name, count: entries.length })); }
  function addIdentity(entry: PickitCatalogDTO["identities"][number]) { addRules([{ action: newAction, expression: `[${entry.kind}item] == ${JSON.stringify(entry.key)}`, summary: { kind: entry.kind === "set" ? "set_item" : "unique_item", params: entry.kind === "set" ? { set_key: entry.key } : { unique_key: entry.key } } }]); }
  function addBase(code: string) { addRules([{ action: newAction, expression: `[name] == ${JSON.stringify(code)}`, summary: { kind: "item_codes", params: { codes: [code] } } }]); }
  function toggleEquipmentType(id: string) {
    setSelectedTypes((current) => current.includes(id) ? current.filter((entry) => entry !== id) : [...current, id]);
    setBuilderErrors((current) => ({ ...current, types: undefined }));
  }
  function closeTypePicker() {
    setTypePickerOpen(false);
    typePickerButtonRef.current?.focus();
  }
  function closeBuilder() {
    setBuilderOpen(false);
    requestAnimationFrame(() => builderToggleRef.current?.focus());
  }
  function selectTabFromKeyboard(event: React.KeyboardEvent<HTMLButtonElement>, values: readonly string[], current: string, select: (value: string) => void) {
    const currentIndex = values.indexOf(current);
    let nextIndex = currentIndex;
    if (event.key === "ArrowRight") nextIndex = (currentIndex + 1) % values.length;
    else if (event.key === "ArrowLeft") nextIndex = (currentIndex - 1 + values.length) % values.length;
    else if (event.key === "Home") nextIndex = 0;
    else if (event.key === "End") nextIndex = values.length - 1;
    else return;
    event.preventDefault();
    select(values[nextIndex]);
    const tabs = event.currentTarget.parentElement?.querySelectorAll<HTMLButtonElement>('[role="tab"]');
    tabs?.[nextIndex]?.focus();
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
    addRules([{ action: newAction, expression: result.expression, summary: { kind: "socket_filter", params: { types: [...new Set(selectedTypeOptions.flatMap((option) => option.codes))], tiers: socketTier ? [socketTier] : undefined, socket_operator: socketOperator, socket_count: Number(socketCount), ethereal: socketEthereal || undefined } } }]);
    setNotice(t("pickit.combinedAdded"));
  }
  function updateRule(index: number, replacement: PickitRuleDTO) { if (!draft) return; const rules = [...draft.rules]; rules[index] = replacement; setDraft({ ...draft, rules }); }
  function moveRule(index: number, delta: number) { if (!draft) return; const target = index + delta; if (target < 0 || target >= draft.rules.length) return; const rules = [...draft.rules]; [rules[index], rules[target]] = [rules[target], rules[index]]; setDraft({ ...draft, rules }); }
  function moveRuleTo(from: number, to: number) { if (!draft || from === to || from < 0 || to < 0 || from >= draft.rules.length || to >= draft.rules.length) return; const rules = [...draft.rules]; const [rule] = rules.splice(from, 1); rules.splice(to, 0, rule); setDraft({ ...draft, rules }); }
  function removeRule(index: number) { if (!draft) return; const rule = draft.rules[index]; setDraft({ ...draft, rules: draft.rules.filter((_, item) => item !== index) }); setRemovedRule({ rule, index }); setNotice(t("pickit.workspace.notices.ruleRemoved")); }
  function undoRemoveRule() { if (!draft || !removedRule) return; const rules = [...draft.rules]; rules.splice(Math.min(removedRule.index, rules.length), 0, removedRule.rule); setDraft({ ...draft, rules }); setRemovedRule(null); setNotice(t("pickit.workspace.notices.ruleRestored")); }
  function reorderAssignment(from: number, to: number) { if (from === to || from < 0 || to < 0 || from >= assignmentProfiles.length || to >= assignmentProfiles.length) return; const next = [...assignmentProfiles]; const [profile] = next.splice(from, 1); next.splice(to, 0, profile); setAssignmentProfiles(next); }

  async function validateDraft() { if (!draft) return; setBusy(true); setError(""); setErrorCode(""); try { const result = await validatePickitProfile({ profile: draft }); setDraft(result.profile); setNotice(t("pickit.validDraft")); } catch (reason) { reportError(reason); } finally { setBusy(false); } }
  async function saveDraft() { if (!draft) return; setBusy(true); setError(""); setErrorCode(""); try { const result = isNew ? await createPickitProfile({ profile: draft }) : await updatePickitProfile(draft.id, { expected_revision: draft.revision, profile: draft }); setProfiles((current) => isNew ? [...current, result] : current.map((profile) => profile.id === result.id ? result : profile)); setDraft(clone(result)); setSaved(JSON.stringify(result)); setIsNew(false); setNotice(t("pickit.workspace.notices.saved")); } catch (reason) { reportError(reason); } finally { setBusy(false); } }
  async function submitCreateProfile() {
    if (!dialog || dialog.kind !== "create") return;
    const name = dialog.name.trim();
    if (!name) {
      setDialogError(t("pickit.emptyName"));
      return;
    }
    const id = uniqueProfileID(profiles, name);
    const profile: PickitProfileDTO = { schema_version: 1, revision: 1, id, name, rules: [] };
    setSelectedID(id); setDraft(profile); setSaved(""); setIsNew(true); setRemovedRule(null); setDialog(null); setBuilderOpen(true); setNotice(t("pickit.workspace.notices.created"));
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
    if (isNew) {
      const nextID = profiles[0]?.id ?? "";
      selectProfile(profiles, nextID);
      setDialog(null);
      return;
    }
    setBusy(true); setError(""); setDialogError("");
    try {
      const usages = profileUsages(assignments, draft.id);
      const result = await deletePickitProfile(draft.id, { expected_revision: draft.revision, expected_assignment_revision: assignments?.revision ?? 0, remove_assignments: usages.length > 0 });
      const next = profiles.filter((profile) => profile.id !== draft.id);
      setAssignments(result.assignments);
      setProfiles(next);
      selectProfile(next, next[0]?.id ?? "");
      setDialog(null);
      setNotice(t(usages.length > 0 ? "pickit.workspace.notices.deletedWithAssignments" : "pickit.profileDeleted"));
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
  async function pasteImport() { if (!importText.trim()) { setError(t("pickit.importRequired")); setErrorCode(""); return; } setBusy(true); try { const result = await importPickit({ text: importText, action: newAction }); addRules(result.rules.map((rule) => ({ action: rule.action, expression: rule.expression, summary: rule.summary }))); setNotice(result.warnings.join(" ")); } catch (reason) { reportError(reason); } finally { setBusy(false); } }
  async function saveAssignment() { if (!assignments || !assignmentCharacter || !assignmentRun) return; if (!assignmentProfiles.length) { setError(t("pickit.assignmentRequired")); setErrorCode(""); return; } setBusy(true); try { const result = await updatePickitAssignment({ character: assignmentCharacter, run_id: assignmentRun, profile_ids: assignmentProfiles, expected_revision: assignments.revision }); setAssignments(result); setNotice(t("pickit.assignmentSaved")); } catch (reason) { reportError(reason); } finally { setBusy(false); } }

  const visibleProfiles = isNew && draft ? [...profiles, draft] : profiles;
  const usages = draft ? profileUsages(assignments, draft.id) : [];
  const availableAssignmentProfiles = profiles.filter((profile) => !assignmentProfiles.includes(profile.id));

  return <div className="pickit-feature">
    <header className="pickit-feature-header"><div><h1>{t("navigation.pickit")}</h1><p>{t("pickit.workspace.description")}</p></div><div className="pickit-view-switch" role="tablist" aria-label={t("pickit.workspace.viewLabel")}><button id={profilesTabID} role="tab" aria-selected={view === "profiles"} aria-controls={profilesPanelID} tabIndex={view === "profiles" ? 0 : -1} onKeyDown={(event) => selectTabFromKeyboard(event, ["profiles", "assignments"], view, (value) => setView(value as PickitView))} onClick={() => setView("profiles")}><Layers3 size={16} />{t("pickit.workspace.profilesTab")}</button><button id={assignmentsTabID} role="tab" aria-selected={view === "assignments"} aria-controls={assignmentsPanelID} tabIndex={view === "assignments" ? 0 : -1} onKeyDown={(event) => selectTabFromKeyboard(event, ["profiles", "assignments"], view, (value) => setView(value as PickitView))} onClick={() => setView("assignments")}><Users size={16} />{t("pickit.workspace.assignmentsTab")}</button></div></header>
    {error && <div className="pickit-feature-toast is-error" role="alert">{error}{["revision_conflict", "state_changed"].includes(errorCode) && <button type="button" className="secondary" onClick={() => void load()}>{t("pickit.reload")}</button>}</div>}
    {notice && <div className="pickit-feature-toast" role="status"><Check size={17} />{notice}{removedRule && <button type="button" onClick={undoRemoveRule}><Undo2 size={15} />{t("pickit.workspace.undo")}</button>}</div>}
    {!catalog && !error && <div className="pickit-empty-rules" role="status">{t("pickit.loading")}</div>}

    {catalog && view === "profiles" && <div id={profilesPanelID} className="pickit-focus-workbench" role="tabpanel" aria-labelledby={profilesTabID}>
      <aside className="pickit-profile-rail"><div className="pickit-rail-heading"><div><span>{t("pickit.workspace.library")}</span><strong>{t("pickit.workspace.profileCount", { count: visibleProfiles.length })}</strong></div><button type="button" aria-label={t("pickit.workspace.createProfile")} onClick={openCreateDialog} disabled={locked || busy}><Plus size={17} /></button></div><div className="pickit-profile-list">{visibleProfiles.map((profile) => { const count = profileUsages(assignments, profile.id).length; return <button type="button" key={profile.id} aria-pressed={profile.id === selectedID} className={profile.id === selectedID ? "is-selected" : ""} onClick={() => { if (profile.id !== selectedID) chooseProfile(profile.id); }}><strong>{profile.name}</strong><span>{t("pickit.workspace.ruleCount", { count: profile.rules.length })}<small>{t("pickit.workspace.usageCount", { count })}</small></span></button>; })}</div></aside>
      <article className="pickit-focus-document">{!draft ? <div className="pickit-empty-rules"><Sparkles size={22} /><strong>{t("pickit.selectOrCreate")}</strong></div> : <>
        <header className="pickit-editor-header"><div className="pickit-editor-title"><label><span>{t("pickit.profileName")}</span><input value={draft.name} disabled={locked} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label><div className="pickit-usage-shell" onKeyDown={(event) => { if (event.key === "Escape" && usageOpen) { event.preventDefault(); setUsageOpen(false); usageButtonRef.current?.focus(); } }}><button ref={usageButtonRef} type="button" className="pickit-usage-button" aria-expanded={usageOpen} aria-controls={usagePopoverID} onClick={() => setUsageOpen((value) => !value)}><Users size={15} />{t("pickit.workspace.usageCount", { count: usages.length })}<ChevronDown size={14} /></button>{usageOpen && <div id={usagePopoverID} className="pickit-usage-popover" role="region" aria-label={t("pickit.workspace.usedBy")}><strong>{t("pickit.workspace.usedBy")}</strong>{usages.length ? <ul>{usages.map((usage) => <li key={`${usage.character}-${usage.route}`}>{usage.character} · {presentRunName(usage.route, t)}</li>)}</ul> : <p>{t("pickit.workspace.notUsed")}</p>}</div>}</div></div><div className="pickit-profile-actions"><button type="button" className="secondary" onClick={openDuplicateDialog} disabled={isNew || busy}><Copy size={15} />{t("pickit.duplicate")}</button><button type="button" className="secondary pickit-more-button" aria-label={t("pickit.workspace.moreActions")}><MoreHorizontal size={18} /></button><button type="button" className="secondary pickit-delete-button" onClick={requestDeleteProfile} disabled={locked || busy}><Trash2 size={15} />{t("pickit.delete")}</button></div></header>
        <div className="pickit-focus-add"><div><WandSparkles size={20} /><span><strong>{t("pickit.workspace.addRuleTitle")}</strong><small>{t("pickit.workspace.addRuleHint")}</small></span></div><button ref={builderToggleRef} type="button" aria-expanded={builderOpen} aria-controls={builderID} onClick={() => setBuilderOpen((value) => !value)}>{builderOpen ? t("pickit.workspace.closeBuilder") : t("pickit.workspace.openBuilder")}<ChevronDown size={16} /></button></div>
        {builderOpen && <div id={builderID} className="pickit-focus-builder"><div className="pickit-builder"><div className="pickit-builder-heading"><div><p className="eyebrow">{t("pickit.workspace.builderEyebrow")}</p><h2>{t("pickit.workspace.builderTitle")}</h2></div><button type="button" className="secondary pickit-builder-close" aria-label={t("pickit.workspace.closeBuilder")} onClick={closeBuilder}><X size={16} /></button></div>
          <div className="pickit-builder-discovery"><div className="pickit-builder-search"><span>{t("pickit.workspace.directSearch")}</span><label className="pickit-item-search"><Search size={17} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t("pickit.searchPlaceholder")} /></label></div><div className="pickit-quick-add"><span>{t("pickit.workspace.quickAdd")}</span><button type="button" onClick={() => addRules([{ action: newAction, expression: `[type] == "rune"`, summary: { kind: "runes", params: {} } }])}>{t("pickit.allRunes")}<Plus size={14} /></button><button type="button" onClick={() => addRules([["pk1"], ["pk2"], ["pk3"]].map(([code]) => ({ action: newAction, expression: `[name] == "${code}"`, summary: { kind: "item_codes" as const, params: { codes: [code] } } })))}>{t("pickit.workspace.allKeys")}<Plus size={14} /></button><button type="button" onClick={() => addRules([["gzv", "gpv"], ["gly", "gpy"], ["glb", "gpb"], ["glg", "gpg"], ["glr", "gpr"], ["glw", "gpw"], ["skl", "skz"]].map((codes) => ({ action: newAction, expression: `[name] == "${codes[0]}" || [name] == "${codes[1]}"`, summary: { kind: "item_codes" as const, params: { codes } } })))}>{t("pickit.workspace.gemsShort")}<Plus size={14} /></button></div></div>
          {normalizedQuery && <div className="search-results" aria-label={t("pickit.catalogMatches")}>{matchingSets.map(([setKey, entries]) => { const name = gameSetName(setKey, entries[0]?.set_name ?? "", gameLanguage); return <button type="button" key={setKey} onClick={() => addSet(name, entries)}>{t("pickit.addSet", { name, count: entries.length })}</button>; })}{matchingIdentities.map((entry) => <button type="button" className="secondary" key={`${entry.kind}-${entry.raw_id}`} onClick={() => addIdentity(entry)}>{gameIdentityName(entry.key, entry.display_name, gameLanguage)}</button>)}{matchingBases.map((entry) => <button type="button" className="secondary" key={entry.txt_file_no} onClick={() => addBase(entry.code)}>{gameBaseItemName(entry.code, entry.name, gameLanguage)}</button>)}</div>}
          <details open><summary><SlidersHorizontal size={16} />{t("pickit.workspace.combineFilters")}</summary><div className="pickit-filter-composer"><div className="pickit-filter-grid">
            <div className="pickit-type-picker pickit-filter-field"><span id="equipment-types-label">{t("pickit.itemTypes")}</span><button ref={typePickerButtonRef} type="button" className="secondary pickit-type-picker-button" aria-labelledby="equipment-types-label equipment-types-selection" aria-expanded={typePickerOpen} aria-controls="equipment-type-options" onClick={() => setTypePickerOpen((value) => !value)}><span id="equipment-types-selection">{selectedTypes.length === 0 ? t("pickit.chooseTypes") : t("pickit.selectedTypes", { count: selectedTypes.length })}</span><ChevronDown size={15} /></button>{builderErrors.types && <small role="alert">{builderErrors.types}</small>}{typePickerOpen && <div id="equipment-type-options" className="pickit-type-picker-panel" onKeyDown={(event) => { if (event.key === "Escape") closeTypePicker(); }}><label><span>{t("pickit.searchTypes")}</span><span className="pickit-type-search"><Search size={15} /><input value={typeQuery} onChange={(event) => setTypeQuery(event.target.value)} autoFocus /></span></label><p role="status">{t("pickit.typeMatches", { matches: matchingTypeOptions.length, selected: selectedTypes.length })}</p><div className="pickit-type-options">{matchingTypeOptions.map((option) => <label className="check" key={option.id}><input type="checkbox" checked={selectedTypes.includes(option.id)} onChange={() => toggleEquipmentType(option.id)} />{equipmentTypeLabel(option.id, t)}</label>)}</div><button type="button" className="secondary" onClick={closeTypePicker}>{t("pickit.closeSelection")}</button></div>}</div>
            <label className="pickit-filter-field"><span>{t("pickit.tier")}</span><select value={socketTier} onChange={(event) => setSocketTier(event.target.value as typeof socketTier)}><option value="">{t("pickit.any")}</option><option value="normal">{t("pickit.normal")}</option><option value="exceptional">{t("pickit.exceptional")}</option><option value="elite">{t("pickit.elite")}</option></select></label>
            <label className="pickit-filter-field"><span>{t("pickit.workspace.sockets")}</span><select value={socketOperator && socketCount ? `${socketOperator}:${socketCount}` : ""} aria-invalid={Boolean(builderErrors.socketsOperator || builderErrors.sockets)} onChange={(event) => { const [operator = "", count = ""] = event.target.value.split(":"); setSocketOperator(operator as SocketOperator | ""); setSocketCount(count); setBuilderErrors((current) => ({ ...current, socketsOperator: undefined, sockets: undefined })); }}><option value="">{t("pickit.workspace.anySockets")}</option>{Array.from({ length: 6 }, (_, index) => index + 1).map((count) => <option key={count} value={`==:${count}`}>{t("pickit.workspace.exactSockets", { count })}</option>)}</select>{(builderErrors.socketsOperator || builderErrors.sockets) && <small role="alert">{builderErrors.socketsOperator ?? builderErrors.sockets}</small>}</label>
            <label className="pickit-filter-toggle"><input type="checkbox" checked={socketEthereal} onChange={(event) => setSocketEthereal(event.target.checked)} /><span className="pickit-toggle-track" aria-hidden="true"><i /></span><span>{t("pickit.ethereal")}</span></label>
          </div><div className="pickit-selected-types"><span>{t("pickit.workspace.selectedTypeStrip")}</span><div>{selectedTypeOptions.length ? selectedTypeOptions.map((option) => { const label = equipmentTypeLabel(option.id, t); return <button type="button" className="secondary" key={option.id} aria-label={t("pickit.workspace.removeType", { name: label })} onClick={() => toggleEquipmentType(option.id)}>{label}<X size={13} /></button>; }) : <small>{t("pickit.workspace.noTypesSelected")}</small>}</div><button type="button" className="secondary" disabled={selectedTypes.length === 0} onClick={() => setSelectedTypes([])}>{t("pickit.workspace.clearTypeSelection")}</button></div></div><button type="button" className="pickit-builder-add" onClick={addCombinedRule} disabled={locked}><Plus size={16} />{t("pickit.workspace.addCombinedRule")}</button></details><p className="pickit-builder-help"><CircleHelp size={15} />{t("pickit.workspace.firstMatchHelp")}</p>
        </div></div>}
        <section className="pickit-rules-section"><div className="pickit-section-heading"><div><p className="eyebrow">{t("pickit.workspace.rulesEyebrow")}</p><h2>{t("pickit.workspace.rulesTitle")}</h2><p>{t("pickit.workspace.rulesHint")}</p></div></div><ol className="pickit-rule-list">{draft.rules.map((rule, index) => { const summary = presentRuleSummary(rule, catalog, gameLanguage, t); return <li key={rule.id} draggable onDragStart={() => setDraggedRule(index)} onDragOver={(event) => event.preventDefault()} onDrop={() => { if (draggedRule !== null) moveRuleTo(draggedRule, index); setDraggedRule(null); }}><GripVertical className="pickit-drag-handle" size={18} /><span className={`pickit-rule-number action-${rule.action}`}>{index + 1}</span><div className="pickit-rule-copy"><strong>{summary.title}</strong><span>{summary.detail}</span></div><span className={`pickit-action-badge action-${rule.action}`}>{t(rule.action === "keep" ? "pickit.keep" : "pickit.sell")}</span><div className="pickit-rule-actions"><button type="button" className="secondary" aria-label={t("pickit.moveUp", { number: index + 1 })} disabled={locked || index === 0} onClick={() => moveRule(index, -1)}><ArrowUp size={15} /></button><button type="button" className="secondary" aria-label={t("pickit.moveDown", { number: index + 1 })} disabled={locked || index === draft.rules.length - 1} onClick={() => moveRule(index, 1)}><ArrowDown size={15} /></button><button type="button" className="secondary" aria-label={t("pickit.workspace.removeRule", { name: summary.title })} disabled={locked} onClick={() => removeRule(index)}><X size={16} /></button></div></li>; })}</ol>{draft.rules.length === 0 && <div className="pickit-empty-rules"><Sparkles size={22} /><strong>{t("pickit.workspace.emptyRulesTitle")}</strong><p>{t("pickit.workspace.emptyRulesDetail")}</p><button type="button" onClick={() => setBuilderOpen(true)}><Plus size={16} />{t("pickit.workspace.addFirstRule")}</button></div>}</section>
        <div className="pickit-advanced"><button type="button" className="secondary" aria-expanded={advanced} aria-controls={advancedPanelID} onClick={() => setAdvanced((value) => !value)}><ListFilter size={16} />{t("pickit.workspace.advanced")}<ChevronDown size={15} /></button>{advanced && <div id={advancedPanelID}><p>{t("pickit.workspace.advancedDetail")}</p><button type="button" className="secondary" aria-expanded={importOpen} aria-controls={importPanelID} onClick={() => setImportOpen((value) => !value)}>{t("pickit.workspace.importRules")}</button>{importOpen && <div id={importPanelID} className="import-box"><label>{t("pickit.nipText")}<textarea value={importText} onChange={(event) => setImportText(event.target.value)} /></label><label>{t("pickit.importFile")}<input type="file" accept=".nip,.txt,text/plain" onChange={(event) => { const file = event.target.files?.[0]; if (file) void file.text().then(setImportText); }} /></label><button type="button" className="secondary" onClick={() => void pasteImport()} disabled={busy}>{t("pickit.importDraft")}</button></div>}<details><summary>{t("pickit.workspace.editExpressions")}</summary><p>{t("pickit.workspace.expressionEditorHint")}</p><ol className="pickit-expression-editors">{draft.rules.map((rule, index) => { const summary = presentRuleSummary(rule, catalog, gameLanguage, t); return <li key={rule.id}><label><span>{index + 1}. {summary.title}</span><textarea rows={3} spellCheck={false} aria-label={t("pickit.workspace.expressionLabel", { number: index + 1, name: summary.title })} value={rule.expression} onChange={(event) => updateRule(index, { ...rule, expression: event.target.value, summary: undefined })} /></label></li>; })}</ol><button type="button" className="secondary" onClick={() => void validateDraft()} disabled={busy}>{t("pickit.validateDraft")}</button></details></div>}</div>
        <footer className={`pickit-save-bar ${dirty ? "is-dirty" : ""}`}><div><i /><span><strong>{t(dirty ? "pickit.workspace.unsavedTitle" : "pickit.workspace.savedTitle")}</strong><small>{t(dirty ? "pickit.workspace.unsavedDetail" : "pickit.workspace.savedDetail")}</small></span></div><div><button type="button" className="secondary" disabled={!dirty || busy} onClick={() => isNew ? selectProfile(profiles, profiles[0]?.id ?? "") : selectProfile(profiles, draft.id)}>{t("pickit.workspace.discard")}</button><button type="button" disabled={locked || busy || !dirty || draft.rules.length === 0} onClick={() => void saveDraft()}>{t(busy ? "pickit.coreChecking" : "pickit.saveProfile")}</button></div></footer>
      </>}</article>
    </div>}

    {catalog && assignments && view === "assignments" && <section id={assignmentsPanelID} className="pickit-assignment-workspace" role="tabpanel" aria-labelledby={assignmentsTabID}><header><div><p className="eyebrow">{t("pickit.workspace.assignmentEyebrow")}</p><h2>{t("pickit.workspace.assignmentTitle", { character: assignmentCharacter })}</h2><p>{t("pickit.workspace.assignmentDescription")}</p></div><span><Users size={16} />{assignmentCharacter}</span></header><div className="pickit-run-tabs" role="tablist" aria-label={t("pickit.workspace.chooseRun")}>{runs.map((route, index) => <button id={`${assignmentsPanelID}-route-${index}`} type="button" key={route} role="tab" aria-selected={assignmentRun === route} aria-controls={`${assignmentsPanelID}-route-panel`} tabIndex={assignmentRun === route ? 0 : -1} onKeyDown={(event) => selectTabFromKeyboard(event, runs, assignmentRun, setAssignmentRun)} onClick={() => setAssignmentRun(route)}>{presentRunName(route, t)}</button>)}</div><div id={`${assignmentsPanelID}-route-panel`} className="pickit-assignment-body" role="tabpanel" aria-labelledby={`${assignmentsPanelID}-route-${Math.max(0, runs.indexOf(assignmentRun))}`}><div><div className="pickit-section-heading"><div><p className="eyebrow">{presentRunName(assignmentRun, t)}</p><h3>{t("pickit.workspace.assignmentOrderTitle")}</h3><p>{t("pickit.workspace.assignmentOrderDetail")}</p></div></div><p id={assignmentKeyboardHintID} className="visually-hidden">{t("pickit.workspace.assignmentKeyboardHint")}</p><ol className="pickit-assignment-chain">{assignmentProfiles.map((id, index) => { const profile = profiles.find((entry) => entry.id === id); const name = profile?.name ?? id; return <li key={id} draggable tabIndex={0} aria-label={t("pickit.workspace.assignmentPosition", { name, position: index + 1, count: assignmentProfiles.length })} aria-describedby={assignmentKeyboardHintID} onKeyDown={(event) => { if (!event.altKey || (event.key !== "ArrowUp" && event.key !== "ArrowDown")) return; event.preventDefault(); reorderAssignment(index, index + (event.key === "ArrowUp" ? -1 : 1)); }} onDragStart={() => setDraggedAssignment(index)} onDragOver={(event) => event.preventDefault()} onDrop={() => { if (draggedAssignment !== null) reorderAssignment(draggedAssignment, index); setDraggedAssignment(null); }}><GripVertical size={18} /><span>{index + 1}</span><div><strong>{name}</strong><small>{t("pickit.workspace.ruleCount", { count: profile?.rules.length ?? 0 })}</small></div><button type="button" className="secondary" aria-label={t("pickit.workspace.removeAssignment", { name })} onClick={() => setAssignmentProfiles((current) => current.filter((entry) => entry !== id))}><X size={16} /></button></li>; })}</ol>{assignmentProfiles.length === 0 && <div className="pickit-empty-rules"><strong>{t("pickit.assignmentRequired")}</strong></div>}</div><aside><h3>{t("pickit.workspace.addProfile")}</h3><p>{t("pickit.workspace.addProfileDetail")}</p><div>{availableAssignmentProfiles.map((profile) => <button type="button" className="secondary" key={profile.id} onClick={() => setAssignmentProfiles((current) => [...current, profile.id])}><Plus size={15} /><span><strong>{profile.name}</strong><small>{t("pickit.workspace.ruleCount", { count: profile.rules.length })}</small></span></button>)}</div></aside></div><footer><span>{t("pickit.workspace.assignmentFooter", { run: presentRunName(assignmentRun, t) })}</span><button type="button" onClick={() => void saveAssignment()} disabled={locked || busy || assignmentProfiles.length === 0}>{t("pickit.saveAssignment")}</button></footer></section>}
    {(dialog?.kind === "create" || dialog?.kind === "duplicate") && <Dialog title={t(dialog.kind === "create" ? "pickit.workspace.createTitle" : "pickit.workspace.duplicateTitle")} onClose={closeDialog} initialFocusRef={dialogInputRef}>
      <p>{t(dialog.kind === "create" ? "pickit.workspace.createDetail" : "pickit.workspace.duplicateDetail", { name: draft?.name ?? "" })}</p>
      <label>{t("pickit.profileName")}<input ref={dialogInputRef} value={dialog.name} disabled={busy} onChange={(event) => { setDialogError(""); setDialog({ ...dialog, name: event.target.value, id: dialog.kind === "duplicate" ? uniqueProfileID(profiles, event.target.value) : "" }); }} placeholder={t("pickit.defaultProfileName")} /></label>
      {dialogError && <p role="alert">{dialogError}</p>}
      <div className="modal-actions">
        <Button variant="secondary" onClick={closeDialog} disabled={busy}>{t("common.cancel")}</Button>
        <Button onClick={() => void (dialog.kind === "create" ? submitCreateProfile() : submitDuplicateProfile())} disabled={busy}>{t(busy ? "pickit.coreChecking" : dialog.kind === "create" ? "pickit.workspace.createAction" : "pickit.workspace.duplicateAction")}</Button>
      </div>
    </Dialog>}
    {dialog?.kind === "delete" && draft && <Dialog title={t("pickit.workspace.deleteTitle", { name: draft.name })} onClose={closeDialog} className="pickit-feature-delete-dialog">
      <p>{t("pickit.workspace.deleteDetail")}</p>
      {usages.length > 0 && <div className="pickit-delete-impact"><Users size={20} /><div><strong>{t("pickit.workspace.deleteImpact", { count: usages.length })}</strong><ul>{usages.map((usage) => <li key={`${usage.character}-${usage.route}`}>{usage.character} · {presentRunName(usage.route, t)}</li>)}</ul></div></div>}
      {usages.length > 0 && <p className="pickit-delete-warning">{t("pickit.workspace.deleteWarning")}</p>}
      {dialogError && <p role="alert">{dialogError}</p>}
      <div className="modal-actions">
        <Button variant="secondary" onClick={closeDialog} disabled={busy}>{t("common.cancel")}</Button>
        <Button variant="danger" onClick={() => void submitDeleteProfile()} disabled={busy}>{t(busy ? "pickit.deleting" : usages.length > 0 ? "pickit.workspace.deleteAction" : "pickit.deletePermanently")}</Button>
      </div>
    </Dialog>}
    {dialog?.kind === "discard" && <Dialog title={t("pickit.discardTitle")} onClose={closeDialog}>
      <p>{t("pickit.discardDetail")}</p>
      <div className="modal-actions">
        <Button variant="secondary" onClick={closeDialog}>{t("common.cancel")}</Button>
        <Button variant="danger" onClick={confirmDiscard}>{t("pickit.discard")}</Button>
      </div>
    </Dialog>}
  </div>;
}

function clone<T>(value: T): T { return JSON.parse(JSON.stringify(value)) as T; }
function message(reason: unknown, t: AppTranslator, fallback: string): string { return presentApiError(reason, t, fallback); }
function uniqueRuleID(existing: Set<string>, suggestion: string): string { let id = suggestion, suffix = 2; while (existing.has(id)) id = `${suggestion}-${suffix++}`; existing.add(id); return id; }
function uniqueProfileID(profiles: PickitProfileDTO[], name: string): string {
  const base = name.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase().replace(/ß/g, "ss").replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "profil";
  const existing = new Set(profiles.map((profile) => profile.id));
  let id = base;
  let suffix = 2;
  while (existing.has(id)) id = `${base}-${suffix++}`;
  return id;
}
function profileUsages(assignments: PickitAssignmentsDTO | null, profileID: string): Array<{ character: string; route: string }> {
  if (!assignments) return [];
  const usages: Array<{ character: string; route: string }> = [];
  for (const [character, routes] of Object.entries(assignments.assignments as Record<string, Record<string, string[]>>)) {
    for (const [route, profiles] of Object.entries(routes)) if (profiles.includes(profileID)) usages.push({ character, route });
  }
  return usages;
}
function presentRuleSummary(rule: PickitRuleDTO, catalog: PickitCatalogDTO, language: string | undefined, t: AppTranslator): { title: string; detail: string } {
  const kind = rule.summary?.kind ?? "custom";
  const params = rule.summary?.params ?? {};
  const action = t(rule.action === "keep" ? "pickit.workspace.ruleSummary.keepDetail" : "pickit.workspace.ruleSummary.sellDetail");
  if (kind === "runes") return { title: t("pickit.workspace.rules.runes.title"), detail: action };
  if (kind === "rejuvenation") return { title: t("pickit.workspace.rules.rejuvenation.title"), detail: action };
  if (kind === "item_codes") {
    const names = (params.codes ?? []).map((code) => {
      const entry = catalog.bases.find((base) => base.code === code);
      return entry ? gameBaseItemName(entry.code, entry.name, language) : "";
    }).filter(Boolean);
    return { title: names.length ? names.join(", ") : t("pickit.workspace.ruleSummary.selectedItems"), detail: action };
  }
  if (kind === "item_types") {
    const types = new Set(params.types ?? []);
    const names = equipmentTypeOptions
      .filter((option) => option.codes.some((code) => types.has(code)))
      .map((option) => equipmentTypeLabel(option.id, t));
    return { title: names.length ? names.join(", ") : t("pickit.workspace.ruleSummary.selectedItems"), detail: action };
  }
  if (kind === "quality") return { title: t("pickit.workspace.ruleSummary.quality", { quality: (params.qualities ?? []).map((value) => pickitQualityLabel(value, t)).join(" / ") }), detail: action };
  if (kind === "tier") return { title: t("pickit.workspace.ruleSummary.tier", { tier: (params.tiers ?? []).map((value) => pickitTierLabel(value, t)).join(" / ") }), detail: action };
  if (kind === "quality_tier") return { title: t("pickit.workspace.ruleSummary.qualityTier", { quality: (params.qualities ?? []).map((value) => pickitQualityLabel(value, t)).join(" / "), tier: (params.tiers ?? []).map((value) => pickitTierLabel(value, t)).join(" / ") }), detail: action };
  if (kind === "socket_filter") return { title: t("pickit.workspace.ruleSummary.socketFilter", { count: params.socket_count ?? "?" }), detail: action };
  if (kind === "set_item" || kind === "unique_item") {
    const key = kind === "set_item" ? params.set_key ?? "" : params.unique_key ?? "";
    const entry = catalog.identities.find((identity) => identity.key === key);
    return { title: entry ? gameIdentityName(entry.key, entry.display_name, language) : t("pickit.workspace.ruleSummary.selectedItems"), detail: action };
  }
  return { title: t("pickit.workspace.ruleSummary.custom"), detail: action };
}
function pickitQualityLabel(quality: string, t: AppTranslator): string {
  const keys = { low_quality: "pickit.qualities.lowQuality", normal: "pickit.qualities.normal", superior: "pickit.qualities.superior", magic: "pickit.qualities.magic", set: "pickit.qualities.set", rare: "pickit.qualities.rare", unique: "pickit.qualities.unique", crafted: "pickit.qualities.crafted" } as const;
  const key = keys[quality as keyof typeof keys];
  return key ? t(key) : quality;
}
function pickitTierLabel(tier: string, t: AppTranslator): string {
  const keys = { normal: "pickit.normal", exceptional: "pickit.exceptional", elite: "pickit.elite" } as const;
  return t(keys[tier as keyof typeof keys] ?? "pickit.any");
}
