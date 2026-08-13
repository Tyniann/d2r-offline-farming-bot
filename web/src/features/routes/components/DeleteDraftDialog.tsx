import { useEffect, useRef } from "react";
import type { RouteCandidateDTO } from "../../../api/generated";
import { candidateTitle, formatCandidateTime } from "../routePresentation";

interface Props { candidate: RouteCandidateDTO; pending: boolean; onClose(): void; onConfirm(): void; }

export function DeleteDraftDialog({ candidate, pending, onClose, onConfirm }: Props) {
  const confirmRef = useRef<HTMLButtonElement>(null);
  useEffect(() => { confirmRef.current?.focus(); }, []);
  return <div className="modal-backdrop" onMouseDown={(event) => { if (event.currentTarget === event.target) onClose(); }}>
    <div className="modal danger-modal" role="alertdialog" aria-modal="true" aria-labelledby="delete-draft-title" aria-describedby="delete-draft-description">
      <h3 id="delete-draft-title">Entwurf löschen?</h3>
      <p><strong>{candidateTitle(candidate)} · aufgenommen {formatCandidateTime(candidate).toLocaleLowerCase()}</strong></p>
      <p id="delete-draft-description">Die Aufnahme wird dauerhaft entfernt. Eine bereits veröffentlichte Route ist davon nicht betroffen.</p>
      <div className="modal-actions"><button type="button" className="secondary" disabled={pending} onClick={onClose}>Abbrechen</button><button ref={confirmRef} type="button" className="danger" disabled={pending} onClick={onConfirm}>{pending ? "Entwurf wird gelöscht …" : "Entwurf löschen"}</button></div>
    </div>
  </div>;
}
