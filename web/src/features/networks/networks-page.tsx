import { ArrowRightIcon, GraphIcon, HashIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { EmptyState } from "@/components/empty-state"
import { IconTile } from "@/components/icon-tile"
import { PageHeader } from "@/components/page-header"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { allServersQuery, type NodeServer } from "@/features/servers/api"
import { type Network, networksQuery } from "./api"
import { CreateNetworkDialog } from "./create-network-dialog"
import { ServerLabel } from "./server-label"
import { findServer } from "./servers"

export function NetworksPage() {
  const manage = useAccess().can("networks.manage")
  const { data: networks, isPending, error } = useQuery(networksQuery)
  const { data: servers } = useQuery(allServersQuery)

  return (
    <>
      <PageHeader icon={GraphIcon} tone="violet" title={t("Networks")} actions={manage && <CreateNetworkDialog />} />
      {isPending ? (
        <div className="grid gap-4 md:grid-cols-2">
          {[0, 1].map((i) => (
            <Skeleton key={i} className="h-48 rounded-xl" />
          ))}
        </div>
      ) : error ? (
        <ErrorCallout error={error} />
      ) : networks.length === 0 ? (
        <EmptyState
          icon={GraphIcon}
          tone="violet"
          title={t("No networks yet")}
          description={t("Create a Velocity proxy and a Paper or Purpur server on your nodes, then connect them to a network.")}
        >
          {manage && <CreateNetworkDialog />}
        </EmptyState>
      ) : (
        <ul className="grid gap-4 md:grid-cols-2">
          {networks.map((network) => (
            <li key={network.id}>
              <NetworkCard network={network} servers={servers} />
            </li>
          ))}
        </ul>
      )}
    </>
  )
}

function NetworkCard({ network, servers }: { network: Network; servers?: NodeServer[] }) {
  const proxy = findServer(servers, network.proxy)
  return (
    <Link
      to="/networks/$networkId"
      params={{ networkId: network.id }}
      className="group surface flex h-full flex-col gap-5 rounded-xl p-5 transition-all outline-none hover:-translate-y-0.5 hover:shadow-lg hover:ring-violet/40 focus-visible:ring-2 focus-visible:ring-ring motion-reduce:hover:translate-y-0"
    >
      <div className="flex items-start gap-3">
        <IconTile icon={GraphIcon} tone="violet" />
        <div className="min-w-0 flex-1">
          <p className="truncate font-semibold">{network.name}</p>
          <p className="text-xs text-muted-foreground">
            {t("{{count}} servers behind the proxy", {
              count: network.backends.length,
              defaultValue_one: "{{count}} server behind the proxy",
            })}
          </p>
        </div>
        {proxy && (
          <Chip icon={HashIcon}>
            <span className="font-mono">{proxy.port}</span>
          </Chip>
        )}
      </div>
      <div className="rounded-lg bg-muted/70 px-3 py-2.5 text-sm">
        <p className="mb-1 text-xs text-muted-foreground">{t("Proxy")}</p>
        <ServerLabel server={proxy} />
      </div>
      <div className="mt-auto flex items-end justify-between gap-3">
        <div className="flex min-w-0 flex-wrap gap-1.5">
          {network.backends.map((b) => (
            <Chip key={b.serverId} className="font-mono font-normal">
              {b.name}
            </Chip>
          ))}
        </div>
        <ArrowRightIcon className="size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
      </div>
    </Link>
  )
}
