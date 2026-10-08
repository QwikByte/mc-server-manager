import { locale } from "./i18n"

const units = ["B", "KiB", "MiB", "GiB", "TiB"]

const sizeFormat = new Intl.NumberFormat(locale, { maximumFractionDigits: 1 })

/** Formats a size in bytes using binary units in the viewer's locale, e.g. "31.3 GiB", "31,3 GiB" or "512 B". */
export function formatBytes(bytes: number): string {
  let value = bytes
  let unit = 0
  for (; value >= 1024 && unit < units.length - 1; unit++) value /= 1024
  return `${sizeFormat.format(value)} ${units[unit]}`
}

/** Formats a size in mebibytes, e.g. 2048 → "2 GiB", 512 → "512 MiB". */
export function formatMegabytes(mb: number): string {
  return formatBytes(mb * 1024 ** 2)
}

/** Formats an ISO timestamp as a date in the viewer's locale, e.g. "30 Dec 2026". */
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(locale, { dateStyle: "medium" })
}

/** Formats an ISO timestamp as date and time in the viewer's locale. */
export function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString(locale, { dateStyle: "medium", timeStyle: "short" })
}

/** Formats seconds in the viewer's locale, as minutes if they are whole ones, e.g. "30 seconds" or "2 minutes". */
export function formatSeconds(seconds: number): string {
  const [unit, value] = seconds % 60 === 0 ? ["minute", seconds / 60] : ["second", seconds]
  return new Intl.NumberFormat(locale, { style: "unit", unit, unitDisplay: "long" }).format(value)
}

/** Formats an IANA time zone as people read it, e.g. "America/New York"; empty is UTC. */
export const formatTimeZone = (zone: string) => (zone || "UTC").replaceAll("_", " ")

/** Formats a duration in its largest whole unit in the viewer's locale, e.g. "3 days" or "5 minutes". */
export function formatDuration(ms: number): string {
  const units = [
    ["day", 86_400_000],
    ["hour", 3_600_000],
    ["minute", 60_000],
  ] as const
  const [unit, size] = units.find(([, size]) => ms >= size) ?? units[2]
  return new Intl.NumberFormat(locale, { style: "unit", unit, unitDisplay: "long" }).format(Math.floor(ms / size))
}

/** Formats minutes as hours and minutes in the viewer's locale, e.g. "45 min" or "12 hr, 5 min". */
export function formatMinutes(minutes: number): string {
  const unit = (unit: string, value: number) => new Intl.NumberFormat(locale, { style: "unit", unit, unitDisplay: "short" }).format(value)
  const [h, m] = [Math.floor(minutes / 60), minutes % 60]
  if (h === 0) return unit("minute", m)
  return m === 0 ? unit("hour", h) : new Intl.ListFormat(locale, { style: "narrow", type: "unit" }).format([unit("hour", h), unit("minute", m)])
}

/** Formats how long something took as minutes and seconds, e.g. "1:05" or "1:02:03". */
export function formatElapsed(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000))
  const [h, m] = [Math.floor(s / 3600), Math.floor((s % 3600) / 60)]
  const pad = (n: number) => String(n).padStart(2, "0")
  return h > 0 ? `${h}:${pad(m)}:${pad(s % 60)}` : `${m}:${pad(s % 60)}`
}

/** Formats how long ago a time was in the viewer's locale, e.g. "3 minutes ago" or "yesterday". */
export function formatAgo(iso: string, now = Date.now()): string {
  const seconds = Math.round((Date.parse(iso) - now) / 1000)
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" })
  if (Math.abs(seconds) < 60) return rtf.format(seconds, "second")
  if (Math.abs(seconds) < 3600) return rtf.format(Math.round(seconds / 60), "minute")
  if (Math.abs(seconds) < 86_400) return rtf.format(Math.round(seconds / 3600), "hour")
  return rtf.format(Math.round(seconds / 86_400), "day")
}
