const parameters = new URLSearchParams(location.search);
const reason = parameters.get("reason") ?? "core_recovery_required";
const restarts = parameters.get("restarts") ?? "0";
document.body.dataset.reason = reason;
document.body.dataset.restarts = restarts;
const target = document.querySelector("#reason");
if (target) target.textContent = `Der lokale Core ist nicht sicher verfügbar (${reason}). Automatische Neustarts: ${restarts}. Es wurden keine weiteren Bot-Aktionen gestartet.`;
