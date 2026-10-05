import { type QueryClient, queryOptions, useMutation, useMutationState, useQuery, useQueryClient } from "@tanstack/react-query"
import type { ModpackChoice } from "@/features/modpacks/api"
import { type Operation, operate } from "@/features/operations/api"
import { api } from "@/lib/api"

export type ServerState = "stopped" | "starting" | "running" | "crashing"

export interface Server {
  id: string
  name: string
  type: string
  version: string
  memoryMb: number
  port: number
  state: ServerState
  /** Crashes since the server was last started, while it crashes or after it stopped because of one. */
  crashes: number
  /** Exit code of the latest crash; 0 if unknown. */
  exitCode: number
  storage: string
  java: string
  restartPolicy: RestartPolicy
  aikarFlags: boolean
  jvmOptions: string[]
  /** CPU cores the server may use; 0 means no limit. */
  cpuLimit: number
  /** Version of the mod loader of a modded server, e.g. one a modpack needs; empty for the newest. */
  loaderVersion: string
  /** Labels such as lobby, sorted; only in lists of servers. */
  tags: string[]
}

export type RestartPolicy = "always" | "on_crash" | "never"

/** Settings of a server that can be changed after it was created. */
export type ServerSettings = Pick<
  Server,
  "name" | "version" | "memoryMb" | "port" | "java" | "restartPolicy" | "aikarFlags" | "jvmOptions" | "cpuLimit" | "loaderVersion"
>

/** A server together with the node it runs on. */
export interface NodeServer extends Server {
  nodeId: string
  nodeName: string
}

/** Identifies a server across nodes, e.g. to select it. */
export const serverKey = (s: { nodeId: string; id: string }) => `${s.nodeId}/${s.id}`

export interface NewServer
  extends Partial<Pick<Server, "java" | "restartPolicy" | "aikarFlags" | "jvmOptions" | "cpuLimit" | "loaderVersion">> {
  name: string
  type: string
  version: string
  memoryMb: number
  port: number
  acceptEula: boolean
  storage: string
  /** Written to server.properties before the first start. */
  properties?: Record<string, string>
  /** A modpack decides the type and the versions of the server. */
  modpack?: ModpackChoice
}

/** How often lists of servers are checked: more often while servers start, to show them running soon. */
const interval = (servers?: Server[]) => (servers?.some((s) => s.state === "starting") ? 2_000 : 5_000)

export const serversQuery = (nodeId: string) =>
  queryOptions({
    queryKey: ["nodes", nodeId, "servers"],
    queryFn: () => api<Server[]>(`/nodes/${nodeId}/servers`),
    refetchInterval: (query) => interval(query.state.data),
  })

/** A server of a node, looked up in the list of the node's servers. */
export function useServer(nodeId: string, serverId: string) {
  const query = useQuery(serversQuery(nodeId))
  return { ...query, server: query.data?.find((s) => s.id === serverId) }
}

/** The servers of all reachable nodes; servers of offline nodes are missing. */
export const allServersQuery = queryOptions({
  queryKey: ["servers"],
  queryFn: () => api<NodeServer[]>("/servers"),
  refetchInterval: (query) => interval(query.state.data),
})

/** Learns of the operation an action becomes, to follow it; see operate. */
export interface Followed {
  onStart?: (op: Operation) => void
}

/**
 * Creates a server, with the projects of Modrinth or Hangar to install on it, e.g. the plugins of a
 * template. pluginError tells why they couldn't be installed; the server exists anyway.
 */
export function useCreateServer() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, server, plugins, onStart }: { nodeId: string; server: NewServer; plugins?: string[] } & Followed) =>
      operate<Server & { pluginError?: string }>(`/nodes/${nodeId}/servers`, { body: { ...server, plugins } }, onStart),
    onSettled: (_data, _error, { nodeId }) => refreshServers(queryClient, nodeId),
  })
}

/** Copies a server with all its data into a new server on the same node. */
export function useDuplicateServer(nodeId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, name, port, onStart }: { id: string; name: string; port: number } & Followed) =>
      operate<Server>(`/nodes/${nodeId}/servers/${id}/duplicate`, { body: { name, port } }, onStart),
    onSettled: () => refreshServers(queryClient, nodeId),
  })
}

export type MovePhase = "stopping" | "copying" | "backups" | "finishing" | "done" | "failed"

/** A server moving to another node; the master shows finished moves for an hour. */
export interface Move {
  serverId: string
  serverName: string
  from: string
  to: string
  toName: string
  phase: MovePhase
  /** How much of the data and backups was copied, as compressed archives. */
  bytes: number
  backups: number
  backupsTotal: number
  /** Why it failed; the server stayed where it was. */
  error?: string
  /** What went wrong once the server was on the new node. */
  warnings: string[]
  startedAt: string
  finishedAt?: string
}

