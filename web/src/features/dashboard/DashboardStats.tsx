import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { BarChart3, Check, CircleAlert, Clock3, ExternalLink, Gauge, RotateCcw, Sparkles } from "lucide-react";
import { Bar, BarChart, Cell, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import {
  getHistoryComparisons, getHistoryRuns, getHistorySummary, type HistoryComparisonDTO,
  type HistoryQuery, type HistoryRunDTO, type HistorySummaryDTO,
} from "../../api/generated";
import { dashboardDifficultyName, dashboardRunName } from "./dashboardText";
import { formatDate, formatDuration, formatNumber, formatPercent } from "../../i18n/format";
import { presentApiError, presentHistoryReason, type AppTranslator } from "../../i18n/presenters";

export type DashboardPeriod = "7" | "30" | "all";

const dashboardTooltipStyle = {
  backgroundColor: "#181419",
  border: "1px solid #5b464d",
  borderRadius: "8px",
  boxShadow: "0 10px 28px #0009",
  color: "#f3e7ec",
};
const dashboardTooltipLabelStyle = { color: "#f3e7ec", fontWeight: 700 };
const dashboardTooltipItemStyle = { color: "#ee9a74" };

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
  const { t } = useTranslation();
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
      setError(presentApiError(reason, t, t("dashboard.stats.loadFailed")));
      setLoading(false);
    });
    return () => controller.abort();
  }, [queries, retry, t]);

  const empty = !!data && data.summary.terminal_runs === 0;
  const loadingLabel = data ? t("dashboard.stats.updating") : t("dashboard.stats.loading");

  return <>
    <div className="dashboard-period-row">
      <span>{t("dashboard.stats.forCharacter", { character: character || t("dashboard.stats.selectedCharacter") })}</span>
      <div className="dashboard-period-switch" aria-label={t("dashboard.stats.period")}>
        {(["7", "30", "all"] as const).map((value) => <button key={value} type="button" aria-pressed={period === value} onClick={() => setPeriod(value)}>{value === "all" ? t("dashboard.stats.all") : t("dashboard.stats.days", { count: Number(value) })}</button>)}
      </div>
    </div>
    {error && <div className="dashboard-stats-error" role="alert"><CircleAlert aria-hidden="true" size={18} /><span>{t("dashboard.stats.unavailable")}</span><button type="button" onClick={() => setRetry((value) => value + 1)}>{t("dashboard.stats.retry")}</button></div>}
    {empty && <div className="dashboard-stats-empty" role="status"><strong>{t("dashboard.stats.empty", { character: character || t("dashboard.stats.thisCharacter"), difficulty: dashboardDifficultyName(difficulty, t) })}</strong><span>{t("dashboard.stats.emptyDetail")}</span></div>}
    <div className={`dashboard-stats-shell ${loading ? "is-loading" : ""}`} aria-busy={loading}>
      {loading && <span className="dashboard-stats-loading" role="status">{loadingLabel}</span>}
      <div className="dashboard-main-grid">
        {farming}
        <section className="dashboard-panel dashboard-metrics-panel" aria-labelledby="dashboard-metrics-title">
          <DashboardPanelHeading eyebrow={t("dashboard.stats.performance")} title={t("dashboard.stats.overview")} id="dashboard-metrics-title" />
          <MetricGrid summary={data?.summary} empty={empty} />
        </section>
        <section className="dashboard-panel dashboard-outcomes-panel" aria-labelledby="dashboard-outcomes-title">
          <DashboardPanelHeading eyebrow={t("dashboard.stats.results")} title={t("dashboard.stats.distribution")} id="dashboard-outcomes-title" />
          <OutcomeRing summary={data?.summary} empty={empty} reducedMotion={reducedMotion} />
        </section>
      </div>
      <div className="dashboard-bottom-grid">
        <section className="dashboard-panel dashboard-routes-panel" aria-labelledby="dashboard-routes-title">
          <DashboardPanelHeading eyebrow={t("dashboard.stats.routes")} title={t("dashboard.stats.worthwhileRoute")} id="dashboard-routes-title" action={<BarChart3 aria-hidden="true" size={18} />} />
          <RouteBars rows={data?.comparisons} runNames={runNames} empty={empty} reducedMotion={reducedMotion} />
        </section>
        <section className="dashboard-panel dashboard-recent-panel" aria-labelledby="dashboard-recent-title">
          <DashboardPanelHeading eyebrow={t("dashboard.stats.recent")} title={t("dashboard.stats.recentRuns")} id="dashboard-recent-title" action={<a href="#history" aria-label={t("dashboard.stats.openHistory")}><ExternalLink aria-hidden="true" size={17} /></a>} />
          <RecentRuns rows={data?.recent} runNames={runNames} />
        </section>
      </div>
    </div>
  </>;
}

