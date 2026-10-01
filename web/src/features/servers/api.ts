import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
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
}

export interface NewServer {
  name: string
  type: string
  version: string
  memoryMb: number
  port: number
  acceptEula: boolean
}

export const serversQuery = (nodeId: string) =>
  queryOptions({
    queryKey: ["nodes", nodeId, "servers"],
    queryFn: () => api<Server[]>(`/nodes/${nodeId}/servers`),
    refetchInterval: 5_000,
  })

export function useCreateServer(nodeId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (server: NewServer) => api<Server>(`/nodes/${nodeId}/servers`, { body: server }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: serversQuery(nodeId).queryKey }),
  })
}

export type ServerAction = "start" | "stop" | "delete"

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
