import { ChatCircleTextIcon, DotsThreeIcon, PaperPlaneTiltIcon, UsersThreeIcon } from "@phosphor-icons/react"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { CsvButton } from "@/components/csv-button"
import { EmptyState } from "@/components/empty-state"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { SortableHead, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useAccess } from "@/features/access/use-access"
import type { Network } from "@/features/networks/api"
import { key, refOf } from "@/features/networks/servers"
import { type NodeServer, serverKey } from "@/features/servers/api"
import { type Sorting, sortBy } from "@/lib/sort"
import { cn } from "@/lib/utils"
import { playerActions } from "./actions"
import type { PlayerAction } from "./api"
import type { OnlinePlayer } from "./online"
import { SelectCell, SelectHead } from "./player-bulk-bar"
import type { PlayerDialog } from "./player-dialog"
import { PlayerName } from "./player-name"
import { useScopes } from "./scopes"
import type { OnlineSort } from "./search"
import type { Picked, Selection } from "./selection"

/** Players shown at most, so that the page stays quick; a search finds the others. */
const shown = 200
const menu: PlayerAction[] = ["kick", "ban", "whitelist_add", "op"]

/** What each column sorts the players by. */
const values: Record<OnlineSort, (p: OnlinePlayer) => string | undefined> = {
  name: (p) => p.name,
  server: (p) => p.server.name,
  network: (p) => p.network?.name,
}

const picked = (p: OnlinePlayer): Picked => ({ name: p.name, servers: [refOf(p.server)] })

/** The players online, with what can be done to each, and to those selected. */
export function OnlinePlayers({
  players,
  sorting,
  selection,
  onAct,
}: {
  players: OnlinePlayer[]
  sorting: Sorting<OnlineSort>
  selection?: Selection
  onAct: (dialog: PlayerDialog) => void
}) {
  if (players.length === 0) {
    return <EmptyState icon={UsersThreeIcon} title={t("No players online")} description={t("Players show up here while they play.")} />
  }
  const all = sortBy(players, sorting.order, values[sorting.by], (p) => p.name)
  const sorted = all.slice(0, shown)
  return (
    <>
      <div className="mb-3 flex justify-end">
        <CsvButton
          name="players"
          rows={() => [
            ["player", "server", "server_id", "node", "node_id", "network"],
            ...all.map((p) => [p.name, p.server.name, p.server.id, p.server.nodeName, p.server.nodeId, p.network?.name]),
          ]}
        />
      </div>
      <div className="surface overflow-hidden rounded-xl">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <SelectHead selection={selection} players={sorted.map(picked)} />
              <SortableHead sorting={sorting} column="name" className={cn(!selection && "pl-4")}>
                {t("Player")}
              </SortableHead>
              <SortableHead sorting={sorting} column="server">{t("Server")}</SortableHead>
              <SortableHead sorting={sorting} column="network" className="max-md:hidden">{t("Network")}</SortableHead>
              <TableHead className="w-0">
                <span className="sr-only">{t("Actions")}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sorted.map((player) => {
              const { name, server, network } = player
              return (
                <TableRow key={`${name}@${serverKey(server)}`} data-state={selection?.has(name) ? "selected" : undefined}>
                  <SelectCell selection={selection} player={picked(player)} />
                  <TableCell className={cn(!selection && "pl-4")}>
                    <PlayerName name={name} />
                  </TableCell>
                  <TableCell>
                    <Link
                      to="/nodes/$nodeId/servers/$serverId"
                      params={{ nodeId: server.nodeId, serverId: server.id }}
                      className="truncate hover:underline"
                    >
                      {server.name}
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
                  <TableCell className="pr-3 text-right">
                    <PlayerMenu player={player} onAct={onAct} />
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
      {players.length > shown && (
        <p className="mt-3 text-sm text-muted-foreground">
          {t("Shows {{shown}} of {{count}} players. Search to find the others.", { shown, count: players.length })}
        </p>
      )}
    </>
  )
}

/** A player of a server, online there unless offline, of a network, or of all servers. */
export interface PlayerAt {
  name: string
  server?: NodeServer
  network?: Network
  offline?: boolean
}

/**
 * What can be done to a player: while they're online, send them a message or send them to another server of the
 * network, kick, ban and more.
 */
export function PlayerMenu({ player: { name, server, network, offline }, onAct }: { player: PlayerAt; onAct: (dialog: PlayerDialog) => void }) {
  const { can } = useAccess()
  const scopesFor = useScopes()
  const online = server && !offline ? server : undefined
  const actions = menu
    .filter((action) => online || action !== "kick")
    .map((action) => ({ action, scopes: scopesFor(action, { server, network }) }))
    .filter((a) => a.scopes.length > 0)
  const from = online && network?.backends.find((b) => key(b) === key(refOf(online)))?.name
  const sends = online && network && network.backends.length > 1 && can("players.manage", network.proxy.nodeId, network.proxy.serverId)
  const messages = online && can("console.commands", online.nodeId, online.id)
  if (actions.length === 0 && !sends && !messages) return null
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={t("Actions for {{name}}", { name })}>
          <DotsThreeIcon weight="bold" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-60">
        {messages && (
          <DropdownMenuItem onSelect={() => onAct({ message: [name], servers: [refOf(online)] })}>
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
        {actions.map(({ action, scopes }) => {
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
