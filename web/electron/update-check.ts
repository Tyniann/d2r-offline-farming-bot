export const githubLatestReleaseAPI = "https://api.github.com/repos/Tyniann/d2r-offline-farming-bot/releases/latest";
export const githubReleasesURL = "https://github.com/Tyniann/d2r-offline-farming-bot/releases";

export type DesktopUpdateStatus =
  | { status: "checking"; current_version: string }
  | { status: "up_to_date"; current_version: string; latest_version: string }
  | { status: "available"; current_version: string; latest_version: string }
  | { status: "unavailable"; current_version: string; reason: "update_check_unavailable" | "update_response_invalid" };

type FetchLike = (input: string, init: RequestInit) => Promise<Pick<Response, "ok" | "json">>;

const stableSemVer = /^(?:v)?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;

export async function checkLatestRelease(
  currentVersion: string,
  fetcher: FetchLike = fetch,
  timeoutMs = 4_000,
): Promise<DesktopUpdateStatus> {
  const current = parseStableVersion(currentVersion);
  if (!current) return { status: "unavailable", current_version: currentVersion, reason: "update_response_invalid" };

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetcher(githubLatestReleaseAPI, {
      method: "GET",
      redirect: "error",
      cache: "no-store",
      signal: controller.signal,
      headers: { Accept: "application/vnd.github+json" },
    });
    if (!response.ok) {
      return { status: "unavailable", current_version: currentVersion, reason: "update_check_unavailable" };
    }
    const body = await response.json() as { tag_name?: unknown; draft?: unknown; prerelease?: unknown };
    if (body.draft !== false || body.prerelease !== false || typeof body.tag_name !== "string") {
      return { status: "unavailable", current_version: currentVersion, reason: "update_response_invalid" };
    }
    const latest = parseStableVersion(body.tag_name);
    if (!latest) {
      return { status: "unavailable", current_version: currentVersion, reason: "update_response_invalid" };
    }
    const latestVersion = latest.join(".");
    return compareVersions(latest, current) > 0
      ? { status: "available", current_version: currentVersion, latest_version: latestVersion }
      : { status: "up_to_date", current_version: currentVersion, latest_version: latestVersion };
  } catch {
    return { status: "unavailable", current_version: currentVersion, reason: "update_check_unavailable" };
  } finally {
    clearTimeout(timeout);
  }
}

function parseStableVersion(value: string): [number, number, number] | undefined {
  const match = stableSemVer.exec(value.trim());
  if (!match) return undefined;
  return [Number(match[1]), Number(match[2]), Number(match[3])];
}

function compareVersions(left: [number, number, number], right: [number, number, number]): number {
  for (let index = 0; index < left.length; index += 1) {
    if (left[index] !== right[index]) return left[index] - right[index];
  }
  return 0;
}
