import { ArrowsClockwiseIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { formatDateTime } from "@/lib/format"
import { updateQuery, useUpdateAction } from "./api"

/** When the master last looked for a new release and what it found, with a button to look now. */
export function UpdateCheck() {
  const { data } = useQuery(updateQuery)
  const check = useUpdateAction("check")
  if (!data) return null
  const found = data.checkError
    ? t("The check failed: {{error}}", { error: data.checkError })
    : data.latest
      ? t("{{version}} is available.", { version: data.latest.version })
      : t("{{version}} is the latest version.", { version: data.version })
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-muted/50 px-4 py-3 text-sm">
      <p className="min-w-0">
        {data.checkedAt ? `${t("Checked {{time}}.", { time: formatDateTime(data.checkedAt) })} ${found}` : t("Not checked yet.")}
      </p>
      <Button
        type="button"
        size="sm"
        variant="outline"
        disabled={check.isPending}
        onClick={() => check.mutate(undefined, { onError: (e) => toast.error(e.message) })}
      >
        <ArrowsClockwiseIcon />
        {check.isPending ? t("Checking…") : t("Check now")}
      </Button>
    </div>
  )
}