function MetricGrid({ summary, empty }: { summary?: HistorySummaryDTO; empty: boolean }) {
  const { t } = useTranslation();
  const keepPerHour = summary?.keep_per_hour ?? (summary && summary.durations.total_ms > 0
    ? summary.funnel.keep_return / (summary.durations.total_ms / 3_600_000)
    : undefined);
  return <div className="dashboard-metric-grid">
    <Metric icon={RotateCcw} label={t("dashboard.stats.runs")} value={summary ? formatNumber(summary.terminal_runs) : "–"} note={empty ? t("dashboard.stats.noRuns") : t("dashboard.stats.inPeriod")} />
    <Metric icon={Clock3} label={t("dashboard.stats.averageDuration")} value={summary ? formatDuration(summary.durations.average_ms) : "–"} note={t("dashboard.stats.perRoute")} />
    <Metric icon={Sparkles} label={t("dashboard.stats.securedItems")} value={summary ? formatNumber(summary.funnel.keep_return) : "–"} note={t("dashboard.stats.inPeriod")} />
    <Metric icon={Gauge} label={t("dashboard.stats.itemsPerHour")} value={rate(keepPerHour)} note={t("dashboard.stats.secured")} />
  </div>;
}

function OutcomeRing({ summary, empty, reducedMotion }: { summary?: HistorySummaryDTO; empty: boolean; reducedMotion: boolean }) {
  const { t } = useTranslation();
  const rows = [
    { name: t("dashboard.stats.successful"), value: summary?.successful ?? 0, color: "#65d5a1" },
    { name: t("dashboard.stats.failed"), value: summary?.failed ?? 0, color: "#f07878" },
    { name: t("dashboard.stats.aborted"), value: summary?.aborted ?? 0, color: "#e5b65c" },
  ];
  const successRate = summary?.success_rate;
  return <>
    <div className="dashboard-outcome-ring" role="img" aria-label={summary ? t("dashboard.stats.distributionAria", { values: rows.map((row) => `${row.name} ${row.value}`).join(", ") }) : t("dashboard.stats.distributionLoading")}>
      {!empty && summary && <ResponsiveContainer width="100%" height="100%"><PieChart accessibilityLayer={false}><Pie data={rows} dataKey="value" nameKey="name" innerRadius="68%" outerRadius="94%" paddingAngle={3} stroke="none" isAnimationActive={!reducedMotion} animationDuration={180} animationEasing="ease-out">{rows.map((row) => <Cell key={row.name} fill={row.color} />)}</Pie><Tooltip contentStyle={dashboardTooltipStyle} labelStyle={dashboardTooltipLabelStyle} itemStyle={dashboardTooltipItemStyle} /></PieChart></ResponsiveContainer>}
      <span>{successRate === undefined ? "–" : formatPercent(successRate)}<small>{t("dashboard.stats.success")}</small></span>
    </div>
    <div className="dashboard-legend">{rows.map((row) => <span key={row.name}><i style={{ background: row.color }} />{row.name} <strong>{summary ? row.value : "–"}</strong></span>)}</div>
  </>;
}

