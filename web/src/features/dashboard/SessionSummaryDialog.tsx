import { useEffect, useId, useMemo, useState } from "react";
import { ChevronRight } from "lucide-react";
import { useTranslation } from "react-i18next";
import { getHistoryItems, getHistorySummary, type HistoryItemDTO, type StatusDTO } from "../../api/generated";
import { Button, Dialog, StateMessage } from "../../app/ui";
import { formatClockDuration, formatNumber } from "../../i18n/format";
import { gameHistoryItemName } from "../../i18n/game";
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

/** SessionSummaryDialog shows wall-clock duration and expandable keep/sell aggregates after a session ends. */
export function SessionSummaryDialog({ sessionID, durationMs, refreshKey, onClose }: Props) {
  const { t, i18n } = useTranslation();
  const [keptOpen, setKeptOpen] = useState(false);
  const [soldOpen, setSoldOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [keptCount, setKeptCount] = useState(0);
  const [soldCount, setSoldCount] = useState(0);
  const [items, setItems] = useState<HistoryItemDTO[]>([]);
  const keptListID = useId();
  const soldListID = useId();

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(false);
    const query = { session: [sessionID], limit: 200 };
    void Promise.all([
      getHistorySummary(query, controller.signal),
      getHistoryItems(query, controller.signal),
    ]).then(([summary, page]) => {
      if (controller.signal.aborted) return;
      setKeptCount(summary.summary.funnel.keep_return);
      setSoldCount(summary.summary.funnel.sold);
      setItems(page.items ?? []);
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
        field="stashed"
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
        field="sold"
        onToggle={() => setSoldOpen((value) => !value)}
      />
    </>}
    <div className="modal-actions"><Button onClick={onClose}>{t("dashboard.sessionSummary.close")}</Button></div>
  </Dialog>;
}

function SessionSummarySection({
  listID, open, count, headerKey, expandKey, collapseKey, emptyKey, items, field, onToggle,
}: {
  listID: string;
  open: boolean;
  count: number;
  headerKey: "keptHeader" | "soldHeader";
  expandKey: "expandKept" | "expandSold";
  collapseKey: "collapseKept" | "collapseSold";
  emptyKey: "emptyKept" | "emptySold";
  items: HistoryItemDTO[];
  field: "stashed" | "sold";
  onToggle(): void;
}) {
  const { t, i18n } = useTranslation();
  return <section className="session-summary-section">
    <button type="button" aria-expanded={open} aria-controls={listID} aria-label={t(`dashboard.sessionSummary.${open ? collapseKey : expandKey}`)} onClick={onToggle}>
      <span>{t(`dashboard.sessionSummary.${headerKey}`, { count: formatNumber(count) })}</span>
      <ChevronRight aria-hidden="true" size={18} className={open ? "is-open" : undefined} />
    </button>
    {open && <ul id={listID} className="session-summary-list">
      {items.length === 0 && <li>{t(`dashboard.sessionSummary.${emptyKey}`)}</li>}
      {items.map((item) => <li key={item.item_key}>{t("dashboard.sessionSummary.itemLine", { count: formatNumber(item[field]), name: gameHistoryItemName(item, i18n.resolvedLanguage) })}</li>)}
    </ul>}
  </section>;
}

function sessionItemRows(items: HistoryItemDTO[], field: "stashed" | "sold", language: string | undefined): HistoryItemDTO[] {
  return items.filter((item) => item[field] > 0).sort((left, right) => {
    if (right[field] !== left[field]) return right[field] - left[field];
    return gameHistoryItemName(left, language).localeCompare(gameHistoryItemName(right, language), language || "de");
  });
}
