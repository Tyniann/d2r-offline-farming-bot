import { useState, type ReactNode } from "react";
import { ChevronRight } from "lucide-react";
import { useTranslation } from "react-i18next";
import { characterAvailabilityText } from "../../app/characterReasons";
import { StateMessage, StatusBadge } from "../../app/ui";
import { characterStatusLabel } from "../characters/characterReasonText";
import {
  AutostartSwitch, BindingsBlock, BudgetFields, ChangeDot, DeleteHistoryButton, DiagnosticBlock, DifficultySelect, EffectiveValues, EventsList,
  HotkeyFields, InputSwitch, InventoryBlock, InventoryPreview, KeyCap, OnboardingRow, PlayersSelect, ProfileBadge, ProfileSwitchBlock, QueueBlock,
  ReadinessNote, ResetButton, RetentionDays, RetentionSwitch, SaveBar, SettingsNotices, SkillsBlock, UpdateActions, UpdateBadge,
} from "./SettingsControls";
import { SettingsFocusPanel } from "./SettingsFocusPanel";
import { useCharacterContext, type SettingsModel } from "./settingsModel";

/** Panel benennt die vier Fokus-Editoren; die drei Charakter-Panels bearbeiten den Draft, Wartung wirkt sofort. */
type Panel = "queue" | "combat" | "inventory" | "maintenance";

/**
 * SettingsOverview ist das Layout „Übersicht & Fokus“: drei flache Flächen – Charakter (pro Charakter, gemeinsam
 * gespeichert), Bot (alle Charaktere, gemeinsam gespeichert) und System (sofort wirksam). Häufige einfache Werte
 * sind direkt in der Zeile editierbar; komplexe Editoren öffnen per Klick auf die ganze Zusammenfassungszeile ein
 * [SettingsFocusPanel]. Die Charakterleiste ist zugleich Auswahl und Kopf der Charakterfläche.
 */
