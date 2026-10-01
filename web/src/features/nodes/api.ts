import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"

export type NodeStatus = "pending" | "online" | "offline"

export interface NodeInfo {
  agentVersion: string
  hostname: string
  os: string
  cpuCount: number
  memoryBytes: number
  runtime: string
}

export interface Node {
  id: string
  name: string
  address: string
  enrolledAt?: string
  createdAt: string
  status: NodeStatus
  info?: NodeInfo
}

export const nodesQuery = queryOptions({
  queryKey: ["nodes"],
  queryFn: () => api<Node[]>("/nodes"),
  refetchInterval: 10_000,
})

export const nodeQuery = (id: string) =>
  queryOptions({
    queryKey: ["nodes", id],
    queryFn: () => api<Node>(`/nodes/${id}`),
    refetchInterval: 10_000,
  })

export function useCreateNode() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: { name: string; address: string }) => api<{ node: Node; joinToken: string }>("/nodes", { body: input }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: nodesQuery.queryKey }),
  })
}

export function useNewJoinToken(id: string) {
  return useMutation({
    mutationFn: () => api<{ joinToken: string }>(`/nodes/${id}/join-token`, { method: "POST" }),
  })
}

export function useDeleteNode() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api(`/nodes/${id}`, { method: "DELETE" }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: nodesQuery.queryKey }),
  })
}
