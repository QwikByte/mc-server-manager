import { ArrowsSplitIcon, CubeIcon, GraphIcon, HardDrivesIcon, HashIcon, ShieldCheckIcon, SlidersHorizontalIcon, UsersThreeIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, Link, Outlet } from "@tanstack/react-router"
import { t } from "i18next"
import { BackLink } from "@/components/back-link"
import { ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { PageHeader } from "@/components/page-header"
import { StatCard } from "@/components/stat-card"
import { TabLink } from "@/components/tab-link"
import { Tabs } from "@/components/tabs"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { allServersQuery } from "@/features/servers/api"
import { serverType } from "@/features/servers/server-types"
import { networkQuery } from "./api"
import { NetworkActions } from "./network-actions"
import { NetworkEditor } from "./network-editor"
import { ProxySettingsEditor } from "./proxy-settings"
import { ServerLabel } from "./server-label"
import { findServer } from "./servers"
import { playersOnline, useNetworkUsage } from "./usage"

const route = getRouteApi("/_app/networks/$networkId")

/** Header, key figures and tabs of a network; the tabs are child routes. */
export function NetworkPage() {
  const { can } = useAccess()
  const { networkId } = route.useParams()
  const { data: network, isPending, error } = useQuery(networkQuery(networkId))
  const { data: servers } = useQuery(allServersQuery)
  const usage = useNetworkUsage(network ? [network] : [])

  if (isPending)
    return (
      <>
        <BackLink to="/networks">{t("Networks")}</BackLink>
        <Skeleton className="h-96 rounded-xl" />
      </>
    )
  if (error)
    return (
      <>
        <BackLink to="/networks">{t("Networks")}</BackLink>
        <ErrorCallout error={error} />
      </>
    )
  const proxy = findServer(servers, network.proxy)
  const backends = network.backends.map((b) => findServer(servers, b))
  const running = backends.filter((s) => s && s.state !== "stopped").length
  const players = playersOnline(network, usage)

  return (
    <>
      <BackLink to="/networks">{t("Networks")}</BackLink>
      <PageHeader
        icon={GraphIcon}
        tone="violet"
        title={network.name}
        description={
          <span className="mt-1 flex flex-wrap gap-2 text-foreground">
            <Chip icon={ArrowsSplitIcon}>{serverType(network.proxyType).label}</Chip>
            <Chip icon={ShieldCheckIcon}>{network.forwarding === "modern" ? t("Modern forwarding") : t("Legacy forwarding")}</Chip>
            {proxy && <Chip icon={HardDrivesIcon}>{proxy.nodeName}</Chip>}
            {proxy && (
              <Chip icon={HashIcon}>
                <span className="font-mono">{proxy.port}</span>
              </Chip>
            )}
          </span>
        }
        actions={<NetworkActions network={network} />}
      />
      <div className="mb-8 grid gap-3 sm:grid-cols-3">
        <StatCard icon={UsersThreeIcon} tone="info" label={t("Players online")} value={players ?? "–"} />
        <StatCard icon={CubeIcon} tone="success" label={t("Servers running")} value={`${running} / ${network.backends.length}`} />
        <StatCard icon={ArrowsSplitIcon} tone="violet" label={t("Proxy")} value={<ProxyLink network={network} />} />
      </div>
      <Tabs label={t("Network")}>
        <TabLink to="/networks/$networkId" params={{ networkId }} activeOptions={{ exact: true }}>
          <GraphIcon className="size-4" weight="duotone" />
          {t("Overview")}
        </TabLink>
        {can("properties.edit", network.proxy.nodeId, network.proxy.serverId) && (
          <TabLink to="/networks/$networkId/proxy" params={{ networkId }}>
            <SlidersHorizontalIcon className="size-4" weight="duotone" />
            {t("Proxy configuration")}
          </TabLink>
        )}
      </Tabs>
      <Outlet />
    </>
  )
}

function ProxyLink({ network }: { network: { proxy: { nodeId: string; serverId: string } } }) {
  const { data: servers } = useQuery(allServersQuery)
  return (
    <Link
      to="/nodes/$nodeId/servers/$serverId"
      params={{ nodeId: network.proxy.nodeId, serverId: network.proxy.serverId }}
      className="text-base hover:underline"
    >
      <ServerLabel server={findServer(servers, network.proxy)} />
    </Link>
  )
}

/** The Overview tab: where players go, the servers and the forwarding. */
export function NetworkOverview() {
  const { networkId } = route.useParams()
  const { data: network } = useQuery(networkQuery(networkId))
  return network ? <NetworkEditor network={network} /> : null
}

/** The Proxy configuration tab: the settings of the proxy in its own file. */
export function NetworkProxy() {
  const { networkId } = route.useParams()
  const { data: network } = useQuery(networkQuery(networkId))
  const { data: servers } = useQuery(allServersQuery)
  if (!network) return null
  const proxy = findServer(servers, network.proxy)
  if (proxy === null) return <Skeleton className="h-96 rounded-xl" />
  return <ProxySettingsEditor proxy={network.proxy} type={network.proxyType} running={!!proxy && proxy.state !== "stopped"} />
}
