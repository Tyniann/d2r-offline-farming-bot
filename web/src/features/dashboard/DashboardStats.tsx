import { useEffect, useMemo, useState, type ReactNode } from "react";
import { BarChart3, Check, CircleAlert, Clock3, ExternalLink, Gauge, RotateCcw, Sparkles } from "lucide-react";
import { Bar, BarChart, Cell, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import {
  getHistoryComparisons, getHistoryRuns, getHistorySummary, type HistoryComparisonDTO,
  type HistoryQuery, type HistoryRunDTO, type HistorySummaryDTO,
} from "../../api/generated";
import { dashboardDifficultyName, dashboardRunName } from "./dashboardText";

export type DashboardPeriod = "7" | "30" | "all";

interface DashboardHistoryData {
  summary: HistorySummaryDTO;
  comparisons: HistoryComparisonDTO[];
  recent: HistoryRunDTO[];
}

interface Props {
  farming: ReactNode;
  character: string;
  difficulty: string;
  runNames: Record<string, string>;
}

/** DashboardStats loads the shared-period statistics and period-independent recent runs. */
export function DashboardStats({ farming, character, difficulty, runNames }: Props) {
  const [period, setPeriod] = useState<DashboardPeriod>("30");
  const [data, setData] = useState<DashboardHistoryData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  const reducedMotion = useReducedMotion();
  const queries = useMemo(() => dashboardHistoryQueries(period, character, difficulty), [period, character, difficulty]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    void Promise.all([
      getHistorySummary(queries.period, controller.signal),
      getHistoryComparisons({ ...queries.period, sort: "keep_per_hour" }, controller.signal),
      getHistoryRuns({ ...queries.recent, limit: 3 }, controller.signal),
    ]).then(([summary, comparisons, recent]) => {
      if (controller.signal.aborted) return;
      setData({
        summary: summary.summary,
        comparisons: [...comparisons.comparisons].sort((left, right) => (right.keep_per_hour ?? 0) - (left.keep_per_hour ?? 0)).slice(0, 5),
        recent: recent.runs.slice(0, 3),
      });
      setLoading(false);
    }).catch((reason: unknown) => {
      if (controller.signal.aborted) return;
      setError(reason instanceof Error ? reason.message : "Statistik konnte nicht geladen werden.");
      setLoading(false);
    });
    return () => controller.abort();
  }, [queries, retry]);

  const empty = !!data && data.summary.terminal_runs === 0;
  const loadingLabel = data ? "Statistik wird aktualisiert" : "Statistik wird geladen";

  return <>
    <div className="dashboard-period-row">
      <span>Statistik für {character || "den ausgewählten Charakter"}</span>
      <div className="dashboard-period-switch" aria-label="Zeitraum">
        {(["7", "30", "all"] as const).map((value) => <button key={value} type="button" aria-pressed={period === value} onClick={() => setPeriod(value)}>{value === "all" ? "Gesamt" : `${value} Tage`}</button>)}
      </div>
    </div>
    {error && <div className="dashboard-stats-error" role="alert"><CircleAlert aria-hidden="true" size={18} /><span>Statistik ist gerade nicht verfügbar. Farming bleibt möglich.</span><button type="button" onClick={() => setRetry((value) => value + 1)}>Erneut versuchen</button></div>}
    {empty && <div className="dashboard-stats-empty" role="status"><strong>Noch keine Runs für {character || "diesen Charakter"} auf {dashboardDifficultyName(difficulty)}.</strong><span>Nach dem ersten Run erscheinen hier Laufzeit, Ergebnisse und Ertrag.</span></div>}
    <div className={`dashboard-stats-shell ${loading ? "is-loading" : ""}`} aria-busy={loading}>
      {loading && <span className="dashboard-stats-loading" role="status">{loadingLabel}</span>}
      <div className="dashboard-main-grid">
        {farming}
        <section className="dashboard-panel dashboard-metrics-panel" aria-labelledby="dashboard-metrics-title">
          <DashboardPanelHeading eyebrow="Leistung" title="Auf einen Blick" id="dashboard-metrics-title" />
          <MetricGrid summary={data?.summary} empty={empty} />
        </section>
        <section className="dashboard-panel dashboard-outcomes-panel" aria-labelledby="dashboard-outcomes-title">
          <DashboardPanelHeading eyebrow="Ergebnisse" title="Verteilung" id="dashboard-outcomes-title" />
          <OutcomeRing summary={data?.summary} empty={empty} reducedMotion={reducedMotion} />
        </section>
      </div>
      <div className="dashboard-bottom-grid">
        <section className="dashboard-panel dashboard-routes-panel" aria-labelledby="dashboard-routes-title">
          <DashboardPanelHeading eyebrow="Routen" title="Welche Route lohnt sich?" id="dashboard-routes-title" action={<BarChart3 aria-hidden="true" size={18} />} />
          <RouteBars rows={data?.comparisons} runNames={runNames} empty={empty} reducedMotion={reducedMotion} />
        </section>
        <section className="dashboard-panel dashboard-recent-panel" aria-labelledby="dashboard-recent-title">
          <DashboardPanelHeading eyebrow="Zuletzt" title="Letzte Runs" id="dashboard-recent-title" action={<a href="#history" aria-label="Historie öffnen"><ExternalLink aria-hidden="true" size={17} /></a>} />
          <RecentRuns rows={data?.recent} runNames={runNames} />
        </section>
      </div>
    </div>
  </>;
}

