export function apiError(code: string, params: Record<string, unknown> = {}, status = 409) {
  return Object.assign(new Error(code), { code, params, requestId: "test-request", status });
}
