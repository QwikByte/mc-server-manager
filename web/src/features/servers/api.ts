import { queryOptions, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { installPlugins } from "@/features/plugins/api"
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
}

export type RestartPolicy = "always" | "on_crash" | "never"

/** Settings of a server that can be changed after it was created. */
export type ServerSettings = Pick<
  Server,
  "name" | "version" | "memoryMb" | "port" | "java" | "restartPolicy" | "aikarFlags" | "jvmOptions" | "cpuLimit"
>

/** A server together with the node it runs on. */
export interface NodeServer extends Server {
  nodeId: string
  nodeName: string
}

export interface NewServer extends Partial<Pick<Server, "java" | "restartPolicy" | "aikarFlags" | "jvmOptions" | "cpuLimit">> {
  name: string
  type: string
  version: string
  memoryMb: number
  port: number
  acceptEula: boolean
  storage: string
  /** Written to server.properties before the first start. */
  properties?: Record<string, string>
}

export const serversQuery = (nodeId: string) =>
  queryOptions({
    queryKey: ["nodes", nodeId, "servers"],
    queryFn: () => api<Server[]>(`/nodes/${nodeId}/servers`),
    refetchInterval: 5_000,
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
  refetchInterval: 5_000,
})

/**
 * Creates a server, then installs Modrinth projects on it, e.g. the plugins of a template.
 * pluginError tells why they couldn't be installed; the server exists anyway.
 */
export function useCreateServer() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ nodeId, server, plugins = [] }: { nodeId: string; server: NewServer; plugins?: string[] }) => {
      const created = await api<Server>(`/nodes/${nodeId}/servers`, { body: server })
      if (plugins.length === 0) return { server: created }
      const [result] = await installPlugins(plugins, [{ nodeId, serverId: created.id }]).catch((e: Error) => [{ error: e.message }])
      return { server: created, pluginError: result.error }
    },
    onSettled: (_data, _error, { nodeId }) => queryClient.invalidateQueries({ queryKey: serversQuery(nodeId).queryKey }),
  })
}

/** Copies a server with all its data into a new server on the same node. */
export function useDuplicateServer(nodeId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, name, port }: { id: string; name: string; port: number }) =>
      api<Server>(`/nodes/${nodeId}/servers/${id}/duplicate`, { body: { name, port } }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: serversQuery(nodeId).queryKey }),
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

export function useServerAction(nodeId: string) {
  const queryClient = useQueryClient()
  return useMutation({
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
    mutationFn: (settings: ServerSettings) => api<Server>(`/nodes/${nodeId}/servers/${serverId}`, { method: "PUT", body: settings }),
    onSettled: () => queryClient.invalidateQueries({ queryKey: serversQuery(nodeId).queryKey }),
  })
}

/** Pulls the server's image again; updated tells whether a newer one came and the container was replaced. */
export function useUpdateImage(nodeId: string, serverId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => api<{ updated: boolean }>(`/nodes/${nodeId}/servers/${serverId}/update-image`, { method: "POST" }),
    onSettled: () => queryClient.invalidateQueries({ queryKey: serversQuery(nodeId).queryKey }),
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
