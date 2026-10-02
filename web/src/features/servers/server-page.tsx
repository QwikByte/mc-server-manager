import {
  ArchiveIcon,
  FolderIcon,
  GearIcon,
  HardDrivesIcon,
  HashIcon,
  MemoryIcon,
  PuzzlePieceIcon,
  SlidersHorizontalIcon,
  TerminalIcon,
} from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, Link, Outlet, useNavigate } from "@tanstack/react-router"
import { ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { BackLink } from "@/components/back-link"
import { PageHeader } from "@/components/page-header"
import { Skeleton } from "@/components/ui/skeleton"
import { nodeQuery } from "@/features/nodes/api"
import { formatMegabytes } from "@/lib/format"
import { useServer } from "./api"
import { Console } from "./console"
import { ServerActions } from "./server-actions"
import { ServerStateBadge } from "./server-state"
import { displayVersion, serverLook, serverType } from "./server-types"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId")

/** Tabs of a server; label is a function for tabs that some types of servers don't have. */
const tabs = [
  { to: "/nodes/$nodeId/servers/$serverId", label: () => "Console", icon: TerminalIcon, exact: true },
  { to: "/nodes/$nodeId/servers/$serverId/files", label: () => "Files", icon: FolderIcon, exact: false },
  {
    to: "/nodes/$nodeId/servers/$serverId/properties",
    label: (type: string) => (serverType(type).proxy ? undefined : "Properties"),
    icon: SlidersHorizontalIcon,
    exact: false,
  },
  {
    to: "/nodes/$nodeId/servers/$serverId/plugins",
    label: (type: string) => {
      const kind = serverType(type).addons?.kind
      return kind && (kind === "mods" ? "Mods" : "Plugins")
    },
    icon: PuzzlePieceIcon,
    exact: false,
  },
  { to: "/nodes/$nodeId/servers/$serverId/backups", label: () => "Backups", icon: ArchiveIcon, exact: false },
  { to: "/nodes/$nodeId/servers/$serverId/settings", label: () => "Settings", icon: GearIcon, exact: false },
] as const

/** Header and tabs of a server; the tabs are child routes. */
export function ServerPage() {
  const { nodeId, serverId } = route.useParams()
  const navigate = useNavigate()
  const { data: node } = useQuery(nodeQuery(nodeId))
  const { server, isPending, error } = useServer(nodeId, serverId)

  return (
    <>
      <BackLink to="/nodes/$nodeId" params={{ nodeId }}>
        {node?.name ?? "Node"}
      </BackLink>
      {isPending ? (
        <Skeleton className="h-96 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : !server ? (
        <p className="text-sm text-muted-foreground">This server no longer exists.</p>
      ) : (
        <>
          <PageHeader
            {...serverLook(server.type)}
            title={server.name}
            badge={<ServerStateBadge state={server.state} />}
            description={
              <span className="mt-1 flex flex-wrap gap-2 text-foreground">
                <Chip>
                  {serverType(server.type).label} {displayVersion(server.version)}
                </Chip>
                <Chip icon={HashIcon}>
                  <span className="font-mono">{server.port}</span>
                </Chip>
                <Chip icon={MemoryIcon}>{formatMegabytes(server.memoryMb)}</Chip>
                {node && <Chip icon={HardDrivesIcon}>{node.name}</Chip>}
              </span>
            }
            actions={
              <ServerActions nodeId={nodeId} server={server} onDeleted={() => navigate({ to: "/nodes/$nodeId", params: { nodeId } })} />
            }
          />
          <nav aria-label="Server" className="mb-8 flex max-w-full gap-1 overflow-x-auto rounded-xl bg-muted/80 p-1 sm:w-fit">
            {tabs
              .map((tab) => ({ ...tab, label: tab.label(server.type) }))
              .filter((tab) => tab.label)
              .map(({ to, label, icon: Icon, exact }) => (
                <Link
                  key={to}
                  to={to}
                  params={{ nodeId, serverId }}
                  activeOptions={{ exact, includeSearch: false }}
                  className="flex items-center gap-2 rounded-lg px-3.5 py-1.5 text-sm font-medium whitespace-nowrap text-muted-foreground transition-colors hover:text-foreground data-[status=active]:bg-card data-[status=active]:text-foreground data-[status=active]:shadow-sm"
                >
                  <Icon className="size-4" weight="duotone" />
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
