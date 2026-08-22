import { i18n } from "./index";
import { isSupportedLanguage, localeForLanguage, type SupportedLanguage } from "./types";

function activeLanguage(): SupportedLanguage {
  return isSupportedLanguage(i18n.resolvedLanguage) ? i18n.resolvedLanguage : "de";
}

function activeLocale(): "de-DE" | "en-US" {
  return localeForLanguage(activeLanguage());
}

export function formatDate(
  value: Date | number | string,
  options: Intl.DateTimeFormatOptions = { dateStyle: "medium" },
): string {
  return new Intl.DateTimeFormat(activeLocale(), options).format(new Date(value));
}

export function formatNumber(value: number, options?: Intl.NumberFormatOptions): string {
  return new Intl.NumberFormat(activeLocale(), options).format(value);
}

export function formatPercent(value: number, options?: Intl.NumberFormatOptions): string {
  return formatNumber(value, { style: "percent", maximumFractionDigits: 1, ...options });
}

export function formatBytes(value: number): string {
  const absolute = Math.abs(value);
  const units = ["B", "KB", "MB", "GB", "TB"] as const;
  const unitIndex = absolute === 0 ? 0 : Math.min(Math.floor(Math.log(absolute) / Math.log(1024)), units.length - 1);
  const scaled = value / (1024 ** unitIndex);
  return `${formatNumber(scaled, { maximumFractionDigits: unitIndex === 0 ? 0 : 1 })} ${units[unitIndex]}`;
}

export function formatDuration(milliseconds: number): string {
  const totalSeconds = Math.max(0, Math.round(milliseconds / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  const locale = activeLocale();
  const parts: string[] = [];
  if (hours > 0) parts.push(new Intl.NumberFormat(locale, { style: "unit", unit: "hour", unitDisplay: "short" }).format(hours));
  if (minutes > 0) parts.push(new Intl.NumberFormat(locale, { style: "unit", unit: "minute", unitDisplay: "short" }).format(minutes));
  if (seconds > 0 || parts.length === 0) parts.push(new Intl.NumberFormat(locale, { style: "unit", unit: "second", unitDisplay: "short" }).format(seconds));
  return parts.join(" ");
}
