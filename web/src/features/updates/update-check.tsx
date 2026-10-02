import { ArrowsClockwiseIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
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
    ? `The check failed: ${data.checkError}`
    : data.latest
      ? `${data.latest.version} is available.`
      : `${data.version} is the latest version.`
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-muted/50 px-4 py-3 text-sm">
      <p className="min-w-0">{data.checkedAt ? `Checked ${formatDateTime(data.checkedAt)}. ${found}` : "Not checked yet."}</p>
      <Button
        type="button"
        size="sm"
        variant="outline"
        disabled={check.isPending}
        onClick={() => check.mutate(undefined, { onError: (e) => toast.error(e.message) })}
      >
        <ArrowsClockwiseIcon />
        {check.isPending ? "Checking…" : "Check now"}
      </Button>
    </div>
  )
}
