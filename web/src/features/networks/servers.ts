import type { NodeServer } from "@/features/servers/api"
import type { Forwarding, Network, ServerRef } from "./api"

export const proxyTypes = ["velocity", "bungeecord", "waterfall"]
/** BungeeCord and its fork Waterfall share their configuration. */
export const isBungee = (type: string) => type === "bungeecord" || type === "waterfall"

/**
 * Game servers that can verify the players a proxy forwards: Paper and Purpur themselves,
 * Fabric with FabricProxy-Lite, which only knows modern forwarding, Forge and NeoForge with
 * Proxy-Compatible-Forge. The panel installs the mods.
 */
export function canJoin(type: string, forwarding: Forwarding) {
  return ["paper", "purpur", "forge", "neoforge"].includes(type) || (type === "fabric" && forwarding === "modern")
}

/** The forwarding mod a server type gets, if any. */
export function forwardingMod(type: string) {
  return { fabric: "FabricProxy-Lite", forge: "Proxy-Compatible-Forge", neoforge: "Proxy-Compatible-Forge" }[type]
}

export const key = ({ nodeId, serverId }: ServerRef) => `${nodeId}/${serverId}`
export const refOf = (server: NodeServer): ServerRef => ({ nodeId: server.nodeId, serverId: server.id })

/** Finds a server; null while the servers are loading, undefined if it is unreachable. */
export function findServer(servers: NodeServer[] | undefined, ref: ServerRef) {
  return servers ? servers.find((s) => key(refOf(s)) === key(ref)) : null
}

/** Servers matching ok that are not part of a network yet, except those of network. */
export function availableServers(servers: NodeServer[] = [], networks: Network[] = [], ok: (s: NodeServer) => boolean, network?: Network) {
  const used = new Set(networks.filter((n) => n.id !== network?.id).flatMap((n) => [n.proxy, ...n.backends]).map(key))
  return servers.filter((s) => ok(s) && !used.has(key(refOf(s))))
}

/** Derives the name players use for a server, e.g. "Survival 2" becomes "survival-2", unique among taken. */
export function backendName(serverName: string, taken: string[]) {
  const slug = serverName.toLowerCase().replace(/[^a-z0-9]+/g, "-")
  let base = slug.slice(0, 28).replace(/^-+|-+$/g, "") || "server"
  if (base === "try") base = "try-server" // reserved for Velocity's list of servers to try
  let name = base
  for (let i = 2; taken.includes(name); i++) name = `${base}-${i}`
  return name
}

/** The command that lets only the proxy's node reach a server's port, for Docker's DOCKER-USER chain. */
export function firewallCommand(port: number, proxyHost: string) {
  return `iptables -I DOCKER-USER -p tcp -m conntrack --ctorigdstport ${port} --ctdir ORIGINAL ! -s ${proxyHost} -j DROP`
}

/** The host of a node's address, e.g. 203.0.113.10 of 203.0.113.10:7443. */
export function hostOf(address?: string) {
  if (!address) return undefined
  const host = address.slice(0, address.lastIndexOf(":"))
  return host.replace(/^\[|\]$/g, "")
}
