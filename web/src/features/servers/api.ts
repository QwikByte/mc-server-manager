import { queryOptions, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"

export type ServerState = "stopped" | "starting" | "running"

export interface Server {
  id: string
  name: string
  type: string
  version: string
  memoryMb: number
  port: number
  state: ServerState
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

export interface NewServer {
  name: string
  type: string
  version: string
  memoryMb: number
  port: number
  acceptEula: boolean
  storage: string
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

export function useCreateServer(nodeId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (server: NewServer) => api<Server>(`/nodes/${nodeId}/servers`, { body: server }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: serversQuery(nodeId).queryKey }),
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

export function useSendCommand(nodeId: string, serverId: string) {
  return useMutation({
    mutationFn: (command: string) => api<{ output: string }>(`/nodes/${nodeId}/servers/${serverId}/command`, { body: { command } }),
  })
}
