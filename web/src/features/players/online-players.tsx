import { DotsThreeIcon, PaperPlaneTiltIcon, UsersThreeIcon } from "@phosphor-icons/react"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { CsvButton } from "@/components/csv-button"
import { EmptyState } from "@/components/empty-state"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { SortableHead, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useAccess } from "@/features/access/use-access"
import { key, refOf } from "@/features/networks/servers"
import { serverKey } from "@/features/servers/api"
import { type Sorting, sortBy } from "@/lib/sort"
import { playerActions } from "./actions"
import type { PlayerAction } from "./api"
import type { OnlinePlayer } from "./online"
import type { PlayerDialog } from "./players-page"
import { useScopes } from "./scopes"
import type { OnlineSort } from "./search"

/** Players shown at most, so that the page stays quick; a search finds the others. */
const shown = 200
const menu: PlayerAction[] = ["kick", "ban", "whitelist_add", "op"]

/** What each column sorts the players by. */
const values: Record<OnlineSort, (p: OnlinePlayer) => string | undefined> = {
  name: (p) => p.name,
  server: (p) => p.server.name,
  network: (p) => p.network?.name,
}

/** The players online, with what can be done to each. */
export function OnlinePlayers({
  players,
  sorting,
  onAct,
}: {
  players: OnlinePlayer[]
  sorting: Sorting<OnlineSort>
  onAct: (dialog: PlayerDialog) => void
}) {
  const { can } = useAccess()
  const scopesFor = useScopes()
  if (players.length === 0) {
    return <EmptyState icon={UsersThreeIcon} title={t("No players online")} description={t("Players show up here while they play.")} />
  }
  const sorted = sortBy(players, sorting.order, values[sorting.by], (p) => p.name)
  return (
    <>
      <div className="mb-3 flex justify-end">
        <CsvButton
          name="players"
          rows={() => [
            ["player", "server", "server_id", "node", "node_id", "network"],
            ...sorted.map((p) => [p.name, p.server.name, p.server.id, p.server.nodeName, p.server.nodeId, p.network?.name]),
          ]}
        />
      </div>
      <div className="surface overflow-hidden rounded-xl">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <SortableHead sorting={sorting} column="name" className="pl-4">{t("Player")}</SortableHead>
              <SortableHead sorting={sorting} column="server">{t("Server")}</SortableHead>
              <SortableHead sorting={sorting} column="network" className="max-md:hidden">{t("Network")}</SortableHead>
              <TableHead className="w-0">
                <span className="sr-only">{t("Actions")}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sorted.slice(0, shown).map(({ name, server, network }) => {
              const actions = menu
                .map((action) => ({ action, scopes: scopesFor(action, { server, network }) }))
                .filter((a) => a.scopes.length > 0)
              const from = network?.backends.find((b) => key(b) === key(refOf(server)))?.name
              const sends = network && network.backends.length > 1 && can("players.manage", network.proxy.nodeId, network.proxy.serverId)
              return (
                <TableRow key={`${name}@${serverKey(server)}`}>
                  <TableCell className="pl-4">
                    <span className="flex items-center gap-3">
                      <span
                        aria-hidden
                        className="grid size-7 shrink-0 place-items-center rounded-full bg-primary/10 text-xs font-semibold text-primary"
                      >
                        {name.replace(/^\./, "")[0]?.toUpperCase()}
                      </span>
                      <span className="truncate font-mono font-medium">{name}</span>
                    </span>
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
                    {(actions.length > 0 || sends) && (
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon-sm" aria-label={t("Actions for {{name}}", { name })}>
                            <DotsThreeIcon weight="bold" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-60">
                          {sends && (
                            <DropdownMenuItem onSelect={() => onAct({ send: name, network, from })}>
                              <PaperPlaneTiltIcon />
                              {t("Send to another server…")}
                            </DropdownMenuItem>
                          )}
                          {actions.map(({ action, scopes }) => {
                            const { icon: Icon, label, destructive } = playerActions[action]
                            return (
                              <DropdownMenuItem
                                key={action}
                                variant={destructive ? "destructive" : "default"}
                                onSelect={() => onAct({ action, name, scopes })}
                              >
                                <Icon />
                                {label()}
                              </DropdownMenuItem>
                            )
                          })}
                        </DropdownMenuContent>
                      </DropdownMenu>
                    )}
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
