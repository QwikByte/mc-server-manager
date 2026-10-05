import { ArrowsSplitIcon, DeviceMobileIcon, GlobeIcon, PlugIcon, ShieldCheckIcon, StackIcon, UsersIcon, UsersThreeIcon } from "@phosphor-icons/react"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { type ReactNode, useState } from "react"
import { StatusDot } from "@/components/status"
import type { NodeServer } from "@/features/servers/api"
import { serverStates, serverType, states, statusOf } from "@/features/servers/server-types"
import type { ServerUsage } from "@/features/usage/api"
import type { Backend, Network } from "./api"
import type { Draft } from "./draft"
import { ServerLabel } from "./server-label"
import { findServer, key, routeOf } from "./servers"

const row = 56 // height of an entry or a server, in pixels
const gap = 64 // width of the connections
const limit = 10 // servers shown until the map is expanded

/**
 * A map of the network: where players connect, the proxy, and the servers it sends them to,
 * with their state and players. It follows the draft, so changes show before they are saved.
 * Of many servers, it shows those players join, fall back to or reach by host name, and those
 * that crash, until it is expanded.
 */
export function Topology({
  network,
  draft,
  servers,
  usage,
  address,
  bedrockAddress,
  isPrivate,
}: {
  network: Network
  draft: Draft
  servers?: NodeServer[]
  usage: (ref: { nodeId: string; serverId: string }) => ServerUsage | undefined
  address?: string
  /** Where Bedrock players connect, if they may. */
  bedrockAddress?: string
  isPrivate: (a: string, b: string) => boolean
}) {
  const proxy = findServer(servers, network.proxy)
  const entries: { icon: typeof GlobeIcon; label: string }[] = [
    { icon: UsersThreeIcon, label: address ?? t("All players") },
    ...(bedrockAddress ? [{ icon: DeviceMobileIcon, label: t("{{address}} (Bedrock)", { address: bedrockAddress }) }] : []),
    ...draft.forcedHosts.map((h) => ({ icon: GlobeIcon, label: h.host || "…" })),
  ]
  const [expanded, setExpanded] = useState(false)
  const routed = new Set([...draft.try, ...draft.forcedHosts.flatMap((h) => h.servers)])
  const wanted = (b: Backend) => routed.has(key(b)) || findServer(servers, b)?.state === "crashing"
  const picked = new Set([...draft.backends.filter(wanted), ...draft.backends.filter((b) => !wanted(b))].slice(0, limit))
  const shown = expanded ? draft.backends : draft.backends.filter((b) => picked.has(b))
  const rest = draft.backends.filter((b) => !shown.includes(b))
  const foldable = draft.backends.length > limit
  const rows = shown.length + Number(foldable)
  const height = Math.max(entries.length, rows, 2) * row
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
        <Connections height={height} from={[height / 2]} to={Array.from({ length: rows }, (_, i) => center(rows, i))} />
        <Column height={height} count={rows}>
          {shown.map((b) => {
            const server = findServer(servers, b)
            const position = draft.try.indexOf(key(b))
            const hosts = draft.forcedHosts.filter((h) => h.servers.includes(key(b))).length
            const players = usage(b)?.players?.online
            const route = routeOf(network, b, isPrivate)
            return (
              <Item key={key(b)}>
                {server ? <StatusDot status={statusOf(server)} label={t(statusOf(server).label)} /> : <StatusDot status={{ tone: "neutral", label: "" }} />}
                <Link to="/nodes/$nodeId/servers/$serverId" params={b} className="min-w-0 flex-1 truncate font-mono text-xs hover:underline">
                  {b.name}
                </Link>
                {position >= 0 && (
                  <span title={position === 0 ? t("Players join here") : t("Fallback {{position}}", { position })} className="grid size-5 place-items-center rounded-md bg-info/15 text-[0.6875rem] font-bold text-info">
                    {position + 1}
                  </span>
                )}
                {hosts > 0 && <GlobeIcon aria-label={t("Host names lead here")} className="size-4 text-violet" weight="duotone" />}
                {route === "private" && (
                  <ShieldCheckIcon aria-label={t("Over the private network")} className="size-4 text-success" weight="duotone">
                    <title>{t("Over the private network")}</title>
                  </ShieldCheckIcon>
                )}
                {route === "public" && (
                  <PlugIcon aria-label={t("At a public port")} className="size-4 text-muted-foreground" weight="duotone">
                    <title>{t("At a public port")}</title>
                  </PlugIcon>
                )}
                {players !== undefined && (
                  <span className="flex items-center gap-1 text-xs text-muted-foreground">
                    <UsersIcon className="size-3.5" />
                    {players}
                  </span>
                )}
              </Item>
            )
          })}
          {foldable && (
            <Item>
              <button
                type="button"
                aria-expanded={expanded}
                onClick={() => setExpanded(!expanded)}
                className="flex min-w-0 flex-1 items-center gap-2 text-left text-xs font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:underline"
              >
                <StackIcon className="size-4 shrink-0" weight="duotone" />
                {expanded ? t("Show fewer") : t("{{count}} more servers", { count: rest.length, defaultValue_one: "{{count}} more server" })}
                {!expanded && (
                  <span className="ml-auto flex items-center gap-2">
                    {states.map((state) => {
                      const count = rest.filter((b) => findServer(servers, b)?.state === state).length
                      return count > 0 ? (
                        <span key={state} className="flex items-center gap-1 tabular-nums">
                          <StatusDot status={serverStates[state]} label={t(serverStates[state].label)} />
                          {count}
                        </span>
                      ) : null
                    })}
                  </span>
                )}
              </button>
            </Item>
          )}
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
