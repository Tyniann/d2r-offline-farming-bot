const onboardingStepQuery = "onboarding_step";

export function readOnboardingResumeStep(lastStep: number): number {
  const value = new URLSearchParams(window.location.search).get(onboardingStepQuery);
  const step = value === null ? Number.NaN : Number(value);
  return Number.isInteger(step) && step >= 0 && step <= lastStep ? step : 0;
}

export function prepareOnboardingResume(step: number): void {
  const url = new URL(window.location.href);
  url.searchParams.set(onboardingStepQuery, String(step));
  window.history.replaceState(null, "", url);
}

export function clearOnboardingResume(): void {
  const url = new URL(window.location.href);
  if (!url.searchParams.has(onboardingStepQuery)) return;
  url.searchParams.delete(onboardingStepQuery);
  window.history.replaceState(null, "", url);
}
