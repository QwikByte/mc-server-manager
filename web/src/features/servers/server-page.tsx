import { CaretLeftIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, Link, useNavigate } from "@tanstack/react-router"
import { Lamp } from "@/components/lamp"
import { PageHeader } from "@/components/page-header"
import { Skeleton } from "@/components/ui/skeleton"
import { nodeQuery } from "@/features/nodes/api"
import { formatMegabytes } from "@/lib/format"
import { serversQuery } from "./api"
import { Console } from "./console"
import { ServerActions } from "./server-actions"
import { ServerStateLabel } from "./server-state"
import { displayVersion, serverStates, serverType } from "./server-types"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId")

export function ServerPage() {
  const { nodeId, serverId } = route.useParams()
  const navigate = useNavigate()
  const { data: node } = useQuery(nodeQuery(nodeId))
  const { data: servers, isPending, error } = useQuery(serversQuery(nodeId))
  const server = servers?.find((s) => s.id === serverId)

  return (
    <>
      <Link
        to="/nodes/$nodeId"
        params={{ nodeId }}
        className="mb-4 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <CaretLeftIcon className="size-3.5" />
        {node?.name ?? "Node"}
      </Link>
      {isPending ? (
        <Skeleton className="h-96" />
      ) : error ? (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      ) : !server ? (
        <p className="text-sm text-muted-foreground">This server no longer exists.</p>
      ) : (
        <>
          <PageHeader
            title={
              <span className="inline-flex items-center gap-3">
                <Lamp state={serverStates[server.state].lamp} className="size-6" />
                {server.name}
              </span>
            }
            description={
              <span className="flex flex-wrap items-center gap-x-4 gap-y-1">
                <ServerStateLabel state={server.state} />
                <span>
                  {serverType(server.type).label} {displayVersion(server.version)} on port {server.port} with{" "}
                  {formatMegabytes(server.memoryMb)} memory
                </span>
              </span>
            }
            actions={
              <ServerActions nodeId={nodeId} server={server} onDeleted={() => navigate({ to: "/nodes/$nodeId", params: { nodeId } })} />
            }
          />
          <Console nodeId={nodeId} server={server} />
        </>
      )}
    </>
  )
}
