import { CpuIcon, CubeIcon, HardDrivesIcon, HashIcon, MemoryIcon, UsersIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { EmptyState } from "@/components/empty-state"
import { IconTile } from "@/components/icon-tile"
import { Section } from "@/components/section"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { type ServerUsage, usageQuery } from "@/features/usage/api"
import { formatCores } from "@/features/usage/format"
import { formatBytes, formatMegabytes } from "@/lib/format"
import { type Server, serversQuery } from "./api"
import { CreateServerDialog } from "./create-server-dialog"
import { ServerActions } from "./server-actions"
import { ServerStateBadge } from "./server-state"
import { displayVersion, serverLook, serverType } from "./server-types"

export function ServerList({ nodeId }: { nodeId: string }) {
  const { can } = useAccess()
  const { data: servers, isPending, error } = useQuery(serversQuery(nodeId))
  const { data: usage } = useQuery(usageQuery(nodeId))
  const create = can("servers.create", nodeId) && <CreateServerDialog nodeId={nodeId} />

  return (
    <Section title={t("Servers")} actions={servers && servers.length > 0 && create}>
      {isPending ? (
        <Skeleton className="h-44 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : servers.length === 0 ? (
        <EmptyState
          icon={CubeIcon}
          title={t("No servers on this node")}
          description={
            create ? t("Create a game server or a proxy that connects servers to a network.") : t("There are no servers you can see.")
          }
        >
          {create}
        </EmptyState>
      ) : (
        <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {servers.map((server) => (
            <ServerCard key={server.id} nodeId={nodeId} server={server} usage={usage?.servers.find((u) => u.id === server.id)} />
          ))}
        </ul>
      )}
    </Section>
  )
}

/**
 * The whole card opens the server; its buttons sit above the link. Running servers show what they
 * use. Cards of servers on different nodes show the node's name.
 */
export function ServerCard({
  nodeId,
  nodeName,
  server,
  usage,
}: {
  nodeId: string
  nodeName?: string
  server: Server
  usage?: ServerUsage
}) {
  const live = usage?.running ? usage : undefined
  const look = serverLook(server.type)
  return (
    <li className="group surface relative flex flex-col gap-4 rounded-xl p-5 transition-all hover:shadow-lg hover:ring-primary/30">
      <div className="flex items-start gap-3">
        <IconTile icon={look.icon} tone={look.tone} />
        <div className="min-w-0 flex-1">
          <Link
            to="/nodes/$nodeId/servers/$serverId"
            params={{ nodeId, serverId: server.id }}
            className="block truncate font-semibold outline-none after:absolute after:inset-0 after:rounded-xl focus-visible:after:ring-2 focus-visible:after:ring-ring"
          >
            {server.name}
          </Link>
          <p className="truncate text-xs text-muted-foreground">
            {serverType(server.type).label} {displayVersion(server.version)}
          </p>
        </div>
        <ServerStateBadge state={server.state} />
      </div>
      <div className="flex flex-wrap gap-2">
        {nodeName && <Chip icon={HardDrivesIcon}>{nodeName}</Chip>}
        <Chip icon={HashIcon}>
          <span className="font-mono">{server.port}</span>
        </Chip>
        {/* While it runs, its memory of the container's limit, which includes what Java needs besides the heap. */}
        <Chip icon={MemoryIcon}>
          {live?.memoryLimitBytes
            ? `${formatBytes(live.memoryBytes)} / ${formatBytes(live.memoryLimitBytes)}`
            : formatMegabytes(server.memoryMb)}
        </Chip>
        {live && <Chip icon={CpuIcon}>{formatCores(live.cpuMillis)}</Chip>}
        {live?.players && (
          <Chip icon={UsersIcon}>
            {live.players.online} / {live.players.max}
          </Chip>
        )}
      </div>
      <div className="relative z-10 mt-auto border-t pt-4 empty:hidden">
        <ServerActions nodeId={nodeId} server={server} />
      </div>
    </li>
  )
}
