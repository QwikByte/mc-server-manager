import { CubeIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { formatMegabytes } from "@/lib/format"
import { type Server, serversQuery } from "./api"
import { CreateServerDialog } from "./create-server-dialog"
import { ServerActions } from "./server-actions"
import { ServerStateLabel } from "./server-state"
import { displayVersion, serverType } from "./server-types"

export function ServerList({ nodeId }: { nodeId: string }) {
  const { data: servers, isPending, error } = useQuery(serversQuery(nodeId))

  return (
    <section className="mt-10" aria-labelledby="servers-heading">
      <div className="mb-4 flex flex-wrap items-end justify-between gap-4">
        <h2 id="servers-heading" className="heading text-xl">
          Servers
        </h2>
        {servers && servers.length > 0 && <CreateServerDialog nodeId={nodeId} />}
      </div>
      {isPending ? (
        <Skeleton className="h-32" />
      ) : error ? (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      ) : servers.length === 0 ? (
        <Empty className="border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <CubeIcon />
            </EmptyMedia>
            <EmptyTitle>No servers on this node</EmptyTitle>
            <EmptyDescription>Create a game server or a proxy that connects servers to a network.</EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <CreateServerDialog nodeId={nodeId} />
          </EmptyContent>
        </Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="hidden sm:table-cell">Software</TableHead>
              <TableHead className="hidden md:table-cell">Port</TableHead>
              <TableHead className="hidden md:table-cell">Memory</TableHead>
              <TableHead>
                <span className="sr-only">Actions</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {servers.map((server) => (
              <ServerRow key={server.id} nodeId={nodeId} server={server} />
            ))}
          </TableBody>
        </Table>
      )}
    </section>
  )
}

function ServerRow({ nodeId, server }: { nodeId: string; server: Server }) {
  return (
    <TableRow>
      <TableCell>
        <Link to="/nodes/$nodeId/servers/$serverId" params={{ nodeId, serverId: server.id }} className="font-medium hover:underline">
          {server.name}
        </Link>
      </TableCell>
      <TableCell>
        <ServerStateLabel state={server.state} />
      </TableCell>
      <TableCell className="hidden sm:table-cell">
        {serverType(server.type).label} <span className="text-muted-foreground">{displayVersion(server.version)}</span>
      </TableCell>
      <TableCell className="hidden font-mono md:table-cell">{server.port}</TableCell>
      <TableCell className="hidden md:table-cell">{formatMegabytes(server.memoryMb)}</TableCell>
      <TableCell>
        <ServerActions nodeId={nodeId} server={server} />
      </TableCell>
    </TableRow>
  )
}
