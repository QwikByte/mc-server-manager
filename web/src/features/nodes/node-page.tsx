import {
  CpuIcon,
  CubeIcon,
  HardDrivesIcon,
  MemoryIcon,
  PlugsIcon,
  ShieldCheckIcon,
  TerminalWindowIcon,
  WarningCircleIcon,
} from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, Link } from "@tanstack/react-router"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { Meter } from "@/components/meter"
import { BackLink } from "@/components/back-link"
import { PageHeader } from "@/components/page-header"
import { StatCard } from "@/components/stat-card"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { NodeActivity } from "@/features/logs/activity"
import { OverlaySection } from "@/features/overlay/overlay-section"
import { assignedMemoryMb, serversQuery } from "@/features/servers/api"
import { ServerList } from "@/features/servers/server-list"
import { usageQuery } from "@/features/usage/api"
import { formatCores } from "@/features/usage/format"
import { UsageHistory } from "@/features/usage/usage-history"
import { formatBytes, formatDate, formatMegabytes } from "@/lib/format"
import { memoryCapacityMb, memoryLimitMb, type Node, type NodeInfo, nodeQuery } from "./api"
import { NewJoinTokenButton, NodeMenu } from "./node-actions"
import { NodeSettingsDialog } from "./node-settings-dialog"
import { NodeStatusBadge } from "./node-status"
import { StorageList } from "./storage-list"

const route = getRouteApi("/_app/nodes/$nodeId")

export function NodePage() {
  const { can } = useAccess()
  const { nodeId } = route.useParams()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const { data: node, isPending, error } = useQuery(nodeQuery(nodeId))

  return (
    <>
      <BackLink to="/nodes">{t("Nodes")}</BackLink>
      {isPending ? (
        <Skeleton className="h-64 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : (
        <>
          <PageHeader
            icon={HardDrivesIcon}
            tone="info"
            title={node.name}
            badge={<NodeStatusBadge status={node.status} />}
            description={node.address && <span className="font-mono">{node.address}</span>}
            actions={
              <>
                {can("nodes.edit", node.id) && <NodeSettingsDialog node={node} />}
                {node.enrolledAt && can("terminal.use") && (
                  <Button variant="outline" asChild>
                    <Link to="/settings/terminal" search={{ target: node.id }}>
                      <TerminalWindowIcon />
                      {t("Terminal")}
                    </Link>
                  </Button>
                )}
                <NodeMenu node={node} />
              </>
            }
          />
          {node.status === "online" && node.info ? (
            <>
              <NodeFacts node={node} info={node.info} />
              <ServerList
                nodeId={node.id}
                nodeName={node.name}
                search={search}
                onSearch={(change) => navigate({ search: (s) => ({ ...s, ...change }), replace: true })}
              />
              {can("nodes.view", node.id) && (
                <UsageHistory
                  nodeId={node.id}
                  charts={[
                    {
                      title: t("CPU"),
                      series: [{ label: t("CPU"), tone: "series-1", value: (p) => p.cpuMillis }],
                      format: formatCores,
                      max: node.info.cpuCount * 1000,
                    },
                    {
                      title: t("Memory"),
                      series: [{ label: t("Memory"), tone: "series-1", value: (p) => p.memoryBytes }],
                      format: (v) => formatBytes(Math.round(v)),
                      max: node.info.memoryBytes,
                    },
                  ]}
                />
              )}
              {node.info.storage && can("nodes.view", node.id) && <StorageList locations={node.info.storage} />}
              {can("nodes.view", node.id) && <OverlaySection node={node} />}
            </>
          ) : node.status === "pending" ? (
            <EmptyState
              icon={PlugsIcon}
              tone="warning"
              title={t("Connect the agent")}
              description={t("This node has no connected agent yet. Create a join token and run the enrollment command on the node.")}
            >
              {can("nodes.enroll") && <NewJoinTokenButton node={node} />}
            </EmptyState>
          ) : (
            <EmptyState
              icon={WarningCircleIcon}
              tone="destructive"
              title={t("The agent can't be reached")}
              description={
                node.address
                  ? t(
                      "The master can't reach the agent at {{address}}. Check that noryx-agent is running and that the port is open for the master.",
                      {
                        address: node.address,
                      },
                    )
                  : t("The master can't reach the agent. Check that noryx-agent is running and that the port is open for the master.")
              }
            />
          )}
          {/* Also while the agent is offline, which its last entries may explain. */}
          {can("logs.view", node.id) && <NodeActivity nodeId={node.id} />}
        </>
      )}
    </>
  )
}