function MetricGrid({ summary, empty }: { summary?: HistorySummaryDTO; empty: boolean }) {
  const keepPerHour = summary?.keep_per_hour ?? (summary && summary.durations.total_ms > 0
    ? summary.funnel.keep_return / (summary.durations.total_ms / 3_600_000)
    : undefined);
  return <div className="dashboard-metric-grid">
    <Metric icon={RotateCcw} label="Runs" value={summary ? String(summary.terminal_runs) : "–"} note={empty ? "Noch keine Runs" : "im Zeitraum"} />
    <Metric icon={Clock3} label="Ø Runzeit" value={summary ? duration(summary.durations.average_ms) : "–"} note="pro Route" />
    <Metric icon={Sparkles} label="Gesicherte Items" value={summary ? String(summary.funnel.keep_return) : "–"} note="im Zeitraum" />
    <Metric icon={Gauge} label="Items pro Stunde" value={rate(keepPerHour)} note="gesichert" />
  </div>;
}

function OutcomeRing({ summary, empty, reducedMotion }: { summary?: HistorySummaryDTO; empty: boolean; reducedMotion: boolean }) {
  const rows = [
    { name: "Erfolgreich", value: summary?.successful ?? 0, color: "#65d5a1" },
    { name: "Fehlgeschlagen", value: summary?.failed ?? 0, color: "#f07878" },
    { name: "Abgebrochen", value: summary?.aborted ?? 0, color: "#e5b65c" },
  ];
  const successRate = summary?.success_rate;
  return <>
    <div className="dashboard-outcome-ring" role="img" aria-label={summary ? `Ergebnisverteilung: ${rows.map((row) => `${row.name} ${row.value}`).join(", ")}` : "Ergebnisverteilung wird geladen"}>
      {!empty && summary && <ResponsiveContainer width="100%" height="100%"><PieChart accessibilityLayer={false}><Pie data={rows} dataKey="value" nameKey="name" innerRadius="68%" outerRadius="94%" paddingAngle={3} stroke="none" isAnimationActive={!reducedMotion} animationDuration={180} animationEasing="ease-out">{rows.map((row) => <Cell key={row.name} fill={row.color} />)}</Pie><Tooltip /></PieChart></ResponsiveContainer>}
      <span>{percent(successRate)}<small>Erfolg</small></span>
    </div>
    <div className="dashboard-legend">{rows.map((row) => <span key={row.name}><i style={{ background: row.color }} />{row.name} <strong>{summary ? row.value : "–"}</strong></span>)}</div>
  </>;
}

