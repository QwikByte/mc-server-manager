import { dayOf } from "./format"
import { locale } from "./i18n"

/** A cell of a CSV file; a missing value is left empty. */
export type Cell = string | number | undefined

/** How CSV files separate their cells, and whether they start with a byte order mark, which Excel needs to read UTF-8. */
export interface CsvFormat {
  separator: "comma" | "semicolon"
  bom: boolean
}

/** The separator spreadsheets of the panel's language expect: semicolons where the decimal separator is a comma. */
export const languageSeparator: CsvFormat["separator"] =
  new Intl.NumberFormat(locale).formatToParts(1.5).find((p) => p.type === "decimal")?.value === "," ? "semicolon" : "comma"

/**
 * Keeps spreadsheets from running a cell as a formula, which a user, an agent or a game server
 * could otherwise slip into a name; the export of the log does the same.
 */
const spreadsheetSafe = (cell: string) => (/^[=+\-@\t\r]/.test(cell) ? `'${cell}` : cell)

/** Quotes a cell where Go's encoding/csv does, which writes the export of the log: also if it holds the separator. */
const quote = (cell: string, separator: string) =>
  /["\r\n]|^\s|^\\\.$/.test(cell) || cell.includes(separator) ? `"${cell.replaceAll('"', '""')}"` : cell

/** Writes rows as the export of the log does: UTF-8, lines ending in \n. */
function csv(rows: Cell[][], { separator, bom }: CsvFormat) {
  const sep = separator === "semicolon" ? ";" : ","
  return (bom ? "\uFEFF" : "") + rows.map((row) => `${row.map((c) => quote(spreadsheetSafe(String(c ?? "")), sep)).join(sep)}\n`).join("")
}

/** Downloads rows as a CSV file named after what they are and today, e.g. noryx-servers-2026-10-07.csv. */
export function downloadCsv(name: string, rows: Cell[][], format: CsvFormat) {
  const url = URL.createObjectURL(new Blob([csv(rows, format)], { type: "text/csv;charset=utf-8" }))
  Object.assign(document.createElement("a"), { href: url, download: `noryx-${name}-${dayOf(Date.now())}.csv` }).click()
  setTimeout(() => URL.revokeObjectURL(url))
}
