import { AlertTriangle, Inbox, LoaderCircle, type LucideIcon } from "lucide-react";
import { type ComponentProps, type ReactNode, type RefObject, useEffect, useId, useRef } from "react";

export function Button({ variant = "primary", className, ...props }: ComponentProps<"button"> & { variant?: "primary" | "secondary" | "danger" }) {
  const classes = [variant === "primary" ? "" : variant, className].filter(Boolean).join(" ");
  return <button type="button" className={classes || undefined} {...props} />;
}

export function PageHeader({ eyebrow, title, description, actions }: { eyebrow: string; title: string; description: string; actions?: ReactNode }) {
  return <header className="page-header">
    <div><p className="eyebrow">{eyebrow}</p><h1>{title}</h1><p>{description}</p></div>
    {actions && <div className="page-header-actions">{actions}</div>}
  </header>;
}

export function StatusBadge({ tone, icon: Icon, children }: { tone: "neutral" | "success" | "warning" | "danger"; icon?: LucideIcon; children: ReactNode }) {
  return <span className={`status-badge status-${tone}`}>{Icon && <Icon aria-hidden="true" size={15} />}<span>{children}</span></span>;
}

export function StateMessage({ kind, title, children }: { kind: "loading" | "empty" | "error"; title: string; children?: ReactNode }) {
  const Icon = kind === "loading" ? LoaderCircle : kind === "error" ? AlertTriangle : Inbox;
  return <div className={`state-message state-${kind}`} role={kind === "error" ? "alert" : "status"}>
    <Icon aria-hidden="true" size={20} /><div><strong>{title}</strong>{children && <p>{children}</p>}</div>
  </div>;
}

export function Dialog({ title, children, onClose, initialFocusRef }: { title: string; children: ReactNode; onClose: () => void; initialFocusRef?: RefObject<HTMLElement | null> }) {
  const titleID = useId();
  const dialogRef = useRef<HTMLDivElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  useEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const dialog = dialogRef.current;
    const focusable = () => Array.from(dialog?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [href], [tabindex]:not([tabindex="-1"])') ?? []);
    (initialFocusRef?.current ?? focusable()[0] ?? dialog)?.focus();
    const handleKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeRef.current();
        return;
      }
      if (event.key !== "Tab") return;
      const entries = focusable();
      if (!entries.length) return;
      const first = entries[0];
      const last = entries[entries.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    window.addEventListener("keydown", handleKey);
    return () => {
      window.removeEventListener("keydown", handleKey);
      previous?.focus();
    };
  }, [initialFocusRef]);

  return <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) closeRef.current(); }}>
    <div ref={dialogRef} role="dialog" aria-modal="true" aria-labelledby={titleID} className="modal" tabIndex={-1}>
      <h2 id={titleID}>{title}</h2>
      {children}
    </div>
  </div>;
}
