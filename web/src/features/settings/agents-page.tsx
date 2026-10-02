import { GearIcon, HardDrivesIcon, TerminalWindowIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { Callout, ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { IconTile } from "@/components/icon-tile"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useAccess } from "@/features/access/use-access"
import { type Node, nodesQuery } from "@/features/nodes/api"
import { NodeSettingsDialog } from "@/features/nodes/node-settings-dialog"
import { NodeStatusBadge } from "@/features/nodes/node-status"
import { formatDate, formatMegabytes } from "@/lib/format"

/** The Agents tab: every node with its agent and settings at a glance. */
export function AgentsSettingsPage() {
  const { data: nodes, isPending, error } = useQuery(nodesQuery)
  if (isPending) return <Skeleton className="h-64 rounded-xl" />
  if (error) return <ErrorCallout error={error} />
  if (nodes.length === 0) {
    return (
      <EmptyState icon={HardDrivesIcon} tone="info" title="No agents yet" description="Add a node to connect its agent to this master.">
        <Button asChild>
          <Link to="/nodes">Go to nodes</Link>
        </Button>
      </EmptyState>
    )
  }
  return (
    <>
      <div className="surface overflow-hidden rounded-xl">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Node</TableHead>
              <TableHead className="hidden md:table-cell">Agent</TableHead>
              <TableHead className="hidden lg:table-cell">Certificate</TableHead>
              <TableHead className="hidden sm:table-cell">Ports</TableHead>
              <TableHead className="hidden sm:table-cell">Memory</TableHead>
              <TableHead className="hidden xl:table-cell">Storage</TableHead>
              <TableHead>
                <span className="sr-only">Actions</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {nodes.map((node) => (
              <AgentRow key={node.id} node={node} />
            ))}
          </TableBody>
        </Table>
      </div>
      <Callout className="mt-6" title="Only the node decides where data is stored">
        Agents accept commands from this master and from their local CLI only. Storage locations can't be added from the panel: run{" "}
        <span className="font-mono">sudo mcsm-agent storage add &lt;name&gt; &lt;path&gt;</span> on the node.
      </Callout>
    </>
  )
}

function AgentRow({ node }: { node: Node }) {
  const { can } = useAccess()
  return (
    <TableRow>
      <TableCell>
        <span className="flex items-center gap-3">
          <IconTile icon={HardDrivesIcon} tone="info" size="sm" />
          <span className="min-w-0">
            <Link to="/nodes/$nodeId" params={{ nodeId: node.id }} className="block truncate font-medium hover:underline">
              {node.name}
            </Link>
            <span className="block truncate font-mono text-xs text-muted-foreground">{node.address}</span>
          </span>
          <NodeStatusBadge status={node.status} />
        </span>
      </TableCell>
      <TableCell className="hidden md:table-cell">
        {node.info ? (
          <>
            <span className="block font-medium">{node.info.agentVersion}</span>
            <span className="block text-xs text-muted-foreground">{node.info.os || node.info.runtime}</span>
          </>
        ) : (
          <span className="text-muted-foreground">–</span>
        )}
      </TableCell>
      <TableCell className="hidden lg:table-cell">
        {node.certificateExpiresAt ? formatDate(node.certificateExpiresAt) : <span className="text-muted-foreground">–</span>}
      </TableCell>
      <TableCell className="hidden font-mono sm:table-cell">{node.portMin === null ? "any" : `${node.portMin}–${node.portMax}`}</TableCell>
      <TableCell className="hidden sm:table-cell">
        {node.memoryReserveMb === null ? "Not limited" : `${formatMegabytes(node.memoryReserveMb)} reserved`}
      </TableCell>
      <TableCell className="hidden xl:table-cell">{node.defaultStorage}</TableCell>
      <TableCell>
        <span className="flex justify-end gap-1">
          {node.enrolledAt && can("terminal.use") && (
            <Button variant="ghost" size="icon-sm" asChild>
              <Link to="/settings/terminal" search={{ target: node.id }} aria-label={`Terminal of ${node.name}`} title="Terminal">
                <TerminalWindowIcon />
              </Link>
            </Button>
          )}
          {can("nodes.edit", node.id) && (
            <NodeSettingsDialog
              node={node}
              trigger={
                <Button variant="ghost" size="icon-sm" aria-label={`Settings of ${node.name}`} title="Settings">
                  <GearIcon />
                </Button>
              }
            />
          )}
        </span>
      </TableCell>
    </TableRow>
  )
}
