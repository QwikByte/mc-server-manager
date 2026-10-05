import {
  CheckCircleIcon,
  CubeIcon,
  GraphIcon,
  HardDrivesIcon,
  MemoryIcon,
  SquaresFourIcon,
  UsersThreeIcon,
  WarningCircleIcon,
} from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import type { ReactNode } from "react"
import { IconTile } from "@/components/icon-tile"
import { Meter } from "@/components/meter"
import { PageHeader } from "@/components/page-header"
import { StatCard } from "@/components/stat-card"
import { StatusDot } from "@/components/status"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { networksQuery } from "@/features/networks/api"
import { useNetworkOf } from "@/features/networks/servers"
import { playersOnline } from "@/features/networks/usage"
import { nodesQuery, onlineCapacityMb } from "@/features/nodes/api"
import { allServersQuery, assignedMemoryMb, type NodeServer, runningCount } from "@/features/servers/api"
import { StateBar } from "@/features/servers/server-state"
import { serverLook, serverType } from "@/features/servers/server-types"
import { useUsages } from "@/features/usage/api"
import { formatCores, formatNumber } from "@/features/usage/format"
import { formatBytes, formatMegabytes } from "@/lib/format"
import { problemsOf } from "./attention"

/** The start page: players, servers and nodes at a glance, and what needs attention. */
export function DashboardPage() {
  const access = useAccess()
  const { data: servers, isPending } = useQuery(allServersQuery)
  const { data: nodes = [] } = useQuery(nodesQuery)
  const { data: networks = [] } = useQuery({ ...networksQuery, enabled: access.can("networks.view") })
  const online = nodes.filter((n) => n.status === "online")
  const usages = useUsages(online.map((n) => n.id))
  const networkOf = useNetworkOf()
  if (isPending || !servers) return <Skeleton className="h-96 rounded-xl" />

  const usage = (s: { nodeId: string; serverId: string }) => usages.server(s.nodeId, s.serverId)
  const ref = (s: NodeServer) => ({ nodeId: s.nodeId, serverId: s.id })
  const gameServers = servers.filter((s) => !serverType(s.type).proxy)
  // A player counts once: in the network they joined, or on a server outside of networks.
  const players =
    networks.reduce((sum, n) => sum + (playersOnline(n, usage) ?? 0), 0) +
    gameServers.filter((s) => !networkOf(ref(s))).reduce((sum, s) => sum + (usage(ref(s))?.players?.online ?? 0), 0)
  const assignedMb = assignedMemoryMb(servers)
  const capacityMb = onlineCapacityMb(nodes)
  const problems = problemsOf(nodes, servers, networks, usages)
  const busiest = gameServers
    .filter((s) => usage(ref(s))?.players?.online)
    .sort((a, b) => usage(ref(b))!.players!.online - usage(ref(a))!.players!.online)
    .slice(0, 5)

  return (
    <>
      <PageHeader icon={SquaresFourIcon} title={t("Overview")} />
      <div className="mb-8 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard icon={UsersThreeIcon} tone="info" label={t("Players online")} value={players} />
        <StatCard
          icon={CubeIcon}
          tone="success"
          label={t("Servers running")}
          value={`${runningCount(servers)} / ${servers.length}`}
        >
          <StateBar servers={servers} />
        </StatCard>
        <StatCard icon={HardDrivesIcon} tone="info" label={t("Nodes online")} value={`${online.length} / ${nodes.length}`} />
        <StatCard icon={MemoryIcon} tone="violet" label={t("Memory assigned")} value={formatMegabytes(assignedMb)}>
          {capacityMb > 0 && (
            <div className="space-y-2">
              <Meter value={assignedMb / capacityMb} label={t("Memory assigned on online nodes")} />
              <p>{t("of {{memory}} on online nodes", { memory: formatMegabytes(capacityMb) })}</p>
            </div>
          )}
        </StatCard>
      </div>
      <div className="grid gap-8 lg:grid-cols-3">
        <div className="min-w-0 space-y-8 lg:col-span-2">
          <Panel title={t("Needs attention")} count={problems.length}>
            {problems.length === 0 ? (
              <div className="flex items-center gap-3 p-5 text-sm text-muted-foreground">
                <IconTile icon={CheckCircleIcon} tone="success" size="sm" />
                {t("Everything runs as it should.")}
              </div>
            ) : (
              <ul className="divide-y">
                {problems.map((p) => (
                  <li key={p.key}>
                    <Link {...p.link} className="flex items-center gap-3 px-5 py-3 transition-colors hover:bg-muted/50">
                      <IconTile icon={WarningCircleIcon} tone={p.tone} size="sm" />
                      <span className="min-w-0">
                        <span className="block truncate text-sm font-medium">{p.title}</span>
                        <span className="block truncate text-xs text-muted-foreground">{p.detail}</span>
                      </span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Panel>
          {networks.length > 0 && (
            <Panel title={t("Networks")} more={{ to: "/networks", label: t("All networks") }}>
              <ul className="divide-y">
                {networks.map((n) => {
                  const backends = servers.filter((s) => n.backends.some((b) => b.nodeId === s.nodeId && b.serverId === s.id))
                  return (
                    <li key={n.id}>
                      <Link
                        to="/networks/$networkId"
                        params={{ networkId: n.id }}
                        className="grid gap-3 px-5 py-3 transition-colors hover:bg-muted/50 sm:grid-cols-[12rem_1fr_auto] sm:items-center"
                      >
                        <span className="flex min-w-0 items-center gap-3">
                          <IconTile icon={GraphIcon} tone="violet" size="sm" />
                          <span className="truncate text-sm font-medium">{n.name}</span>
                        </span>
                        <StateBar servers={backends} />
                        <span className="text-sm text-muted-foreground tabular-nums sm:text-right">
                          {t("{{count}} online", { count: playersOnline(n, usage) ?? 0 })}
                        </span>
                      </Link>
                    </li>
                  )
                })}
              </ul>
            </Panel>
          )}
        </div>
        <div className="min-w-0 space-y-8">
          <Panel title={t("Nodes")} more={{ to: "/nodes", label: t("All nodes") }}>
            <ul className="divide-y">
              {nodes.map((n) => {
                const live = usages.node(n.id)
                return (
                  <li key={n.id}>
                    <Link
                      to="/nodes/$nodeId"
                      params={{ nodeId: n.id }}
                      className="block space-y-2 px-5 py-3 transition-colors hover:bg-muted/50"
                    >
                      <span className="flex items-center gap-2 text-sm font-medium">
                        <StatusDot
                          status={{
                            tone: n.status === "online" ? "success" : n.status === "pending" ? "warning" : "destructive",
                            label: "",
                          }}
                        />
                        {n.name}
                        <span className="ml-auto text-xs font-normal text-muted-foreground">
                          {t("{{count}} servers", {
                            count: servers.filter((s) => s.nodeId === n.id).length,
                            defaultValue_one: "{{count}} server",
                          })}
                        </span>
                      </span>
                      {live?.cpuCount && live.memoryTotalBytes ? (
                        <span className="grid grid-cols-2 gap-3 text-xs text-muted-foreground">
                          <Load
                            label={t("CPU")}
                            value={live.cpuMillis / (live.cpuCount * 1000)}
                            detail={t("{{used}} of {{total}}", { used: formatNumber(live.cpuMillis / 1000), total: formatCores(live.cpuCount * 1000) })}
                          />
                          <Load
                            label={t("Memory")}
                            value={live.memoryUsedBytes / live.memoryTotalBytes}
                            detail={formatBytes(live.memoryUsedBytes)}
                          />
                        </span>
                      ) : (
                        n.status !== "online" && <span className="block text-xs text-muted-foreground">{t("Not reachable")}</span>
                      )}
                    </Link>
                  </li>
                )
              })}
            </ul>
          </Panel>
          {busiest.length > 0 && (
            <Panel title={t("Most players")}>
              <ol className="divide-y">
                {busiest.map((s) => (
                  <li key={`${s.nodeId}/${s.id}`}>
                    <Link
                      to="/nodes/$nodeId/servers/$serverId"
                      params={{ nodeId: s.nodeId, serverId: s.id }}
                      className="flex items-center gap-3 px-5 py-2.5 transition-colors hover:bg-muted/50"
                    >
                      <IconTile {...serverLook(s.type)} size="sm" />
                      <span className="min-w-0 flex-1 truncate text-sm font-medium">{s.name}</span>
                      <span className="text-sm tabular-nums">{usage(ref(s))?.players?.online}</span>
                    </Link>
                  </li>
                ))}
              </ol>
            </Panel>
          )}
        </div>
      </div>
    </>
  )
}

/** A titled list on the dashboard, with a link to all of it. */
function Panel({
  title,
  count,
  more,
  children,
}: {
  title: string
  count?: number
  more?: { to: "/nodes" | "/networks"; label: string }
  children: ReactNode
}) {
  return (
    <section aria-label={title}>
      <div className="mb-3 flex items-center gap-2">
        <h2 className="heading text-lg">{title}</h2>
        {!!count && (
          <span className="rounded-full bg-destructive/10 px-2 text-xs font-semibold text-destructive tabular-nums">{count}</span>
        )}
        {more && (
          <Link to={more.to} className="ml-auto text-sm text-muted-foreground hover:text-foreground">
            {more.label}
          </Link>
        )}
      </div>
      <div className="surface overflow-hidden rounded-xl">{children}</div>
    </section>
  )
}

function Load({ label, value, detail }: { label: string; value: number; detail: string }) {
  return (
    <span className="space-y-1">
      <span className="flex justify-between gap-2">
        <span>{label}</span>
        <span className="tabular-nums">{detail}</span>
      </span>
      <Meter value={value} label={label} />
    </span>
  )
}
