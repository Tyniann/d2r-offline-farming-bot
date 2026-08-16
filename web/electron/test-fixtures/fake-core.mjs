import { createConnection } from "node:net";
import { createServer } from "node:http";
import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { parseArgs } from "node:util";

const { values } = parseArgs({
  args: process.argv.slice(2),
  options: {
    mode: { type: "string", default: "valid" },
    "desktop-handshake-pipe": { type: "string" },
    "data-root": { type: "string" },
    "provision-data-root": { type: "boolean" },
    "defaults-root": { type: "string" },
    "import-root": { type: "string" },
  },
  strict: false,
});
const mode = String(values.mode);
if (mode === "start-fail") {
  process.stderr.write("error: pickit assignment invalid for MrHammer/countess: pickit_assignment_missing\n");
  process.exit(1);
}
const pipe = String(values["desktop-handshake-pipe"] ?? "");
let state = mode === "exit-active" || mode === "active-command" || mode === "steady-active" ? "running_run" : "idle";
let generation = 1;
let pendingIntent = "none";
let commandCount = 0;
const token = "a".repeat(43);

if (values["provision-data-root"]) {
  if (mode === "provision-fail") {
    process.stderr.write("error: data_import_conflict: Testziel ist nicht leer.\n");
    process.exit(1);
  }
  const root = String(values["data-root"] ?? "");
  await mkdir(join(root, "configs"), { recursive: true });
  await writeFile(join(root, "configs", "config.yaml"), "fake: true\n", "utf8");
  process.stdout.write(JSON.stringify({
    schema_version: 1,
    status: values["import-root"] ? "existing" : "published",
    diagnostic_count: values["import-root"] ? 1 : 0,
  }));
  process.exit(0);
}

const server = createServer((request, response) => {
  const path = request.url ?? "/";
  response.setHeader("Content-Security-Policy", "default-src 'self'; script-src 'self'; connect-src 'self'; style-src 'self'; object-src 'none'; base-uri 'none'");
  if (path.startsWith("/api/v1/status")) return json(response, { schema_version: 1, core_version: "fake", state, generation, pending_intent: pendingIntent, compatibility: { state: "compatible", privilege_mismatch: false } });
  if (path.startsWith("/api/test/commands")) return json(response, { count: commandCount, state, generation, pending_intent: pendingIntent });
  if (request.method === "POST" && path.startsWith("/api/v1/session/")) {
    commandCount++;
    const intent = path.endsWith("pause-after-run") ? "pause_after_run" : path.endsWith("stop-after-run") ? "stop_after_run" : "emergency_stop";
    pendingIntent = intent === "emergency_stop" ? "none" : intent;
    if (intent === "emergency_stop") state = "cancelling";
    generation++;
    const complete = () => json(response, { schema_version: 1, command_id: `fake-${commandCount}`, state, generation });
    if (mode === "active-command") setTimeout(complete, 150);
    else complete();
    return;
  }
  if (path.startsWith("/api/v1/routes/workflow")) return json(response, { schema_version: 1, generation: 1, state: "idle" });
  if (path.startsWith("/app.js")) {
    response.setHeader("Content-Type", "text/javascript; charset=utf-8");
    response.end("history.replaceState(null,'','/');fetch('/api/v1/status').then(r=>r.json()).then(s=>{document.querySelector('[data-testid=state]').textContent=s.state;document.body.dataset.snapshot='loaded'})");
    return;
  }
  response.setHeader("Content-Type", "text/html; charset=utf-8");
  response.end("<!doctype html><html lang=de><head><meta charset=utf-8><title>Fake Core</title></head><body><h1>Fake-Core-Snapshot</h1><div data-testid=state>wird geladen</div><script src=/app.js></script></body></html>");
});

server.listen(0, "127.0.0.1", () => {
  const address = server.address();
  if (!address || typeof address === "string") process.exit(2);
  const base = `http://127.0.0.1:${address.port}`;
  const handshake = {
    schema_version: 1,
    core_pid: mode === "wrong" ? process.pid + 1 : process.pid,
    generation: 1,
    base_url: base,
    bootstrap_url: `${base}/#control_token=${token}`,
  };
  const send = () => {
    const socket = createConnection(pipe, () => {
      if (mode === "aborted") socket.end();
      else socket.end(`${JSON.stringify(handshake)}\n`);
    });
  };
  if (mode === "delayed") setTimeout(send, 1_000);
  else send();
  if (mode === "exit-idle" || mode === "exit-active") setTimeout(() => process.exit(17), 650);
  if (mode === "steady-active") setTimeout(() => { state = "idle"; pendingIntent = "none"; generation++; }, 1_200);
});

process.on("SIGTERM", () => process.exit(0));
process.on("SIGINT", () => process.exit(0));

function json(response, value) {
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  response.end(JSON.stringify(value));
}
