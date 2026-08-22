import { useEffect, useRef, useState } from "react";
import {
  Bar, BarChart, CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis,
} from "recharts";
import {
  downloadHistoryExport, getHistoryComparisons, getHistoryItems, getHistoryRun, getHistoryRuns, getHistorySummary,
  type HistoryComparisonDTO, type HistoryItemsResponse, type HistoryQuery, type HistoryRunDetailResponse,
  type HistoryRunsResponse, type HistorySummaryResponse,
} from "../../api/generated";
import { useTranslation } from "react-i18next";
import { formatDate, formatDuration, formatNumber, formatPercent } from "../../i18n/format";
import { presentApiError, presentDifficultyName, presentHistoryReason, presentRunName, type AppTranslator } from "../../i18n/presenters";
import { gameHistoryItemName } from "../../i18n/game";

interface Props { characters: string[]; selectedCharacter: string; selectedDifficulty: string; onSelectedCharacterChange?(character: string): void; onSelectedDifficultyChange?(difficulty: string): void; runs: string[]; refreshKey: number }
interface FilterDraft { period: string; from: string; to: string; run: string; outcome: string; reason: string; pickitProfile: string }

const emptyFilter: FilterDraft = { period: "30d", from: "", to: "", run: "", outcome: "", reason: "", pickitProfile: "" };
const outcomeKeys = ["success", "failed", "aborted", "incomplete", "running"] as const;

