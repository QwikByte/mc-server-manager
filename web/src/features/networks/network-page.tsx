import { ArrowRightIcon, ArrowsSplitIcon, CubeIcon, GraphIcon, type Icon, UsersThreeIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, Link } from "@tanstack/react-router"
import { t } from "i18next"
import type { ReactNode } from "react"
import { Trans } from "react-i18next"
import { Callout, ErrorCallout } from "@/components/callout"
import { IconTile } from "@/components/icon-tile"
import { BackLink } from "@/components/back-link"
import { PageHeader } from "@/components/page-header"
import type { Tone } from "@/components/tone"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { allServersQuery } from "@/features/servers/api"
import { networkQuery } from "./api"
import { BackendList } from "./backend-list"
import { ApplyNetworkButton, DeleteNetworkButton } from "./network-actions"
import { ServerLabel } from "./server-label"
import { findServer } from "./servers"

const route = getRouteApi("/_app/networks/$networkId")

export function NetworkPage() {
  const manage = useAccess().can("networks.manage")
  const { networkId } = route.useParams()
  const { data: network, isPending, error } = useQuery(networkQuery(networkId))
  const { data: servers } = useQuery(allServersQuery)
  const proxy = network && findServer(servers, network.proxy)

  return (
    <>
      <BackLink to="/networks">{t("Networks")}</BackLink>
      {isPending ? (
        <Skeleton className="h-64 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : (
        <>
          <PageHeader
            icon={GraphIcon}
            tone="violet"
            title={network.name}
            description={proxy ? t("Players connect to {{node}} at port {{port}}.", { node: proxy.nodeName, port: proxy.port }) : undefined}
            actions={
              manage && (
                <>
                  <ApplyNetworkButton network={network} />
                  <DeleteNetworkButton network={network} />
                </>
              )
            }
          />
          <div className="grid items-center gap-3 lg:grid-cols-[1fr_auto_1fr_auto_1fr]">
            <Step icon={UsersThreeIcon} tone="info" label={t("Players")}>
              {proxy ? t("{{node}}, port {{port}}", { node: proxy.nodeName, port: proxy.port }) : "–"}
            </Step>
            <Arrow />
            <Step icon={ArrowsSplitIcon} tone="violet" label={t("Proxy · Velocity modern forwarding")}>
              <Link
                to="/nodes/$nodeId/servers/$serverId"
                params={{ nodeId: network.proxy.nodeId, serverId: network.proxy.serverId }}
                className="hover:underline"
              >
                <ServerLabel server={proxy} />
              </Link>
            </Step>
            <Arrow />
            <Step icon={CubeIcon} tone="success" label={t("Players join")}>
              {network.backends[0]?.name ?? "–"}
              <span className="font-normal text-muted-foreground">
                {network.backends.length > 1 && ` ${t("and {{count}} more", { count: network.backends.length - 1 })}`}
              </span>
            </Step>
          </div>
          <BackendList network={network} servers={servers} />
          <Callout role="note" title={t("Only players who join through the proxy can play")} className="mt-8">
            <Trans
              i18nKey="The servers of this network check the identity the proxy forwards and turn away direct connections. New Minecraft servers start with a whitelist: allow players with <command/> in the console of each server."
              components={{ command: <code className="font-mono">whitelist add &lt;name&gt;</code> }}
            />
          </Callout>
        </>
      )}
    </>
  )
}

function Step({ icon, tone, label, children }: { icon: Icon; tone: Tone; label: string; children: ReactNode }) {
  return (
    <div className="surface flex min-w-0 items-center gap-3 rounded-xl p-4">
      <IconTile icon={icon} tone={tone} />
      <div className="min-w-0">
        <p className="truncate text-xs text-muted-foreground">{label}</p>
        <div className="truncate text-sm font-semibold">{children}</div>
      </div>
    </div>
  )
}

function Arrow() {
  return <ArrowRightIcon aria-hidden className="mx-auto size-5 text-muted-foreground/60 max-lg:rotate-90" weight="bold" />
}
