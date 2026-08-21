// PROTOTYPE: Three dashboard directions on the existing route, selected with `?variant=A|B|C`.
import { useEffect, useMemo, useState } from "react";
import {
  Activity, BarChart3, Check, ChevronLeft, ChevronRight, CircleAlert, Clock3, ExternalLink,
  Gauge, History, OctagonX, Pause, Play, RotateCcw, ShieldCheck, Sparkles, Square, Target, Trophy,
} from "lucide-react";
import {
  Area, AreaChart, Bar, BarChart, CartesianGrid, Cell, Pie, PieChart, ResponsiveContainer, Tooltip,
  XAxis, YAxis,
} from "recharts";
import {
  getHistoryComparisons, getHistoryRuns, getHistorySummary, type CatalogDTO, type HistoryComparisonDTO,
  type HistoryRunDTO, type RunCatalogEntry, type StatusDTO,
} from "../../api/generated";
import "./DashboardPrototype.css";

export type DashboardPrototypeVariant = "A" | "B" | "C";
type PrototypeMode = "idle" | "active";

interface Props {
  variant: DashboardPrototypeVariant;
  status: StatusDTO | null;
  catalog: CatalogDTO | null;
  character: string;
  difficulty: string;
  confirmedDifficultyLabel: string;
  queueIDs: string[];
  availableRuns: RunCatalogEntry[];
  connected: boolean;
  hotkeys?: { pause: string; stopAfterRun: string; emergencyStop: string };
}

interface DashboardData {
  runs: number;
  successful: number;
  failed: number;
  aborted: number;
  successRate: number;
  averageDurationMs: number;
  keep: number;
  keepPerHour: number;
  daily: Array<{ date: string; runs: number; success: number }>;
  comparisons: Array<{ name: string; rate: number }>;
  recent: HistoryRunDTO[];
  source: "core" | "demo";
}

const demoData: DashboardData = {
  runs: 184,
  successful: 169,
  failed: 9,
  aborted: 6,
  successRate: 0.918,
  averageDurationMs: 78_400,
  keep: 43,
  keepPerHour: 2.7,
  daily: [
    { date: "15.8.", runs: 18, success: 0.89 }, { date: "16.8.", runs: 24, success: 0.92 },
    { date: "17.8.", runs: 21, success: 0.86 }, { date: "18.8.", runs: 32, success: 0.94 },
    { date: "19.8.", runs: 27, success: 0.93 }, { date: "20.8.", runs: 34, success: 0.97 },
    { date: "21.8.", runs: 28, success: 0.93 },
  ],
  comparisons: [
    { name: "Unter-Kurast", rate: 3.8 }, { name: "Mephisto", rate: 2.9 },
    { name: "Gräfin", rate: 2.2 }, { name: "Nihlathak", rate: 1.5 },
  ],
  recent: [
    demoRun("lower-kurast", "success", 64_000, 2, "2026-08-21T10:24:00Z"),
    demoRun("mephisto", "success", 91_000, 1, "2026-08-21T10:21:00Z"),
    demoRun("countess", "failed", 118_000, 0, "2026-08-21T10:17:00Z", "Boss nicht gefunden"),
    demoRun("lower-kurast", "success", 61_000, 1, "2026-08-21T10:14:00Z"),
  ],
  source: "demo",
};

export function DashboardPrototype(props: Props) {
  const [mode, setMode] = useState<PrototypeMode>(() => prototypeModeFromURL());
  const [period, setPeriod] = useState<"7" | "30" | "all">("30");
  const [notice, setNotice] = useState("");
  const data = useDashboardData(props.character, props.difficulty);
  const difficultyLabel = props.catalog?.difficulties.find((entry) => entry.id === props.difficulty)?.display_name ?? labelDifficulty(props.difficulty);
  const confirmedCharacter = props.status?.selection.character ?? "MrHammer";
  const confirmedDifficulty = props.confirmedDifficultyLabel || "Hölle";
  const differs = props.character !== confirmedCharacter || difficultyLabel !== confirmedDifficulty;
  const queue = queueRows(props.queueIDs, props.availableRuns);
  const activeRun = queue[0] ?? { id: "lower-kurast", name: "Unter-Kurast", ready: true };
  const common: VariantProps = {
    ...props,
    mode,
    data,
    queue,
    activeRun,
    difficultyLabel,
    confirmedCharacter,
    confirmedDifficulty,
    differs,
    period,
    onPeriod: setPeriod,
    act: (message) => { setNotice(message); window.setTimeout(() => setNotice(""), 3200); },
  };

  return <div className={`dashboard-prototype dashboard-variant-${props.variant.toLowerCase()} dashboard-mode-${mode}`}>
    {props.variant === "A" && <VariantA {...common} />}
    {props.variant === "B" && <VariantB {...common} />}
    {props.variant === "C" && <VariantC {...common} />}
    {notice && <div className="prototype-toast" role="status"><Check aria-hidden="true" size={17} />{notice}</div>}
    <PrototypeSwitcher variant={props.variant} mode={mode} onMode={setMode} />
  </div>;
}

