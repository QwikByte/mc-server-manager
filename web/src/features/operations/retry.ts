import { t } from "i18next"
import type { ServerRef } from "@/features/networks/api"
import { key } from "@/features/networks/servers"
import type { Done } from "./use-operation"

/** How an action on many servers ended on one; failed ones have an error. */
export type ServerResult = ServerRef & { error?: string }

/** The servers on which an action on many servers failed. */
export const failedOf = (results: ServerResult[]): ServerRef[] =>
  results.filter((r) => r.error).map(({ nodeId, serverId }) => ({ nodeId, serverId }))

/** The results of an action on many servers, with those of trying it again in place of the earlier ones. */
export function mergeResults<T extends ServerRef>(earlier: T[] | undefined, retried: T[]): T[] {
  const byKey = new Map(retried.map((r) => [key(r), r]))
  return earlier ? earlier.map((r) => byKey.get(key(r)) ?? r) : retried
}

/** The action of a notification that tries an action on many servers again, on those where it failed. */
export function retryAction(results: ServerResult[], onRetry: (servers: ServerRef[]) => void): Done["action"] {
  const failed = failedOf(results)
  return failed.length > 0 ? { label: t("Retry the failed ones"), onClick: () => onRetry(failed) } : undefined
}
