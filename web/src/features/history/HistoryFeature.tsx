import { useEffect, useRef, useState } from "react";
import {
  downloadHistoryExport, getHistoryComparisons, getHistoryItems, getHistoryRun, getHistoryRuns, getHistorySummary,
  type HistoryComparisonDTO, type HistoryItemsResponse, type HistoryQuery, type HistoryRunDetailResponse,
  type HistoryRunsResponse, type HistorySummaryResponse,
} from "../../api/generated";

interface Props { characters: string[]; runs: string[]; refreshKey: number }
interface FilterDraft { period: string; from: string; to: string; character: string; run: string; difficulty: string; outcome: string; reason: string; pickitProfile: string }

const emptyFilter: FilterDraft = { period: "all", from: "", to: "", character: "", run: "", difficulty: "", outcome: "", reason: "", pickitProfile: "" };
const outcomeLabels: Record<string, string> = { success: "Erfolg", failed: "Fehlgeschlagen", aborted: "Abgebrochen", incomplete: "Unvollständig", running: "Aktiv" };

export function HistoryFeature({ characters, runs, refreshKey }: Props) {
  const [draft, setDraft] = useState<FilterDraft>(emptyFilter);
  const [query, setQuery] = useState<HistoryQuery>({});
  const [summary, setSummary] = useState<HistorySummaryResponse | null>(null);
  const [comparisons, setComparisons] = useState<HistoryComparisonDTO[]>([]);
  const [items, setItems] = useState<HistoryItemsResponse | null>(null);
  const [runList, setRunList] = useState<HistoryRunsResponse | null>(null);
  const [detail, setDetail] = useState<HistoryRunDetailResponse | null>(null);
  const [comparisonSort, setComparisonSort] = useState("keep_per_hour");
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
        if (!signal?.aborted) setError(reason instanceof Error ? reason.message : "Historie konnte nicht geladen werden.");
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
  }, [query, refreshKey, comparisonSort]); // `history_changed`, Filter und Core-Sortierung laden serialisiert neu.

  const applyFilters = () => setQuery(filterQuery(draft));
  const resetFilters = () => { setDraft(emptyFilter); setQuery({}); };

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
      setError(reason instanceof Error ? reason.message : "Weitere Historienzeilen konnten nicht geladen werden.");
    } finally {
      setLoadingMore(false);
    }
  };

  const openDetail = async (runID: string, includeRaw = false) => {
    try { setDetail(await getHistoryRun(runID, includeRaw)); setError(""); }
    catch (reason: unknown) { setError(reason instanceof Error ? reason.message : "Run-Details konnten nicht geladen werden."); }
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
      setExportError(reason instanceof Error ? reason.message : "Historienexport fehlgeschlagen.");
    } finally { setExporting(false); }
  };

  const filtered = Object.values(query).some((value) => Array.isArray(value) ? value.length > 0 : value !== undefined && value !== "");
  const noRuns = !loading && !error && summary?.summary.runs === 0;

  return <section id="history" aria-labelledby="history-title" className="history-feature">
    <div className="section-heading"><div><p className="eyebrow">Schema-3-Auswertung</p><h2 id="history-title">Historie</h2></div><button type="button" className="secondary" onClick={() => void load(query)} disabled={loading}>Aktualisieren</button></div>
    <p>Gesicherter Return, Zeitverlust und Routenleistung aus Core-korrelierten produktiven Runs. Verkauf bleibt getrennt von Keep.</p>

    <form className="history-filters" onSubmit={(event) => { event.preventDefault(); applyFilters(); }}>
      <label>Zeitraum<select value={draft.period} onChange={(event) => setDraft({ ...draft, period: event.target.value })}><option value="all">Gesamte Historie</option><option value="today">Heute</option><option value="7d">Letzte 7 Tage</option><option value="30d">Letzte 30 Tage</option><option value="custom">Freier Zeitraum</option></select></label>
      <label>Von<input type="datetime-local" value={draft.from} onChange={(event) => setDraft({ ...draft, from: event.target.value })} /></label>
      <label>Bis<input type="datetime-local" value={draft.to} onChange={(event) => setDraft({ ...draft, to: event.target.value })} /></label>
      <label>Charakter<select value={draft.character} onChange={(event) => setDraft({ ...draft, character: event.target.value })}><option value="">Alle</option>{characters.map((value) => <option key={value}>{value}</option>)}</select></label>
      <label>Run<select value={draft.run} onChange={(event) => setDraft({ ...draft, run: event.target.value })}><option value="">Alle</option>{runs.map((value) => <option key={value}>{value}</option>)}</select></label>
      <label>Schwierigkeit<select value={draft.difficulty} onChange={(event) => setDraft({ ...draft, difficulty: event.target.value })}><option value="">Alle</option><option value="normal">Normal</option><option value="nightmare">Alptraum</option><option value="hell">Hölle</option></select></label>
      <label>Ergebnis<select value={draft.outcome} onChange={(event) => setDraft({ ...draft, outcome: event.target.value })}><option value="">Alle</option>{Object.entries(outcomeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
      <label>Reason-Code<input value={draft.reason} placeholder="z. B. boss_not_found" onChange={(event) => setDraft({ ...draft, reason: event.target.value })} /></label>
      <label>Pickit-Profil<input value={draft.pickitProfile} placeholder="optional" onChange={(event) => setDraft({ ...draft, pickitProfile: event.target.value })} /></label>
      <div className="inline-actions"><button type="submit">Filter anwenden</button><button type="button" className="secondary" onClick={resetFilters}>Zurücksetzen</button></div>
    </form>
    <p className="active-filters" aria-live="polite"><strong>Aktive Filter:</strong> {activeFilterText(query)}</p>

    {error && <p role="alert">{error}</p>}
    {loading && <p role="status">Historie wird geladen …</p>}
    {noRuns && <div className="history-empty"><h3>{filtered ? "Keine passenden Runs" : "Noch keine Historie"}</h3><p>{filtered ? "Die gewählten Filter liefern keine produktiven Runs." : "Nach dem ersten Schema-3-Farming-Run erscheinen hier Kennzahlen und Details."}</p></div>}
    {!loading && summary && summary.summary.runs > 0 && <>
      <div className="cards history-summary">
        <article><span>Terminale Runs</span><strong>{summary.summary.terminal_runs}</strong><small>{summary.summary.successful} erfolgreich · {summary.summary.failed} fehlgeschlagen · {summary.summary.aborted} abgebrochen</small></article>
        <article><span>Erfolgsquote</span><strong>{percent(summary.summary.success_rate)}</strong><small>{summary.summary.boss_kills} bestätigte Bosskills</small></article>
        <article><span>Ø Runzeit</span><strong>{duration(summary.summary.durations.average_ms)}</strong><small>Median {duration(summary.summary.durations.median_ms)}</small></article>
        <article><span>Gesicherter Keep</span><strong>{summary.summary.funnel.keep_return}</strong><small>Verkauft {summary.summary.funnel.sold}</small></article>
        <article><span>Keep / Stunde</span><strong>{rate(summary.summary.keep_per_hour)}</strong><small>{rate(summary.summary.keep_per_run)} / Run · {rate(summary.summary.keep_per_kill)} / Kill</small></article>
        <article><span>Loot-Verlust vor / nach Pickup</span><strong>{summary.summary.funnel.pickup_lost} / {summary.summary.funnel.post_pickup_lost}</strong><small>Getrennte Core-Zähler</small></article>
      </div>
      {(summary.summary.running > 0 || summary.summary.incomplete > 0) && <p role="status">Nicht aggregiert: {summary.summary.running} aktiv, {summary.summary.incomplete} unvollständig.</p>}
      {summary.summary.top_failure && <article className="history-priority"><span>Größter Fehler- und Zeitverlust</span><strong>{summary.summary.top_failure.reason_message}</strong><small>{summary.summary.top_failure.count} × in {summary.summary.top_failure.step || "unbekanntem Schritt"} · {duration(summary.summary.top_failure.lost_duration_ms)} verloren</small></article>}
      {summary.meta.diagnostics.length > 0 && <aside className="history-diagnostics" aria-labelledby="history-diagnostics-title"><h3 id="history-diagnostics-title">Dateidiagnose</h3><p>{summary.meta.diagnostics.length} Datei(en) wurden isoliert; {summary.meta.ignored_files} ältere Datei(en) liegen vor der auswertbaren Epoche.</p><ul>{summary.meta.diagnostics.map((item) => <li key={`${item.file}-${item.code}`}><strong>{item.file}</strong><span>{item.message}</span></li>)}</ul></aside>}

      <div className="section-heading"><h3>Boss- und Routenvergleich</h3><label>Sortierung<select value={comparisonSort} onChange={(event) => setComparisonSort(event.target.value)}><option value="keep_per_hour">Keep / Stunde</option><option value="success_rate">Erfolgsquote</option><option value="average_duration">Ø Dauer</option></select></label></div>
      <div className="table-scroll"><table className="comparison-table"><caption>Vergleich derselben Charakter-, Difficulty- und Run-Definition nach Route</caption><thead><tr><th>Route</th><th>Sample</th><th>Keep / Stunde</th><th>Keep / Run</th><th>Keep / Kill</th><th>Sell</th><th>Erfolg</th><th>Ø Dauer</th><th>Stages R/K/L/S</th><th>Fehler</th></tr></thead><tbody>{comparisons.map((row) => <tr key={row.id}><th scope="row" data-label="Route">{row.run}<small>{row.character} · {row.difficulty} · {row.route_id}</small></th><td data-label="Sample">{row.boss_kills}{row.low_sample && <span className="sample-warning">Kleine Stichprobe (&lt; 10 Kills)</span>}</td><td data-label="Keep / Stunde">{rate(row.keep_per_hour)}</td><td data-label="Keep / Run">{rate(row.keep_per_run)}</td><td data-label="Keep / Kill">{rate(row.keep_per_kill)}</td><td data-label="Verkauft">{row.funnel.sold}</td><td data-label="Erfolg">{percent(row.success_rate)}</td><td data-label="Ø Dauer">{duration(row.durations.average_ms)}</td><td data-label="Stages">{duration(row.stages.travel_ms)} / {duration(row.stages.combat_ms)} / {duration(row.stages.loot_ms)} / {duration(row.stages.return_town_ms)}</td><td data-label="Fehler">{row.top_failure?.reason_message ?? "–"}</td></tr>)}</tbody></table></div>

      <h3>Itemausbeute</h3>
      <div className="table-scroll"><table className="item-table"><caption>Keep, Verkauf und Verluste pro stabiler Itemidentität</caption><thead><tr><th>Item</th><th>Gesehen</th><th>Gematcht</th><th>Aufgehoben</th><th>Keep</th><th>Verkauft</th><th>Pickup verloren</th><th>Nach Pickup verloren</th><th>Ertrag / Run</th><th>Ertrag / Kill</th><th>Ertrag / Stunde</th></tr></thead><tbody>{items?.items.map((item) => <tr key={item.item_key}><th scope="row" data-label="Item">{item.item_name}<small>{item.item_key}</small></th><td data-label="Gesehen">{item.seen}</td><td data-label="Gematcht">{item.matched}</td><td data-label="Aufgehoben">{item.picked_up}</td><td data-label="Keep">{item.stashed}</td><td data-label="Verkauft">{item.sold}</td><td data-label="Pickup verloren">{item.pickup_lost}</td><td data-label="Nach Pickup verloren">{item.post_pickup_lost}</td><td data-label="Ertrag / Run">{rate(item.yield_per_run)}</td><td data-label="Ertrag / Kill">{rate(item.yield_per_kill)}</td><td data-label="Ertrag / Stunde">{rate(item.yield_per_hour)}</td></tr>)}</tbody></table></div>
      {items?.next_cursor && <button type="button" className="secondary" disabled={loadingMore} onClick={() => void loadMore("items")}>Mehr Items laden</button>}

      <h3>Runs</h3>
      <div className="table-scroll"><table className="run-table"><caption>Neueste Runs zuerst</caption><thead><tr><th>Start (lokal)</th><th>Run / Route</th><th>Ergebnis</th><th>Dauer</th><th>Keep</th><th>Sell</th><th>Verlust vor/nach Pickup</th><th>Aktion</th></tr></thead><tbody>{runList?.runs.map((row) => <tr key={row.run_id}><td data-label="Start">{new Date(row.started_at).toLocaleString("de-DE")}</td><th scope="row" data-label="Run / Route">{row.run}<small>{row.route_id}</small></th><td data-label="Ergebnis"><span className={`outcome outcome-${row.outcome}`}>{outcomeLabels[row.outcome] ?? row.outcome}</span>{row.reason_message && <small>{row.reason_message}</small>}</td><td data-label="Dauer">{duration(row.duration_ms)}</td><td data-label="Keep">{row.funnel.keep_return}</td><td data-label="Sell">{row.funnel.sold}</td><td data-label="Verlust">{row.funnel.pickup_lost} / {row.funnel.post_pickup_lost}</td><td data-label="Aktion"><button type="button" className="secondary" onClick={() => void openDetail(row.run_id)}>Run öffnen</button></td></tr>)}</tbody></table></div>
      {runList?.next_cursor && <button type="button" className="secondary" disabled={loadingMore} onClick={() => void loadMore("runs")}>Mehr Runs laden</button>}

      {detail && <article className="history-detail" aria-labelledby="history-detail-title"><div className="section-heading"><h3 id="history-detail-title">Run {detail.run.run_id}</h3><button type="button" className="secondary" onClick={() => setDetail(null)}>Schließen</button></div><dl><div><dt>Kontext</dt><dd>{detail.run.character} · {detail.run.difficulty} · {detail.run.run}</dd></div><div><dt>Route</dt><dd>{detail.run.route_id}</dd></div><div><dt>Ergebnis</dt><dd>{outcomeLabels[detail.run.outcome] ?? detail.run.outcome}{detail.run.reason_message ? ` · ${detail.run.reason_message}` : ""}</dd></div>{detail.run.reason && <div><dt>Fehlerstelle</dt><dd><code>{detail.run.reason}</code>{detail.run.last_step ? ` · Step ${detail.run.last_step}` : ""}</dd></div>}<div><dt>Boss</dt><dd>{detail.run.boss_kills} Memory-bestätigte Kills</dd></div><div><dt>Funnel</dt><dd>{detail.run.funnel.seen} gesehen → {detail.run.funnel.matched} gematcht → {detail.run.funnel.picked_up} aufgehoben → {detail.run.funnel.keep_return} Keep / {detail.run.funnel.sold} Sell</dd></div><div><dt>Stages</dt><dd>Reise {duration(detail.run.stages.travel_ms)} · Kampf {duration(detail.run.stages.combat_ms)} · Loot {duration(detail.run.stages.loot_ms)} · Stadt {duration(detail.run.stages.return_town_ms)} · Sonstiges {duration(detail.run.stages.other_ms)}</dd></div></dl><h4>Itempfade</h4>{detail.run.items.length === 0 ? <p>Kein Loot erfunden: Für diesen Run liegen keine Itemereignisse vor.</p> : <ul>{detail.run.items.map((item) => <li key={item.unit_id}><strong>{item.item_name || item.item_key}</strong><span>{item.stashed ? "gesichert" : item.sold ? "bestätigt verkauft" : item.pickup_lost ? "vor Pickup verloren" : item.post_pickup_lost ? "nach Pickup verloren" : "beobachtet"}</span>{item.pickit_profile_id && <small>Pickit {item.pickit_profile_id} Revision {item.pickit_profile_revision} · Regel {item.pickit_rule_id || "–"} · Aktion {item.pickit_action || "–"} · Assignment-Revision {item.pickit_assignment_revision}</small>}</li>)}</ul>}<details onToggle={(event) => { if (event.currentTarget.open && !detail.run.raw_events) void openDetail(detail.run.run_id, true); }}><summary>Rohereignisse anzeigen</summary><pre>{detail.run.raw_events ? JSON.stringify(detail.run.raw_events, null, 2) : "Rohereignisse werden geladen …"}</pre></details></article>}

      <div className="history-export" aria-labelledby="history-export-title"><h3 id="history-export-title">Export</h3><div className="inline-actions"><button type="button" disabled={exporting} onClick={() => void exportData("json")}>JSON-Report</button><button type="button" disabled={exporting} onClick={() => void exportData("csv", "runs")}>Run-CSV</button><button type="button" disabled={exporting} onClick={() => void exportData("csv", "items")}>Item-CSV</button></div>{exportError && <p role="alert">{exportError}</p>}</div>
    </>}
  </section>;
}

function filterQuery(filter: FilterDraft): HistoryQuery {
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
    character: filter.character ? [filter.character] : undefined,
    run: filter.run ? [filter.run] : undefined,
    difficulty: filter.difficulty ? [filter.difficulty] : undefined,
    outcome: filter.outcome ? [filter.outcome] : undefined,
		reason: filter.reason.trim() ? [filter.reason.trim()] : undefined,
		pickit_profile: filter.pickitProfile.trim() ? [filter.pickitProfile.trim()] : undefined,
  };
}

function activeFilterText(query: HistoryQuery): string {
	const labels: string[] = [];
	if (query.from || query.to) labels.push(`${query.from ? new Date(query.from).toLocaleString("de-DE") : "Anfang"} bis ${query.to ? new Date(query.to).toLocaleString("de-DE") : "jetzt"}`);
	for (const [label, values] of [["Charakter", query.character], ["Run", query.run], ["Difficulty", query.difficulty], ["Ergebnis", query.outcome], ["Reason", query.reason], ["Pickit", query.pickit_profile]] as const) {
		if (values?.length) labels.push(`${label}: ${values.join(", ")}`);
	}
	return labels.length ? labels.join(" · ") : "keine";
}

function duration(milliseconds: number): string { return `${(milliseconds / 1000).toLocaleString("de-DE", { maximumFractionDigits: 1 })} s`; }
function percent(value?: number): string { return value === undefined ? "–" : `${(value * 100).toLocaleString("de-DE", { maximumFractionDigits: 1 })} %`; }
function rate(value?: number): string { return value === undefined ? "–" : value.toLocaleString("de-DE", { maximumFractionDigits: 2 }); }
