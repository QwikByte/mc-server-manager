import { ArrowsSplitIcon, GlobeIcon, UsersIcon, UsersThreeIcon } from "@phosphor-icons/react"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import type { ReactNode } from "react"
import { StatusDot } from "@/components/status"
import type { NodeServer } from "@/features/servers/api"
import { serverStates, serverType } from "@/features/servers/server-types"
import type { ServerUsage } from "@/features/usage/api"
import type { Network } from "./api"
import type { Draft } from "./draft"
import { ServerLabel } from "./server-label"
import { findServer, key } from "./servers"

const row = 56 // height of an entry or a server, in pixels
const gap = 64 // width of the connections

/**
 * A map of the network: where players connect, the proxy, and the servers it sends them to,
 * with their state and players. It follows the draft, so changes show before they are saved.
 */
export function Topology({
  network,
  draft,
  servers,
  usage,
  address,
}: {
  network: Network
  draft: Draft
  servers?: NodeServer[]
  usage: (ref: { nodeId: string; serverId: string }) => ServerUsage | undefined
  address?: string
}) {
  const proxy = findServer(servers, network.proxy)
  const entries: { icon: typeof GlobeIcon; label: string }[] = [
    { icon: UsersThreeIcon, label: address ?? t("All players") },
    ...draft.forcedHosts.map((h) => ({ icon: GlobeIcon, label: h.host || "…" })),
  ]
  const height = Math.max(entries.length, draft.backends.length, 2) * row
  const center = (count: number, i: number) => (height - count * row) / 2 + row * (i + 0.5)

  return (
    <div className="surface overflow-x-auto rounded-xl p-5" aria-label={t("Map of the network")} role="figure">
      <div className="grid min-w-[44rem] items-center" style={{ gridTemplateColumns: `minmax(10rem, 15rem) ${gap}px 13rem ${gap}px minmax(14rem, 1fr)` }}>
        <Column height={height} count={entries.length}>
          {entries.map(({ icon: Icon, label }, i) => (
            <Item key={i}>
              <Icon className="size-4 shrink-0 text-muted-foreground" weight="duotone" />
              <span className="truncate font-mono text-xs">{label}</span>
            </Item>
          ))}
        </Column>
        <Connections height={height} from={entries.map((_, i) => center(entries.length, i))} to={[height / 2]} />
        <div className="rounded-xl bg-violet/10 p-4 ring-1 ring-violet/25">
          <p className="mb-1 flex items-center gap-1.5 text-xs font-medium text-violet">
            <ArrowsSplitIcon className="size-3.5" weight="bold" />
            {serverType(network.proxyType).label}
          </p>
          <Link to="/nodes/$nodeId/servers/$serverId" params={network.proxy} className="text-sm hover:underline">
            <ServerLabel server={proxy} />
          </Link>
          <p className="mt-1 text-xs text-muted-foreground">
            {draft.forwarding === "modern" ? t("Modern forwarding") : t("Legacy forwarding")}
          </p>
        </div>
        <Connections height={height} from={[height / 2]} to={draft.backends.map((_, i) => center(draft.backends.length, i))} />
        <Column height={height} count={draft.backends.length}>
          {draft.backends.map((b) => {
            const server = findServer(servers, b)
            const position = draft.try.indexOf(key(b))
            const hosts = draft.forcedHosts.filter((h) => h.servers.includes(key(b))).length
            const players = usage(b)?.players?.online
            return (
              <Item key={key(b)}>
                {server ? <StatusDot status={serverStates[server.state]} label={t(serverStates[server.state].label)} /> : <StatusDot status={{ tone: "neutral", label: "" }} />}
                <Link to="/nodes/$nodeId/servers/$serverId" params={b} className="min-w-0 flex-1 truncate font-mono text-xs hover:underline">
                  {b.name}
                </Link>
                {position >= 0 && (
                  <span title={position === 0 ? t("Players join here") : t("Fallback {{position}}", { position })} className="grid size-5 place-items-center rounded-md bg-info/15 text-[0.6875rem] font-bold text-info">
                    {position + 1}
                  </span>
                )}
                {hosts > 0 && <GlobeIcon aria-label={t("Host names lead here")} className="size-4 text-violet" weight="duotone" />}
                {players !== undefined && (
                  <span className="flex items-center gap-1 text-xs text-muted-foreground">
                    <UsersIcon className="size-3.5" />
                    {players}
                  </span>
                )}
              </Item>
            )
          })}
        </Column>
      </div>
    </div>
  )
}

function Column({ height, count, children }: { height: number; count: number; children: ReactNode }) {
  return (
    <ul className="flex flex-col justify-center" style={{ height, paddingBlock: (height - count * row) / 2 }}>
      {children}
    </ul>
  )
}

function Item({ children }: { children: ReactNode }) {
  return (
    <li className="flex items-center" style={{ height: row }}>
      <span className="flex h-10 w-full min-w-0 items-center gap-2 rounded-lg bg-muted/70 px-3 ring-1 ring-foreground/5">{children}</span>
    </li>
  )
}

/** Curves from each point on the left to each point on the right. */
function Connections({ height, from, to }: { height: number; from: number[]; to: number[] }) {
  return (
    <svg aria-hidden width={gap} height={height} className="overflow-visible text-muted-foreground/40">
      {from.flatMap((y1, i) =>
        to.map((y2, j) => (
          <path key={`${i}-${j}`} d={`M0 ${y1} C${gap / 2} ${y1}, ${gap / 2} ${y2}, ${gap} ${y2}`} fill="none" stroke="currentColor" strokeWidth={1.5} />
        )),
      )}
      {to.length === 1 && <circle cx={gap} cy={to[0]} r={3} className="fill-current" />}
    </svg>
  )
}
