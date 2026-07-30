import { useState } from "react";
import { RefreshCw } from "lucide-react";
import { Button, StateMessage, StatusBadge } from "../../app/ui";

/** AppTab verwaltet den Desktop-Store mit Sofortspeichern und Versionsprüfung. */
export function AppTab({
  desktop, updateStatus, busy, onOpenOnboarding, onToggleAutostart, onCheckUpdates, onOpenRelease,
}: {
  desktop: DesktopSettingsView | null;
  updateStatus: DesktopUpdateStatus | null;
  busy: boolean;
  onOpenOnboarding?: () => void;
  onToggleAutostart: (value: boolean) => Promise<void>;
  onCheckUpdates: () => void;
  onOpenRelease: () => void;
}) {
  const [savedFlash, setSavedFlash] = useState(false);

  const toggleAutostart = async (value: boolean) => {
    await onToggleAutostart(value);
    setSavedFlash(true);
    window.setTimeout(() => setSavedFlash(false), 1800);
  };

  return <div className="settings-tab-body settings-scope-app">
    <p className="settings-scope-line">Wird sofort gespeichert.</p>

    <section>
      <div className="section-heading">
        <div><h2>Start &amp; Einrichtung</h2><p>Lokale Desktopwerte, getrennt vom Core-Farming-Dokument.</p></div>
        <StatusBadge tone={desktop ? "success" : "neutral"}>{desktop ? "Lokal" : "Bridge fehlt"}</StatusBadge>
      </div>
      {desktop ? <div className="settings-form-width settings-grid">
        <label className="check">
          <input type="checkbox" checked={desktop.autostart} disabled={busy || !window.d2rDesktop} onChange={(event) => void toggleAutostart(event.target.checked)} />
          App mit Windows starten
          {savedFlash && <span className="settings-inline-saved" role="status">Gespeichert ✓</span>}
        </label>
        <p className="hint">Onboarding: <strong>{desktop.onboarding_completed ? "abgeschlossen" : "noch offen"}</strong></p>
        <p className="hint">Autostart ist standardmäßig aus und startet weder D2R noch eine Farming-Session. Fensterbounds verwaltet ausschließlich Electron Main.</p>
        {onOpenOnboarding && <Button variant="secondary" onClick={onOpenOnboarding}>First-Run-Assistent öffnen</Button>}
      </div> : <StateMessage kind="empty" title="Desktop-Bridge nicht aktiv">Im installierten Produkt werden hier Autostart und Onboarding aus `desktop-settings.json` verwaltet.</StateMessage>}
    </section>

    <section>
      <div className="section-heading">
        <div><h2>Version</h2><p>Manuelle Prüfung ohne automatischen Download.</p></div>
        <StatusBadge tone={updateStatus?.status === "available" ? "warning" : updateStatus?.status === "up_to_date" ? "success" : "neutral"}>{updateLabel(updateStatus)}</StatusBadge>
      </div>
      <p>{updateDescription(updateStatus)}</p>
      <div className="inline-actions">
        <Button variant="secondary" onClick={onCheckUpdates} disabled={busy || !window.d2rDesktop?.checkForUpdates}><RefreshCw aria-hidden="true" size={16} /> Erneut prüfen</Button>
        {updateStatus?.status === "available" && <Button onClick={onOpenRelease}>Feste Release-Seite öffnen</Button>}
      </div>
      <p className="hint">Es gibt keinen automatischen Download und keine Installation. Fehler, Offlinebetrieb und private/fehlende Releases bleiben neutral.</p>
    </section>
  </div>;
}

function updateLabel(status: DesktopUpdateStatus | null): string {
  if (!status) return "Desktop-Bridge fehlt";
  if (status.status === "checking") return "Prüfung läuft";
  if (status.status === "available") return "Neue Version";
  if (status.status === "up_to_date") return "Aktuell";
  return "Nicht verfügbar";
}

function updateDescription(status: DesktopUpdateStatus | null): string {
  if (!status) return "Der Versionshinweis ist nur in der Desktop-App verfügbar.";
  if (status.status === "checking") return `Version ${status.current_version} wird einmalig geprüft.`;
  if (status.status === "available") return `Installiert: ${status.current_version}. Veröffentlicht: ${status.latest_version}.`;
  if (status.status === "up_to_date") return `Installiert: ${status.current_version}. Kein neueres stabiles Release gefunden.`;
  return `Installiert: ${status.current_version}. Die Prüfung ist derzeit neutral nicht verfügbar.`;
}
