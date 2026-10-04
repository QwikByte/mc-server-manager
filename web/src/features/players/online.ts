import { useQuery } from "@tanstack/react-query"
import { useAccess } from "@/features/access/use-access"
import type { Network } from "@/features/networks/api"
import { useNetworkOf } from "@/features/networks/servers"
import { allServersQuery, type NodeServer } from "@/features/servers/api"
import { serverType } from "@/features/servers/server-types"
import { useUsages } from "@/features/usage/api"

export interface OnlinePlayer {
  name: string
  server: NodeServer
  network?: Network
}

/**
 * The players on the game servers the user may see, as their nodes measured them last. Servers
 * whose console doesn't answer only tell some names; unnamed counts the others.
 */
export function useOnlinePlayers(enabled = true) {
  const { canSomewhere } = useAccess()
  const { data: servers = [], isPending } = useQuery({ ...allServersQuery, enabled: enabled && canSomewhere("servers.view") })
  const games = servers.filter((s) => !serverType(s.type).proxy && s.state !== "stopped")
  const usages = useUsages(games.map((s) => s.nodeId))
  const networkOf = useNetworkOf()
  const players: OnlinePlayer[] = []
  let unnamed = 0
  for (const server of games) {
    const online = usages.server(server.nodeId, server.id)?.players
    if (!online) continue
    const network = networkOf({ nodeId: server.nodeId, serverId: server.id })
    for (const name of online.names) players.push({ name, server, network })
    unnamed += Math.max(online.online - online.names.length, 0)
  }
  players.sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: "base" }))
  return { players, unnamed, servers, isPending }
}
