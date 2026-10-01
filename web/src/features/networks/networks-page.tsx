import { GraphIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { PageHeader } from "@/components/page-header"
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { allServersQuery } from "@/features/servers/api"
import { networksQuery } from "./api"
import { CreateNetworkDialog } from "./create-network-dialog"
import { ServerLabel } from "./server-label"
import { findServer } from "./servers"

export function NetworksPage() {
  const { data: networks, isPending, error } = useQuery(networksQuery)
  const { data: servers } = useQuery(allServersQuery)

  return (
    <>
      <PageHeader
        title="Networks"
        description="Players join through a Velocity proxy and switch between the servers behind it."
        actions={<CreateNetworkDialog />}
      />
      {isPending ? (
        <Skeleton className="h-40" />
      ) : error ? (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      ) : networks.length === 0 ? (
        <Empty className="border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <GraphIcon />
            </EmptyMedia>
            <EmptyTitle>No networks yet</EmptyTitle>
            <EmptyDescription>
              Create a Velocity proxy and a Paper or Purpur server on your nodes, then connect them to a network.
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <CreateNetworkDialog />
          </EmptyContent>
        </Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Proxy</TableHead>
              <TableHead className="hidden sm:table-cell">Port</TableHead>
              <TableHead className="hidden md:table-cell">Servers</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {networks.map((network) => {
              const proxy = findServer(servers, network.proxy)
              return (
                <TableRow key={network.id}>
                  <TableCell>
                    <Link to="/networks/$networkId" params={{ networkId: network.id }} className="font-medium hover:underline">
                      {network.name}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <ServerLabel server={proxy} />
                  </TableCell>
                  <TableCell className="hidden font-mono sm:table-cell">{proxy?.port ?? "–"}</TableCell>
                  <TableCell className="hidden font-mono text-muted-foreground md:table-cell">
                    {network.backends.map((b) => b.name).join(", ")}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
    </>
  )
}