export function HistoryFeature({ characters, selectedCharacter, selectedDifficulty, onSelectedCharacterChange, onSelectedDifficultyChange, runs, refreshKey }: Props) {
  const { t, i18n } = useTranslation();
  const [draft, setDraft] = useState<FilterDraft>(emptyFilter);
  const [query, setQuery] = useState<HistoryQuery>(() => filterQuery(emptyFilter, selectedCharacter, selectedDifficulty));
  const [summary, setSummary] = useState<HistorySummaryResponse | null>(null);
  const [comparisons, setComparisons] = useState<HistoryComparisonDTO[]>([]);
  const [items, setItems] = useState<HistoryItemsResponse | null>(null);
  const [runList, setRunList] = useState<HistoryRunsResponse | null>(null);
  const [detail, setDetail] = useState<HistoryRunDetailResponse | null>(null);
  const [comparisonSort, setComparisonSort] = useState("keep_per_hour");
  const [dailyMetric, setDailyMetric] = useState<"terminal_runs" | "success_rate" | "keep_per_hour">("terminal_runs");
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState("");
  const [exportError, setExportError] = useState("");
  const refreshSerial = useRef(Promise.resolve());

  const load = (nextQuery: HistoryQuery, signal?: AbortSignal) => {
    refreshSerial.current = refreshSerial.current.catch(() => undefined).then(async () => {
      setLoading(true);
      try {
        const [nextSummary, nextComparisons, nextItems, nextRuns] = await Promise.all([
          getHistorySummary(nextQuery, signal), getHistoryComparisons({ ...nextQuery, sort: comparisonSort as HistoryQuery["sort"] }, signal),
          getHistoryItems({ ...nextQuery, limit: 10 }, signal), getHistoryRuns({ ...nextQuery, limit: 10 }, signal),
        ]);
        if (signal?.aborted) return;
        setSummary(nextSummary); setComparisons(nextComparisons.comparisons); setItems(nextItems); setRunList(nextRuns);
        setDetail(null); setError("");
      } catch (reason: unknown) {
        if (!signal?.aborted) setError(presentApiError(reason, t, t("history.loadFailed")));
      } finally {
        if (!signal?.aborted) setLoading(false);
      }
    });
    return refreshSerial.current;
  };

  useEffect(() => {
    const controller = new AbortController();
    void load(query, controller.signal);
    return () => controller.abort();
  }, [query, refreshKey, comparisonSort, t]); // `history_changed`, Filter und Core-Sortierung laden serialisiert neu.

  useEffect(() => {
    setQuery((current) => {
      const character = selectedCharacter ? [selectedCharacter] : undefined;
      const difficulty = selectedDifficulty ? [selectedDifficulty] : undefined;
      if (current.character?.[0] === character?.[0] && current.difficulty?.[0] === difficulty?.[0]) return current;
      return { ...current, character, difficulty };
    });
  }, [selectedCharacter, selectedDifficulty]);

  const applyFilters = () => setQuery(filterQuery(draft, selectedCharacter, selectedDifficulty));
  const resetFilters = () => { setDraft(emptyFilter); setQuery(filterQuery(emptyFilter, selectedCharacter, selectedDifficulty)); };

  const loadMore = async (kind: "runs" | "items") => {
    const cursor = kind === "runs" ? runList?.next_cursor : items?.next_cursor;
    if (!cursor || loadingMore) return;
    setLoadingMore(true); setError("");
    try {
      if (kind === "runs") {
        const page = await getHistoryRuns({ ...query, limit: 10, cursor });
        setRunList((current) => current ? { ...page, runs: [...current.runs, ...page.runs] } : page);
      } else {
        const page = await getHistoryItems({ ...query, limit: 10, cursor });
        setItems((current) => current ? { ...page, items: [...current.items, ...page.items] } : page);
      }
    } catch (reason: unknown) {
      setError(presentApiError(reason, t, t("history.moreFailed")));
    } finally {
      setLoadingMore(false);
    }
  };

  const openDetail = async (runID: string, includeRaw = false) => {
    try { setDetail(await getHistoryRun(runID, includeRaw)); setError(""); }
    catch (reason: unknown) { setError(presentApiError(reason, t, t("history.detailFailed"))); }
  };

  const exportData = async (format: "json" | "csv", dataset: "" | "runs" | "items" = "") => {
    setExporting(true); setExportError("");
    try {
      const download = await downloadHistoryExport(format, dataset, query);
      const href = URL.createObjectURL(download.blob);
      const anchor = document.createElement("a");
      anchor.href = href; anchor.download = download.filename; anchor.click();
      URL.revokeObjectURL(href);
    } catch (reason: unknown) {
      setExportError(presentApiError(reason, t, t("history.exportFailed")));
    } finally { setExporting(false); }
  };

  const filtered = draft.period !== "30d" || [query.run, query.outcome, query.reason, query.pickit_profile].some((value) => value?.length);
  const noRuns = !loading && !error && summary?.summary.runs === 0;

  return <section id="history" aria-labelledby="history-title" className="history-feature">
    <div className="section-heading"><div><p className="eyebrow">{t("history.eyebrow")}</p><h2 id="history-title">{t("history.title")}</h2></div><button type="button" className="secondary" onClick={() => void load(query)} disabled={loading}>{t("history.refresh")}</button></div>
    <p>{t("history.description")}</p>

    <form className="history-filters" onSubmit={(event) => { event.preventDefault(); applyFilters(); }}>
      <label>{t("history.period")}<select value={draft.period} onChange={(event) => setDraft({ ...draft, period: event.target.value })}><option value="all">{t("history.allHistory")}</option><option value="today">{t("history.today")}</option><option value="7d">{t("history.last7Days")}</option><option value="30d">{t("history.last30Days")}</option><option value="custom">{t("history.customPeriod")}</option></select></label>
      <label>{t("history.from")}<input type="datetime-local" value={draft.from} onChange={(event) => setDraft({ ...draft, from: event.target.value })} /></label>
      <label>{t("history.to")}<input type="datetime-local" value={draft.to} onChange={(event) => setDraft({ ...draft, to: event.target.value })} /></label>
      <label>{t("history.character")}<select value={selectedCharacter} onChange={(event) => onSelectedCharacterChange?.(event.target.value)}>{characters.map((value) => <option key={value}>{value}</option>)}</select></label>
      <label>{t("history.run")}<select value={draft.run} onChange={(event) => setDraft({ ...draft, run: event.target.value })}><option value="">{t("history.all")}</option>{runs.map((value) => <option key={value}>{presentRunName(value, t)}</option>)}</select></label>
      <label>{t("history.difficulty")}<select value={selectedDifficulty} onChange={(event) => onSelectedDifficultyChange?.(event.target.value)}>{["normal", "nightmare", "hell"].map((id) => <option key={id} value={id}>{presentDifficultyName(id, t)}</option>)}</select></label>
      <label>{t("history.result")}<select value={draft.outcome} onChange={(event) => setDraft({ ...draft, outcome: event.target.value })}><option value="">{t("history.all")}</option>{outcomeKeys.map((value) => <option key={value} value={value}>{outcomeLabel(value, t)}</option>)}</select></label>
      <label>{t("history.reasonCode")}<input value={draft.reason} placeholder={t("history.reasonPlaceholder")} onChange={(event) => setDraft({ ...draft, reason: event.target.value })} /></label>
      <label>{t("history.pickitProfile")}<input value={draft.pickitProfile} placeholder={t("history.optional")} onChange={(event) => setDraft({ ...draft, pickitProfile: event.target.value })} /></label>
      <div className="inline-actions"><button type="submit">{t("history.applyFilters")}</button><button type="button" className="secondary" onClick={resetFilters}>{t("history.reset")}</button></div>
    </form>
    <p className="active-filters" aria-live="polite"><strong>{t("history.activeFilters")}</strong> {activeFilterText(query, t)}</p>

    {error && <p role="alert">{error}</p>}
    {loading && <p role="status">{t("history.loading")}</p>}
    {noRuns && <div className="history-empty"><h3>{t(filtered ? "history.noMatches" : "history.noHistory")}</h3><p>{t(filtered ? "history.noMatchesDetail" : "history.noHistoryDetail")}</p></div>}
    {!loading && summary && summary.summary.runs > 0 && <>
      <div className="cards history-summary">
        <article><span>{t("history.terminalRuns")}</span><strong>{formatNumber(summary.summary.terminal_runs)}</strong><small>{t("history.runResults", { successful: formatNumber(summary.summary.successful), failed: formatNumber(summary.summary.failed), aborted: formatNumber(summary.summary.aborted) })}</small></article>
        <article><span>{t("history.successRate")}</span><strong>{percent(summary.summary.success_rate)}</strong><small>{t("history.bossKills", { count: formatNumber(summary.summary.boss_kills) })}</small></article>
        <article><span>{t("history.averageDuration")}</span><strong>{duration(summary.summary.durations.average_ms)}</strong><small>{t("history.median", { duration: duration(summary.summary.durations.median_ms) })}</small></article>
        <article><span>{t("history.securedKeep")}</span><strong>{formatNumber(summary.summary.funnel.keep_return)}</strong><small>{t("history.soldCount", { count: formatNumber(summary.summary.funnel.sold) })}</small></article>
        <article><span>{t("history.keepPerHour")}</span><strong>{rate(summary.summary.keep_per_hour)}</strong><small>{t("history.perRunKill", { runRate: rate(summary.summary.keep_per_run), killRate: rate(summary.summary.keep_per_kill) })}</small></article>
        <article><span>{t("history.lootLoss")}</span><strong>{formatNumber(summary.summary.funnel.pickup_lost)} / {formatNumber(summary.summary.funnel.post_pickup_lost)}</strong><small>{t("history.coreCounters")}</small></article>
      </div>
      {(summary.summary.running > 0 || summary.summary.incomplete > 0) && <p role="status">{t("history.notAggregated", { running: formatNumber(summary.summary.running), incomplete: formatNumber(summary.summary.incomplete) })}</p>}
      {summary.summary.top_failure && <article className="history-priority"><span>{t("history.topFailure")}</span><strong>{presentHistoryReason(summary.summary.top_failure.reason, t)}</strong><small>{t("history.failureDetail", { count: formatNumber(summary.summary.top_failure.count), step: summary.summary.top_failure.step || t("history.unknownStep"), duration: duration(summary.summary.top_failure.lost_duration_ms) })}</small></article>}
      {summary.meta.diagnostics.length > 0 && <aside className="history-diagnostics" aria-labelledby="history-diagnostics-title"><h3 id="history-diagnostics-title">{t("history.fileDiagnostics")}</h3><p>{t("history.diagnosticsDetail", { diagnostics: formatNumber(summary.meta.diagnostics.length), ignored: formatNumber(summary.meta.ignored_files) })}</p><ul>{summary.meta.diagnostics.map((item) => <li key={`${item.file}-${item.code}`}><strong>{item.file}</strong><span>{presentHistoryReason(item.code, t)}</span></li>)}</ul></aside>}

      <HistoryCharts summary={summary} comparisons={comparisons} dailyMetric={dailyMetric} onDailyMetric={setDailyMetric} />

      <div className="section-heading"><h3>{t("history.comparison")}</h3><label>{t("history.sorting")}<select value={comparisonSort} onChange={(event) => setComparisonSort(event.target.value)}><option value="keep_per_hour">{t("history.keepPerHour")}</option><option value="success_rate">{t("history.successRate")}</option><option value="average_duration">{t("history.average")}</option></select></label></div>
      <div className="table-scroll"><table className="comparison-table"><caption>{t("history.comparisonCaption")}</caption><thead><tr><th>{t("history.route")}</th><th>{t("history.sample")}</th><th>{t("history.keepPerHour")}</th><th>{t("history.keepPerRun")}</th><th>{t("history.keepPerKill")}</th><th>{t("history.sell")}</th><th>{t("history.success")}</th><th>{t("history.average")}</th><th>{t("history.stagesShort")}</th><th>{t("history.errors")}</th></tr></thead><tbody>{comparisons.map((row) => <tr key={row.id}><th scope="row" data-label={t("history.route")}>{presentRunName(row.run, t)}<small>{row.character} · {presentDifficultyName(row.difficulty, t)} · {row.route_id}</small></th><td data-label={t("history.sample")}>{formatNumber(row.run === "lower-kurast" ? row.terminal_runs : row.boss_kills)}{row.low_sample && <span className="sample-warning">{t(row.run === "lower-kurast" ? "history.smallRuns" : "history.smallKills")}</span>}</td><td data-label={t("history.keepPerHour")}>{rate(row.keep_per_hour)}</td><td data-label={t("history.keepPerRun")}>{rate(row.keep_per_run)}</td><td data-label={t("history.keepPerKill")}>{rate(row.keep_per_kill)}</td><td data-label={t("history.sold")}>{formatNumber(row.funnel.sold)}</td><td data-label={t("history.success")}>{percent(row.success_rate)}</td><td data-label={t("history.average")}>{duration(row.durations.average_ms)}</td><td data-label={t("history.stages")}>{duration(row.stages.travel_ms)} / {duration(row.stages.combat_ms)} / {duration(row.stages.loot_ms)} / {duration(row.stages.return_town_ms)}</td><td data-label={t("history.errors")}>{row.top_failure ? presentHistoryReason(row.top_failure.reason, t) : "–"}</td></tr>)}</tbody></table></div>

      <h3>{t("history.itemYield")}</h3>
      <div className="table-scroll"><table className="item-table"><caption>{t("history.itemsCaption")}</caption><thead><tr><th>{t("history.item")}</th><th>{t("history.seen")}</th><th>{t("history.matched")}</th><th>{t("history.pickedUp")}</th><th>{t("history.keep")}</th><th>{t("history.sold")}</th><th>{t("history.pickupLost")}</th><th>{t("history.postPickupLost")}</th><th>{t("history.yieldPerRun")}</th><th>{t("history.yieldPerKill")}</th><th>{t("history.yieldPerHour")}</th></tr></thead><tbody>{items?.items.map((item) => <tr key={item.item_key}><th scope="row" data-label={t("history.item")}>{gameHistoryItemName(item, i18n.resolvedLanguage)}<small>{item.item_key}</small></th><td data-label={t("history.seen")}>{formatNumber(item.seen)}</td><td data-label={t("history.matched")}>{formatNumber(item.matched)}</td><td data-label={t("history.pickedUp")}>{formatNumber(item.picked_up)}</td><td data-label={t("history.keep")}>{formatNumber(item.stashed)}</td><td data-label={t("history.sold")}>{formatNumber(item.sold)}</td><td data-label={t("history.pickupLost")}>{formatNumber(item.pickup_lost)}</td><td data-label={t("history.postPickupLost")}>{formatNumber(item.post_pickup_lost)}</td><td data-label={t("history.yieldPerRun")}>{rate(item.yield_per_run)}</td><td data-label={t("history.yieldPerKill")}>{rate(item.yield_per_kill)}</td><td data-label={t("history.yieldPerHour")}>{rate(item.yield_per_hour)}</td></tr>)}</tbody></table></div>
      {items?.next_cursor && <button type="button" className="secondary" disabled={loadingMore} onClick={() => void loadMore("items")}>{t("history.loadMoreItems")}</button>}

      <h3>{t("history.runs")}</h3>
      <div className="table-scroll"><table className="run-table"><caption>{t("history.runsCaption")}</caption><thead><tr><th>{t("history.localStart")}</th><th>{t("history.runRoute")}</th><th>{t("history.result")}</th><th>{t("history.duration")}</th><th>{t("history.keep")}</th><th>{t("history.sell")}</th><th>{t("history.loss")}</th><th>{t("history.action")}</th></tr></thead><tbody>{runList?.runs.map((row) => <tr key={row.run_id}><td data-label={t("history.localStart")}>{formatDate(row.started_at, { dateStyle: "short", timeStyle: "short" })}</td><th scope="row" data-label={t("history.runRoute")}>{presentRunName(row.run, t)}<small>{row.route_id}</small></th><td data-label={t("history.result")}><span className={`outcome outcome-${row.outcome}`}>{outcomeLabel(row.outcome, t)}</span>{row.reason && <small>{presentHistoryReason(row.reason, t)}</small>}</td><td data-label={t("history.duration")}>{duration(row.duration_ms)}</td><td data-label={t("history.keep")}>{formatNumber(row.funnel.keep_return)}</td><td data-label={t("history.sell")}>{formatNumber(row.funnel.sold)}</td><td data-label={t("history.loss")}>{formatNumber(row.funnel.pickup_lost)} / {formatNumber(row.funnel.post_pickup_lost)}</td><td data-label={t("history.action")}><button type="button" className="secondary" onClick={() => void openDetail(row.run_id)}>{t("history.openRun")}</button></td></tr>)}</tbody></table></div>
      {runList?.next_cursor && <button type="button" className="secondary" disabled={loadingMore} onClick={() => void loadMore("runs")}>{t("history.loadMoreRuns")}</button>}

      {detail && <article className="history-detail" aria-labelledby="history-detail-title"><div className="section-heading"><h3 id="history-detail-title">{t("history.run")} {detail.run.run_id}</h3><button type="button" className="secondary" onClick={() => setDetail(null)}>{t("history.close")}</button></div><dl><div><dt>{t("history.context")}</dt><dd>{detail.run.character} · {presentDifficultyName(detail.run.difficulty, t)} · {presentRunName(detail.run.run, t)}</dd></div><div><dt>{t("history.route")}</dt><dd>{detail.run.route_id}</dd></div><div><dt>{t("history.result")}</dt><dd>{outcomeLabel(detail.run.outcome, t)}{detail.run.reason ? ` · ${presentHistoryReason(detail.run.reason, t)}` : ""}</dd></div>{detail.run.reason && <div><dt>{t("history.errorLocation")}</dt><dd><code>{detail.run.reason}</code>{detail.run.last_step ? ` · ${t("history.step")} ${detail.run.last_step}` : ""}</dd></div>}<div><dt>{t("history.boss")}</dt><dd>{detail.run.run === "lower-kurast" ? t("history.noBoss") : t("history.confirmedKills", { count: formatNumber(detail.run.boss_kills) })}</dd></div><div><dt>{t("history.funnel")}</dt><dd>{t("history.funnelDetail", { seen: formatNumber(detail.run.funnel.seen), matched: formatNumber(detail.run.funnel.matched), picked: formatNumber(detail.run.funnel.picked_up), keep: formatNumber(detail.run.funnel.keep_return), sold: formatNumber(detail.run.funnel.sold) })}</dd></div><div><dt>{t("history.stages")}</dt><dd>{t("history.stageDetail", { travel: duration(detail.run.stages.travel_ms), combat: duration(detail.run.stages.combat_ms), loot: duration(detail.run.stages.loot_ms), town: duration(detail.run.stages.return_town_ms), other: duration(detail.run.stages.other_ms) })}</dd></div></dl><h4>{t("history.itemPaths")}</h4>{detail.run.items.length === 0 ? <p>{t("history.noItemEvents")}</p> : <ul>{detail.run.items.map((item) => <li key={item.unit_id}><strong>{gameHistoryItemName(item, i18n.resolvedLanguage)}</strong><span>{t(item.stashed ? "history.secured" : item.sold ? "history.confirmedSold" : item.pickup_lost ? "history.lostBeforePickup" : item.post_pickup_lost ? "history.lostAfterPickup" : "history.observed")}</span>{item.pickit_profile_id && <small>{t("history.pickitTrace", { profile: item.pickit_profile_id, revision: item.pickit_profile_revision, rule: item.pickit_rule_id || "–", action: item.pickit_action || "–", assignment: item.pickit_assignment_revision })}</small>}</li>)}</ul>}<RawEventDetails detail={detail} onLoad={() => void openDetail(detail.run.run_id, true)} /></article>}

      <div className="history-export" aria-labelledby="history-export-title"><h3 id="history-export-title">{t("history.export")}</h3><div className="inline-actions"><button type="button" disabled={exporting} onClick={() => void exportData("json")}>{t("history.jsonReport")}</button><button type="button" disabled={exporting} onClick={() => void exportData("csv", "runs")}>{t("history.runCsv")}</button><button type="button" disabled={exporting} onClick={() => void exportData("csv", "items")}>{t("history.itemCsv")}</button></div>{exportError && <p role="alert">{exportError}</p>}</div>
    </>}
  </section>;
}

