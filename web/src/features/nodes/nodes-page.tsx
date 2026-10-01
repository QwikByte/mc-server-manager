import { HardDrivesIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { PageHeader } from "@/components/page-header"
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { formatBytes } from "@/lib/format"
import { AddNodeDialog } from "./add-node-dialog"
import { nodesQuery } from "./api"
import { NodeStatusLabel } from "./node-status"

export function NodesPage() {
  const { data: nodes, isPending, error } = useQuery(nodesQuery)

  return (
    <>
      <PageHeader title="Nodes" description="Machines that run the agent and host your servers." actions={<AddNodeDialog />} />
      {isPending ? (
        <Skeleton className="h-40" />
      ) : error ? (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      ) : nodes.length === 0 ? (
        <Empty className="border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <HardDrivesIcon />
            </EmptyMedia>
            <EmptyTitle>No nodes yet</EmptyTitle>
            <EmptyDescription>Add the first machine that should run Minecraft servers.</EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <AddNodeDialog />
          </EmptyContent>
        </Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="hidden sm:table-cell">Address</TableHead>
              <TableHead className="hidden md:table-cell">Resources</TableHead>
              <TableHead className="hidden md:table-cell">Agent</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {nodes.map((node) => (
              <TableRow key={node.id}>
                <TableCell>
                  <Link to="/nodes/$nodeId" params={{ nodeId: node.id }} className="font-medium hover:underline">
                    {node.name}
                  </Link>
                </TableCell>
                <TableCell>
                  <NodeStatusLabel status={node.status} />
                </TableCell>
                <TableCell className="hidden font-mono text-muted-foreground sm:table-cell">{node.address}</TableCell>
                <TableCell className="hidden text-muted-foreground md:table-cell">
                  {node.info?.cpuCount ? `${node.info.cpuCount} CPUs, ${formatBytes(node.info.memoryBytes)}` : "–"}
                </TableCell>
                <TableCell className="hidden text-muted-foreground md:table-cell">{node.info?.agentVersion ?? "–"}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </>
  )
}
