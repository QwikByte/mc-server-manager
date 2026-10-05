import { ArrowRightIcon, CpuIcon, CubeIcon, HardDrivesIcon, MemoryIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { IconTile } from "@/components/icon-tile"
import { Meter } from "@/components/meter"
import { PageHeader } from "@/components/page-header"
import { StatCard } from "@/components/stat-card"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { OverlaySettingsSection } from "@/features/overlay/overlay-settings"
import { allServersQuery, assignedMemoryMb, type NodeServer, runningCount } from "@/features/servers/api"
import { formatBytes, formatMegabytes } from "@/lib/format"
import { AddNodeDialog } from "./add-node-dialog"
import { memoryCapacityMb, type Node, nodesQuery, onlineCapacityMb } from "./api"
import { NodeStatusBadge } from "./node-status"

export function NodesPage() {
  const { can } = useAccess()
  const { data: nodes, isPending, error } = useQuery(nodesQuery)
  const add = can("nodes.enroll") && <AddNodeDialog />
  const { data: servers } = useQuery(allServersQuery)

  return (
    <>
      <PageHeader icon={HardDrivesIcon} tone="info" title={t("Nodes")} actions={add} />
      {isPending ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-52 rounded-xl" />
          ))}
        </div>
      ) : error ? (
        <ErrorCallout error={error} />
      ) : nodes.length === 0 ? (
        <EmptyState icon={HardDrivesIcon} tone="info" title={t("No nodes yet")}>
          {add}
        </EmptyState>
      ) : (
        <>
          <Overview nodes={nodes} servers={servers} />
          <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {nodes.map((node) => (
              <li key={node.id}>
                <NodeCard node={node} servers={servers?.filter((s) => s.nodeId === node.id)} />
              </li>
            ))}
          </ul>
          <OverlaySettingsSection />
        </>
      )}
    </>
  )
}

/** Totals across all nodes, counted like the dashboard does; the master can't list the servers of offline nodes. */
function Overview({ nodes, servers }: { nodes: Node[]; servers?: NodeServer[] }) {
  const online = nodes.filter((n) => n.status === "online")
  const assignedMb = servers && assignedMemoryMb(servers)
  const capacityMb = onlineCapacityMb(nodes)
  return (
    <div className="mb-8 grid grid-cols-2 gap-4 lg:grid-cols-4">
      <StatCard icon={HardDrivesIcon} tone="info" label={t("Nodes online")} value={`${online.length} / ${nodes.length}`} />
      <StatCard icon={CubeIcon} tone="success" label={t("Servers running")} value={servers ? `${runningCount(servers)} / ${servers.length}` : "–"} />
      <StatCard
        icon={MemoryIcon}
        tone="violet"
        label={t("Memory assigned")}
        value={assignedMb === undefined ? "–" : formatMegabytes(assignedMb)}
      >
        {capacityMb > 0 && t("of {{memory}} on online nodes", { memory: formatMegabytes(capacityMb) })}
      </StatCard>
      <StatCard icon={CpuIcon} tone="warning" label={t("CPU cores")} value={online.reduce((sum, n) => sum + (n.info?.cpuCount ?? 0), 0)} />
    </div>
  )
}

function NodeCard({ node, servers }: { node: Node; servers?: NodeServer[] }) {
  const info = node.info
  const capacityMb = memoryCapacityMb(node)
  const assignedMb = assignedMemoryMb(servers ?? [])
  return (
    <Link
      to="/nodes/$nodeId"
      params={{ nodeId: node.id }}
      className="group surface flex h-full flex-col gap-5 rounded-xl p-5 transition-all outline-none hover:-translate-y-0.5 hover:shadow-lg hover:ring-info/40 focus-visible:ring-2 focus-visible:ring-ring motion-reduce:hover:translate-y-0"
    >
      <div className="flex items-start gap-3">
        <IconTile icon={HardDrivesIcon} tone="info" />
        <div className="min-w-0 flex-1">
          <p className="truncate font-semibold">{node.name}</p>
          {node.address && <p className="truncate font-mono text-xs text-muted-foreground">{node.address}</p>}
        </div>
        <NodeStatusBadge status={node.status} />
      </div>
      {info ? (
        <>
          <dl className="grid grid-cols-3 gap-2">
            <Fact label={t("CPUs")} value={info.cpuCount} />
            <Fact label={t("Memory")} value={formatBytes(info.memoryBytes)} />
            <Fact
              label={t("Servers")}
              value={servers ? `${runningCount(servers)} / ${servers.length}` : "–"}
            />
          </dl>
          {capacityMb !== undefined && servers && (
            <div className="space-y-2">
              <div className="flex justify-between text-xs text-muted-foreground">
                <span>{t("Memory assigned")}</span>
                <span className="tabular-nums">
                  {t("{{used}} of {{total}}", { used: formatMegabytes(assignedMb), total: formatMegabytes(capacityMb) })}
                </span>
              </div>
              <Meter value={capacityMb ? assignedMb / capacityMb : 1} label={t("Memory assigned on {{name}}", { name: node.name })} />
            </div>
          )}
        </>
      ) : (
        <p className="text-sm text-muted-foreground">
          {node.status === "pending" ? t("Connect the agent to start hosting servers.") : t("The master can't reach the agent right now.")}
        </p>
      )}
      <div className="mt-auto flex items-center justify-between gap-3 border-t pt-4 text-xs text-muted-foreground">
        <span className="truncate">
          {info
            ? [info.os, info.agentVersion && t("agent {{version}}", { version: info.agentVersion })].filter(Boolean).join(" · ")
            : t("No agent connected")}
        </span>
        <ArrowRightIcon className="size-4 shrink-0 transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
      </div>
    </Link>
  )
}

function Fact({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="min-w-0 rounded-lg bg-muted/70 px-2.5 py-2">
      <dt className="text-[0.6875rem] text-muted-foreground">{label}</dt>
      <dd className="mt-0.5 truncate text-sm font-semibold tabular-nums">{value}</dd>
    </div>
  )
}
