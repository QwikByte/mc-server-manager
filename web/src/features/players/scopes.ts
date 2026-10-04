import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { useAccess } from "@/features/access/use-access"
import type { Network, ServerRef } from "@/features/networks/api"
import { key, refOf } from "@/features/networks/servers"
import { allServersQuery, type NodeServer } from "@/features/servers/api"
import { serverType } from "@/features/servers/server-types"
import { needs } from "./actions"
import type { PlayerAction } from "./api"
import type { Scope } from "./player-action-dialog"

/**
 * Where an action on a player can go: the player's server, its network or all game servers,
 * each with the servers on which the user may do it, the first as the choice to start with.
 * Kicks only go to the player's server.
 */
export function useScopes() {
  const { can } = useAccess()
  const { data: servers = [] } = useQuery(allServersQuery)
  const games = servers.filter((s) => !serverType(s.type).proxy).map(refOf)

  return (action: PlayerAction, { server, network }: { server?: NodeServer; network?: Network } = {}): Scope[] => {
    const allowed = (refs: ServerRef[]) => refs.filter((r) => needs(action).every((p) => can(p, r.nodeId, r.serverId)))
    const scopes: Scope[] = []
    if (server) scopes.push({ label: t("Only {{server}}", { server: server.name }), servers: allowed([refOf(server)]) })
    if (action !== "kick") {
      if (network) scopes.push({ label: t("The network {{name}}", { name: network.name }), servers: allowed(network.backends) })
      scopes.push({ label: t("All servers"), servers: allowed(games) })
    }
    // A ban is meant for the whole network, if the player plays in one.
    if (action === "ban" && server && network) scopes.push(scopes.shift()!)
    // Scopes with the same servers, e.g. a network of all servers, are offered once.
    const seen = new Set<string>()
    return scopes.filter((s) => {
      const id = s.servers.map(key).sort().join()
      if (s.servers.length === 0 || seen.has(id)) return false
      seen.add(id)
      return true
    })
  }
}
