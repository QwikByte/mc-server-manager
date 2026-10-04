import { ClockIcon, CrownSimpleIcon, GavelIcon, ListChecksIcon, WarningIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { Callout, ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useAccess } from "@/features/access/use-access"
import type { Network, ServerRef } from "@/features/networks/api"
import { findServer } from "@/features/networks/servers"
import { allServersQuery } from "@/features/servers/api"
import { formatDate } from "@/lib/format"
import { msg } from "@/lib/i18n"
import { needs, playerActions } from "./actions"
import { type Listed, type PlayerAction, type PlayerLists, playerListsQuery } from "./api"
import type { PlayerDialog } from "./players-page"
import { useScopes } from "./scopes"
import type { PlayerSearch } from "./search"

export type ListKind = NonNullable<PlayerSearch["tab"]>

const kinds: Record<
  ListKind,
  { icon: typeof GavelIcon; add: PlayerAction; remove: PlayerAction; empty: string; where: (name: string) => string }
> = {
  banned: {
    icon: GavelIcon,
    add: "ban",
    remove: "pardon",
    empty: msg("Nobody is banned"),
    where: (name) => t("Where {{name}} is banned", { name }),
  },
  whitelisted: {
    icon: ListChecksIcon,
    add: "whitelist_add",
    remove: "whitelist_remove",
    empty: msg("Nobody is on the whitelist"),
    where: (name) => t("Where {{name}} is on the whitelist", { name }),
  },
  operators: {
    icon: CrownSimpleIcon,
    add: "op",
    remove: "deop",
    empty: msg("Nobody is an operator"),
    where: (name) => t("Where {{name}} is an operator", { name }),
  },
}

/** Who is banned, whitelisted or operator on the game servers of a network or on all, joined by player. */
export function PlayerListTab({
  kind,
  network,
  query,
  onAct,
}: {
  kind: ListKind
  network?: Network
  query: string
  onAct: (dialog: PlayerDialog) => void
}) {
  const { can } = useAccess()
  const scopesFor = useScopes()
  const { data, isPending, error } = useQuery(playerListsQuery(network?.id))
  const { data: servers } = useQuery(allServersQuery)
  if (isPending) return <Skeleton className="h-64 rounded-xl" />
  if (error) return <ErrorCallout error={error} />

  const { icon, add, remove, empty, where } = kinds[kind]
  const total = data.servers.length
  const entries = data[kind].filter((e) => e.name.toLowerCase().includes(query.toLowerCase()))
  const nameOf = (ref: ServerRef) => findServer(servers, ref)?.name ?? ref.serverId
  const failed = data.servers.filter((s) => s.error)
  const pending = data.servers.flatMap((s) => s.pending.map((p) => ({ ...p, server: nameOf(s) })))
  const addScopes = scopesFor(add, { network })
  const allowed = (refs: ServerRef[]) => refs.filter((r) => needs(remove).every((p) => can(p, r.nodeId, r.serverId)))
  const AddIcon = playerActions[add].icon

  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">
          {t("Lists of {{count}} game servers", { count: total, defaultValue_one: "Lists of {{count}} game server" })}
        </p>
        <div className="flex flex-wrap gap-2">
          {kind === "whitelisted" && <WhitelistSwitch lists={data} network={network} onAct={onAct} />}
          {addScopes.length > 0 && (
            <Button variant="outline" onClick={() => onAct({ action: add, scopes: addScopes })}>
              <AddIcon />
              {playerActions[add].label()}
            </Button>
          )}
        </div>
      </div>
      {failed.length > 0 && (
        <Callout tone="warning" icon={WarningIcon} title={t("Some servers didn't answer")}>
          {failed.map((s) => `${nameOf(s)}: ${s.error}`).join(" · ")}
        </Callout>
      )}
      {pending.length > 0 && (
        <Callout
          icon={ClockIcon}
          title={t("{{count}} changes wait for stopped servers", {
            count: pending.length,
            defaultValue_one: "A change waits for a stopped server",
          })}
        >
          <ul className="mt-1 grid gap-0.5">
            {pending.slice(0, 10).map((p, i) => (
              <li key={i}>
                {p.server}: {playerActions[p.action].title(p.name ?? "")}
              </li>
            ))}
          </ul>
        </Callout>
      )}
      {entries.length === 0 ? (
        <EmptyState icon={icon} title={query ? t("Nobody found") : t(empty)} />
      ) : (
        <div className="surface overflow-hidden rounded-xl">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="pl-4">{t("Player")}</TableHead>
                {kind === "banned" && <TableHead className="max-md:hidden">{t("Reason")}</TableHead>}
                <TableHead>{t("Servers")}</TableHead>
                <TableHead className="w-0">
                  <span className="sr-only">{t("Actions")}</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.map((entry) => (
                <ListRow
                  key={entry.name}
                  entry={entry}
                  total={total}
                  banned={kind === "banned"}
                  names={entry.servers.map(nameOf)}
                  onRemove={
                    allowed(entry.servers).length > 0
                      ? () =>
                          onAct({
                            action: remove,
                            name: entry.name,
                            scopes: [{ label: where(entry.name), servers: allowed(entry.servers) }],
                          })
                      : undefined
                  }
                  remove={playerActions[remove]}
                />
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}

function ListRow({
  entry,
  total,
  banned,
  names,
  onRemove,
  remove,
}: {
  entry: Listed
  total: number
  banned: boolean
  names: string[]
  onRemove?: () => void
  remove: (typeof playerActions)[PlayerAction]
}) {
  return (
    <TableRow>
      <TableCell className="pl-4 font-mono font-medium">{entry.name}</TableCell>
      {banned && (
        <TableCell className="max-w-80 whitespace-normal text-muted-foreground max-md:hidden">
          {entry.reason}
          {entry.since && <span className="block text-xs">{t("Since {{date}}", { date: formatDate(entry.since) })}</span>}
        </TableCell>
      )}
      <TableCell title={names.slice(0, 20).join(", ")} className="tabular-nums">
        {entry.servers.length === total
          ? t("All {{count}} servers", { count: total, defaultValue_one: "The only server" })
          : t("{{done}} of {{count}} servers", { done: entry.servers.length, count: total })}
      </TableCell>
      <TableCell className="pr-3 text-right">
        {onRemove && (
          <Button variant="ghost" size="sm" onClick={onRemove} aria-label={remove.label()}>
            <remove.icon />
            <span className="max-sm:hidden">{remove.label()}</span>
          </Button>
        )}
      </TableCell>
    </TableRow>
  )
}

/** Whether the whitelist is on, and switching it on or off on all servers. */
function WhitelistSwitch({ lists, network, onAct }: { lists: PlayerLists; network?: Network; onAct: (dialog: PlayerDialog) => void }) {
  const scopesFor = useScopes()
  const on = lists.servers.filter((s) => s.whitelistEnabled).length
  const next: PlayerAction = on === lists.servers.length ? "whitelist_off" : "whitelist_on"
  const scopes = scopesFor(next, { network })
  return (
    <>
      <span className="self-center text-sm text-muted-foreground">
        {t("Whitelist active on {{done}} of {{count}} servers", { done: on, count: lists.servers.length })}
      </span>
      {scopes.length > 0 && (
        <Button variant="outline" onClick={() => onAct({ action: next, scopes })}>
          {playerActions[next].label()}
        </Button>
      )}
    </>
  )
}
