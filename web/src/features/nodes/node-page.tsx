import { CaretLeftIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, Link } from "@tanstack/react-router"
import type { ReactNode } from "react"
import { PageHeader } from "@/components/page-header"
import { Skeleton } from "@/components/ui/skeleton"
import { ServerList } from "@/features/servers/server-list"
import { formatBytes, formatDate } from "@/lib/format"
import { type Node, nodeQuery } from "./api"
import { NewJoinTokenButton, RemoveNodeButton, RenewCertificateButton } from "./node-actions"
import { NodeStatusLabel } from "./node-status"
import { StorageList } from "./storage-list"

const route = getRouteApi("/_app/nodes/$nodeId")

export function NodePage() {
  const { nodeId } = route.useParams()
  const { data: node, isPending, error } = useQuery(nodeQuery(nodeId))

  return (
    <>
      <Link to="/nodes" className="mb-4 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <CaretLeftIcon className="size-3.5" />
        Nodes
      </Link>
      {isPending ? (
        <Skeleton className="h-48" />
      ) : error ? (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      ) : (
        <>
          <PageHeader
            title={node.name}
            description={<span className="font-mono">{node.address}</span>}
            actions={
              <>
                {node.status === "online" && <RenewCertificateButton node={node} />}
                {node.enrolledAt && <NewJoinTokenButton node={node} />}
                <RemoveNodeButton node={node} />
              </>
            }
          />
          <NodeFacts node={node} />
          <NodeBody node={node} />
        </>
      )}
    </>
  )
}

function NodeFacts({ node }: { node: Node }) {
  const info = node.info
  const facts: [string, ReactNode][] = [
    ["Status", <NodeStatusLabel key="status" status={node.status} />],
    ["Hostname", info?.hostname],
    ["System", info?.os],
    ["CPUs", info?.cpuCount || undefined],
    ["Memory", info?.memoryBytes ? formatBytes(info.memoryBytes) : undefined],
    ["Runtime", info?.runtime],
    ["Agent", info?.agentVersion],
    ["Certificate valid until", node.certificateExpiresAt && formatDate(node.certificateExpiresAt)],
  ]
  return (
    <dl className="grid grid-cols-2 gap-x-6 gap-y-4 border-y py-4 sm:grid-cols-4 lg:grid-cols-8">
      {facts
        .filter(([, value]) => value !== undefined && value !== "")
        .map(([term, value]) => (
          <div key={term} className="min-w-0">
            <dt className="text-xs text-muted-foreground">{term}</dt>
            <dd className="mt-1 text-sm break-words">{value}</dd>
          </div>
        ))}
    </dl>
  )
}

function NodeBody({ node }: { node: Node }) {
  if (node.status === "online")
    return (
      <>
        <ServerList nodeId={node.id} />
        {node.info?.storage && <StorageList locations={node.info.storage} />}
      </>
    )
  return (
    <section className="mt-10 max-w-2xl space-y-4">
      <h2 className="heading text-xl">{node.status === "pending" ? "Connect the agent" : "The agent can't be reached"}</h2>
      <p className="text-sm text-muted-foreground">
        {node.status === "pending"
          ? "This node has no connected agent yet. Create a join token and run the enrollment command on the node."
          : `The master can't reach the agent at ${node.address}. Check that mcsm-agent is running and that the port is open for the master.`}
      </p>
      {node.status === "pending" && <NewJoinTokenButton node={node} variant="default" />}
    </section>
  )
}