/** The moves of the servers the user may see, checked often while one is in progress. */
export const movesQuery = queryOptions({
  queryKey: ["moves"],
  queryFn: () => api<Move[]>("/moves"),
  refetchInterval: (query) => (query.state.data?.some((m) => !m.finishedAt) ? 1_500 : 15_000),
})

/** The latest move of a server, if it moved within the last hour or is moving. */
export function useMove(serverId: string) {
  return useQuery(movesQuery).data?.find((m) => m.serverId === serverId)
}

export interface MoveRequest {
  node: string
  port: number
  storage: string
  /** Whether the backups move too; otherwise they are deleted with the server on its old node. */
  backups: boolean
}

export function useMoveServer(nodeId: string, serverId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: MoveRequest) => api<Move>(`/nodes/${nodeId}/servers/${serverId}/move`, { body: input }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: movesQuery.queryKey }),
  })
}

export type ServerAction = "start" | "stop" | "restart" | "delete"

export type BulkAction = { action: "start" | "stop" | "restart" } | { action: "command"; command: string }

/** How an action ended on each server; failed ones have an error. */
export interface BulkResult {
  nodeId: string
  serverId: string
  error?: string
}

const refsOf = (servers: Pick<NodeServer, "nodeId" | "id">[]) => servers.map((s) => ({ nodeId: s.nodeId, serverId: s.id }))

/** Refreshes the lists of servers of a node, or of all nodes, after a change to servers on it. */
const refreshServers = (queryClient: QueryClient, nodeId?: string) =>
  queryClient.invalidateQueries({
    predicate: ({ queryKey }) =>
      queryKey[0] === "servers" || (queryKey[0] === "nodes" && (!nodeId || queryKey[1] === nodeId) && queryKey[2] === "servers"),
  })

/** Starts, stops or restarts servers on any nodes, or sends them a console command. */
export function useBulkAction() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationKey: ["server-action"],
    mutationFn: ({ servers, onStart, ...action }: BulkAction & { servers: NodeServer[] } & Followed) =>
      operate<{ results: BulkResult[] }>("/servers/actions", { body: { ...action, servers: refsOf(servers) } }, onStart).then(
        (r) => r.results,
      ),
    onSettled: () => refreshServers(queryClient),
  })
}

/** Adds and removes tags of servers. */
export function useChangeTags() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ servers, add = [], remove = [] }: { servers: Pick<NodeServer, "nodeId" | "id">[]; add?: string[]; remove?: string[] }) =>
      api("/servers/tags", { body: { servers: refsOf(servers), add, remove } }),
    onSettled: () => refreshServers(queryClient),
  })
}

/**
 * The action that runs on a server right now, started in this browser, e.g. stop while the
 * server shuts down.
 */
export function usePendingAction(nodeId: string, serverId: string): BulkAction["action"] | "delete" | undefined {
  const pending = useMutationState({
    filters: { mutationKey: ["server-action"], status: "pending" },
    select: (m) => ({
      node: m.options.mutationKey?.[1],
      vars: m.state.variables as { id: string; action: ServerAction } | (BulkAction & { servers: NodeServer[] }),
    }),
  })
  for (const { node, vars } of pending) {
    if ("servers" in vars && vars.servers.some((s) => s.nodeId === nodeId && s.id === serverId)) return vars.action
    if ("id" in vars && node === nodeId && vars.id === serverId) return vars.action
  }
  return undefined
}

export function useServerAction(nodeId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationKey: ["server-action", nodeId],
    mutationFn: ({ id, action }: { id: string; action: ServerAction }) =>
      action === "delete"
        ? api(`/nodes/${nodeId}/servers/${id}`, { method: "DELETE" })
        : api(`/nodes/${nodeId}/servers/${id}/${action}`, { method: "POST" }),
    onSettled: () => queryClient.invalidateQueries({ queryKey: serversQuery(nodeId).queryKey }),
  })
}

export function useUpdateServer(nodeId: string, serverId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ settings, onStart }: { settings: ServerSettings } & Followed) =>
      operate<Server>(`/nodes/${nodeId}/servers/${serverId}`, { method: "PUT", body: settings }, onStart),
    onSettled: () => refreshServers(queryClient, nodeId),
  })
}

/** Pulls the server's image again; updated tells whether a newer one came and the container was replaced. */
export function useUpdateImage(nodeId: string, serverId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ onStart }: Followed = {}) =>
      operate<{ updated: boolean }>(`/nodes/${nodeId}/servers/${serverId}/update-image`, { method: "POST" }, onStart),
    onSettled: () => refreshServers(queryClient, nodeId),
  })
}

export function useSendCommand(nodeId: string, serverId: string) {
  return useMutation({
    mutationFn: (command: string) => api<{ output: string }>(`/nodes/${nodeId}/servers/${serverId}/command`, { body: { command } }),
  })
}

/** A server, or all servers of a node (including later ones) if serverId is empty. */
export interface Target {
  nodeId: string
  serverId: string
}
