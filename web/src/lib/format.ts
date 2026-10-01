/** Formats a size in megabytes, e.g. 2048 → "2 GB", 512 → "512 MB". */
export function formatMegabytes(mb: number): string {
  if (mb < 1024) return `${mb} MB`
  const [value, unit] = mb < 1024 ** 2 ? [mb / 1024, "GB"] : [mb / 1024 ** 2, "TB"]
  return `${Number.isInteger(value) ? value : value.toFixed(1)} ${unit}`
}

/** Formats a size in bytes using binary units, e.g. "31.3 GB". */
export function formatBytes(bytes: number): string {
  return formatMegabytes(Math.round(bytes / 2 ** 20))
}

/** Formats an ISO timestamp as a date in the viewer's locale, e.g. "30 Dec 2026". */
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { dateStyle: "medium" })
}
