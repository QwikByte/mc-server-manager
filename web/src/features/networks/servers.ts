import type { NodeServer } from "@/features/servers/api"
import type { Network, ServerRef } from "./api"

/** Velocity is the only proxy whose player forwarding can't be spoofed. */
export const proxyTypes = ["velocity"]
/** Paper and Purpur verify the players the proxy forwards. */
export const backendTypes = ["paper", "purpur"]

export const key = ({ nodeId, serverId }: ServerRef) => `${nodeId}/${serverId}`
export const refOf = (server: NodeServer): ServerRef => ({ nodeId: server.nodeId, serverId: server.id })

/** Finds a server; null while the servers are loading, undefined if it is unreachable. */
export function findServer(servers: NodeServer[] | undefined, ref: ServerRef) {
  return servers ? servers.find((s) => key(refOf(s)) === key(ref)) : null
}

/** Servers of the given types that are not part of a network yet. */
export function availableServers(servers: NodeServer[] = [], networks: Network[] = [], types: string[]) {
  const used = new Set(networks.flatMap((n) => [n.proxy, ...n.backends]).map(key))
  return servers.filter((s) => types.includes(s.type) && !used.has(key(refOf(s))))
}
