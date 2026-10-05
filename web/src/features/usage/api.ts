import { keepPreviousData, queryOptions, useQueries, useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { msg } from "@/lib/i18n"

export interface NodeUsage {
  /** CPU time used, in thousandths of a core. */
  cpuMillis: number
  cpuCount: number
  memoryUsedBytes: number
  memoryTotalBytes: number
}

export interface ServerUsage {
  id: string
  /** Stopped servers only have the size of their data. */
  running: boolean
  cpuMillis: number
  memoryBytes: number
  memoryLimitBytes: number
  /** Bytes per second. */
  networkReceived: number
  networkSent: number
  /** Size of the data, measured every few minutes; 0 until it was measured. */
  diskBytes: number
  /** Missing if the server didn't answer; for a proxy, the players of the whole network. */
  players?: { online: number; max: number; names: string[] }
  /** Ticks per second over the last minute; only Paper and its forks except Folia tell it. */
  tps?: number
  /** A game server outside of networks runs in offline mode: anyone who reaches it joins under any name. */
  offlineMode?: boolean
}

/** The latest measurement of a node's agent, with what the user may see. */
export interface Usage {
  time: string
  node?: NodeUsage
  servers: ServerUsage[]
}

export const usageQuery = (nodeId: string) =>
  queryOptions({
    queryKey: ["nodes", nodeId, "usage"],
    queryFn: () => api<Usage>(`/nodes/${nodeId}/usage`),
    refetchInterval: 5_000,
  })

/** The latest usage of the given nodes and their servers. */
export function useUsages(nodeIds: string[]) {
  const nodes = [...new Set(nodeIds)]
  const usages = useQueries({ queries: nodes.map(usageQuery) })
  const servers = new Map<string, ServerUsage>()
  nodes.forEach((nodeId, i) => usages[i].data?.servers.forEach((u) => servers.set(`${nodeId}/${u.id}`, u)))
  return {
    server: (nodeId: string, serverId: string) => servers.get(`${nodeId}/${serverId}`),
    node: (nodeId: string) => usages[nodes.indexOf(nodeId)]?.data?.node,
  }
}

export function useServerUsage(nodeId: string, serverId: string) {
  const query = useQuery(usageQuery(nodeId))
  return { ...query, usage: query.data?.servers.find((s) => s.id === serverId) }
}

export type UsageRange = "day" | "week"

export const ranges: Record<UsageRange, { label: string; span: number }> = {
  day: { label: msg("24 hours"), span: 24 * 3_600_000 },
  week: { label: msg("7 days"), span: 7 * 24 * 3_600_000 },
}

/** The average usage over a step; players is the most during the step. */
export interface UsagePoint {
  time: string
  cpuMillis: number
  memoryBytes: number
  networkReceived: number
  networkSent: number
  diskBytes: number
  players: number | null
  tps: number | null
}

export interface UsageHistory {
  /** Length of the steps in seconds. Steps in which nothing ran are missing. */
  step: number
  points: UsagePoint[]
}

/** The history of a node, or of one of its servers. */
export const historyQuery = (nodeId: string, serverId: string | undefined, range: UsageRange) =>
  queryOptions({
    queryKey: ["nodes", nodeId, "usage", serverId ?? "", range],
    queryFn: () =>
      api<UsageHistory>(
        serverId ? `/nodes/${nodeId}/servers/${serverId}/usage/history?range=${range}` : `/nodes/${nodeId}/usage/history?range=${range}`,
      ),
    refetchInterval: 60_000,
    placeholderData: keepPreviousData,
  })
