/** Run-Availability-Texte für Dashboard und Settings-Queue-Editor. */

import { presentApiError, type AppTranslator } from "../i18n/presenters";

const reasonPriority = [
  "profile_class_mismatch",
  "character_profile_run_incompatible",
  "profile_run_strategy_unavailable",
  "route_assignment_missing",
  "route_set_binding_mismatch",
  "leg_acquisition_route_missing",
  "leg_acquisition_route_stale",
  "cow_sweep_route_missing",
  "cow_sweep_route_stale",
  "route_lifecycle_unavailable",
  "route_stale",
];

export function isRunStartable(status: string) {
  return status === "available" || status === "runtime_validation_required";
}

export function runAvailabilityText(status: string, reasons: string[] = [], _characterClass = "", t: AppTranslator) {
  if (status === "available") {
    return { title: t("reasons.readyTitle"), detail: t("reasons.readyDetail") };
  }
  if (status === "runtime_validation_required") {
    return { title: t("reasons.readyTitle"), detail: t("reasons.runtimeValidationDetail") };
  }
  const matched = reasonPriority.find((reason) => reasons.includes(reason));
  switch (matched) {
    case "profile_class_mismatch":
    case "character_profile_run_incompatible":
      return { title: t("reasons.unavailableTitle"), detail: t("reasons.routeBindingMismatch") };
    case "profile_run_strategy_unavailable":
      return { title: t("reasons.unavailableTitle"), detail: t("reasons.profileRunUnavailable") };
    case "route_assignment_missing":
      return { title: t("reasons.notConfiguredTitle"), detail: t("reasons.routeMissing") };
    case "leg_acquisition_route_missing":
      return { title: t("reasons.notConfiguredTitle"), detail: t("reasons.legRouteMissing") };
    case "leg_acquisition_route_stale":
      return { title: t("reasons.notReadyTitle"), detail: t("reasons.legRouteStale") };
    case "cow_sweep_route_missing":
      return { title: t("reasons.notConfiguredTitle"), detail: t("reasons.cowRouteMissing") };
    case "cow_sweep_route_stale":
      return { title: t("reasons.notReadyTitle"), detail: t("reasons.cowRouteStale") };
    case "route_set_binding_mismatch":
      return { title: t("reasons.unavailableTitle"), detail: t("reasons.cowRouteMismatch") };
    case "route_lifecycle_unavailable":
      return { title: t("reasons.notReadyTitle"), detail: t("reasons.routeUnavailable") };
    case "route_stale":
      return { title: t("reasons.notReadyTitle"), detail: t("reasons.routeStale") };
    default:
      return { title: t("reasons.notReadyTitle"), detail: t("reasons.fallback") };
  }
}

export function queueStartErrorText(reason: unknown, t: AppTranslator): string {
  return presentApiError(reason, t, t("app.sessionCommandFailed"));
}

export function selectionErrorText(reason: unknown, t: AppTranslator, fallback = t("app.selectionFailed")): string {
  return presentApiError(reason, t, fallback);
}
