import { type ServerUsage, useUsages } from "@/features/usage/api"
import type { Network, ServerRef } from "./api"

/** The latest usage of the servers of networks. */
export function useNetworkUsage(networks: Network[] = []) {
  const usages = useUsages(networks.flatMap((n) => [n.proxy, ...n.backends]).map((r) => r.nodeId))
  return (ref: ServerRef) => usages.server(ref.nodeId, ref.serverId)
}

/**
 * The players online in a network: those the proxy counts, or else those of its servers, as
 * BungeeCord doesn't answer the panel's status requests.
 */
export function playersOnline(network: Network, usage: (ref: ServerRef) => ServerUsage | undefined) {
  const proxy = usage(network.proxy)?.players
  if (proxy) return proxy.online
  const counts = network.backends.map((b) => usage(b)?.players?.online)
  return counts.some((c) => c !== undefined) ? counts.reduce<number>((sum, c) => sum + (c ?? 0), 0) : undefined
}