interface VariantProps extends Props {
  mode: PrototypeMode;
  data: DashboardData;
  queue: QueueRow[];
  activeRun: QueueRow;
  difficultyLabel: string;
  confirmedCharacter: string;
  confirmedDifficulty: string;
  differs: boolean;
  period: "7" | "30" | "all";
  onPeriod(value: "7" | "30" | "all"): void;
  act(message: string): void;
}

function VariantA(props: VariantProps) {
  const active = props.mode === "active";
  return <>
    <header className="dash-a-header">
      <div>
        <p className="prototype-kicker">Dashboard</p>
        <h1>{active ? `${props.activeRun.name} läuft` : `${props.character} ist bereit`}</h1>
        <p>{active ? "D2R wird gesteuert. Den Fortschritt siehst du in den Run-Etappen." : `${props.difficultyLabel} · ${queueSize(props.queue)} Runs pro Spiel · nächster Start in D2R`}</p>
      </div>
      <Readiness connected={props.connected} active={active} area={props.status?.world.area_name || "Schurkenlager"} />
    </header>

    {props.differs && <SelectionNotice {...props} />}
    {active && <ActiveRunHero {...props} />}

    <div className="dash-a-period-row">
      <span>Statistik für {props.character}</span>
      <PeriodSwitch value={props.period} onChange={props.onPeriod} />
    </div>

    <div className="dash-a-main">
      <section className="prototype-panel farm-card">
        <PanelHeading eyebrow="Farming" title="Deine Run-Reihenfolge" action={<button className="prototype-text-button" onClick={() => props.act("Die Queue-Einstellungen würden geöffnet.")}>Bearbeiten <ExternalLink size={14} /></button>} />
        <QueueList rows={props.queue} active={active} />
        {!active && <button className="prototype-primary prototype-start" onClick={() => props.act(`Die Queue für ${props.character} würde jetzt geprüft und gestartet.`)}><Play aria-hidden="true" size={18} />Jetzt farmen</button>}
        <p className="prototype-footnote">{queueSize(props.queue)} Runs · geschätzt 5 Min. pro Spiel</p>
      </section>

      <section className="prototype-panel dash-a-metric-card">
        <PanelHeading eyebrow="Leistung" title="Auf einen Blick" />
        <div className="dash-a-metric-grid">
          <Score icon={RotateCcw} label="Runs" value={String(props.data.runs)} note="im Zeitraum" />
          <Score icon={Clock3} label="Ø Runzeit" value={duration(props.data.averageDurationMs)} note="pro Route" />
          <Score icon={Sparkles} label="Gesicherte Items" value={String(props.data.keep)} note="im Zeitraum" />
          <Score icon={Gauge} label="Items pro Stunde" value={number(props.data.keepPerHour)} note="gesichert" positive />
        </div>
      </section>

      <section className="prototype-panel dash-a-outcome-card">
        <PanelHeading eyebrow="Ergebnisse" title="Verteilung" />
        <OutcomeRing data={props.data} large />
        <div className="legend-list"><span><i className="success" />Erfolgreich <strong>{props.data.successful}</strong></span><span><i className="failed" />Fehlgeschlagen <strong>{props.data.failed}</strong></span><span><i className="aborted" />Abgebrochen <strong>{props.data.aborted}</strong></span></div>
      </section>
    </div>

    <div className="dash-a-bottom">
      <section className="prototype-panel dash-a-routes">
        <PanelHeading eyebrow="Routen" title="Welche Route lohnt sich?" action={<BarChart3 size={18} />} />
        <RouteBars rows={props.data.comparisons} />
      </section>
      <section className="prototype-panel recent-card">
        <PanelHeading eyebrow="Zuletzt" title="Letzte Runs" action={<button className="prototype-icon-button" aria-label="Historie öffnen" onClick={() => props.act("Die gefilterte Historie würde geöffnet.")}><ExternalLink size={17} /></button>} />
        <RecentRuns runs={props.data.recent.slice(0, 3)} />
      </section>
    </div>
  </>;
}