function NodeFacts({ node, info }: { node: Node; info: NodeInfo }) {
  const { data: servers } = useQuery(serversQuery(node.id))
  // Missing without the permission to see the node, empty if the agent can't measure the machine.
  const usage = useQuery(usageQuery(node.id)).data?.node
  const live = usage?.cpuCount && usage.memoryTotalBytes ? usage : undefined
  const assignedMb = servers && assignedMemoryMb(servers)
  const capacityMb = memoryCapacityMb(node)
  const limitMb = memoryLimitMb(node)
  // Only there with the permission to see the node.
  const details = [
    [t("Hostname"), info.hostname],
    [t("System"), info.os],
    [t("Runtime"), info.runtime],
    [t("Agent"), info.agentVersion],
  ].filter((detail): detail is [string, string] => !!detail[1])
  return (
    <>
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard
          icon={CpuIcon}
          tone="warning"
          label={t("CPU")}
          value={live ? formatCores(live.cpuMillis) : t("{{count}} cores", { count: info.cpuCount, defaultValue_one: "{{count}} core" })}
        >
          {live && (
            <div className="space-y-2">
              <Meter value={live.cpuMillis / (live.cpuCount * 1000)} label={t("CPU used")} />
              <p>{t("of {{count}} cores in use", { count: info.cpuCount, defaultValue_one: "of {{count}} core in use" })}</p>
            </div>
          )}
        </StatCard>
        <StatCard icon={MemoryIcon} tone="violet" label={t("Memory")} value={formatBytes(live ? live.memoryUsedBytes : info.memoryBytes)}>
          {live && <Meter value={live.memoryUsedBytes / live.memoryTotalBytes} label={t("Memory used")} className="mb-2" />}
          {live && `${t("of {{memory}} in use", { memory: formatBytes(info.memoryBytes) })} · `}
          {limitMb === undefined ? t("Not limited for servers") : t("{{memory}} usable by servers", { memory: formatMegabytes(limitMb) })}
        </StatCard>
        <StatCard icon={CubeIcon} tone="success" label={t("Assigned")} value={assignedMb === undefined ? "–" : formatMegabytes(assignedMb)}>
          {assignedMb !== undefined && capacityMb ? (
            <div className="space-y-2">
              <Meter value={assignedMb / capacityMb} label={t("Memory assigned to servers")} />
              <p>{t("of {{limit}}, including what Java needs besides the servers", { limit: formatMegabytes(capacityMb) })}</p>
            </div>
          ) : null}
        </StatCard>
        <StatCard
          icon={ShieldCheckIcon}
          tone="info"
          label={t("Certificate valid until")}
          value={node.certificateExpiresAt ? formatDate(node.certificateExpiresAt) : "–"}
        >
          {t("Renewed automatically")}
        </StatCard>
      </div>
      {details.length > 0 && (
        <dl className="mt-4 surface grid grid-cols-2 gap-x-6 gap-y-4 rounded-xl px-5 py-4 md:grid-cols-4">
          {details.map(([term, value]) => (
            <div key={term} className="min-w-0">
              <dt className="text-xs text-muted-foreground">{term}</dt>
              <dd className="mt-0.5 truncate text-sm font-medium" title={value}>
                {value}
              </dd>
            </div>
          ))}
        </dl>
      )}
    </>
  )
}
