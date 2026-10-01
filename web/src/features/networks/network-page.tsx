import { CaretLeftIcon, InfoIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, Link } from "@tanstack/react-router"
import { PageHeader } from "@/components/page-header"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"
import { allServersQuery } from "@/features/servers/api"
import { networkQuery } from "./api"
import { BackendList } from "./backend-list"
import { ApplyNetworkButton, DeleteNetworkButton } from "./network-actions"
import { ServerLabel } from "./server-label"
import { findServer } from "./servers"

const route = getRouteApi("/_app/networks/$networkId")

export function NetworkPage() {
  const { networkId } = route.useParams()
  const { data: network, isPending, error } = useQuery(networkQuery(networkId))
  const { data: servers } = useQuery(allServersQuery)
  const proxy = network && findServer(servers, network.proxy)

  return (
    <>
      <Link to="/networks" className="mb-4 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <CaretLeftIcon className="size-3.5" />
        Networks
      </Link>
      {isPending ? (
        <Skeleton className="h-64" />
      ) : error ? (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      ) : (
        <>
          <PageHeader
            title={network.name}
            description={proxy ? `Players connect to ${proxy.nodeName} at port ${proxy.port}.` : undefined}
            actions={
              <>
                <ApplyNetworkButton network={network} />
                <DeleteNetworkButton network={network} />
              </>
            }
          />
          <dl className="flex flex-wrap gap-x-10 gap-y-4 border-y py-4">
            <div>
              <dt className="text-xs text-muted-foreground">Proxy</dt>
              <dd className="mt-1 text-sm">
                <Link
                  to="/nodes/$nodeId/servers/$serverId"
                  params={{ nodeId: network.proxy.nodeId, serverId: network.proxy.serverId }}
                  className="hover:underline"
                >
                  <ServerLabel server={proxy} />
                </Link>
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">Player forwarding</dt>
              <dd className="mt-1 text-sm">Velocity modern</dd>
            </div>
          </dl>
          <BackendList network={network} servers={servers} />
          <Alert role="note" className="mt-8 max-w-3xl">
            <InfoIcon />
            <AlertTitle>Only players who join through the proxy can play</AlertTitle>
            <AlertDescription>
              <p>
                The servers of this network check the identity the proxy forwards and turn away direct connections. New Minecraft servers
                start with a whitelist: allow players with <code className="font-mono">whitelist add &lt;name&gt;</code> in the console of
                each server.
              </p>
            </AlertDescription>
          </Alert>
        </>
      )}
    </>
  )
}
