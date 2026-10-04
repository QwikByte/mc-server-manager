import { ArrowRightIcon, GraphIcon, ShieldCheckIcon, ShieldWarningIcon, UsersThreeIcon, WrenchIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { EmptyState } from "@/components/empty-state"
import { IconTile } from "@/components/icon-tile"
import { PageHeader } from "@/components/page-header"
import { Pill, StatusDot } from "@/components/status"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { allServersQuery, type NodeServer } from "@/features/servers/api"
import { StateBar } from "@/features/servers/server-state"
import { serverStates, serverType } from "@/features/servers/server-types"
import type { ServerUsage } from "@/features/usage/api"
import { maintenanceQuery, type Network, networksQuery, type ServerRef } from "./api"
import { CreateNetworkDialog } from "./create-network-dialog"
import { ServerLabel } from "./server-label"
import { findServer, key } from "./servers"
import { playersOnline, useNetworkUsage } from "./usage"

export function NetworksPage() {
  const manage = useAccess().can("networks.manage")
  const { data: networks, isPending, error } = useQuery(networksQuery)
  const { data: servers } = useQuery(allServersQuery)
  const usage = useNetworkUsage(networks)

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
          description={t("Create a Velocity, BungeeCord or Waterfall proxy and game servers on your nodes, then connect them to a network.")}
        >
          {manage && <CreateNetworkDialog />}
        </EmptyState>
      ) : (
        <ul className="grid gap-4 md:grid-cols-2">
          {networks.map((network) => (
            <li key={network.id}>
              <NetworkCard network={network} servers={servers} usage={usage} />
            </li>
          ))}
        </ul>
      )}
    </>
  )
}

const maxChips = 8

function NetworkCard({ network, servers, usage }: { network: Network; servers?: NodeServer[]; usage: (ref: ServerRef) => ServerUsage | undefined }) {
  const proxy = findServer(servers, network.proxy)
  const players = playersOnline(network, usage)
  const { data: maintenance } = useQuery(maintenanceQuery(network.id))
  const backends = network.backends.map((b) => findServer(servers, b))
  // A few servers by name; those that crash come first.
  const crashing = (i: number) => Number(backends[i]?.state !== "crashing")
  const shown = network.backends
    .map((b, i) => ({ b, i }))
    .sort((x, y) => crashing(x.i) - crashing(y.i))
    .slice(0, maxChips)
    .map(({ b }) => b)
  return (
    <Link
      to="/networks/$networkId"
      params={{ networkId: network.id }}
      className="group surface flex h-full flex-col gap-5 rounded-xl p-5 transition-all outline-none hover:-translate-y-0.5 hover:shadow-lg hover:ring-violet/40 focus-visible:ring-2 focus-visible:ring-ring motion-reduce:hover:translate-y-0"
    >
      <div className="flex items-start gap-3">
        <IconTile icon={GraphIcon} tone="violet" />
        <div className="min-w-0 flex-1">
          <p className="flex items-center gap-2 font-semibold">
            <span className="truncate">{network.name}</span>
            {maintenance?.enabled && (
              <Pill tone="warning" className="px-2">
                <WrenchIcon weight="bold" />
                {t("Maintenance")}
              </Pill>
            )}
          </p>
          <p className="text-xs text-muted-foreground">
            {t("{{count}} servers behind the proxy", { count: network.backends.length, defaultValue_one: "{{count}} server behind the proxy" })}
          </p>
        </div>
        {players !== undefined && (
          <Chip icon={UsersThreeIcon}>{t("{{count}} online", { count: players })}</Chip>
        )}
      </div>
      <div className="flex items-center justify-between gap-3 rounded-lg bg-muted/70 px-3 py-2.5 text-sm">
        <div className="min-w-0">
          <p className="mb-1 text-xs text-muted-foreground">{serverType(network.proxyType).label}</p>
          <ServerLabel server={proxy} />
        </div>
        <Chip icon={network.forwarding === "modern" ? ShieldCheckIcon : ShieldWarningIcon}>
          {network.forwarding === "modern" ? t("Modern") : t("Legacy")}
        </Chip>
      </div>
      <div className="mt-auto space-y-3">
        {backends.some(Boolean) && <StateBar servers={backends.filter((b) => !!b)} />}
        <div className="flex items-end justify-between gap-3">
          <div className="flex min-w-0 flex-wrap gap-1.5">
            {shown.map((b) => {
              const server = findServer(servers, b)
              return (
                <Chip key={key(b)} className="font-mono font-normal">
                  {server && <StatusDot status={serverStates[server.state]} />}
                  {b.name}
                </Chip>
              )
            })}
            {network.backends.length > shown.length && (
              <Chip className="text-muted-foreground">{t("+{{count}} more", { count: network.backends.length - shown.length })}</Chip>
            )}
          </div>
          <ArrowRightIcon className="size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
        </div>
      </div>
    </Link>
  )
}
