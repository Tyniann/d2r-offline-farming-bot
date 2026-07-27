const onboardingStepQuery = "onboarding_step";

export function carryOnboardingStep(currentURL: string, targetURL: string, lastStep: number): string {
  if (!currentURL) return targetURL;
  try {
    const value = new URL(currentURL).searchParams.get(onboardingStepQuery);
    const step = value === null ? Number.NaN : Number(value);
    if (!Number.isInteger(step) || step < 0 || step > lastStep) return targetURL;
    const target = new URL(targetURL);
    target.searchParams.set(onboardingStepQuery, String(step));
    return target.toString();
  } catch {
    return targetURL;
  }
}