function RouteBars({ rows, runNames, empty, reducedMotion }: { rows?: HistoryComparisonDTO[]; runNames: Record<string, string>; empty: boolean; reducedMotion: boolean }) {
  if (!rows) return <div className="dashboard-chart-placeholder">Routenvergleich wird geladen</div>;
  if (empty || rows.length === 0) return <div className="dashboard-chart-placeholder">Noch keine vergleichbaren Routen</div>;
  const chartRows = rows.map((row) => ({ ...row, name: runNames[row.run] ?? labelRun(row.run), rate: row.keep_per_hour ?? 0 }));
  return <>
    <div className="dashboard-route-chart" role="img" aria-label={`Gesicherte Items pro Stunde: ${chartRows.map((row) => `${row.name} ${rate(row.rate)}`).join(", ")}`}><ResponsiveContainer width="100%" height="100%"><BarChart accessibilityLayer={false} data={chartRows} layout="vertical" margin={{ top: 4, right: 18, left: 8, bottom: 4 }}><XAxis type="number" hide /><YAxis type="category" dataKey="name" width={100} tick={{ fill: "#c7b9bf", fontSize: 12 }} axisLine={false} tickLine={false} /><Tooltip /><Bar dataKey="rate" name="Items pro Stunde" fill="#e97845" radius={[0, 6, 6, 0]} barSize={18} isAnimationActive={!reducedMotion} animationDuration={180} animationEasing="ease-out" /></BarChart></ResponsiveContainer></div>
    <ul className="dashboard-route-values">{chartRows.map((row) => <li key={row.id}><span>{row.name}</span><strong>{rate(row.rate)}</strong></li>)}</ul>
  </>;
}

function RecentRuns({ rows, runNames }: { rows?: HistoryRunDTO[]; runNames: Record<string, string> }) {
  if (!rows) return <div className="dashboard-recent-placeholder">Letzte Runs werden geladen</div>;
  if (rows.length === 0) return <div className="dashboard-recent-placeholder">Noch keine Runs vorhanden</div>;
  return <ul className="dashboard-recent-runs">{rows.map((run) => {
    const success = run.outcome === "success";
    return <li key={run.run_id}><i className={success ? "success" : "failed"}>{success ? <Check aria-hidden="true" size={13} /> : <CircleAlert aria-hidden="true" size={13} />}</i><div><strong>{runNames[run.run] ?? labelRun(run.run)}</strong><span>{new Date(run.started_at).toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" })} · {duration(run.duration_ms)}</span></div><small>{run.funnel.keep_return ? `${run.funnel.keep_return} gesichert` : run.reason_message || outcomeLabel(run.outcome)}</small></li>;
  })}</ul>;
}

function DashboardPanelHeading({ eyebrow, title, id, action }: { eyebrow: string; title: string; id: string; action?: ReactNode }) {
  return <div className="dashboard-panel-heading"><div><span>{eyebrow}</span><h2 id={id}>{title}</h2></div>{action}</div>;
}

function Metric({ icon: Icon, label, value, note }: { icon: typeof RotateCcw; label: string; value: string; note: string }) {
  return <div className="dashboard-metric"><Icon aria-hidden="true" size={18} /><span>{label}</span><strong>{value}</strong><small>{note}</small></div>;
}

export function dashboardHistoryQueries(period: DashboardPeriod, character: string, difficulty: string, now = new Date()): { period: HistoryQuery; recent: HistoryQuery } {
  const common: HistoryQuery = {
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
    character: character ? [character] : undefined,
    difficulty: difficulty ? [difficulty] : undefined,
  };
  if (period === "all") return { period: common, recent: common };
  const from = new Date(now);
  from.setDate(from.getDate() - Number(period));
  return { period: { ...common, from: from.toISOString(), to: now.toISOString() }, recent: common };
}

function labelRun(value: string): string {
  return dashboardRunName(value, value);
}
function outcomeLabel(value: string): string { return ({ success: "Erfolgreich", failed: "Fehlgeschlagen", aborted: "Abgebrochen" } as Record<string, string>)[value] ?? "Keine Items"; }
function duration(milliseconds: number): string { const seconds = Math.round(milliseconds / 1000); return seconds >= 60 ? `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}` : `0:${String(seconds).padStart(2, "0")}`; }
function percent(value?: number): string { return value === undefined ? "–" : `${(value * 100).toLocaleString("de-DE", { maximumFractionDigits: 1 })} %`; }
function rate(value?: number): string { return value === undefined || !Number.isFinite(value) ? "–" : value.toLocaleString("de-DE", { maximumFractionDigits: 1 }); }

function useReducedMotion(): boolean {
  const [reduced, setReduced] = useState(() => typeof window.matchMedia === "function" && window.matchMedia("(prefers-reduced-motion: reduce)").matches);
  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setReduced(media.matches);
    media.addEventListener?.("change", update);
    return () => media.removeEventListener?.("change", update);
  }, []);
  return reduced;
}
