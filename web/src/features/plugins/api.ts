import { infiniteQueryOptions, keepPreviousData, queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import type { ServerRef } from "@/features/networks/api"
import { serverType } from "@/features/servers/server-types"
import { ApiError, api } from "@/lib/api"

/** A plugin or mod on Modrinth. */
export interface Project {
  id: string
  slug: string
  title: string
  /** Served by the panel, which fetches it from Modrinth. */
  icon?: string
}

export interface SearchHit extends Project {
  description: string
  author: string
  downloads: number
  /** Loaders of manageable servers the project supports, e.g. "paper" or "fabric". */
  loaders: string[]
}

/** A plugin file on a server; files from Modrinth are recognised by their hash. */
export interface InstalledPlugin {
  fileName: string
  size: number
  project?: Project
  version?: string
  /** A newer release that suits the server. */
  update?: string
}

export interface PluginListing {
  folder: "plugins" | "mods"
  plugins: InstalledPlugin[]
  /** Why the plugins couldn't be looked up on Modrinth. */
  catalogueError?: string
}

export interface InstallResult extends ServerRef {
  installed: { projectId: string; fileName: string; version: string }[]
  error?: string
}

/** Whether a project runs on a server type. */
export function supports(loaders: string[], type: string) {
  return serverType(type).addons?.loaders.some((l) => loaders.includes(l)) ?? false
}

/** Searches Modrinth for plugins and mods of a server type and Minecraft version, both optional. */
export const searchQuery = (params: { query: string; type?: string; version?: string }) =>
  infiniteQueryOptions({
    queryKey: ["plugins", "search", params],
    queryFn: ({ pageParam }) =>
      api<{ hits: SearchHit[]; total: number }>(
        `/plugins/search?${new URLSearchParams({ ...params, type: params.type ?? "", version: params.version ?? "", offset: String(pageParam) })}`,
      ),
    initialPageParam: 0,
    getNextPageParam: (last, pages) => {
      const loaded = pages.reduce((n, p) => n + p.hits.length, 0)
      return loaded < last.total && last.hits.length > 0 ? loaded : undefined
    },
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  })

const base = ({ nodeId, serverId }: ServerRef) => `/nodes/${nodeId}/servers/${serverId}/plugins`

export const pluginsQuery = (ref: ServerRef) =>
  queryOptions({
    queryKey: ["plugins", ref.nodeId, ref.serverId],
    queryFn: () => api<PluginListing>(base(ref)),
  })

/** Installs or updates projects, with the projects they require, on servers. */
export const installPlugins = (projects: string[], servers: ServerRef[]) =>
  api<{ results: InstallResult[] }>("/plugins/install", { body: { projects, servers } }).then((res) => res.results)

export function useInstallPlugins() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ projects, servers }: { projects: string[]; servers: ServerRef[] }) => installPlugins(projects, servers),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["plugins"] }),
  })
}

export type PluginChange = { action: "remove"; fileName: string } | { action: "upload"; file: File }

export function useChangePlugins(ref: ServerRef) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (change: PluginChange) => {
      if (change.action === "remove") return api(`${base(ref)}/${encodeURIComponent(change.fileName)}`, { method: "DELETE" })
      const res = await fetch(`/api${base(ref)}/${encodeURIComponent(change.file.name)}`, { method: "PUT", body: change.file })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new ApiError(res.status, data.error ?? `The upload failed with status ${res.status}.`)
      }
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: pluginsQuery(ref).queryKey }),
  })
}