function RouteBars({ rows, runNames, empty, reducedMotion }: { rows?: HistoryComparisonDTO[]; runNames: Record<string, string>; empty: boolean; reducedMotion: boolean }) {
  const { t } = useTranslation();
  if (!rows) return <div className="dashboard-chart-placeholder">{t("dashboard.stats.routesLoading")}</div>;
  if (empty || rows.length === 0) return <div className="dashboard-chart-placeholder">{t("dashboard.stats.routesEmpty")}</div>;
  const chartRows = rows.map((row) => ({ ...row, name: runNames[row.run] ?? labelRun(row.run, t), rate: row.keep_per_hour ?? 0 }));
  return <div className="dashboard-route-chart" role="img" aria-label={t("dashboard.stats.itemsPerHourAria", { values: chartRows.map((row) => `${row.name} ${rate(row.rate)}`).join(", ") })}><ResponsiveContainer width="100%" height="100%"><BarChart accessibilityLayer={false} data={chartRows} layout="vertical" margin={{ top: 4, right: 18, left: 8, bottom: 4 }}><XAxis type="number" hide /><YAxis type="category" dataKey="name" width={100} tick={{ fill: "#c7b9bf", fontSize: 12 }} axisLine={false} tickLine={false} /><Tooltip formatter={(value) => rate(Number(value))} contentStyle={dashboardTooltipStyle} labelStyle={dashboardTooltipLabelStyle} itemStyle={dashboardTooltipItemStyle} cursor={{ fill: "#2b2327" }} /><Bar dataKey="rate" name={t("dashboard.stats.itemsPerHour")} fill="#e97845" radius={[0, 6, 6, 0]} barSize={18} isAnimationActive={!reducedMotion} animationDuration={180} animationEasing="ease-out" /></BarChart></ResponsiveContainer></div>;
}

function RecentRuns({ rows, runNames }: { rows?: HistoryRunDTO[]; runNames: Record<string, string> }) {
  const { t } = useTranslation();
  if (!rows) return <div className="dashboard-recent-placeholder">{t("dashboard.stats.recentLoading")}</div>;
  if (rows.length === 0) return <div className="dashboard-recent-placeholder">{t("dashboard.stats.recentEmpty")}</div>;
  return <ul className="dashboard-recent-runs">{rows.map((run) => {
    const success = run.outcome === "success";
    const statusText = run.funnel.keep_return
      ? t("dashboard.stats.securedCount", { count: run.funnel.keep_return })
      : outcomeLabel(run.outcome, t);
    const reasonText = !run.funnel.keep_return && run.reason ? presentHistoryReason(run.reason, t) : undefined;
    return <li key={run.run_id}>
      <i className={success ? "success" : "failed"}>{success ? <Check aria-hidden="true" size={13} /> : <CircleAlert aria-hidden="true" size={13} />}</i>
      <div>
        <strong>{runNames[run.run] ?? labelRun(run.run, t)}</strong>
        <span>{formatDate(run.started_at, { hour: "2-digit", minute: "2-digit" })} · {formatDuration(run.duration_ms)}</span>
        {reasonText && <small className="dashboard-recent-reason">{reasonText}</small>}
      </div>
      <small className="dashboard-recent-status">{statusText}</small>
    </li>;
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

function labelRun(value: string, t: AppTranslator): string {
  return dashboardRunName(value, t);
}
function outcomeLabel(value: string, t: AppTranslator): string {
  const keys = { success: "dashboard.stats.successful", failed: "dashboard.stats.failed", aborted: "dashboard.stats.aborted" } as const;
  return t(keys[value as keyof typeof keys] ?? "dashboard.stats.noItems");
}
function rate(value?: number): string { return value === undefined || !Number.isFinite(value) ? "–" : formatNumber(value, { maximumFractionDigits: 1 }); }

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