export function SettingsOverview({ model }: { model: SettingsModel }) {
  const { t, draft } = model;
  const ctx = useCharacterContext(model, model.activeCharacter);
  const catalog = model.props.catalog ?? null;
  const [panel, setPanel] = useState<Panel | null>(null);

  if (!draft) return null;
  const queue = ctx.settings?.queue ?? [];
  const runLabel = (id: string) => ctx.runs.find((run) => run.id === id)?.label ?? id;
  const changedAt = (path: string) => model.diffPaths.some((entry) => entry === path || entry.startsWith(`${path}.`) || entry.startsWith(`${path}[`));
  const slugPath = `characters.${ctx.slug}`;
  const boundKeys = Object.values(ctx.bindingValue.skills).filter(Boolean);
  const beltKeys = [ctx.bindingValue.belt.slot_1, ctx.bindingValue.belt.slot_2, ctx.bindingValue.belt.slot_3, ctx.bindingValue.belt.slot_4];
  const spareRuns = ctx.runs.filter((run) => !queue.includes(run.id) && (run.status === "available" || run.status === "runtime_validation_required")).length;
  const characterPanel = panel === "queue" || panel === "combat" || panel === "inventory";
  const barVisible = model.dirty || !model.mutable;
  // Ersetzt den auf dieser Seite ausgeblendeten Shell-Hinweis: Stimmt der in D2R bestätigte Kontext mit dem gespeicherten Entwurf überein?
  const selection = model.props.status?.selection;
  const liveInD2R = !!selection?.character && !!ctx.settings
    && selection.character.toLowerCase() === ctx.name.toLowerCase() && selection.difficulty === ctx.settings.last_difficulty;
  const systemEyebrow = `${t("settings.sectionSystem")} · ${t("settings.immediate")}`;

  const panelTitle: Record<Panel, string> = {
    queue: t("settings.queueTitle"), combat: t("characters.bindings"), inventory: t("characters.inventory"), maintenance: t("settings.maintenance"),
  };

  return <div className={`settings-overview${barVisible ? " settings-overview-with-bar" : ""}`}>
    <SettingsNotices model={model} />

    <section className="settings-surface">
      <div className="settings-surface-head settings-surface-head-characters">
        <div className="settings-character-tabs" role="group" aria-label={t("characters.listAria")}>
          {model.characterNames.map((slug) => {
            const entry = catalog?.characters.find((item) => item.slug === slug || item.name.toLowerCase() === slug);
            const stored = draft.characters[slug];
            const active = slug === model.activeCharacter;
            // Nicht unterstützte Klassen lehnt die App-Auswahl ab; der Eintrag bleibt sichtbar, aber nicht auswählbar.
            const selectable = active || !!entry?.selectable;
            return <button key={slug} type="button" aria-pressed={active} disabled={!selectable} title={selectable ? undefined : t("characters.statusUnsupported")} className={`settings-character-tab${active ? " active" : ""}`} onClick={() => model.selectCharacter(slug)}>
              <ProfileBadge size={active ? "md" : "sm"} profileID={stored?.combat_profile ?? ""} characterClass={stored?.character_class ?? entry?.expected_class ?? ""} muted={!entry?.selectable} />
              <span className="settings-character-tab-text">
                <strong>{entry?.name ?? slug}{model.characterChanged(slug) && <ChangeDot title={t("settings.changed")} />}</strong>
                {active
                  ? <small>{ctx.className}{ctx.supported ? ` · ${ctx.profileName}` : ""}</small>
                  : <small>{characterStatusLabel(entry, catalog, t)}</small>}
              </span>
              {active && <StatusBadge tone={ctx.farmReady ? "success" : "warning"}>{ctx.statusLabel}</StatusBadge>}
            </button>;
          })}
        </div>
        {ctx.settings && <div className="settings-head-fields">
          <label className={changedAt(`${slugPath}.last_difficulty`) ? "settings-changed" : undefined}>{t("settings.lastDifficulty")}<DifficultySelect ctx={ctx} model={model} /></label>
          <label className={changedAt(`${slugPath}.players`) ? "settings-changed" : undefined}>{t("characters.playersTitle")}<PlayersSelect ctx={ctx} model={model} /></label>
          {selection?.character && <p className={`settings-live-note ${liveInD2R ? "live" : "pending"}`}>{t(liveInD2R ? "sidebar.activeInD2R" : "sidebar.notActiveInD2R")}</p>}
        </div>}
      </div>

      {ctx.settings ? <>
        <Line label={t("settings.queueTitle")} changed={changedAt(`${slugPath}.queue`)} openLabel={t("settings.openPanel")} onOpen={() => setPanel("queue")}>
          {queue.length === 0
            ? <span className="hint">{t("settings.queueEmpty")}</span>
            : <span className="settings-chips">{queue.map((id, index) => <span key={id} className="settings-chip-run"><b>{index + 1}</b>{runLabel(id)}</span>)}</span>}
          {spareRuns > 0 && <small className="hint">{t("settings.spareRuns", { count: spareRuns })}</small>}
        </Line>
        <Line label={t("characters.bindings")} changed={changedAt(`${slugPath}.profile_bindings`)} openLabel={t("settings.openPanel")} onOpen={() => setPanel("combat")}>
          <span className="settings-chips">
            <span className="settings-chip-text">{ctx.profileName}</span>
            <span className="settings-keys">{boundKeys.length > 0 ? boundKeys.map((key, index) => <KeyCap key={`${key}-${index}`}>{key}</KeyCap>) : <span className="hint">{t("characters.bindingsMissing")}</span>}</span>
            <span className="settings-keys"><span className="hint">{t("characters.belt")}</span>{beltKeys.map((key, index) => <KeyCap key={index}>{key || "·"}</KeyCap>)}</span>
            <StatusBadge tone={ctx.profile?.bindings_ready ? "success" : "warning"}>{t(ctx.profile?.bindings_ready ? "characters.bindingsComplete" : "characters.bindingsMissing")}</StatusBadge>
          </span>
        </Line>
        <Line label={t("characters.inventory")} changed={changedAt(`${slugPath}.inventory_lock`)} openLabel={t("settings.openPanel")} onOpen={() => setPanel("inventory")}>
          <span className="settings-chips">
            <InventoryPreview ctx={ctx} />
            <span className="settings-chip-text">{ctx.inventoryIsConfigured ? t("settings.inventorySummary", { locked: ctx.lockedCells, total: 40 }) : t("characters.inventoryUnconfirmed")}</span>
            {ctx.cowWarning && <StatusBadge tone="warning">{t("settings.cowUnsuitable")}</StatusBadge>}
          </span>
        </Line>
      </> : <div className="settings-line settings-line-note">
        <StateMessage kind="empty" title={ctx.statusLabel}>{ctx.entry && catalog ? characterAvailabilityText(ctx.entry, catalog, t) : t("settings.noCharacterSettings")}</StateMessage>
      </div>}
    </section>

    <section className="settings-surface">
      <div className="settings-surface-head"><h2>{t("settings.sectionBot")}</h2><span className="settings-tag">{t("settings.savedTogether")}</span><small className="hint">{t("settings.appliesToAll")}</small></div>
      <Line label={t("settings.budgets")} hint={t("settings.budgetsDetail")} changed={model.budgetsChanged}><BudgetFields model={model} /></Line>
      <Line label={t("settings.inputTitle")} hint={t("settings.inputDetail")} changed={model.inputChanged}>
        <InputSwitch model={model} />
        <HotkeyFields model={model} />
      </Line>
      <Line label={t("settings.retentionTitle")} hint={t("settings.retentionDetail")} changed={model.historyChanged}>
        <span className="settings-inline"><RetentionSwitch model={model} /><RetentionDays model={model} /></span>
      </Line>
    </section>

    <section className="settings-surface">
      <div className="settings-surface-head"><h2>{t("settings.sectionSystem")}</h2><span className="settings-tag">{t("settings.immediate")}</span></div>
      <Line label={t("settings.appStartTitle")}><AutostartSwitch model={model} /></Line>
      <Line label={t("settings.version")}>
        <span className="settings-inline"><UpdateBadge model={model} /><small className="hint">{model.updateDescription()}</small><UpdateActions model={model} /></span>
      </Line>
      <Line label={t("settings.setupTitle")}><OnboardingRow model={model} /></Line>
      <Line label={t("settings.maintenance")} openLabel={t("settings.openPanel")} onOpen={() => setPanel("maintenance")}>
        <span className="settings-chips">
          <span className="settings-chip-text">{t("settings.eventCount", { count: model.props.events.length })}</span>
          <span className="settings-chip-text">{t("settings.diagnosticTitle")}</span>
          <span className="settings-chip-text">{t("settings.deleteHistory")}</span>
          <span className="settings-chip-text">{t("settings.resetTitle")}</span>
        </span>
      </Line>
    </section>

    {barVisible
      ? <SaveBar model={model} />
      : <p className="settings-foot hint">{t("settings.noOpenChanges")} · {t("settings.revision", { revision: model.settings?.revision ?? 0 })} · {t("settings.shortcutSave")}</p>}

    {panel && <SettingsFocusPanel
      title={panelTitle[panel]}
      eyebrow={characterPanel ? ctx.name : systemEyebrow}
      badge={characterPanel ? <ProfileBadge size="sm" profileID={ctx.profileID} characterClass={ctx.characterClass} /> : undefined}
      footerNote={characterPanel ? t("settings.panelSaveHint") : t("settings.maintenanceScope")}
      onClose={() => setPanel(null)}
    >
      {panel === "queue" && <><p className="hint">{t("settings.queueDetail")}</p><QueueBlock ctx={ctx} model={model} /></>}
      {panel === "combat" && <>
        <h3>{t("characters.wizardCombatProfile")}</h3>
        <ProfileSwitchBlock ctx={ctx} model={model} />
        <details className="settings-details"><summary>{t("characters.requiredSkills")}</summary><SkillsBlock ctx={ctx} /></details>
        <h3>{t("characters.bindings")}</h3>
        <BindingsBlock ctx={ctx} model={model} />
      </>}
      {panel === "inventory" && <><InventoryBlock ctx={ctx} model={model} /><ReadinessNote ctx={ctx} model={model} /></>}
      {panel === "maintenance" && <>
        <h3>{t("settings.diagnosticTitle")}</h3>
        <DiagnosticBlock model={model} />
        <h3>{t("settings.liveEvents")}</h3>
        <EventsList model={model} />
        <h3>{t("settings.effectiveTitle")}</h3>
        <div className="settings-stack"><EffectiveValues model={model} /></div>
        <div className="settings-danger-block">
          <div className="settings-danger-row"><div><strong>{t("settings.deleteHistory")}</strong><small className="hint">{t("settings.deleteHistoryDetail")}</small></div><DeleteHistoryButton model={model} /></div>
          <div className="settings-danger-row"><div><strong>{t("settings.resetTitle")}</strong><small className="hint">{t("settings.resetDetail")}</small></div><ResetButton model={model} /></div>
        </div>
      </>}
    </SettingsFocusPanel>}
  </div>;
}

/**
 * Line ist eine Zeile der flachen Fläche: Beschriftung links, Inhalt rechts. Mit `onOpen` wird die gesamte
 * Zeile zur Schaltfläche für das Fokus-Panel (Chevron rechts); ohne `onOpen` enthält sie direkt editierbare Controls.
 */
function Line({ label, hint, changed, openLabel, onOpen, children }: { label: string; hint?: string; changed?: boolean; openLabel?: string; onOpen?: () => void; children: ReactNode }) {
  const { t } = useTranslation();
  const body = <>
    <span className="settings-line-label"><strong>{label}{changed && <ChangeDot title={t("settings.changed")} />}</strong>{hint && <small>{hint}</small>}</span>
    <span className="settings-line-body">{children}</span>
    {onOpen && <ChevronRight className="settings-line-chevron" size={18} aria-hidden="true" />}
  </>;
  const className = `settings-line${changed ? " changed" : ""}`;
  return onOpen
    ? <button type="button" className={`${className} settings-line-open`} title={openLabel} onClick={onOpen}>{body}</button>
    : <div className={className}>{body}</div>;
}
