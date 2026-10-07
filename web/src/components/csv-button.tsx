import { DownloadSimpleIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { Button } from "@/components/ui/button"
import { type Cell, downloadCsv } from "@/lib/csv"

/** Downloads what a table lists, filtered and sorted as shown, as a CSV file for spreadsheets. */
export function CsvButton({ name, rows }: { name: string; rows: () => Cell[][] }) {
  return (
    <Button variant="outline" onClick={() => downloadCsv(name, rows())}>
      <DownloadSimpleIcon />
      <span className="max-sm:sr-only">{t("Export CSV")}</span>
    </Button>
  )
}
