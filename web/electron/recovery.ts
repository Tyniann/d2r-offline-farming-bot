export function renderRecovery(targetDocument: Document, search: string): void {
  const parameters = new URLSearchParams(search);
  const language = parameters.get("language");
  const reason = parameters.get("reason");
  const restarts = parameters.get("restarts");
  const title = parameters.get("title");
  const body = parameters.get("body");

  if (language === "de" || language === "en") targetDocument.documentElement.lang = language;
  if (reason) targetDocument.body.dataset.reason = reason;
  if (restarts) targetDocument.body.dataset.restarts = restarts;
  if (title) {
    targetDocument.title = title;
    const heading = targetDocument.querySelector("#title");
    if (heading) heading.textContent = title;
  }
  if (body) {
    const target = targetDocument.querySelector("#reason");
    if (target) target.textContent = body;
  }
}

renderRecovery(document, location.search);
