import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState } from "react";
import { expect, it } from "vitest";
import { Dialog, StateMessage, StatusBadge } from "./ui";

function DialogHarness() {
  const [open, setOpen] = useState(false);
  return <>
    <button type="button" onClick={() => setOpen(true)}>Öffnen</button>
    {open && <Dialog title="Bestätigung" onClose={() => setOpen(false)}>
      <button type="button">Erste Aktion</button>
      <button type="button">Letzte Aktion</button>
    </Dialog>}
  </>;
}

it("fängt Dialogfokus ein, schließt per Escape und gibt Fokus zurück", async () => {
  render(<DialogHarness />);
  const opener = screen.getByRole("button", { name: "Öffnen" });
  opener.focus();
  fireEvent.click(opener);
  const first = await screen.findByRole("button", { name: "Erste Aktion" });
  const last = screen.getByRole("button", { name: "Letzte Aktion" });
  await waitFor(() => expect(first).toHaveFocus());
  last.focus();
  fireEvent.keyDown(window, { key: "Tab" });
  expect(first).toHaveFocus();
  fireEvent.keyDown(window, { key: "Escape" });
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(opener).toHaveFocus();
});

it("ergänzt Farbzustände immer um sichtbaren Text und dekorative Icons", () => {
  const { container } = render(<>
    <StatusBadge tone="danger">Core getrennt</StatusBadge>
    <StateMessage kind="error" title="Verbindung fehlgeschlagen">Erneut versuchen.</StateMessage>
  </>);
  expect(screen.getByText("Core getrennt")).toBeInTheDocument();
  expect(screen.getByRole("alert")).toHaveTextContent("Verbindung fehlgeschlagen");
  expect(container.querySelectorAll('svg[aria-hidden="true"]')).toHaveLength(1);
});
