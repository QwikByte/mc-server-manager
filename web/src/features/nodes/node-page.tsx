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
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { Meter } from "@/components/meter"
import { BackLink } from "@/components/back-link"
import { PageHeader } from "@/components/page-header"
import { StatCard } from "@/components/stat-card"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { serversQuery } from "@/features/servers/api"
import { ServerList } from "@/features/servers/server-list"
import { formatBytes, formatDate, formatMegabytes } from "@/lib/format"
import { memoryCapacityMb, memoryLimitMb, type Node, type NodeInfo, nodeQuery } from "./api"
import { NewJoinTokenButton, RemoveNodeButton, RenewCertificateButton } from "./node-actions"
import { NodeSettingsDialog } from "./node-settings-dialog"
import { NodeStatusBadge } from "./node-status"
import { StorageList } from "./storage-list"

const route = getRouteApi("/_app/nodes/$nodeId")

export function NodePage() {
  const { can } = useAccess()
  const { nodeId } = route.useParams()
  const { data: node, isPending, error } = useQuery(nodeQuery(nodeId))

  return (
    <>
      <BackLink to="/nodes">Nodes</BackLink>
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
            description={<span className="font-mono">{node.address}</span>}
            actions={
              <>
                {can("nodes.edit", node.id) && <NodeSettingsDialog node={node} />}
                {node.enrolledAt && can("terminal.use") && (
                  <Button variant="outline" asChild>
                    <Link to="/settings/terminal" search={{ target: node.id }}>
                      <TerminalWindowIcon />
                      Terminal
                    </Link>
                  </Button>
                )}
                {node.status === "online" && can("nodes.certificates", node.id) && <RenewCertificateButton node={node} />}
                {node.enrolledAt && can("nodes.enroll") && <NewJoinTokenButton node={node} />}
                {can("nodes.delete", node.id) && <RemoveNodeButton node={node} />}
              </>
            }
          />
          {node.status === "online" && node.info ? (
            <>
              <NodeFacts node={node} info={node.info} />
              <ServerList nodeId={node.id} />
              {node.info.storage && can("nodes.view", node.id) && <StorageList locations={node.info.storage} />}
            </>
          ) : node.status === "pending" ? (
            <EmptyState
              icon={PlugsIcon}
              tone="warning"
              title="Connect the agent"
              description="This node has no connected agent yet. Create a join token and run the enrollment command on the node."
            >
              {can("nodes.enroll") && <NewJoinTokenButton node={node} variant="default" />}
            </EmptyState>
          ) : (
            <EmptyState
              icon={WarningCircleIcon}
              tone="destructive"
              title="The agent can't be reached"
              description={`The master can't reach the agent at ${node.address}. Check that mcsm-agent is running and that the port is open for the master.`}
            />
          )}
        </>
      )}
    </>
  )
}

function NodeFacts({ node, info }: { node: Node; info: NodeInfo }) {
  const { data: servers } = useQuery(serversQuery(node.id))
  const assignedMb = servers?.reduce((sum, s) => sum + s.memoryMb, 0)
  const capacityMb = memoryCapacityMb(node)
  const limitMb = memoryLimitMb(node)
  const details: [string, string][] = [
    ["Hostname", info.hostname],
    ["System", info.os],
    ["Runtime", info.runtime],
    ["Agent", info.agentVersion],
  ]
  return (
    <>
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard icon={CpuIcon} tone="warning" label="CPU" value={`${info.cpuCount} cores`} />
        <StatCard icon={MemoryIcon} tone="violet" label="Memory" value={formatBytes(info.memoryBytes)}>
          {limitMb === undefined ? "Not limited for servers" : `${formatMegabytes(limitMb)} usable by servers`}
        </StatCard>
        <StatCard icon={CubeIcon} tone="success" label="Assigned" value={assignedMb === undefined ? "–" : formatMegabytes(assignedMb)}>
          {assignedMb !== undefined && capacityMb ? (
            <div className="space-y-2">
              <Meter value={assignedMb / capacityMb} label="Memory assigned to servers" />
              <p>of {formatMegabytes(capacityMb)}</p>
            </div>
          ) : null}
        </StatCard>
        <StatCard
          icon={ShieldCheckIcon}
          tone="info"
          label="Certificate valid until"
          value={node.certificateExpiresAt ? formatDate(node.certificateExpiresAt) : "–"}
        >
          Renewed automatically
        </StatCard>
      </div>
      <dl className="mt-4 surface grid grid-cols-2 gap-x-6 gap-y-4 rounded-xl px-5 py-4 md:grid-cols-4">
        {details
          .filter(([, value]) => value)
          .map(([term, value]) => (
            <div key={term} className="min-w-0">
              <dt className="text-xs text-muted-foreground">{term}</dt>
              <dd className="mt-0.5 truncate text-sm font-medium" title={value}>
                {value}
              </dd>
            </div>
          ))}
      </dl>
    </>
  )
}
