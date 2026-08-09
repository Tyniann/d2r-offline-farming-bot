import { Save } from "lucide-react";
import { Button } from "../../app/ui";

/** SettingsActionBar ist die sticky Commit-Leiste für das Core-Farming-Dokument. */
export function SettingsActionBar({
  dirty, locked, revision, summary, busy, collapsed, onDiscard, onSave, onShowFarming,
}: {
  dirty: boolean;
  locked: boolean;
  revision: number;
  summary: string;
  busy: boolean;
  collapsed?: boolean;
  onDiscard: () => void;
  onSave: () => void;
  onShowFarming?: () => void;
}) {
  if (collapsed) {
    return <div className={`settings-actionbar${dirty ? " dirty" : ""}`} role="status">
      <div><strong>Ungespeicherte Einstellungsänderungen</strong><p>{summary}</p></div>
      {onShowFarming && <Button variant="secondary" onClick={onShowFarming}>Ansehen</Button>}
    </div>;
  }

  if (locked) {
    return <div className="settings-actionbar locked" role="status">
      <div><strong>Gesperrt, solange eine Session läuft.</strong><p>Speichern ist wieder möglich, sobald der Bot inaktiv ist.</p></div>
      <Button disabled><Save aria-hidden="true" size={16} /> Speichern</Button>
    </div>;
  }

  const changeCount = dirty ? summary.split(", ").filter(Boolean).length : 0;
  return <div className={`settings-actionbar${dirty ? " dirty" : ""}`} role="status">
    <div>
      {dirty
        ? <strong>{changeCount} Änderung{changeCount === 1 ? "" : "en"}: {summary}</strong>
        : <><strong>Keine offenen Änderungen</strong><p>Revision {revision}</p></>}
    </div>
    <div className="inline-actions">
      {dirty && <Button variant="secondary" onClick={onDiscard} disabled={busy}>Verwerfen</Button>}
      <Button onClick={onSave} disabled={!dirty || busy}><Save aria-hidden="true" size={16} /> {busy ? "Core prüft …" : "Speichern"}</Button>
    </div>
  </div>;
}
