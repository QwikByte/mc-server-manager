import { ChatCircleTextIcon, XIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { TableCell, TableHead } from "@/components/ui/table"
import { useAccess } from "@/features/access/use-access"
import type { Network } from "@/features/networks/api"
import { playerActions } from "./actions"
import { maxPlayers, type PlayerAction } from "./api"
import type { PlayerDialog } from "./player-dialog"
import { useScopes } from "./scopes"
import { type Picked, type Selection, unique } from "./selection"

/** The head of the column that selects players, which selects all of them. */
export function SelectHead({ selection, players }: { selection?: Selection; players: Picked[] }) {
  if (!selection) return null
  const count = players.filter((p) => selection.has(p.name)).length
  return (
    <TableHead className="w-10 pl-4">
      <Checkbox
        aria-label={t("Select all players")}
        checked={count > 0 && count === players.length ? true : count > 0 ? "indeterminate" : false}
        onCheckedChange={(on) => selection.set(players, on === true)}
      />
    </TableHead>
  )
}

/** The cell that selects a player. */
export function SelectCell({ selection, player }: { selection?: Selection; player: Picked }) {
  if (!selection) return null
  return (
    <TableCell className="pl-4">
      <Checkbox
        checked={selection.has(player.name)}
        aria-label={t("Select {{name}}", { name: player.name })}
        onCheckedChange={(on) => selection.set([player], on === true)}
      />
    </TableCell>
  )
}

/**
 * Acts on the selected players at once, up to maxPlayers: the actions of a list, e.g. a ban, where they are (named by
 * where), in their network or on all servers, and a message to them where they are.
 */
export function PlayerBulkBar({
  selection,
  actions,
  where,
  network,
  onAct,
}: {
  selection: Selection
  actions: (PlayerAction | "message")[]
  where: string
  network?: Network
  onAct: (dialog: PlayerDialog) => void
}) {
  const { can } = useAccess()
  const scopesFor = useScopes()
  const { picked } = selection
  if (picked.length === 0) return null
  const names = picked.map((p) => p.name)
  const servers = unique(picked.flatMap((p) => p.servers))
  const tooMany = names.length > maxPlayers
  return (
    <div className="sticky bottom-4 z-20 mt-6 flex flex-wrap items-center gap-2 rounded-xl bg-popover/90 px-3 py-2.5 shadow-2xl ring-1 ring-foreground/10 backdrop-blur-xl">
      <Button variant="ghost" size="icon-sm" aria-label={t("Clear the selection")} title={t("Clear the selection")} onClick={selection.clear}>
        <XIcon />
      </Button>
      <p className="mr-auto text-sm font-medium tabular-nums">
        {tooMany ? t("Select up to {{max}} players", { max: maxPlayers }) : t("{{count}} selected", { count: names.length })}
      </p>
      {actions.map((action) => {
        if (action === "message") {
          const to = servers.filter((r) => can("console.commands", r.nodeId, r.serverId))
          return (
            <Button key={action} size="sm" variant="outline" disabled={tooMany || to.length === 0} onClick={() => onAct({ message: names, servers: to })}>
              <ChatCircleTextIcon />
              {t("Message…")}
            </Button>
          )
        }
        const scopes = scopesFor(action, { network, where: { label: where, servers } })
        const { icon: Icon, label } = playerActions[action]
        return (
          <Button key={action} size="sm" variant="outline" disabled={tooMany || scopes.length === 0} onClick={() => onAct({ action, names, scopes })}>
            <Icon />
            {label()}
          </Button>
        )
      })}
    </div>
  )
}
