/** Formats a size in megabytes, e.g. 2048 → "2 GB", 512 → "512 MB". */
export function formatMegabytes(mb: number): string {
  if (mb < 1024) return `${mb} MB`
  const gb = mb / 1024
  return `${Number.isInteger(gb) ? gb : gb.toFixed(1)} GB`
}

/** Formats a size in bytes using binary units, e.g. "31.3 GB". */
export function formatBytes(bytes: number): string {
  return formatMegabytes(Math.round(bytes / 2 ** 20))
}

/** Formats an ISO timestamp as a date in the viewer's locale, e.g. "30 Dec 2026". */
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { dateStyle: "medium" })
}
