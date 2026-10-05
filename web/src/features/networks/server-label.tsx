import { t } from "i18next"
import { StatusDot } from "@/components/status"
import { Skeleton } from "@/components/ui/skeleton"
import type { NodeServer } from "@/features/servers/api"
import { statusOf } from "@/features/servers/server-types"
import { msg } from "@/lib/i18n"

/**
 * A server of a network with its state and node. null means the servers are still
 * loading, undefined that the server's node can't be reached.
 */
export function ServerLabel({ server }: { server: NodeServer | null | undefined }) {
  if (server === null) return <Skeleton className="h-4 w-32" />
  if (!server) {
    return (
      <span className="inline-flex items-center gap-2 text-muted-foreground">
        <StatusDot status={{ tone: "neutral", label: msg("Unreachable") }} />
        {t("Unreachable")}
      </span>
    )
  }
  return (
    <span className="inline-flex min-w-0 items-center gap-2 whitespace-nowrap">
      <StatusDot status={statusOf(server)} label={t(statusOf(server).label)} />
      <span className="truncate font-medium">{server.name}</span>
      <span className="hidden truncate text-muted-foreground sm:inline">{server.nodeName}</span>
    </span>
  )
}
