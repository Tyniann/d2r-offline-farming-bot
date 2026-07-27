import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ProvisioningFeature } from "./ProvisioningFeature";

describe("ProvisioningFeature", () => {
  const chooseImportRoot = vi.fn();
  const provision = vi.fn();

  beforeEach(() => {
    chooseImportRoot.mockResolvedValue({ selected: true, label: "Bestehender Datenroot ausgewählt" });
    provision.mockResolvedValue({ schema_version: 1, status: "published", diagnostic_count: 0 });
    window.d2rDesktop = { chooseImportRoot, provision } as unknown as D2RDesktopBridge;
  });
  afterEach(() => { cleanup(); delete window.d2rDesktop; vi.clearAllMocks(); });

  it("bietet vor Core-Start ausschließlich Neu oder Import an", async () => {
    render(<ProvisioningFeature />);
    fireEvent.click(screen.getByRole("button", { name: "Neuen Datenroot anlegen" }));
    await waitFor(() => expect(provision).toHaveBeenCalledWith({ mode: "new" }));
    expect(screen.queryByText("Queue prüfen und starten")).not.toBeInTheDocument();
  });

  it("gibt keinen Importpfad an React und importiert erst nach nativer Auswahl", async () => {
    render(<ProvisioningFeature />);
    const submit = screen.getByRole("button", { name: "Ausgewählten Datenroot importieren" });
    expect(submit).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Bestehenden Datenroot auswählen" }));
    expect(await screen.findByText("Bestehender Datenroot ausgewählt")).toBeInTheDocument();
    fireEvent.click(submit);
    await waitFor(() => expect(provision).toHaveBeenCalledWith({ mode: "import" }));
  });
});
