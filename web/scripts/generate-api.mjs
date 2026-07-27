import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const schemaPath = path.resolve(root, "../internal/api/schema/openapi.json");
const outputPath = path.resolve(root, "src/api/generated.ts");
const schema = JSON.parse(await readFile(schemaPath, "utf8"));

function typeOf(value) {
  if (value.$ref) return value.$ref.split("/").at(-1);
  if (value.enum) return value.enum.map((entry) => JSON.stringify(entry)).join(" | ");
  if (value.type === "array") return `Array<${typeOf(value.items)}>`;
  if (value.type === "integer" || value.type === "number") return "number";
  if (value.type === "boolean") return "boolean";
  if (value.type === "object" && typeof value.additionalProperties === "object") return `Record<string, ${typeOf(value.additionalProperties)}>`;
  if (value.type === "object") return "Record<string, unknown>";
  return "string";
}

const definitions = Object.entries(schema.components.schemas).map(([name, definition]) => {
  const required = new Set(definition.required ?? []);
  const fields = Object.entries(definition.properties ?? {}).map(([field, value]) =>
    `  ${field}${required.has(field) ? "" : "?"}: ${typeOf(value)};`,
  );
  return `export interface ${name} {\n${fields.join("\n")}\n}`;
});

const client = `export const API_VERSION = "v1" as const;

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) { const error = await response.json().catch(() => null) as { message?: string } | null; throw new Error(error?.message ?? \`API-Abfrage fehlgeschlagen (\${response.status})\`); }
  return response.json() as Promise<T>;
}

async function sendJSON<T>(path: string, method: string, body: unknown, token = "", signal?: AbortSignal): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json", "Content-Type": "application/json" };
  if (token) headers["X-D2RBot-Control-Token"] = token;
  const response = await fetch(path, { method, body: JSON.stringify(body), signal, headers });
  if (!response.ok) { const error = await response.json().catch(() => null) as { message?: string } | null; throw new Error(error?.message ?? \`API-Mutation fehlgeschlagen (\${response.status})\`); }
  return response.status === 204 ? (undefined as T) : response.json() as Promise<T>;
}

export function getStatus(signal?: AbortSignal): Promise<StatusDTO> {
  return getJSON<StatusDTO>("/api/v1/status", signal);
}

export function getCatalog(signal?: AbortSignal): Promise<CatalogDTO> {
  return getJSON<CatalogDTO>("/api/v1/catalog", signal);
}

export function getOperatorSettings(signal?: AbortSignal): Promise<OperatorSettingsDTO> {
  return getJSON<OperatorSettingsDTO>("/api/v1/settings/operator", signal);
}

export function previewOperatorSettings(request: OperatorSettingsMutationRequest, signal?: AbortSignal): Promise<OperatorSettingsChangeDTO> {
  return sendJSON<OperatorSettingsChangeDTO>("/api/v1/settings/operator/preview", "POST", request, "", signal);
}

export function updateOperatorSettings(request: OperatorSettingsMutationRequest, token: string, signal?: AbortSignal): Promise<OperatorSettingsChangeDTO> {
  return sendJSON<OperatorSettingsChangeDTO>("/api/v1/settings/operator", "PUT", request, token, signal);
}

export function resetOperatorSettings(request: OperatorSettingsResetRequest, token: string, signal?: AbortSignal): Promise<OperatorSettingsChangeDTO> {
  return sendJSON<OperatorSettingsChangeDTO>("/api/v1/settings/operator/reset", "POST", request, token, signal);
}

export function previewResetOperatorSettings(request: OperatorSettingsResetRequest, signal?: AbortSignal): Promise<OperatorSettingsChangeDTO> {
  return sendJSON<OperatorSettingsChangeDTO>("/api/v1/settings/operator/reset/preview", "POST", request, "", signal);
}

export interface HistoryQuery {
  from?: string;
  to?: string;
  timezone?: string;
  run?: string[];
  character?: string[];
  difficulty?: string[];
  outcome?: string[];
  reason?: string[];
  pickit_profile?: string[];
  sort?: "keep_per_hour" | "success_rate" | "average_duration";
  limit?: number;
  cursor?: string;
}

function historyQuery(query: HistoryQuery = {}): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === "" || Array.isArray(value) && value.length === 0) continue;
    if (Array.isArray(value)) value.forEach((entry) => params.append(key, entry));
    else params.set(key, String(value));
  }
  const encoded = params.toString();
  return encoded ? "?" + encoded : "";
}

export function getHistorySummary(query: HistoryQuery = {}, signal?: AbortSignal): Promise<HistorySummaryResponse> { return getJSON<HistorySummaryResponse>("/api/v1/history/summary" + historyQuery(query), signal); }
export function getHistoryComparisons(query: HistoryQuery = {}, signal?: AbortSignal): Promise<HistoryComparisonsResponse> { return getJSON<HistoryComparisonsResponse>("/api/v1/history/comparisons" + historyQuery(query), signal); }
export function getHistoryItems(query: HistoryQuery = {}, signal?: AbortSignal): Promise<HistoryItemsResponse> { return getJSON<HistoryItemsResponse>("/api/v1/history/items" + historyQuery(query), signal); }
export function getHistoryRuns(query: HistoryQuery = {}, signal?: AbortSignal): Promise<HistoryRunsResponse> { return getJSON<HistoryRunsResponse>("/api/v1/history/runs" + historyQuery(query), signal); }
export function getHistoryRun(runID: string, includeRaw = false, signal?: AbortSignal): Promise<HistoryRunDetailResponse> { return getJSON<HistoryRunDetailResponse>(\`/api/v1/history/runs/\${encodeURIComponent(runID)}?include_raw=\${includeRaw}\`, signal); }
export function getHistoryExportURL(format: "json" | "csv", dataset: "" | "runs" | "items" = "", query: HistoryQuery = {}): string {
  const suffix = historyQuery(query);
  const separator = suffix ? "&" : "?";
  return "/api/v1/history/export" + suffix + separator + new URLSearchParams({ format, ...(dataset ? { dataset } : {}) }).toString();
}
export async function downloadHistoryExport(format: "json" | "csv", dataset: "" | "runs" | "items" = "", query: HistoryQuery = {}, signal?: AbortSignal): Promise<{ blob: Blob; filename: string }> {
  const response = await fetch(getHistoryExportURL(format, dataset, query), { signal, headers: { Accept: format === "json" ? "application/json" : "text/csv" } });
  if (!response.ok) { const error = await response.json().catch(() => null) as { message?: string } | null; throw new Error(error?.message ?? \`Historienexport fehlgeschlagen (\${response.status})\`); }
  const disposition = response.headers.get("Content-Disposition") ?? "";
  const filename = disposition.match(/filename="([A-Za-z0-9._-]+)"/)?.[1] ?? \`d2r-history.\${format}\`;
  return { blob: await response.blob(), filename };
}

export function previewHistoryDeleteAll(request: HistoryDeletePreviewRequest, token: string, signal?: AbortSignal): Promise<HistoryDeletePreviewDTO> {
  return sendJSON<HistoryDeletePreviewDTO>("/api/v1/history/delete-all/preview", "POST", request, token, signal);
}
export function confirmHistoryDeleteAll(request: HistoryDeleteConfirmRequest, token: string, signal?: AbortSignal): Promise<HistoryDeleteResultDTO> {
  return sendJSON<HistoryDeleteResultDTO>("/api/v1/history/delete-all/confirm", "POST", request, token, signal);
}
export function createDiagnosticBundle(request: DiagnosticBundleRequest, token: string, signal?: AbortSignal): Promise<DiagnosticBundleDTO> {
  return sendJSON<DiagnosticBundleDTO>("/api/v1/diagnostics/bundle", "POST", request, token, signal);
}

export function getPickitCatalog(signal?: AbortSignal): Promise<PickitCatalogDTO> { return getJSON<PickitCatalogDTO>("/api/v1/pickit/catalog", signal); }
export function getPickitProfiles(signal?: AbortSignal): Promise<PickitProfilesDTO> { return getJSON<PickitProfilesDTO>("/api/v1/pickit/profiles", signal); }
export function validatePickitProfile(request: PickitValidationRequest, signal?: AbortSignal): Promise<PickitValidationDTO> { return sendJSON<PickitValidationDTO>("/api/v1/pickit/profiles/validate", "POST", request, "", signal); }
export function previewPickit(request: PickitPreviewRequest, signal?: AbortSignal): Promise<PickitPreviewDTO> { return sendJSON<PickitPreviewDTO>("/api/v1/pickit/preview", "POST", request, "", signal); }
export function createPickitProfile(request: PickitCreateRequest, token: string, signal?: AbortSignal): Promise<PickitProfileDTO> { return sendJSON<PickitProfileDTO>("/api/v1/pickit/profiles", "POST", request, token, signal); }
export function updatePickitProfile(id: string, request: PickitUpdateRequest, token: string, signal?: AbortSignal): Promise<PickitProfileDTO> { return sendJSON<PickitProfileDTO>(\`/api/v1/pickit/profiles/\${encodeURIComponent(id)}\`, "PUT", request, token, signal); }
export function duplicatePickitProfile(id: string, request: PickitDuplicateRequest, token: string, signal?: AbortSignal): Promise<PickitProfileDTO> { return sendJSON<PickitProfileDTO>(\`/api/v1/pickit/profiles/\${encodeURIComponent(id)}/duplicate\`, "POST", request, token, signal); }
export function deletePickitProfile(id: string, request: PickitDeleteRequest, token: string, signal?: AbortSignal): Promise<void> { return sendJSON<void>(\`/api/v1/pickit/profiles/\${encodeURIComponent(id)}\`, "DELETE", request, token, signal); }
export function getPickitAssignments(signal?: AbortSignal): Promise<PickitAssignmentsDTO> { return getJSON<PickitAssignmentsDTO>("/api/v1/pickit/assignments", signal); }
export function updatePickitAssignment(request: PickitAssignmentUpdateRequest, token: string, signal?: AbortSignal): Promise<PickitAssignmentsDTO> { return sendJSON<PickitAssignmentsDTO>("/api/v1/pickit/assignments", "PUT", request, token, signal); }
export function importPickit(request: PickitImportRequest, signal?: AbortSignal): Promise<PickitImportDTO> { return sendJSON<PickitImportDTO>("/api/v1/pickit/import", "POST", request, "", signal); }
export function exportPickitProfile(id: string, signal?: AbortSignal): Promise<PickitExportDTO> { return getJSON<PickitExportDTO>(\`/api/v1/pickit/profiles/\${encodeURIComponent(id)}/export\`, signal); }

export function getRouteLibrary(character = "", archived = false, signal?: AbortSignal): Promise<RouteLibraryDTO> {
  const query = new URLSearchParams({ character, include_archived: archived ? "true" : "false" });
  return getJSON<RouteLibraryDTO>("/api/v1/routes?" + query.toString(), signal);
}

export function getRecordingOptions(signal?: AbortSignal): Promise<Array<RecordingOptionDTO>> {
  return getJSON<Array<RecordingOptionDTO>>("/api/v1/route-recording/options", signal);
}

export function getRouteCandidates(signal?: AbortSignal): Promise<Array<RouteCandidateDTO>> {
  return getJSON<Array<RouteCandidateDTO>>("/api/v1/routes/candidates", signal);
}

export function getSystemRouteStatus(signal?: AbortSignal): Promise<Array<SystemRouteStatusDTO>> {
  return getJSON<Array<SystemRouteStatusDTO>>("/api/v1/system-routes/status", signal);
}

export function getHotkeyHelp(signal?: AbortSignal): Promise<HotkeyHelpDTO> {
  return getJSON<HotkeyHelpDTO>("/api/v1/routes/hotkeys", signal);
}

export function getRouteWorkflow(signal?: AbortSignal): Promise<RouteWorkflowDTO> {
  return getJSON<RouteWorkflowDTO>("/api/v1/routes/workflow", signal);
}`;
const output = `// Code generated from internal/api/schema/openapi.json; DO NOT EDIT.\n\n${definitions.join("\n\n")}\n\n${client}\n`;
if (process.argv.includes("--check")) {
  const current = await readFile(outputPath, "utf8").catch(() => "");
  if (current !== output) throw new Error("src/api/generated.ts is stale; run pnpm generate");
} else {
  await writeFile(outputPath, output, "utf8");
}
