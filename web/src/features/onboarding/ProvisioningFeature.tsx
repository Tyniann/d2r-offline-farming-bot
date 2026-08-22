import { useState } from "react";
import { FolderInput, ShieldCheck, Sparkles } from "lucide-react";
import { Button, StateMessage } from "../../app/ui";
import { useTranslation } from "react-i18next";
import { presentApiError } from "../../i18n/presenters";

export function ProvisioningFeature() {
  const { t } = useTranslation();
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
      setError(presentApiError(reason, t, t("provisioning.failedFallback")));
      setPending(false);
    }
  }

  return <main className="provisioning-shell">
    <header className="provisioning-header">
      <img src="./portal-mark.png" width="62" height="62" alt="" />
      <p className="eyebrow">{t("provisioning.eyebrow")}</p>
      <h1>{t("provisioning.title")}</h1>
      <p>{t("provisioning.description")}</p>
    </header>
    {error && <StateMessage kind="error" title={t("provisioning.failed")}>{error}</StateMessage>}
    <div className="provisioning-options">
      <article>
        <Sparkles aria-hidden="true" />
        <h2>{t("provisioning.newTitle")}</h2>
        <p>{t("provisioning.newDetail")}</p>
        <Button disabled={pending} onClick={() => void provision("new")}>{t(pending ? "provisioning.checking" : "provisioning.create")}</Button>
      </article>
      <article>
        <FolderInput aria-hidden="true" />
        <h2>{t("provisioning.importTitle")}</h2>
        <p>{t("provisioning.importDetail")}</p>
        <Button variant="secondary" disabled={pending} onClick={() => void chooseImport()}>{t("provisioning.choose")}</Button>
        {importSelected && <p className="success-text"><ShieldCheck aria-hidden="true" size={17} /> {importLabel}</p>}
        <Button disabled={pending || !importSelected} onClick={() => void provision("import")}>{t("provisioning.import")}</Button>
      </article>
    </div>
    <p className="hint">{t("provisioning.hint")}</p>
  </main>;
}
