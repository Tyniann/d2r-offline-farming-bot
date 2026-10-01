import { useEffect, useId, useMemo, useState } from "react";
import { ChevronRight } from "lucide-react";
import { useTranslation } from "react-i18next";
import { getHistoryItems, getHistorySummary, type CompactedMaterial, type HistoryItemDTO, type StatusDTO } from "../../api/generated";
import { Button, Dialog, StateMessage } from "../../app/ui";
import { formatClockDuration, formatNumber } from "../../i18n/format";
import { gameBaseItemName, gameHistoryItemName } from "../../i18n/game";
import "./session-summary.css";

const terminalSessionStates = new Set(["idle", "idle_in_game", "stopped_error"]);

/** sessionSummaryFromTransition opens the dialog only when a live session reaches a terminal state. */
export function sessionSummaryFromTransition(
  before: string | undefined,
  status: Pick<StatusDTO, "state" | "last_result"> | null,
): { sessionID: string; durationMs: number } | null {
  const sessionID = status?.last_result?.session_id;
  if (!sessionID || !before || terminalSessionStates.has(before) || !status || !terminalSessionStates.has(status.state)) return null;
  return { sessionID, durationMs: status.last_result?.duration_ms ?? 0 };
}

interface Props {
  sessionID: string;
  durationMs: number;
  refreshKey: number;
  onClose(): void;
}

/** SessionSummaryDialog shows duration, loot and separately confirmed compaction results after a session ends. */
export function SessionSummaryDialog({ sessionID, durationMs, refreshKey, onClose }: Props) {
  const { t, i18n } = useTranslation();
  const [keptOpen, setKeptOpen] = useState(false);
  const [soldOpen, setSoldOpen] = useState(false);
  const [compactedOpen, setCompactedOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [keptCount, setKeptCount] = useState(0);
  const [soldCount, setSoldCount] = useState(0);
  const [items, setItems] = useState<HistoryItemDTO[]>([]);
  const [compacted, setCompacted] = useState<CompactedMaterial[]>([]);
  const keptListID = useId();
  const soldListID = useId();
  const compactedListID = useId();

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(false);
    const session = { session: [sessionID] };
    void Promise.all([
      getHistorySummary(session, controller.signal),
      getHistoryItems({ ...session, limit: 200, item_disposition: "kept_sold" }, controller.signal),
    ]).then(([summary, page]) => {
      if (controller.signal.aborted) return;
      setKeptCount(summary.summary.funnel.keep_return);
      setSoldCount(summary.summary.funnel.sold);
      setItems(page.items ?? []);
      setCompacted(summary.summary.compacted ?? []);
      setLoading(false);
    }).catch(() => {
      if (controller.signal.aborted) return;
      setError(true);
      setLoading(false);
    });
    return () => controller.abort();
  }, [sessionID, refreshKey, reloadKey]);

  const keptItems = useMemo(() => sessionItemRows(items, "stashed", i18n.resolvedLanguage), [items, i18n.resolvedLanguage]);
  const soldItems = useMemo(() => sessionItemRows(items, "sold", i18n.resolvedLanguage), [items, i18n.resolvedLanguage]);
  const compactedItems = useMemo(() => compacted.map((item) => ({ key: item.code, count: item.count, name: gameBaseItemName(item.code, item.code, i18n.resolvedLanguage) })).sort((left, right) => right.count - left.count || left.name.localeCompare(right.name, i18n.resolvedLanguage)), [compacted, i18n.resolvedLanguage]);

  return <Dialog title={t("dashboard.sessionSummary.title")} className="session-summary-dialog" onClose={onClose}>
    <p className="session-summary-duration">{t("dashboard.sessionSummary.duration", { duration: formatClockDuration(durationMs) })}</p>
    {loading && <StateMessage kind="loading" title={t("dashboard.sessionSummary.loading")} />}
    {error && <StateMessage kind="error" title={t("dashboard.sessionSummary.error")}><Button variant="secondary" onClick={() => setReloadKey((value) => value + 1)}>{t("dashboard.sessionSummary.retry")}</Button></StateMessage>}
    {!loading && !error && <>
      <SessionSummarySection
        listID={keptListID}
        open={keptOpen}
        count={keptCount}
        headerKey="keptHeader"
        expandKey="expandKept"
        collapseKey="collapseKept"
        emptyKey="emptyKept"
        items={keptItems}
        onToggle={() => setKeptOpen((value) => !value)}
      />
      <SessionSummarySection
        listID={soldListID}
        open={soldOpen}
        count={soldCount}
        headerKey="soldHeader"
        expandKey="expandSold"
        collapseKey="collapseSold"
        emptyKey="emptySold"
        items={soldItems}
        onToggle={() => setSoldOpen((value) => !value)}
      />
      <SessionSummarySection
        listID={compactedListID}
        open={compactedOpen}
        count={compacted.reduce((sum, item) => sum + item.count, 0)}
        headerKey="compactedHeader"
        expandKey="expandCompacted"
        collapseKey="collapseCompacted"
        emptyKey="emptyCompacted"
        items={compactedItems}
        onToggle={() => setCompactedOpen((value) => !value)}
      />
    </>}
    <div className="modal-actions"><Button onClick={onClose}>{t("dashboard.sessionSummary.close")}</Button></div>
  </Dialog>;
}

function SessionSummarySection({
  listID, open, count, headerKey, expandKey, collapseKey, emptyKey, items, onToggle,
}: {
  listID: string;
  open: boolean;
  count: number;
  headerKey: "keptHeader" | "soldHeader" | "compactedHeader";
  expandKey: "expandKept" | "expandSold" | "expandCompacted";
  collapseKey: "collapseKept" | "collapseSold" | "collapseCompacted";
  emptyKey: "emptyKept" | "emptySold" | "emptyCompacted";
  items: Array<{ key: string; name: string; count: number }>;
  onToggle(): void;
}) {
  const { t } = useTranslation();
  return <section className="session-summary-section">
    <button type="button" aria-expanded={open} aria-controls={listID} aria-label={t(`dashboard.sessionSummary.${open ? collapseKey : expandKey}`)} onClick={onToggle}>
      <span>{t(`dashboard.sessionSummary.${headerKey}`, { count: formatNumber(count) })}</span>
      <ChevronRight aria-hidden="true" size={18} className={open ? "is-open" : undefined} />
    </button>
    {open && <ul id={listID} className="session-summary-list">
      {items.length === 0 && <li>{t(`dashboard.sessionSummary.${emptyKey}`)}</li>}
      {items.map((item) => <li key={item.key}>{t("dashboard.sessionSummary.itemLine", { count: formatNumber(item.count), name: item.name })}</li>)}
    </ul>}
  </section>;
}

function sessionItemRows(items: HistoryItemDTO[], field: "stashed" | "sold", language: string | undefined): Array<{ key: string; name: string; count: number }> {
  return items.filter((item) => item[field] > 0).sort((left, right) => {
    if (right[field] !== left[field]) return right[field] - left[field];
    return gameHistoryItemName(left, language).localeCompare(gameHistoryItemName(right, language), language || "de");
  }).map((item) => ({ key: item.item_key, count: item[field], name: gameHistoryItemName(item, language) }));
}
