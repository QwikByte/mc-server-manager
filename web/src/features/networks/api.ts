import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"

export interface ServerRef {
  nodeId: string
  serverId: string
}

/** A game server behind the proxy. Players switch to it with /server <name>. */
export interface Backend extends ServerRef {
  name: string
}

export interface Network {
  id: string
  name: string
  proxy: ServerRef
  /** Players join the first backend. */
  backends: Backend[]
  createdAt: string
}

export interface NewNetwork {
  name: string
  proxy: ServerRef
  lobby: ServerRef
}

export const networksQuery = queryOptions({
  queryKey: ["networks"],
  queryFn: () => api<Network[]>("/networks"),
})

export const networkQuery = (id: string) =>
  queryOptions({
    queryKey: ["networks", id],
    queryFn: () => api<Network>(`/networks/${id}`),
  })

export function useCreateNetwork() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (network: NewNetwork) => api<Network>("/networks", { body: network }),
    // A failed apply still saves the network, so the list is refreshed either way.
    onSettled: () => queryClient.invalidateQueries({ queryKey: networksQuery.queryKey }),
  })
}

export type NetworkChange =
  | { action: "apply" | "delete" }
  | { action: "add"; server: ServerRef }
  | { action: "remove" | "default"; serverId: string }

/** Changes a network. The master reconfigures and, if needed, restarts the affected servers. */
export function useChangeNetwork(id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (change: NetworkChange) => {
      const path = `/networks/${id}`
      switch (change.action) {
        case "apply":
          return api(`${path}/apply`, { method: "POST" })
        case "delete":
          return api(path, { method: "DELETE" })
        case "add":
          return api(`${path}/backends`, { body: change.server })
        case "remove":
          return api(`${path}/backends/${change.serverId}`, { method: "DELETE" })
        case "default":
          return api(`${path}/backends/${change.serverId}/default`, { method: "POST" })
      }
    },
    // A deleted network is only refreshed in the list, as its page is left.
    onSettled: (_data, _error, change) =>
      queryClient.invalidateQueries({ queryKey: networksQuery.queryKey, exact: change.action === "delete" }),
  })
}
