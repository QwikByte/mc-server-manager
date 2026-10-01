import { Lamp } from "@/components/lamp"
import { Skeleton } from "@/components/ui/skeleton"
import type { NodeServer } from "@/features/servers/api"
import { serverStates } from "@/features/servers/server-types"

/**
 * A server of a network with its state and node. null means the servers are still
 * loading, undefined that the server's node can't be reached.
 */
export function ServerLabel({ server }: { server: NodeServer | null | undefined }) {
  if (server === null) return <Skeleton className="h-4 w-32" />
  if (!server) {
    return (
      <span className="inline-flex items-center gap-2 text-muted-foreground">
        <Lamp state="unset" />
        Unreachable
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-2 whitespace-nowrap">
      <Lamp state={serverStates[server.state].lamp} label={serverStates[server.state].label} />
      {server.name}
      <span className="hidden text-muted-foreground sm:inline">{server.nodeName}</span>
    </span>
  )
}
