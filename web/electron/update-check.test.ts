import { describe, expect, it, vi } from "vitest";
import { checkLatestRelease, githubLatestReleaseAPI } from "./update-check.js";

function response(status: number, body: unknown) {
  return { ok: status >= 200 && status < 300, json: async () => body };
}

describe("checkLatestRelease", () => {
  it.each([
    ["gleich", "1.2.3", "v1.2.3", "up_to_date"],
    ["älter", "1.2.3", "1.2.2", "up_to_date"],
    ["neuer", "1.2.3", "1.3.0", "available"],
  ])("%s", async (_name, current, latest, status) => {
    const fetcher = vi.fn(async (_input: string, _init: RequestInit) => response(200, { tag_name: latest, draft: false, prerelease: false }));
    await expect(checkLatestRelease(current, fetcher)).resolves.toMatchObject({ status });
    expect(fetcher).toHaveBeenCalledOnce();
    expect(fetcher.mock.calls[0][0]).toBe(githubLatestReleaseAPI);
  });

  it("lehnt Prereleases neutral ab", async () => {
    const fetcher = vi.fn(async () => response(200, { tag_name: "v1.3.0-beta.1", draft: false, prerelease: true }));
    await expect(checkLatestRelease("1.2.3", fetcher)).resolves.toEqual({
      status: "unavailable", current_version: "1.2.3", reason: "update_response_invalid",
    });
  });

  it.each([404, 403, 429])("behandelt HTTP %s neutral", async (status) => {
    await expect(checkLatestRelease("1.2.3", async () => response(status, {}))).resolves.toMatchObject({
      status: "unavailable", reason: "update_check_unavailable",
    });
  });

  it("behandelt Offlinefehler neutral", async () => {
    await expect(checkLatestRelease("1.2.3", async () => { throw new Error("offline"); })).resolves.toMatchObject({
      status: "unavailable", reason: "update_check_unavailable",
    });
  });

  it("bricht am Timeout neutral ab", async () => {
    const fetcher = (_input: string, init: RequestInit) => new Promise<never>((_resolve, reject) => {
      init.signal?.addEventListener("abort", () => reject(new Error("aborted")));
    });
    await expect(checkLatestRelease("1.2.3", fetcher, 1)).resolves.toMatchObject({
      status: "unavailable", reason: "update_check_unavailable",
    });
  });

  it("behandelt malformed JSON neutral", async () => {
    await expect(checkLatestRelease("1.2.3", async () => ({ ok: true, json: async () => { throw new SyntaxError(); } }))).resolves.toMatchObject({
      status: "unavailable", reason: "update_check_unavailable",
    });
  });

  it("lehnt strukturell ungültige Antworten ab", async () => {
    await expect(checkLatestRelease("1.2.3", async () => response(200, { tag_name: 123 }))).resolves.toMatchObject({
      status: "unavailable", reason: "update_response_invalid",
    });
  });
});