function VariantB(props: VariantProps) {
  const active = props.mode === "active";
  return <>
    <header className="dash-b-command">
      <div className="dash-b-title">
        <span className="prototype-kicker">{active ? "Session läuft" : "Bereit zum Farmen"}</span>
        <h1>{props.character} <small>{props.difficultyLabel}</small></h1>
      </div>
      <div className="dash-b-readiness"><span><i className="ready-dot" />D2R bereit</span><span><ShieldCheck size={16} />Steuerung bereit</span><span><Target size={16} />{props.status?.world.area_name || "Schurkenlager"}</span></div>
      {active
        ? <div className="dash-b-actions"><button className="prototype-secondary"><Pause size={17} />Nach Run pausieren</button><button className="prototype-danger"><Square size={17} />Nach Run stoppen</button></div>
        : <button className="prototype-primary" onClick={() => props.act(`Queue für ${props.character} würde geprüft und gestartet.`)}><Play size={18} />{queueSize(props.queue)} Runs starten</button>}
    </header>
    {props.differs && <SelectionNotice {...props} />}

    <section className="dash-b-scoreboard">
      <Score icon={RotateCcw} label="Runs" value={String(props.data.runs)} note="letzte 30 Tage" />
      <Score icon={Trophy} label="Erfolgreich" value={percent(props.data.successRate)} note={`${props.data.successful} Runs`} />
      <Score icon={Clock3} label="Ø Runzeit" value={duration(props.data.averageDurationMs)} note="4 s schneller" positive />
      <Score icon={Sparkles} label="Gesicherte Items" value={String(props.data.keep)} note={`${number(props.data.keepPerHour)} pro Stunde`} positive />
    </section>

    {active && <ActiveRunStrip {...props} />}

    <div className="dash-b-grid">
      <section className="prototype-panel dash-b-routes">
        <PanelHeading eyebrow="Vergleich" title="Welche Route lohnt sich?" action={<span className="prototype-source">30 Tage</span>} />
        <RouteBars rows={props.data.comparisons} />
      </section>
      <section className="prototype-panel dash-b-outcomes">
        <PanelHeading eyebrow="Qualität" title="Run-Ergebnisse" />
        <OutcomeRing data={props.data} large />
        <div className="legend-list"><span><i className="success" />Erfolgreich <strong>{props.data.successful}</strong></span><span><i className="failed" />Fehlgeschlagen <strong>{props.data.failed}</strong></span><span><i className="aborted" />Abgebrochen <strong>{props.data.aborted}</strong></span></div>
      </section>
      <section className="prototype-panel dash-b-queue">
        <PanelHeading eyebrow="Nächstes Spiel" title="Run-Plan" action={<button className="prototype-text-button" onClick={() => props.act("Die Queue-Einstellungen würden geöffnet.")}>Bearbeiten</button>} />
        <QueueTimeline rows={props.queue} active={active} />
      </section>
      <section className="prototype-panel dash-b-activity">
        <PanelHeading eyebrow="Aktivität" title="Gerade passiert" action={<Activity size={18} />} />
        <RecentRuns runs={props.data.recent.slice(0, 4)} />
      </section>
    </div>
  </>;
}

