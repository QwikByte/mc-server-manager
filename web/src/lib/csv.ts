/** A cell of a CSV file; a missing value is left empty. */
export type Cell = string | number | undefined

/**
 * Keeps spreadsheets from running a cell as a formula, which a user, an agent or a game server
 * could otherwise slip into a name; the export of the log does the same.
 */
const spreadsheetSafe = (cell: string) => (/^[=+\-@\t\r]/.test(cell) ? `'${cell}` : cell)

/** Quotes a cell where Go's encoding/csv does, which writes the export of the log. */
const quote = (cell: string) => (/[",\r\n]|^\s|^\\\.$/.test(cell) ? `"${cell.replaceAll('"', '""')}"` : cell)

/** Writes rows as the export of the log does: UTF-8 without a byte order mark, lines ending in \n. */
const csv = (rows: Cell[][]) => rows.map((row) => `${row.map((c) => quote(spreadsheetSafe(String(c ?? "")))).join(",")}\n`).join("")

/** Downloads rows as a CSV file named after what they are and today, e.g. noryx-servers-2026-10-07.csv. */
export function downloadCsv(name: string, rows: Cell[][]) {
  const now = new Date()
  const date = [now.getFullYear(), now.getMonth() + 1, now.getDate()].map((n) => String(n).padStart(2, "0")).join("-")
  const url = URL.createObjectURL(new Blob([csv(rows)], { type: "text/csv;charset=utf-8" }))
  Object.assign(document.createElement("a"), { href: url, download: `noryx-${name}-${date}.csv` }).click()
  setTimeout(() => URL.revokeObjectURL(url))
}