function RawEventDetails({ detail, onLoad }: { detail: HistoryRunDetailResponse; onLoad: () => void }) {
  const { t } = useTranslation();
  const [eventFilter, setEventFilter] = useState("");
  const events = detail.run.raw_events ?? [];
  const eventNames = [...new Set(events.map((event) => String(event.event ?? "")).filter(Boolean))].sort();
  const visibleEvents = eventFilter ? events.filter((event) => event.event === eventFilter) : events;

  return <details onToggle={(event) => { if (event.currentTarget.open && !detail.run.raw_events) onLoad(); }}>
    <summary>{t("history.rawEvents")}</summary>
    {!detail.run.raw_events ? <p>{t("history.rawLoading")}</p> : <>
      <h5>{t("history.sharedContext")}</h5>
      <pre>{JSON.stringify(detail.run.raw_context ?? {}, null, 2)}</pre>
      <label>{t("history.eventType")}<select value={eventFilter} onChange={(event) => setEventFilter(event.target.value)}><option value="">{t("history.all")} ({formatNumber(events.length)})</option>{eventNames.map((name) => <option key={name} value={name}>{name}</option>)}</select></label>
      <p className="hint">{t("history.eventCount", { visible: formatNumber(visibleEvents.length), total: formatNumber(events.length) })}</p>
      <pre>{JSON.stringify(visibleEvents, null, 2)}</pre>
    </>}
  </details>;
}