function VariantC(props: VariantProps) {
  const active = props.mode === "active";
  return <>
    <div className="dash-c-shell">
      <aside className="dash-c-control">
        <p className="prototype-kicker">{active ? "Bot arbeitet" : "Farming starten"}</p>
        <h1>{active ? props.activeRun.name : props.character}</h1>
        <p>{active ? `Run 1 von ${queueSize(props.queue)}` : `${props.difficultyLabel} · ${queueSize(props.queue)} Runs in der Queue`}</p>
        <Readiness connected={props.connected} active={active} area={props.status?.world.area_name || "Schurkenlager"} vertical />
        {props.differs && <SelectionNotice {...props} compact />}
        {active && <div className="dash-c-controls"><button className="prototype-secondary"><Pause size={17} />Nach Run pausieren</button><button className="prototype-danger"><Square size={17} />Nach Run stoppen</button></div>}
        <div className="dash-c-queue"><span className="dash-c-label">Run-Plan</span><QueueList rows={props.queue} active={active} minimal /></div>
        {!active && <button className="prototype-primary prototype-start" onClick={() => props.act(`Queue für ${props.character} würde gestartet.`)}><Play size={18} />Jetzt farmen</button>}
        <button className="prototype-text-button dash-c-edit" onClick={() => props.act("Die Queue-Einstellungen würden geöffnet.")}>Run-Plan bearbeiten <ExternalLink size={14} /></button>
      </aside>

      <main className="dash-c-insights">
        <header><div><p className="prototype-kicker">Leistung · 30 Tage</p><h2>Was hat {props.character} erreicht?</h2></div><span className="prototype-source">{props.data.source === "core" ? "Aktuelle Daten" : "Prototypdaten"}</span></header>
        <div className="dash-c-hero-metric">
          <div><span>Gesicherte Items pro Stunde</span><strong>{number(props.data.keepPerHour)}</strong><small><b>+12 %</b> gegenüber den vorherigen 30 Tagen</small></div>
          <TrendChart data={props.data} compact />
        </div>
        <div className="dash-c-metrics">
          <Score icon={RotateCcw} label="Runs" value={String(props.data.runs)} note={`${props.data.successful} erfolgreich`} />
          <Score icon={Gauge} label="Erfolgsquote" value={percent(props.data.successRate)} note="stabil" positive />
          <Score icon={Clock3} label="Ø Runzeit" value={duration(props.data.averageDurationMs)} note="Median 1:14" />
          <Score icon={Sparkles} label="Gesicherte Items" value={String(props.data.keep)} note="9 verkauft" />
        </div>
        <div className="dash-c-lower">
          <section>
            <PanelHeading eyebrow="Routen" title="Ertrag pro Stunde" action={<BarChart3 size={18} />} />
            <RouteBars rows={props.data.comparisons} />
          </section>
          <section>
            <PanelHeading eyebrow="Letzte Runs" title="Neueste Ergebnisse" action={<button className="prototype-icon-button" aria-label="Historie öffnen" onClick={() => props.act("Die gefilterte Historie würde geöffnet.")}><ExternalLink size={17} /></button>} />
            <RecentRuns runs={props.data.recent.slice(0, 3)} />
          </section>
        </div>
      </main>
    </div>
  </>;
}

export function DashboardPrototypeContext({ catalog, character, difficulty, confirmedCharacter, locked, onCharacter, onDifficulty }: {
  catalog: CatalogDTO | null;
  character: string;
  difficulty: string;
  confirmedCharacter: string;
  locked: boolean;
  onCharacter(value: string): void;
  onDifficulty(value: string): void;
}) {
  const characters = catalog?.characters.filter((entry) => entry.selectable) ?? [];
  const difficulties = catalog?.difficulties ?? [];
  return <div className="prototype-global-context">
    <span className="prototype-global-label">Ausgewählter Charakter</span>
    <label><span className="visually-hidden">Globaler Charakter</span><select aria-label="Globaler Charakter" value={character} disabled={locked} onChange={(event) => onCharacter(event.target.value)}>
      {(characters.length ? characters : [{ name: "MrHammer", slug: "mrhammer", selectable: true, farm_ready: true }, { name: "MrBones", slug: "mrbones", selectable: true, farm_ready: true }]).map((entry) => <option key={entry.slug} value={entry.name}>{entry.name}</option>)}
    </select></label>
    <label><span className="visually-hidden">Globale Schwierigkeit</span><select aria-label="Globale Schwierigkeit" value={difficulty} disabled={locked} onChange={(event) => onDifficulty(event.target.value)}>
      {(difficulties.length ? difficulties : [{ id: "normal", display_name: "Normal" }, { id: "nightmare", display_name: "Alptraum" }, { id: "hell", display_name: "Hölle" }]).map((entry) => <option key={entry.id} value={entry.id}>{entry.display_name}</option>)}
    </select></label>
    <span className={`prototype-confirmed ${character === confirmedCharacter ? "is-confirmed" : "is-pending"}`}><i />{!confirmedCharacter ? "Noch kein Charakter in D2R bestätigt" : character === confirmedCharacter ? "In D2R aktiv" : `${confirmedCharacter} in D2R aktiv`}</span>
    {locked && <small>Während der Session gesperrt</small>}
  </div>;
}

