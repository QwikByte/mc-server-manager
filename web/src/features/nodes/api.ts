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
  /** Directories the node allows for server data, the default location first. */
  storage: StorageLocation[]
}

export interface StorageLocation {
  name: string
  path: string
  freeBytes: number
  totalBytes: number
}

/** Limits for the servers of a node; new nodes get the defaults of the master's settings. */
export interface NodeLimits {
  /** Port range of servers; null allows any port. */
  portMin: number | null
  portMax: number | null
  /** Memory kept free for the system; null allows assigning more than the node has. */
  memoryReserveMb: number | null
}

/** Settings that apply to the servers of a node. */
export interface NodeSettings extends NodeLimits {
  /** Storage location preselected for new servers. */
  defaultStorage: string
}

/** A join token works once, until it expires. */
export interface JoinToken {
  joinToken: string
  joinTokenExpiresAt: string
}

export interface Node extends NodeSettings {
  id: string
  name: string
  address: string
  enrolledAt?: string
  createdAt: string
  status: NodeStatus
  info?: NodeInfo
  certificateExpiresAt?: string
}

/** Memory in MB that servers on a node can get, or undefined if it isn't limited or known. */
export function memoryLimitMb(node: Node): number | undefined {
  if (node.memoryReserveMb === null || !node.info?.memoryBytes) return undefined
  return Math.max(0, Math.floor(node.info.memoryBytes / 1024 ** 2) - node.memoryReserveMb)
}

/** Memory in MB that servers on a node can get in total: the limit, or else all of the node's memory. */
export function memoryCapacityMb(node: Node): number | undefined {
  return memoryLimitMb(node) ?? (node.info?.memoryBytes ? Math.floor(node.info.memoryBytes / 1024 ** 2) : undefined)
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
    mutationFn: (input: { name: string; address: string }) => api<JoinToken & { node: Node }>("/nodes", { body: input }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: nodesQuery.queryKey }),
  })
}

export function useUpdateNode(id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: NodeSettings & { name: string; address: string }) => api<Node>(`/nodes/${id}`, { method: "PUT", body: input }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: nodesQuery.queryKey }),
  })
}

export function useNewJoinToken(id: string) {
  return useMutation({
    mutationFn: () => api<JoinToken>(`/nodes/${id}/join-token`, { method: "POST" }),
  })
}

export function useDeleteNode() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api(`/nodes/${id}`, { method: "DELETE" }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: nodesQuery.queryKey }),
  })
}

export function useRenewCertificate(id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => api<{ certificateExpiresAt: string }>(`/nodes/${id}/certificate`, { method: "POST" }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: nodesQuery.queryKey }),
  })
}