function filterQuery(filter: FilterDraft, character: string, difficulty: string): HistoryQuery {
	const now = new Date();
	let from = filter.from ? new Date(filter.from) : undefined;
	let to = filter.to ? new Date(filter.to) : undefined;
	if (filter.period === "today") {
		from = new Date(now.getFullYear(), now.getMonth(), now.getDate());
		to = new Date(from); to.setDate(to.getDate() + 1);
	} else if (filter.period === "7d" || filter.period === "30d") {
		to = now; from = new Date(now); from.setDate(from.getDate() - (filter.period === "7d" ? 7 : 30));
	} else if (filter.period === "all") {
		from = undefined; to = undefined;
	}
  return {
    from: from?.toISOString(),
    to: to?.toISOString(),
    timezone: browserTimezone(),
    character: character ? [character] : undefined,
    run: filter.run ? [filter.run] : undefined,
    difficulty: difficulty ? [difficulty] : undefined,
    outcome: filter.outcome ? [filter.outcome] : undefined,
		reason: filter.reason.trim() ? [filter.reason.trim()] : undefined,
		pickit_profile: filter.pickitProfile.trim() ? [filter.pickitProfile.trim()] : undefined,
  };
}

function browserTimezone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

function HistoryCharts({ summary, comparisons, dailyMetric, onDailyMetric }: {
  summary: HistorySummaryResponse;
  comparisons: HistoryComparisonDTO[];
  dailyMetric: "terminal_runs" | "success_rate" | "keep_per_hour";
  onDailyMetric(value: "terminal_runs" | "success_rate" | "keep_per_hour"): void;
}) {
  const { t } = useTranslation();
  const metric = {
    terminal_runs: { label: t("history.runs"), unit: t("history.runUnit"), color: "#f1a65a" },
    success_rate: { label: t("history.successRate"), unit: t("history.shareUnit"), color: "#77c7a5" },
    keep_per_hour: { label: t("history.keepPerHour"), unit: t("history.hourUnit"), color: "#87a8ff" },
  }[dailyMetric];
  const funnelRows = [
    { stage: t("history.seen"), value: summary.summary.funnel.seen },
    { stage: t("history.matched"), value: summary.summary.funnel.matched },
    { stage: t("history.pickedUp"), value: summary.summary.funnel.picked_up },
    { stage: t("history.securedKeep"), value: summary.summary.funnel.keep_return },
    { stage: t("history.sold"), value: summary.summary.funnel.sold },
  ];
  const lowSample = comparisons.some((row) => row.low_sample);

  return <div className="history-charts" aria-label={t("history.chartsAria")}>
    <figure className="history-chart">
      <figcaption><div><h3>{t("history.daily")}</h3><p>{t("history.unitTimezone", { unit: metric.unit, timezone: summary.meta.timezone })}</p></div><label>{t("history.metric")}<select aria-label={t("history.dailyMetricAria")} value={dailyMetric} onChange={(event) => onDailyMetric(event.target.value as typeof dailyMetric)}><option value="terminal_runs">{t("history.runs")}</option><option value="success_rate">{t("history.successRate")}</option><option value="keep_per_hour">{t("history.keepPerHour")}</option></select></label></figcaption>
      {summary.daily_buckets.length === 0 ? <p className="chart-empty">{t("history.noDaily")}</p> : <ResponsiveContainer width="100%" height={250}><LineChart data={summary.daily_buckets} accessibilityLayer><CartesianGrid strokeDasharray="3 3" /><XAxis dataKey="date" /><YAxis /><Tooltip /><Legend /><Line name={metric.label} dataKey={dailyMetric} stroke={metric.color} strokeWidth={2} connectNulls={false} isAnimationActive={false} /></LineChart></ResponsiveContainer>}
      <div className="table-scroll"><table><caption>{t("history.dailyCaption")}</caption><thead><tr><th>{t("history.localDay")}</th><th>{t("history.utcBounds")}</th><th>{t("history.runs")}</th><th>{t("history.success")}</th><th>{t("history.activeHours")}</th><th>{t("history.keep")}</th><th>{t("history.keepPerHour")}</th></tr></thead><tbody>{summary.daily_buckets.map((row) => <tr key={row.date}><th scope="row">{row.date}</th><td>{row.start_utc} – {row.end_utc}</td><td>{formatNumber(row.terminal_runs)}</td><td>{percent(row.success_rate)}</td><td>{rate(row.active_hours)}</td><td>{formatNumber(row.keep_return)}</td><td>{rate(row.keep_per_hour)}</td></tr>)}</tbody></table></div>
    </figure>

    <figure className="history-chart">
      <figcaption><div><h3>{t("history.routeComparison")}</h3><p>{t("history.routeUnit")}</p></div></figcaption>
      {lowSample && <p className="sample-warning">{t("history.routeSmallSample")}</p>}
      {comparisons.length === 0 ? <p className="chart-empty">{t("history.noComparableRoutes")}</p> : <ResponsiveContainer width="100%" height={250}><BarChart data={comparisons} accessibilityLayer><CartesianGrid strokeDasharray="3 3" /><XAxis dataKey="route_id" /><YAxis /><Tooltip /><Legend /><Bar name={t("history.keepPerHour")} dataKey="keep_per_hour" fill="#f1a65a" isAnimationActive={false} /></BarChart></ResponsiveContainer>}
      <div className="table-scroll"><table><caption>{t("history.routeChartCaption")}</caption><thead><tr><th>{t("history.route")}</th><th>{t("history.terminalRuns")}</th><th>{t("history.bossKillsLabel")}</th><th>{t("history.keepPerHour")}</th></tr></thead><tbody>{comparisons.map((row) => <tr key={row.id}><th scope="row">{row.route_id}</th><td>{formatNumber(row.terminal_runs)}</td><td>{formatNumber(row.boss_kills)}</td><td>{rate(row.keep_per_hour)}</td></tr>)}</tbody></table></div>
    </figure>

    <figure className="history-chart">
      <figcaption><div><h3>{t("history.runStages")}</h3><p>{t("history.stageUnit")}</p></div></figcaption>
      {lowSample && <p className="sample-warning">{t("history.stageSmallSample")}</p>}
      {comparisons.length === 0 ? <p className="chart-empty">{t("history.noStages")}</p> : <ResponsiveContainer width="100%" height={250}><BarChart data={comparisons} accessibilityLayer><CartesianGrid strokeDasharray="3 3" /><XAxis dataKey="route_id" /><YAxis /><Tooltip /><Legend /><Bar name={t("history.travel")} dataKey="stages.travel_ms" stackId="stages" fill="#87a8ff" isAnimationActive={false} /><Bar name={t("history.combat")} dataKey="stages.combat_ms" stackId="stages" fill="#e97872" isAnimationActive={false} /><Bar name={t("history.loot")} dataKey="stages.loot_ms" stackId="stages" fill="#77c7a5" isAnimationActive={false} /><Bar name={t("history.town")} dataKey="stages.return_town_ms" stackId="stages" fill="#f1a65a" isAnimationActive={false} /><Bar name={t("history.other")} dataKey="stages.other_ms" stackId="stages" fill="#9896a6" isAnimationActive={false} /></BarChart></ResponsiveContainer>}
      <div className="table-scroll"><table><caption>{t("history.stageCaption")}</caption><thead><tr><th>{t("history.route")}</th><th>{t("history.travel")}</th><th>{t("history.combat")}</th><th>{t("history.loot")}</th><th>{t("history.town")}</th><th>{t("history.other")}</th></tr></thead><tbody>{comparisons.map((row) => <tr key={row.id}><th scope="row">{row.route_id}</th><td>{formatNumber(row.stages.travel_ms)}</td><td>{formatNumber(row.stages.combat_ms)}</td><td>{formatNumber(row.stages.loot_ms)}</td><td>{formatNumber(row.stages.return_town_ms)}</td><td>{formatNumber(row.stages.other_ms)}</td></tr>)}</tbody></table></div>
    </figure>

    <figure className="history-chart">
      <figcaption><div><h3>{t("history.lootFunnel")}</h3><p>{t("history.funnelUnit")}</p></div></figcaption>
      {summary.summary.boss_kills < 10 && <p className="sample-warning">{t("history.funnelSmallSample")}</p>}
      {funnelRows.every((row) => row.value === 0) ? <p className="chart-empty">{t("history.noFunnel")}</p> : <ResponsiveContainer width="100%" height={250}><BarChart data={funnelRows} layout="vertical" accessibilityLayer><CartesianGrid strokeDasharray="3 3" /><XAxis type="number" /><YAxis type="category" dataKey="stage" width={110} /><Tooltip /><Legend /><Bar name={t("history.itemUnits")} dataKey="value" fill="#77c7a5" isAnimationActive={false} /></BarChart></ResponsiveContainer>}
      <div className="table-scroll"><table><caption>{t("history.funnelCaption")}</caption><thead><tr><th>{t("history.level")}</th><th>{t("history.itemUnits")}</th></tr></thead><tbody>{funnelRows.map((row) => <tr key={row.stage}><th scope="row">{row.stage}</th><td>{formatNumber(row.value)}</td></tr>)}</tbody></table></div>
    </figure>
  </div>;
}