function PrototypeSwitcher({ variant, mode, onMode }: { variant: DashboardPrototypeVariant; mode: PrototypeMode; onMode(value: PrototypeMode): void }) {
  const labels = { A: "Kommandozentrale", B: "Analyse-Raster", C: "Geteiltes Cockpit" };
  const cycle = (direction: -1 | 1) => {
    const variants: DashboardPrototypeVariant[] = ["A", "B", "C"];
    const next = variants[(variants.indexOf(variant) + direction + variants.length) % variants.length];
    updatePrototypeURL(next, mode);
  };
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target?.matches("input, textarea, select, [contenteditable]")) return;
      if (event.key === "ArrowLeft") cycle(-1);
      if (event.key === "ArrowRight") cycle(1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });
  return <div className="prototype-switcher" aria-label="Dashboard-Prototyp steuern">
    <button aria-label="Vorherige Variante" onClick={() => cycle(-1)}><ChevronLeft size={18} /></button>
    <strong>{variant} · {labels[variant]}</strong>
    <button aria-label="Nächste Variante" onClick={() => cycle(1)}><ChevronRight size={18} /></button>
    <span className="prototype-switch-divider" />
    <button className={mode === "idle" ? "active" : ""} onClick={() => { onMode("idle"); updatePrototypeURL(variant, "idle", false); }}>Leerlauf</button>
    <button className={mode === "active" ? "active" : ""} onClick={() => { onMode("active"); updatePrototypeURL(variant, "active", false); }}>Aktiver Run</button>
  </div>;
}

function Readiness({ connected, active, area, vertical = false }: { connected: boolean; active: boolean; area: string; vertical?: boolean }) {
  return <div className={`prototype-readiness ${vertical ? "vertical" : ""}`}>
    <span><i className={connected ? "ready-dot" : "warning-dot"} />{connected ? "Verbunden" : "Verbindung fehlt"}</span>
    <span><ShieldCheck size={15} />{active ? "Bot steuert D2R" : "D2R bereit"}</span>
    <span><Target size={15} />{area}</span>
  </div>;
}

function SelectionNotice(props: VariantProps & { compact?: boolean }) {
  return <div className={`prototype-selection-notice ${props.compact ? "compact" : ""}`}>
    <CircleAlert aria-hidden="true" size={19} />
    <div><strong>{props.character} ist in der App ausgewählt</strong><span>In D2R ist noch {props.confirmedCharacter} · {props.confirmedDifficulty} aktiv.</span></div>
    <button onClick={() => props.act(`${props.character} · ${props.difficultyLabel} würde jetzt in D2R angewendet.`)}>In D2R verwenden</button>
  </div>;
}

function ActiveRunHero(props: VariantProps) {
  const progress = { current: 6, total: 13, label: "Kellergeschoss 3 von 5" };
  const hotkeys = props.hotkeys ?? { pause: "Pause", stopAfterRun: "F10", emergencyStop: "F11" };
  return <section className="prototype-active-hero">
    <div className="active-run-icon"><Play size={22} fill="currentColor" /></div>
    <div className="active-run-copy">
      <span>Run 1 von {queueSize(props.queue)}</span>
      <strong>{props.activeRun.name}</strong>
      <div className="active-progress-steps" aria-label={`Etappe ${progress.current} von ${progress.total}`}>
        {Array.from({ length: progress.total }, (_, index) => <i key={index} className={index < progress.current - 1 ? "complete" : index === progress.current - 1 ? "current" : ""} />)}
      </div>
      <small>{progress.label} · Etappe {progress.current} von {progress.total} · 0:48 vergangen</small>
    </div>
    <div className="active-hotkeys" aria-label="Steuerung im Spiel">
      <HotkeyHint icon={Pause} keyLabel={hotkeys.pause} label="Nach diesem Run pausieren" />
      <HotkeyHint icon={Square} keyLabel={hotkeys.stopAfterRun} label="Nach diesem Run stoppen" />
      <HotkeyHint icon={OctagonX} keyLabel={hotkeys.emergencyStop} label="Sofort stoppen" danger />
    </div>
  </section>;
}

