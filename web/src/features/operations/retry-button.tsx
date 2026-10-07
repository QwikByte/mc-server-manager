import { ArrowClockwiseIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { Button } from "@/components/ui/button"
import type { ServerRef } from "@/features/networks/api"
import { failedOf, type ServerResult } from "./retry"

/** Tries an action on many servers again with the same input, on those where it failed. */
export function RetryButton({ results, onRetry }: { results: ServerResult[]; onRetry: (servers: ServerRef[]) => void }) {
  const failed = failedOf(results)
  if (failed.length === 0) return null
  return (
    <Button variant="outline" onClick={() => onRetry(failed)}>
      <ArrowClockwiseIcon />
      {t("Retry the failed ones")}
    </Button>
  )
}