function activeFilterText(query: HistoryQuery, t: AppTranslator): string {
	const labels: string[] = [];
	if (query.from || query.to) labels.push(t("history.filterRange", { from: query.from ? formatDate(query.from, { dateStyle: "short", timeStyle: "short" }) : t("history.beginning"), to: query.to ? formatDate(query.to, { dateStyle: "short", timeStyle: "short" }) : t("history.now") }));
	if (query.timezone) labels.push(t("history.timezone", { timezone: query.timezone }));
	for (const [label, values] of [[t("history.character"), query.character], [t("history.run"), query.run], [t("history.difficulty"), query.difficulty], [t("history.result"), query.outcome?.map((value) => outcomeLabel(value, t))], [t("history.reasonFilter"), query.reason], [t("history.pickitProfile"), query.pickit_profile]] as const) {
		if (values?.length) labels.push(t("history.filterValue", { label, values: values.join(", ") }));
	}
	return labels.length ? labels.join(" · ") : t("history.none");
}

function outcomeLabel(value: string, t: AppTranslator): string {
  return outcomeKeys.includes(value as typeof outcomeKeys[number]) ? t(`history.${value as typeof outcomeKeys[number]}`) : value;
}

function duration(milliseconds: number): string { return formatDuration(milliseconds); }
function percent(value?: number): string { return value === undefined ? "–" : formatPercent(value); }
function rate(value?: number): string { return value === undefined ? "–" : formatNumber(value, { maximumFractionDigits: 2 }); }
