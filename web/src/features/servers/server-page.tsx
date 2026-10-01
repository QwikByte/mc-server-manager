import { CaretLeftIcon, FolderIcon, SlidersHorizontalIcon, TerminalIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, Link, Outlet, useNavigate } from "@tanstack/react-router"
import { Lamp } from "@/components/lamp"
import { PageHeader } from "@/components/page-header"
import { Skeleton } from "@/components/ui/skeleton"
import { nodeQuery } from "@/features/nodes/api"
import { formatMegabytes } from "@/lib/format"
import { useServer } from "./api"
import { Console } from "./console"
import { ServerActions } from "./server-actions"
import { ServerStateLabel } from "./server-state"
import { displayVersion, serverStates, serverType } from "./server-types"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId")

const tabs = [
  { to: "/nodes/$nodeId/servers/$serverId", label: "Console", icon: TerminalIcon, exact: true },
  { to: "/nodes/$nodeId/servers/$serverId/files", label: "Files", icon: FolderIcon, exact: false },
  { to: "/nodes/$nodeId/servers/$serverId/properties", label: "Properties", icon: SlidersHorizontalIcon, exact: false, game: true },
] as const

/** Header and tabs of a server; the tabs are child routes. */
export function ServerPage() {
  const { nodeId, serverId } = route.useParams()
  const navigate = useNavigate()
  const { data: node } = useQuery(nodeQuery(nodeId))
  const { server, isPending, error } = useServer(nodeId, serverId)

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
          <nav aria-label="Server" className="mb-8 flex gap-1 overflow-x-auto border-b">
            {tabs
              .filter((tab) => !("game" in tab && serverType(server.type).proxy))
              .map(({ to, label, icon: Icon, exact }) => (
                <Link
                  key={to}
                  to={to}
                  params={{ nodeId, serverId }}
                  activeOptions={{ exact, includeSearch: false }}
                  className="-mb-px flex items-center gap-2 border-b-2 border-transparent px-3 py-2 text-sm text-muted-foreground hover:text-foreground data-[status=active]:border-primary data-[status=active]:font-medium data-[status=active]:text-foreground"
                >
                  <Icon className="size-4" />
                  {label}
                </Link>
              ))}
          </nav>
          <Outlet />
        </>
      )}
    </>
  )
}

export function ServerConsole() {
  const { nodeId, serverId } = route.useParams()
  const { server } = useServer(nodeId, serverId)
  return server ? <Console nodeId={nodeId} server={server} /> : null
}