function HotkeyHint({ icon: Icon, keyLabel, label, danger = false }: { icon: typeof Pause; keyLabel: string; label: string; danger?: boolean }) {
  return <div className={`active-hotkey ${danger ? "danger" : ""}`} title={label}>
    <Icon aria-hidden="true" size={16} />
    <kbd>{keyLabel}</kbd>
    <span className="visually-hidden">{label}</span>
  </div>;
}

function PeriodSwitch({ value, onChange }: { value: "7" | "30" | "all"; onChange(value: "7" | "30" | "all"): void }) {
  return <div className="prototype-period-switch" aria-label="Statistik-Zeitraum">
    {([ ["7", "7 Tage"], ["30", "30 Tage"], ["all", "Gesamt"] ] as const).map(([id, label]) => <button key={id} type="button" aria-pressed={value === id} onClick={() => onChange(id)}>{label}</button>)}
  </div>;
}

function ActiveRunStrip(props: VariantProps) {
  return <section className="dash-b-active-strip"><div><span>Aktiver Run</span><strong>{props.activeRun.name}</strong></div><div className="active-progress"><i style={{ width: "62%" }} /></div><span>Boss-Suche · 0:48</span></section>;
}

function PanelHeading({ eyebrow, title, action }: { eyebrow: string; title: string; action?: React.ReactNode }) {
  return <div className="prototype-panel-heading"><div><span>{eyebrow}</span><h2>{title}</h2></div>{action}</div>;
}

function MetricGrid({ data, compact }: { data: DashboardData; compact: boolean }) {
  return <div className={`prototype-metrics ${compact ? "compact" : ""}`}>
    <div><span>Runs</span><strong>{data.runs}</strong></div>
    <div><span>Ø Runzeit</span><strong>{duration(data.averageDurationMs)}</strong></div>
    <div><span>Gesicherte Items</span><strong>{data.keep}</strong></div>
    <div><span>Items / Stunde</span><strong>{number(data.keepPerHour)}</strong></div>
  </div>;
}

function Score({ icon: Icon, label, value, note, positive = false }: { icon: typeof Activity; label: string; value: string; note: string; positive?: boolean }) {
  return <article className="prototype-score"><Icon aria-hidden="true" size={19} /><span>{label}</span><strong>{value}</strong><small className={positive ? "positive" : ""}>{note}</small></article>;
}

function QueueList({ rows, active, minimal = false }: { rows: QueueRow[]; active: boolean; minimal?: boolean }) {
  const visible = rows.length ? rows : [{ id: "lower-kurast", name: "Unter-Kurast", ready: true }, { id: "mephisto", name: "Mephisto", ready: true }, { id: "countess", name: "Gräfin", ready: true }];
  return <ol className={`prototype-queue-list ${minimal ? "minimal" : ""}`}>{visible.map((row, index) => <li key={row.id} className={active && index === 0 ? "is-active" : ""}>
    <span>{index + 1}</span><strong>{row.name}</strong>{!minimal && <small>{active && index === 0 ? "Läuft" : row.ready ? "Bereit" : "Route fehlt"}</small>}
  </li>)}</ol>;
}

function QueueTimeline({ rows, active }: { rows: QueueRow[]; active: boolean }) {
  const visible = rows.length ? rows : [{ id: "lower-kurast", name: "Unter-Kurast", ready: true }, { id: "mephisto", name: "Mephisto", ready: true }, { id: "countess", name: "Gräfin", ready: true }];
  return <ol className="prototype-queue-timeline">{visible.map((row, index) => <li key={row.id} className={active && index === 0 ? "is-active" : ""}><i>{index + 1}</i><div><strong>{row.name}</strong><span>{active && index === 0 ? "Läuft gerade" : "Bereit"}</span></div></li>)}</ol>;
}

