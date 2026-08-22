import { useEffect, useRef } from "react";
import type { RouteCandidateDTO } from "../../../api/generated";
import { candidateTitle, formatCandidateTime } from "../routePresentation";
import { useTranslation } from "react-i18next";

interface Props { candidate: RouteCandidateDTO; pending: boolean; onClose(): void; onConfirm(): void; }

export function DeleteDraftDialog({ candidate, pending, onClose, onConfirm }: Props) {
  const { t } = useTranslation();
  const confirmRef = useRef<HTMLButtonElement>(null);
  useEffect(() => { confirmRef.current?.focus(); }, []);
  return <div className="modal-backdrop" onMouseDown={(event) => { if (event.currentTarget === event.target) onClose(); }}>
    <div className="modal danger-modal" role="alertdialog" aria-modal="true" aria-labelledby="delete-draft-title" aria-describedby="delete-draft-description">
      <h3 id="delete-draft-title">{t("routes.deleteDraftTitle")}</h3>
      <p><strong>{t("routes.recordedAt", { title: candidateTitle(candidate), time: formatCandidateTime(candidate) })}</strong></p>
      <p id="delete-draft-description">{t("routes.deleteDraftDetail")}</p>
      <div className="modal-actions"><button type="button" className="secondary" disabled={pending} onClick={onClose}>{t("common.cancel")}</button><button ref={confirmRef} type="button" className="danger" disabled={pending} onClick={onConfirm}>{t(pending ? "routes.deletingDraft" : "routes.deleteDraft")}</button></div>
    </div>
  </div>;
}
