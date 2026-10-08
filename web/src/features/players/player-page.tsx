import {
  CalendarBlankIcon,
  ChatCircleTextIcon,
  ClockCounterClockwiseIcon,
  CrownSimpleIcon,
  DotsThreeIcon,
  GavelIcon,
  HardDrivesIcon,
  HourglassIcon,
  ListChecksIcon,
  PaperPlaneTiltIcon,
  UserIcon,
} from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, Link } from "@tanstack/react-router"
import { t } from "i18next"
import { useState } from "react"
import { BackLink } from "@/components/back-link"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { PageHeader } from "@/components/page-header"
import { usePageName } from "@/components/page-title"
import { Section } from "@/components/section"
import { StatCard } from "@/components/stat-card"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useAccess } from "@/features/access/use-access"
import type { ServerRef } from "@/features/networks/api"
import { findServer, key, refOf, useNetworkOf } from "@/features/networks/servers"
import { allServersQuery } from "@/features/servers/api"
import { formatAgo, formatDate, formatDateTime, formatMinutes } from "@/lib/format"
import { locale } from "@/lib/i18n"
import { useNow } from "@/lib/use-now"
import { needs, playerActions } from "./actions"
import { type Listed, type PlayerAction, type PlayerHistory, playerHistoryQuery, playerListsQuery, validPlayerName } from "./api"
import { useOnlinePlayers } from "./online"
import type { PlayerAt } from "./online-players"
import { type PlayerDialog, PlayerDialogs } from "./player-dialog"
import { PlayerAvatar } from "./player-name"
import { useScopes } from "./scopes"

const route = getRouteApi("/_app/players/$name")

/** Days the chart of activity shows at most. */
const chartDays = 60

/** A player: where and when they played in the time the master keeps, their entries in the lists, and what can be done to them. */
export function PlayerPage() {
  const { name } = route.useParams()
  usePageName(name)
  return (
    <>
      <BackLink to="/players">{t("Players")}</BackLink>
      {validPlayerName(name) ? (
        <Player name={name} />
      ) : (
        <EmptyState icon={UserIcon} title={t("Not a name of a player")} description={t("Names have up to 16 letters, digits and underscores.")} />
      )}
    </>
  )
}

