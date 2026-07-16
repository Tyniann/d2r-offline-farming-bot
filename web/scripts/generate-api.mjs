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
  if (!response.ok) throw new Error(\`API-Abfrage fehlgeschlagen (\${response.status})\`);
  return response.json() as Promise<T>;
}

export function getStatus(signal?: AbortSignal): Promise<StatusDTO> {
  return getJSON<StatusDTO>("/api/v1/status", signal);
}

export function getCatalog(signal?: AbortSignal): Promise<CatalogDTO> {
  return getJSON<CatalogDTO>("/api/v1/catalog", signal);
}`;
const output = `// Code generated from internal/api/schema/openapi.json; DO NOT EDIT.\n\n${definitions.join("\n\n")}\n\n${client}\n`;
if (process.argv.includes("--check")) {
  const current = await readFile(outputPath, "utf8").catch(() => "");
  if (current !== output) throw new Error("src/api/generated.ts is stale; run pnpm generate");
} else {
  await writeFile(outputPath, output, "utf8");
}
