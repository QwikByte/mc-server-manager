import { InfoIcon, MagnifyingGlassIcon, UsersThreeIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi } from "@tanstack/react-router"
import { t } from "i18next"
import { useState } from "react"
import { Callout } from "@/components/callout"
import { Choice } from "@/components/choice"
import { PageHeader } from "@/components/page-header"
import { Segmented } from "@/components/segmented"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import { SelectItem } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { type Network, networksQuery } from "@/features/networks/api"
import type { PlayerAction } from "./api"
import { useOnlinePlayers } from "./online"
import { OnlinePlayers } from "./online-players"
import { PlayerActionDialog, type Scope } from "./player-action-dialog"
import { type ListKind, PlayerListTab } from "./player-lists"
import type { PlayerSearch } from "./search"
import { SendDialog } from "./send-dialog"

const route = getRouteApi("/_app/players")

/** What a page about players asks of a dialog: an action on a player, or sending one to another server. */
export type PlayerDialog = { action: PlayerAction; name?: string; scopes: Scope[] } | { send: string; network: Network; from?: string }

/** The players online on all servers, and who is banned, whitelisted and operator, to act on them. */
export function PlayersPage() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const { can } = useAccess()
  const { data: networks = [] } = useQuery({ ...networksQuery, enabled: can("networks.view") })
  const { players, unnamed, isPending } = useOnlinePlayers()
  const [dialog, setDialog] = useState<PlayerDialog>()
  const network = networks.find((n) => n.id === search.network)
  const tab = search.tab ?? "online"
  const query = (search.q ?? "").toLowerCase()
  const set = (change: Partial<PlayerSearch>) => void navigate({ search: (s) => ({ ...s, ...change }), replace: true })
  const online = players.filter(
    (p) =>
      (!network || p.network?.id === network.id) && (p.name.toLowerCase().includes(query) || p.server.name.toLowerCase().includes(query)),
  )

  return (
    <>
      <PageHeader
        icon={UsersThreeIcon}
        tone="info"
        title={t("Players")}
        description={t("{{count}} players online", { count: players.length + unnamed, defaultValue_one: "{{count}} player online" })}
      />
      <div className="mb-6 flex flex-wrap items-center gap-3">
        <Segmented<ListKind | "online">
          label={t("List")}
          value={tab}
          onChange={(tab) => set({ tab: tab === "online" ? undefined : tab })}
          options={[
            { value: "online", label: t("Online") },
            { value: "banned", label: t("Banned") },
            { value: "whitelisted", label: t("Whitelist") },
            { value: "operators", label: t("Operators") },
          ]}
        />
        {networks.length > 0 && (
          <Choice label={t("Network")} value={network?.id} onChange={(network) => set({ network })} everything={t("All servers")}>
            {networks.map((n) => (
              <SelectItem key={n.id} value={n.id}>
                {n.name}
              </SelectItem>
            ))}
          </Choice>
        )}
        <InputGroup className="w-full sm:max-w-xs">
          <InputGroupAddon>
            <MagnifyingGlassIcon />
          </InputGroupAddon>
          <InputGroupInput
            type="search"
            placeholder={t("Search players")}
            aria-label={t("Search players")}
            value={search.q ?? ""}
            onChange={(e) => set({ q: e.target.value || undefined })}
          />
        </InputGroup>
      </div>
      {tab === "online" ? (
        <>
          {unnamed > 0 && (
            <Callout icon={InfoIcon} className="mb-4">
              {t("{{count}} more players are online on servers whose console doesn't answer, which only tell how many play.", {
                count: unnamed,
                defaultValue_one: "Another player is online on a server whose console doesn't answer, which only tells how many play.",
              })}
            </Callout>
          )}
          {isPending ? <Skeleton className="h-64 rounded-xl" /> : <OnlinePlayers players={online} onAct={setDialog} />}
        </>
      ) : (
        <PlayerListTab kind={tab} network={network} query={query} onAct={setDialog} />
      )}
      {dialog &&
        ("send" in dialog ? (
          <SendDialog name={dialog.send} network={dialog.network} from={dialog.from} onClose={() => setDialog(undefined)} />
        ) : (
          <PlayerActionDialog action={dialog.action} name={dialog.name} scopes={dialog.scopes} onClose={() => setDialog(undefined)} />
        ))}
    </>
  )
}
