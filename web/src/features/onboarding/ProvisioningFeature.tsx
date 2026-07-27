import { useState } from "react";
import { FolderInput, ShieldCheck, Sparkles } from "lucide-react";
import { Button, StateMessage } from "../../app/ui";

export function ProvisioningFeature() {
  const bridge = window.d2rDesktop;
  const [importSelected, setImportSelected] = useState(false);
  const [importLabel, setImportLabel] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  async function chooseImport() {
    if (!bridge || pending) return;
    setError("");
    const result = await bridge.chooseImportRoot();
    setImportSelected(result.selected);
    setImportLabel(result.label);
  }

  async function provision(mode: "new" | "import") {
    if (!bridge || pending) return;
    setPending(true);
    setError("");
    try {
      await bridge.provision({ mode });
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Der Datenroot konnte nicht angelegt werden.");
      setPending(false);
    }
  }

  return <main className="provisioning-shell">
    <header className="provisioning-header">
      <img src="./portal-mark.svg" width="62" height="62" alt="" />
      <p className="eyebrow">Erster Start</p>
      <h1>Datenbasis einrichten</h1>
      <p>Bevor der produktive Core startet, braucht die App genau einen vollständig validierten lokalen Datenroot.</p>
    </header>
    {error && <StateMessage kind="error" title="Einrichtung fehlgeschlagen">{error}</StateMessage>}
    <div className="provisioning-options">
      <article>
        <Sparkles aria-hidden="true" />
        <h2>Neu beginnen</h2>
        <p>Legt sichere Standardkonfigurationen an. Es wird weder D2R noch eine Farming-Session gestartet.</p>
        <Button disabled={pending} onClick={() => void provision("new")}>{pending ? "Daten werden geprüft …" : "Neuen Datenroot anlegen"}</Button>
      </article>
      <article>
        <FolderInput aria-hidden="true" />
        <h2>Bestehende Daten importieren</h2>
        <p>Übernimmt einen geschlossenen vorhandenen Stand atomar. Die ausgewählte Quelle bleibt unverändert.</p>
        <Button variant="secondary" disabled={pending} onClick={() => void chooseImport()}>Bestehenden Datenroot auswählen</Button>
        {importSelected && <p className="success-text"><ShieldCheck aria-hidden="true" size={17} /> {importLabel}</p>}
        <Button disabled={pending || !importSelected} onClick={() => void provision("import")}>Ausgewählten Datenroot importieren</Button>
      </article>
    </div>
    <p className="hint">Die Auswahl wird von Electron entgegengenommen. Anlage, Import, Validierung und Veröffentlichung führt ausschließlich der kurzlebige Go-Core aus.</p>
  </main>;
}
