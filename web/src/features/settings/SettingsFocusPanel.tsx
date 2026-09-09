import { useEffect, useId, useRef, type KeyboardEvent, type ReactNode } from "react";
import { X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "../../app/ui";

const focusableSelector = 'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [href], summary, [tabindex]:not([tabindex="-1"])';

/**
 * SettingsFocusPanel ist das seitlich eingeschobene Bearbeitungsfenster für komplexe Editoren.
 *
 * Modal-Vertrag: `role=dialog` mit `aria-modal`, Escape und Backdrop-Klick schließen, Tab bleibt im Panel,
 * der Seitenhintergrund scrollt nicht, und beim Schließen kehrt der Fokus zum auslösenden Element zurück.
 * Tastatur wird am Panel selbst behandelt, nicht am Fenster: Bestätigungsdialoge, die aus dem Panel geöffnet
 * werden (z. B. Zurücksetzen), liegen außerhalb des Panels und fangen Escape deshalb selbst.
 */
export function SettingsFocusPanel({ title, eyebrow, badge, footerNote, onClose, children }: {
  title: string;
  eyebrow: string;
  badge?: ReactNode;
  footerNote: string;
  onClose: () => void;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  const titleID = useId();
  const panelRef = useRef<HTMLElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  useEffect(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    panelRef.current?.focus({ preventScroll: true });
    return () => {
      document.body.style.overflow = previousOverflow;
      opener?.focus({ preventScroll: true });
    };
  }, []);

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      closeRef.current();
      return;
    }
    if (event.key !== "Tab" || !panelRef.current) return;
    const entries = Array.from(panelRef.current.querySelectorAll<HTMLElement>(focusableSelector));
    if (entries.length === 0) return;
    const first = entries[0];
    const last = entries[entries.length - 1];
    if (event.shiftKey && (document.activeElement === first || document.activeElement === panelRef.current)) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  return <div className="settings-focus-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) closeRef.current(); }}>
    <aside ref={panelRef} className="settings-focus" role="dialog" aria-modal="true" aria-labelledby={titleID} tabIndex={-1} onKeyDown={onKeyDown}>
      <header className="settings-focus-head">
        {badge}
        <div><p className="eyebrow">{eyebrow}</p><h2 id={titleID}>{title}</h2></div>
        <Button variant="secondary" className="settings-focus-close" aria-label={t("settings.closePanel")} onClick={onClose}><X size={18} aria-hidden="true" /></Button>
      </header>
      <div className="settings-focus-body">{children}</div>
      <footer className="settings-focus-foot">
        <small className="hint">{footerNote}</small>
        <Button variant="secondary" onClick={onClose}>{t("settings.done")}</Button>
      </footer>
    </aside>
  </div>;
}
