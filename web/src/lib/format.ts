const units = ["B", "KB", "MB", "GB", "TB"]

/** Formats a size in bytes using binary units, e.g. "31.3 GB" or "512 B". */
export function formatBytes(bytes: number): string {
  let value = bytes
  let unit = 0
  for (; value >= 1024 && unit < units.length - 1; unit++) value /= 1024
  return `${Number.isInteger(value) ? value : value.toFixed(1)} ${units[unit]}`
}

/** Formats a size in megabytes, e.g. 2048 → "2 GB", 512 → "512 MB". */
export function formatMegabytes(mb: number): string {
  return formatBytes(mb * 1024 ** 2)
}

/** Formats an ISO timestamp as a date in the viewer's locale, e.g. "30 Dec 2026". */
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { dateStyle: "medium" })
}

/** Formats an ISO timestamp as date and time in the viewer's locale. */
export function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" })
}

/** Formats a duration in its largest whole unit in the viewer's locale, e.g. "3 days" or "5 minutes". */
export function formatDuration(ms: number): string {
  const units = [
    ["day", 86_400_000],
    ["hour", 3_600_000],
    ["minute", 60_000],
  ] as const
  const [unit, size] = units.find(([, size]) => ms >= size) ?? units[2]
  return new Intl.NumberFormat(undefined, { style: "unit", unit, unitDisplay: "long" }).format(Math.floor(ms / size))
}
