import { useQueries } from "@tanstack/react-query"
import { type ServerUsage, usageQuery } from "@/features/usage/api"
import type { Network, ServerRef } from "./api"
import { key } from "./servers"

/** The latest usage of the servers of networks, by key. */
export function useNetworkUsage(networks: Network[] = []) {
  const refs: ServerRef[] = networks.flatMap((n) => [n.proxy, ...n.backends])
  const usages = useQueries({ queries: [...new Set(refs.map((r) => r.nodeId))].map(usageQuery) })
  const byKey = new Map<string, ServerUsage>()
  for (const [i, nodeId] of [...new Set(refs.map((r) => r.nodeId))].entries())
    for (const usage of usages[i]?.data?.servers ?? []) byKey.set(key({ nodeId, serverId: usage.id }), usage)
  return (ref: ServerRef) => byKey.get(key(ref))
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