function OutcomeRing({ data, large = false }: { data: DashboardData; large?: boolean }) {
  const rows = [{ name: "Erfolgreich", value: data.successful, color: "#65d5a1" }, { name: "Fehlgeschlagen", value: data.failed, color: "#f07878" }, { name: "Abgebrochen", value: data.aborted, color: "#e5b65c" }];
  return <div className={`prototype-ring ${large ? "large" : ""}`}><ResponsiveContainer width="100%" height="100%"><PieChart><Pie data={rows} dataKey="value" nameKey="name" innerRadius="68%" outerRadius="94%" paddingAngle={3} stroke="none">{rows.map((row) => <Cell key={row.name} fill={row.color} />)}</Pie><Tooltip contentStyle={{ background: "#211b22", border: "1px solid #59464f", borderRadius: 8 }} /></PieChart></ResponsiveContainer><span>{percent(data.successRate)}<small>Erfolg</small></span></div>;
}

function TrendChart({ data, compact = false }: { data: DashboardData; compact?: boolean }) {
  return <div className={`prototype-trend ${compact ? "compact" : ""}`}><ResponsiveContainer width="100%" height="100%"><AreaChart data={data.daily} margin={{ top: 8, right: 6, left: compact ? -30 : -20, bottom: 0 }}><defs><linearGradient id={`trend-${compact}`} x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="#ee7845" stopOpacity={0.5} /><stop offset="100%" stopColor="#ee7845" stopOpacity={0} /></linearGradient></defs>{!compact && <CartesianGrid vertical={false} stroke="#46383f" strokeDasharray="3 5" />}<XAxis dataKey="date" hide={compact} tick={{ fill: "#a99ca2", fontSize: 11 }} axisLine={false} tickLine={false} /><YAxis hide={compact} tick={{ fill: "#a99ca2", fontSize: 11 }} axisLine={false} tickLine={false} /><Tooltip contentStyle={{ background: "#211b22", border: "1px solid #59464f", borderRadius: 8 }} /><Area type="monotone" dataKey="runs" name="Runs" stroke="#f18a55" strokeWidth={3} fill={`url(#trend-${compact})`} /></AreaChart></ResponsiveContainer></div>;
}

function RouteBars({ rows }: { rows: DashboardData["comparisons"] }) {
  return <div className="prototype-route-bars"><ResponsiveContainer width="100%" height="100%"><BarChart data={rows} layout="vertical" margin={{ top: 4, right: 16, left: 12, bottom: 4 }}><XAxis type="number" hide /><YAxis type="category" dataKey="name" width={92} tick={{ fill: "#c7b9bf", fontSize: 12 }} axisLine={false} tickLine={false} /><Tooltip contentStyle={{ background: "#211b22", border: "1px solid #59464f", borderRadius: 8 }} /><Bar dataKey="rate" name="Items / Stunde" fill="#e97845" radius={[0, 6, 6, 0]} barSize={18} /></BarChart></ResponsiveContainer></div>;
}

