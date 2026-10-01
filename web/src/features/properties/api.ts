import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"

export interface ServerProperties {
  /** False until the server started once and created server.properties. */
  exists: boolean
  properties: Record<string, string>
  /** Properties the manager sets itself, with the reason. */
  locked: { key: string; reason: string }[]
}

const path = (nodeId: string, serverId: string) => `/nodes/${nodeId}/servers/${serverId}/properties`

export const propertiesQuery = (nodeId: string, serverId: string) =>
  queryOptions({
    queryKey: ["properties", nodeId, serverId],
    queryFn: () => api<ServerProperties>(path(nodeId, serverId)),
  })

export function useUpdateProperties(nodeId: string, serverId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (properties: Record<string, string>) => api(path(nodeId, serverId), { method: "PUT", body: { properties } }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: propertiesQuery(nodeId, serverId).queryKey }),
  })
}
