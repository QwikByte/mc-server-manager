import { useQuery } from "@tanstack/react-query"
import { useAccess } from "@/features/access/use-access"
import { datastoresQuery } from "@/features/datastores/api"
import { networksQuery } from "@/features/networks/api"
import { useNetworkOf } from "@/features/networks/servers"
import { playersOnline } from "@/features/networks/usage"
import { nodesQuery } from "@/features/nodes/api"
import { overlayQuery } from "@/features/overlay/api"
import { allServersQuery, type NodeServer } from "@/features/servers/api"
import { serverType } from "@/features/servers/server-types"
import { useUsages, warningsQuery } from "@/features/usage/api"
import { problemsOf } from "./attention"
import { useAutomationTasks } from "./tasks"

/** What the widgets of the overview show. Each widget loads it; the queries are shared. */
export function useOverview() {
  const access = useAccess()
  const { data: servers } = useQuery(allServersQuery)
  const { data: nodes = [] } = useQuery(nodesQuery)
  const { data: networks = [] } = useQuery({ ...networksQuery, enabled: access.can("networks.view") })
  const online = nodes.filter((n) => n.status === "online")
  const usages = useUsages(online.map((n) => n.id))
  const networkOf = useNetworkOf()
  const usage = (s: { nodeId: string; serverId: string }) => usages.server(s.nodeId, s.serverId)
  const ref = (s: NodeServer) => ({ nodeId: s.nodeId, serverId: s.id })
  const gameServers = (servers ?? []).filter((s) => !serverType(s.type).proxy)
  // A player counts once: in the network they joined, or on a server outside of networks.
  const players =
    networks.reduce((sum, n) => sum + (playersOnline(n, usage) ?? 0), 0) +
    gameServers.filter((s) => !networkOf(ref(s))).reduce((sum, s) => sum + (usage(ref(s))?.players?.online ?? 0), 0)
  // The share of the CPU cores of the online nodes in use, once they told it.
  const live = online.flatMap((n) => usages.node(n.id) ?? [])
  const cores = live.reduce((sum, u) => sum + u.cpuCount * 1000, 0)
  const load = cores ? live.reduce((sum, u) => sum + u.cpuMillis, 0) / cores : undefined
  return { servers, nodes, online, networks, usages, usage, ref, gameServers, players, load }
}

/** What needs an operator, the most urgent first. */
export function useProblems() {
  const access = useAccess()
  const { nodes, servers = [], networks, usages } = useOverview()
  const { data: overlay } = useQuery(overlayQuery)
  const { data: datastores } = useQuery({ ...datastoresQuery, enabled: access.can("datastores.view") })
  const tasks = useAutomationTasks()
  const { data: warnings } = useQuery(warningsQuery)
  return problemsOf(nodes, servers, networks, usages, overlay, datastores, tasks, warnings)
}