function Player({ name }: { name: string }) {
  const { canSomewhere } = useAccess()
  const { data: history, isPending, error } = useQuery(playerHistoryQuery(name))
  const { data: lists } = useQuery({ ...playerListsQuery(), enabled: canSomewhere("servers.view") })
  const { data: servers } = useQuery(allServersQuery)
  const { players } = useOnlinePlayers()
  const networkOf = useNetworkOf()
  const now = useNow(true, 60_000)
  const [dialog, setDialog] = useState<PlayerDialog>()
  if (isPending) return <Skeleton className="h-96 rounded-xl" />
  if (error) return <ErrorCallout error={error} />

  const same = (other: string) => other.toLowerCase() === name.toLowerCase()
  const online = players.filter((p) => same(p.name))
  const shown = online[0]?.name ?? history.name
  const nameOf = (ref: ServerRef) => findServer(servers, ref)?.name ?? ref.serverId
  // Actions go to where the player plays now, or played last.
  const last = history.servers[0] && findServer(servers, history.servers[0])
  const at: PlayerAt = online[0] ?? { name: shown, server: last ?? undefined, network: last ? networkOf(refOf(last)) : undefined, offline: true }
  const entries = {
    banned: lists?.banned.find((e) => same(e.name)),
    whitelisted: lists?.whitelisted.find((e) => same(e.name)),
    operators: lists?.operators.find((e) => same(e.name)),
  }

  return (
    <>
      <PageHeader
        title={
          <span className="flex min-w-0 items-center gap-4">
            <PlayerAvatar name={shown} size="lg" />
            <span className="min-w-0 font-mono break-all">{shown}</span>
          </span>
        }
        badge={
          <>
            {online.length > 0 && (
              <Pill tone="success">{t("Online on {{server}}", { server: online.map((p) => p.server.name).join(", ") })}</Pill>
            )}
            {shown.startsWith(".") && <Pill tone="neutral">{t("Bedrock")}</Pill>}
          </>
        }
        description={
          history.servers.length > 0
            ? t("Seen on {{count}} servers in the time the master keeps", {
                count: history.servers.length,
                defaultValue_one: "Seen on one server in the time the master keeps",
              })
            : t("Not seen on your servers in the time the master keeps")
        }
        actions={<PlayerActions at={at} entries={entries} onAct={setDialog} />}
      />
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard icon={ClockCounterClockwiseIcon} tone="info" label={t("Last seen")} value={online.length > 0 ? t("Now") : history.servers.length > 0 ? formatAgo(history.lastSeen, now) : "–"}>
          {history.servers.length > 0 && formatDateTime(history.lastSeen)}
        </StatCard>
        <StatCard icon={HourglassIcon} tone="violet" label={t("Playtime")} value={formatMinutes(history.minutes)} />
        <StatCard icon={HardDrivesIcon} tone="success" label={t("Servers")} value={history.servers.length} />
        <StatCard icon={CalendarBlankIcon} tone="warning" label={t("First seen")} value={history.servers.length > 0 ? formatDate(history.firstSeen) : "–"} />
      </div>
      {history.days.length > 0 && (
        <Section title={t("Activity")} description={t("Minutes played per day, in UTC")}>
          <Activity days={history.days} now={now} />
        </Section>
      )}
      {history.servers.length > 0 && (
        <Section title={t("Servers")}>
          <div className="surface overflow-hidden rounded-xl">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="pl-4">{t("Server")}</TableHead>
                  <TableHead className="max-md:hidden">{t("Network")}</TableHead>
                  <TableHead>{t("Last seen")}</TableHead>
                  <TableHead className="pr-4 text-right">{t("Playtime")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {history.servers.map((s) => {
                  const network = networkOf(s)
                  return (
                    <TableRow key={key(s)}>
                      <TableCell className="pl-4">
                        <Link to="/nodes/$nodeId/servers/$serverId" params={{ nodeId: s.nodeId, serverId: s.serverId }} className="font-medium hover:underline">
                          {nameOf(s)}
                        </Link>
                      </TableCell>
                      <TableCell className="text-muted-foreground max-md:hidden">
                        {network ? (
                          <Link to="/networks/$networkId" params={{ networkId: network.id }} className="hover:underline">
                            {network.name}
                          </Link>
                        ) : (
                          "–"
                        )}
                      </TableCell>
                      <TableCell className="text-muted-foreground" title={formatDateTime(s.lastSeen)}>
                        {online.some((p) => key(refOf(p.server)) === key(s)) ? t("Now") : formatAgo(s.lastSeen, now)}
                      </TableCell>
                      <TableCell className="pr-4 text-right tabular-nums">{formatMinutes(s.minutes)}</TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        </Section>
      )}
      {lists && (
        <Section title={t("Lists")} description={t("Where the player is banned, on the whitelist or an operator, of {{count}} game servers", { count: lists.servers.length })}>
          <Entries entries={entries} total={lists.servers.length} nameOf={nameOf} />
        </Section>
      )}
      <PlayerDialogs dialog={dialog} onClose={() => setDialog(undefined)} />
    </>
  )
}

type Entries = Record<"banned" | "whitelisted" | "operators", Listed | undefined>

/** The minutes of each day, of the last chartDays up to today, as bars. */
function Activity({ days, now }: { days: PlayerHistory["days"]; now: number }) {
  const minutes = new Map(days.map((d) => [d.day, d.minutes]))
  const today = Math.floor(now / 86_400_000) * 86_400_000 // midnight in UTC
  const first = Math.max(Date.parse(days[0].day), today - (chartDays - 1) * 86_400_000)
  const shown = Array.from({ length: (today - first) / 86_400_000 + 1 }, (_, i) => new Date(first + i * 86_400_000).toISOString().slice(0, 10))
  const top = Math.max(...shown.map((d) => minutes.get(d) ?? 0), 1)
  const label = (day: string) => new Date(day).toLocaleDateString(locale, { dateStyle: "medium", timeZone: "UTC" })
  return (
    <div className="surface rounded-xl p-4">
      <div
        role="img"
        aria-label={t("Played on {{count}} of the last {{days}} days", { count: shown.filter((d) => minutes.has(d)).length, days: shown.length })}
        className="flex h-28 items-end gap-0.5"
      >
        {shown.map((day) => {
          const m = minutes.get(day) ?? 0
          return (
            <div key={day} title={`${label(day)}: ${formatMinutes(m)}`} className="flex h-full min-w-0 flex-1 items-end">
              <div className="w-full rounded-t-sm bg-primary/80" style={{ height: m ? `${Math.max((m / top) * 100, 4)}%` : "1px", opacity: m ? 1 : 0.3 }} />
            </div>
          )
        })}
      </div>
      <div className="mt-2 flex justify-between text-xs text-muted-foreground">
        <span>{label(shown[0])}</span>
        <span>{label(shown[shown.length - 1])}</span>
      </div>
    </div>
  )
}

const listInfo = {
  banned: { icon: GavelIcon, title: () => t("Banned") },
  whitelisted: { icon: ListChecksIcon, title: () => t("On the whitelist") },
  operators: { icon: CrownSimpleIcon, title: () => t("Operator") },
}

/** The player's entries in the lists, with their servers. */
function Entries({ entries, total, nameOf }: { entries: Entries; total: number; nameOf: (ref: ServerRef) => string }) {
  const found = (Object.keys(listInfo) as (keyof Entries)[]).filter((k) => entries[k])
  if (found.length === 0) return <p className="text-sm text-muted-foreground">{t("Not banned, on a whitelist or an operator on any server.")}</p>
  return (
    <ul className="grid gap-3 md:grid-cols-3">
      {found.map((kind) => {
        const entry = entries[kind]!
        const { icon: Icon, title } = listInfo[kind]
        return (
          <li key={kind} className="surface grid content-start gap-2 rounded-xl p-4">
            <p className="flex items-center gap-2 font-semibold">
              <Icon className="size-4 text-muted-foreground" />
              {title()}
              <span className="ml-auto text-xs font-normal text-muted-foreground tabular-nums">
                {t("{{done}} of {{count}} servers", { done: entry.servers.length, count: total })}
              </span>
            </p>
            {kind === "banned" && (entry.reason || entry.until) && (
              <p className="text-sm text-muted-foreground">
                {entry.reason}
                {entry.until && <span className="block text-xs">{t("Until {{date}}", { date: formatDateTime(entry.until) })}</span>}
              </p>
            )}
            <p className="text-sm break-words">{entry.servers.map(nameOf).join(", ")}</p>
          </li>
        )
      })}
    </ul>
  )
}

/** What can be done to the player: where online, send and kick, and everywhere ban, pardon, whitelist and make operator. */
function PlayerActions({ at, entries, onAct }: { at: PlayerAt; entries: Entries; onAct: (dialog: PlayerDialog) => void }) {
  const { can } = useAccess()
  const scopesFor = useScopes()
  const { name, server, network, offline } = at
  // Removing from a list goes to the servers whose list has the player.
  const where = (action: PlayerAction, entry: Listed | undefined, label: string) => {
    const servers = (entry?.servers ?? []).filter((r) => needs(action).every((p) => can(p, r.nodeId, r.serverId)))
    return servers.length > 0 ? [{ label, servers }] : []
  }
  const actions: { action: PlayerAction; scopes: ReturnType<typeof scopesFor> }[] = [
    { action: "kick", scopes: offline ? [] : scopesFor("kick", { server, network }) },
    { action: "ban", scopes: scopesFor("ban", { server, network }) },
    { action: "pardon", scopes: where("pardon", entries.banned, t("Where {{name}} is banned", { name })) },
    { action: "whitelist_add", scopes: scopesFor("whitelist_add", { server, network }) },
    { action: "whitelist_remove", scopes: where("whitelist_remove", entries.whitelisted, t("Where {{name}} is on the whitelist", { name })) },
    { action: "op", scopes: scopesFor("op", { server, network }) },
    { action: "deop", scopes: where("deop", entries.operators, t("Where {{name}} is an operator", { name })) },
  ]
  const allowed = actions.filter((a) => a.scopes.length > 0)
  const from = server && network?.backends.find((b) => key(b) === key(refOf(server)))?.name
  const sends = server && !offline && network && network.backends.length > 1 && can("players.manage", network.proxy.nodeId, network.proxy.serverId)
  const messages = server && !offline && can("console.commands", server.nodeId, server.id)
  if (allowed.length === 0 && !sends && !messages) return null
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline">
          <DotsThreeIcon weight="bold" />
          {t("Actions")}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        {messages && (
          <DropdownMenuItem onSelect={() => onAct({ message: [name], servers: [refOf(server)] })}>
            <ChatCircleTextIcon />
            {t("Message…")}
          </DropdownMenuItem>
        )}
        {sends && (
          <DropdownMenuItem onSelect={() => onAct({ send: name, network, from })}>
            <PaperPlaneTiltIcon />
            {t("Send to another server…")}
          </DropdownMenuItem>
        )}
        {allowed.map(({ action, scopes }) => {
          const { icon: Icon, label, destructive } = playerActions[action]
          return (
            <DropdownMenuItem key={action} variant={destructive ? "destructive" : "default"} onSelect={() => onAct({ action, names: [name], scopes })}>
              <Icon />
              {label()}
            </DropdownMenuItem>
          )
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