function RecentRuns({ runs }: { runs: HistoryRunDTO[] }) {
  return <ul className="prototype-recent-runs">{runs.map((run) => <li key={run.run_id}><i className={run.outcome === "success" ? "success" : "failed"}>{run.outcome === "success" ? <Check size={13} /> : <CircleAlert size={13} />}</i><div><strong>{labelRun(run.run)}</strong><span>{new Date(run.started_at).toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" })} · {duration(run.duration_ms)}</span></div><small>{run.funnel.keep_return ? `${run.funnel.keep_return} gesichert` : run.reason_message || "Keine Items"}</small></li>)}</ul>;
}

function useDashboardData(character: string, difficulty: string): DashboardData {
  const [data, setData] = useState<DashboardData>(demoData);
  const query = useMemo(() => {
    const to = new Date();
    const from = new Date(to); from.setDate(from.getDate() - 30);
    return { from: from.toISOString(), to: to.toISOString(), timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC", character: [character], difficulty: [difficulty] };
  }, [character, difficulty]);
  useEffect(() => {
    const controller = new AbortController();
    void Promise.all([
      getHistorySummary(query, controller.signal),
      getHistoryComparisons({ ...query, sort: "keep_per_hour" }, controller.signal),
      getHistoryRuns({ ...query, limit: 5 }, controller.signal),
    ]).then(([summary, comparisons, runs]) => {
      if (controller.signal.aborted) return;
      setData(fromHistory(summary.summary, summary.daily_buckets, comparisons.comparisons, runs.runs));
    }).catch(() => { if (!controller.signal.aborted) setData(demoData); });
    return () => controller.abort();
  }, [query]);
  return data;
}

function fromHistory(summary: Awaited<ReturnType<typeof getHistorySummary>>["summary"], daily: Awaited<ReturnType<typeof getHistorySummary>>["daily_buckets"], comparisons: HistoryComparisonDTO[], recent: HistoryRunDTO[]): DashboardData {
  return {
    runs: summary.terminal_runs,
    successful: summary.successful,
    failed: summary.failed,
    aborted: summary.aborted,
    successRate: summary.success_rate ?? 0,
    averageDurationMs: summary.durations.average_ms,
    keep: summary.funnel.keep_return,
    keepPerHour: summary.keep_per_hour ?? 0,
    daily: daily.map((row) => ({ date: new Date(`${row.date}T00:00:00`).toLocaleDateString("de-DE", { day: "2-digit", month: "2-digit" }), runs: row.terminal_runs, success: row.success_rate ?? 0 })),
    comparisons: comparisons.slice(0, 5).map((row) => ({ name: labelRun(row.run), rate: row.keep_per_hour ?? 0 })),
    recent,
    source: "core",
  };
}

interface QueueRow { id: string; name: string; ready: boolean }
function queueRows(ids: string[], runs: RunCatalogEntry[]): QueueRow[] {
  return ids.map((id) => {
    const run = runs.find((entry) => entry.run_id === id);
    return { id, name: labelRun(run?.display_name || id), ready: !run || run.status === "runtime_validation_required" || run.status === "ready" };
  });
}

function updatePrototypeURL(variant: DashboardPrototypeVariant, mode: PrototypeMode, reload = true) {
  const url = new URL(window.location.href);
  url.searchParams.set("variant", variant);
  url.searchParams.set("state", mode);
  url.hash = url.hash || "dashboard";
  window.history.replaceState(null, "", url);
  if (reload) window.location.reload();
}

function prototypeModeFromURL(): PrototypeMode { return new URLSearchParams(window.location.search).get("state") === "active" ? "active" : "idle"; }
function labelDifficulty(value: string): string { return ({ normal: "Normal", nightmare: "Alptraum", hell: "Hölle" } as Record<string, string>)[value] ?? value; }
function labelRun(value: string): string { return ({ countess: "Gräfin", summoner: "Beschwörer", mephisto: "Mephisto", nihlathak: "Nihlathak", "lower-kurast": "Unter-Kurast", cows: "Kuhlevel", "cow-level": "Kuhlevel" } as Record<string, string>)[value.toLowerCase()] ?? value; }
function queueSize(rows: QueueRow[]): number { return rows.length || 3; }
function duration(milliseconds: number): string { const seconds = Math.round(milliseconds / 1000); return seconds >= 60 ? `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}` : `0:${String(seconds).padStart(2, "0")}`; }
function percent(value: number): string { return `${(value * 100).toLocaleString("de-DE", { maximumFractionDigits: 1 })} %`; }
function number(value: number): string { return value.toLocaleString("de-DE", { maximumFractionDigits: 1 }); }
function demoRun(run: string, outcome: string, durationMs: number, keep: number, startedAt: string, reasonMessage = ""): HistoryRunDTO {
  return { run_id: `${run}-${startedAt}`, started_at: startedAt, observed_at: startedAt, character: "MrHammer", difficulty: "hell", run, definition_id: run, route_id: `${run}-standard`, outcome, reason_message: reasonMessage, duration_ms: durationMs, boss_kills: outcome === "success" ? 1 : 0, funnel: { seen: keep + 4, matched: keep, picked_up: keep, stashed: keep, sold: 0, keep_return: keep, pickup_lost: 0, post_pickup_lost: 0 } };
}
