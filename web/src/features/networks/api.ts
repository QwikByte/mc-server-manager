import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { operate } from "@/features/operations/api"
import type { Followed } from "@/features/servers/api"
import { api } from "@/lib/api"

export interface ServerRef {
  nodeId: string
  serverId: string
}

/** Velocity's modern forwarding, or BungeeCord's, which Velocity calls legacy. */
export type Forwarding = "modern" | "legacy"

/** A game server behind the proxy. Players switch to it with /server <name>. */
export interface Backend extends ServerRef {
  name: string
  /** BungeeCord: only players with the permission bungeecord.server.<name> may join it. */
  restricted: boolean
  /** BungeeCord: the MOTD shown for host names that lead to it; empty uses the proxy's. */
  motd: string
}

/** Players who connect through the host name join its servers, tried in this order. */
export interface ForcedHost {
  host: string
  servers: string[]
}

/** What can be changed about a network; its proxy stays. */
export interface NetworkSettings {
  name: string
  forwarding: Forwarding
  /** The operator confirmed that only the proxy's node reaches the servers on other nodes. */
  firewalled: boolean
  backends: Backend[]
  /** Names of the servers players join and fall back to, in this order. */
  try: string[]
  forcedHosts: ForcedHost[]
}

export interface Network extends NetworkSettings {
  id: string
  proxy: ServerRef
  /** velocity, bungeecord or waterfall. */
  proxyType: string
  createdAt: string
}

export interface NewNetwork {
  name: string
  proxy: ServerRef
  forwarding: Forwarding
  firewalled: boolean
  /** Players join the first. */
  servers: ServerRef[]
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
    mutationFn: ({ onStart, ...network }: NewNetwork & Followed) => operate<Network>("/networks", { body: network }, onStart),
    // A failed apply still saves the network, so the list is refreshed either way.
    onSettled: () => queryClient.invalidateQueries({ queryKey: networksQuery.queryKey }),
  })
}

/** Saves the settings of a network; the master configures its servers. */
export function useUpdateNetwork(id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ settings, onStart }: { settings: NetworkSettings } & Followed) =>
      operate<Network>(`/networks/${id}`, { method: "PUT", body: settings }, onStart),
    onSettled: () => queryClient.invalidateQueries({ queryKey: networksQuery.queryKey }),
  })
}

export type NetworkAction = { action: "apply" | "delete" | "start" | "stop" | "restart" } | { action: "broadcast"; message: string }

/** Acts on a network or on all its servers. */
export function useNetworkAction(id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (a: NetworkAction & Followed) => {
      const path = `/networks/${id}`
      switch (a.action) {
        case "delete":
          return operate(path, { method: "DELETE" }, a.onStart)
        case "broadcast":
          return api(`${path}/broadcast`, { body: { message: a.message } })
        default:
          return operate(`${path}/${a.action}`, { method: "POST" }, a.onStart)
      }
    },
    // A deleted network is only refreshed in the list, as its page is left.
    onSettled: (_data, _error, a) => {
      void queryClient.invalidateQueries({ queryKey: networksQuery.queryKey, exact: a.action === "delete" })
      if (a.action !== "apply" && a.action !== "delete") void queryClient.invalidateQueries({ queryKey: ["servers"] })
    },
  })
}

/** A value of a proxy's configuration: text, a number, a switch or a list of texts. */
export type SettingValue = string | number | boolean | string[]

/** The settings of a proxy in its own configuration file, by their path in it. */
export interface ProxySettings {
  /** False until the proxy started once and created its configuration. */
  exists: boolean
  /** e.g. velocity.toml */
  file: string
  settings: Record<string, SettingValue>
  /** Settings the panel or the network decides, with the reason. */
  locked: { key: string; reason: string }[]
}

const proxyPath = ({ nodeId, serverId }: ServerRef) => `/nodes/${nodeId}/servers/${serverId}/proxy`

export const proxySettingsQuery = (proxy: ServerRef) =>
  queryOptions({
    queryKey: ["proxy-settings", proxy.nodeId, proxy.serverId],
    queryFn: () => api<ProxySettings>(proxyPath(proxy)),
  })

/** Changes settings of a proxy, which reloads its configuration if it runs. */
export function useUpdateProxySettings(proxy: ServerRef) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (settings: Record<string, SettingValue>) =>
      api<{ reloaded: boolean }>(proxyPath(proxy), { method: "PUT", body: { settings } }),
    onSettled: () => queryClient.invalidateQueries({ queryKey: proxySettingsQuery(proxy).queryKey }),
  })
}
