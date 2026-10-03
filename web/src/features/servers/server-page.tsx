import {
  ArchiveIcon,
  ChartLineIcon,
  ClockCounterClockwiseIcon,
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
import { getRouteApi, Outlet, useNavigate } from "@tanstack/react-router"
import { ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { BackLink } from "@/components/back-link"
import { EmptyState } from "@/components/empty-state"
import { PageHeader } from "@/components/page-header"
import { TabLink } from "@/components/tab-link"
import { Tabs } from "@/components/tabs"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { nodeQuery } from "@/features/nodes/api"
import { formatMegabytes } from "@/lib/format"
import { useServer } from "./api"
import { Console } from "./console"
import { MoveStatus } from "./move-status"
import { ServerActions } from "./server-actions"
import { CrashNotice, ServerStateBadge } from "./server-state"
import { displayVersion, serverLook, serverType } from "./server-types"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId")

/**
 * Tabs of a server with the permission they need; label is a function for tabs that some
 * types of servers don't have.
 */
const tabs = [
  { to: "/nodes/$nodeId/servers/$serverId", label: () => "Console", icon: TerminalIcon, exact: true, permission: "console.view" },
  { to: "/nodes/$nodeId/servers/$serverId/usage", label: () => "Usage", icon: ChartLineIcon, exact: false, permission: "servers.view" },
  { to: "/nodes/$nodeId/servers/$serverId/files", label: () => "Files", icon: FolderIcon, exact: false, permission: "files.read" },
  {
    to: "/nodes/$nodeId/servers/$serverId/properties",
    label: (type: string) => (serverType(type).proxy ? undefined : "Properties"),
    icon: SlidersHorizontalIcon,
    exact: false,
    permission: "properties.edit",
  },
  {
    to: "/nodes/$nodeId/servers/$serverId/plugins",
    label: (type: string) => {
      const kind = serverType(type).addons?.kind
      return kind && (kind === "mods" ? "Mods" : "Plugins")
    },
    icon: PuzzlePieceIcon,
    exact: false,
    permission: "servers.view",
  },
  { to: "/nodes/$nodeId/servers/$serverId/backups", label: () => "Backups", icon: ArchiveIcon, exact: false, permission: "backups.view" },
  {
    to: "/nodes/$nodeId/servers/$serverId/activity",
    label: () => "Activity",
    icon: ClockCounterClockwiseIcon,
    exact: false,
    permission: "logs.view",
  },
  {
    to: "/nodes/$nodeId/servers/$serverId/settings",
    label: () => "Settings",
    icon: GearIcon,
    exact: false,
    permission: "servers.settings",
  },
] as const

/** Header and tabs of a server; the tabs are child routes. */
export function ServerPage() {
  const { can } = useAccess()
  const { nodeId, serverId } = route.useParams()
  const navigate = useNavigate()
  const { data: node } = useQuery(nodeQuery(nodeId))
  const { server, isPending, error } = useServer(nodeId, serverId)

  return (
    <>
      <BackLink to="/nodes/$nodeId" params={{ nodeId }}>
        {node?.name ?? "Node"}
      </BackLink>
      <MoveStatus nodeId={nodeId} serverId={serverId} />
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
          <CrashNotice server={server} />
          <Tabs label="Server">
            {tabs
              .map((tab) => ({ ...tab, label: tab.label(server.type) }))
              .filter((tab) => tab.label && can(tab.permission, nodeId, serverId))
              .map(({ to, label, icon: Icon, exact }) => (
                <TabLink key={to} to={to} params={{ nodeId, serverId }} activeOptions={{ exact, includeSearch: false }}>
                  <Icon className="size-4" weight="duotone" />
                  {label}
                </TabLink>
              ))}
          </Tabs>
          <Outlet />
        </>
      )}
    </>
  )
}

export function ServerConsole() {
  const { can } = useAccess()
  const { nodeId, serverId } = route.useParams()
  const { server } = useServer(nodeId, serverId)
  if (!server) return null
  if (!can("console.view", nodeId, serverId)) {
    return (
      <EmptyState
        icon={TerminalIcon}
        tone="neutral"
        title="No console"
        description="Your groups don't let you read the console of this server."
      />
    )
  }
  return <Console key={`${nodeId}/${server.id}`} nodeId={nodeId} server={server} />
}
