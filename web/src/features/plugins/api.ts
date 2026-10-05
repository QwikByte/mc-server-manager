import { infiniteQueryOptions, keepPreviousData, queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { t } from "i18next"
import type { ServerRef } from "@/features/networks/api"
import { serverType } from "@/features/servers/server-types"
import { type Operation, operate } from "@/features/operations/api"
import { api, responseError } from "@/lib/api"

/** A plugin or mod on Modrinth, or a plugin on Hangar. */
export interface Project {
  id: string
  slug: string
  title: string
  /** Served by the panel, which fetches it from Modrinth or Hangar. */
  icon?: string
}

export interface SearchHit extends Project {
  description: string
  author: string
  downloads: number
  follows: number
  updated: string
  /** Whether players need the project too. */
  clientSide: "required" | "optional" | "unsupported" | "unknown"
  /** Loaders of manageable servers the project supports, e.g. "paper" or "fabric". */
  loaders: string[]
  /** The main categories of the project, e.g. "economy". */
  categories: string[]
}

export type Sort = "relevance" | "downloads" | "follows" | "newest" | "updated"

/** What servers load from Modrinth. */
export type Kind = "plugins" | "mods"

/** Where plugins come from: Modrinth has plugins and mods, Hangar plugins of Paper, Velocity and Waterfall. */
export type Source = "modrinth" | "hangar"

/** The page of a project on Modrinth or Hangar, whose project IDs start with "hangar-". */
export const projectUrl = (project: Project) =>
  project.id.startsWith("hangar-") ? `https://hangar.papermc.io/${project.slug}` : `https://modrinth.com/project/${project.slug}`

/** Whether Hangar has plugins for a server type: Paper and its forks except Folia, and the proxies. */
export const onHangar = (type: string) =>
  serverType(type).addons?.loaders.some((l) => ["paper", "velocity", "waterfall"].includes(l)) ?? false

/** A search on Modrinth or Hangar; without a kind, type or version, projects for any of them are found. */
export interface Search {
  source?: Source
  query: string
  /** Modpacks are searched to create servers from them. */
  kind?: Kind | "modpacks"
  type?: string
  version?: string
  /** Categories the projects must all be in. */
  categories: string[]
  sort: Sort
  /** Only projects that players don't have to install. */
  serverOnly: boolean
}

/** A version of a project. */
export interface ProjectVersion {
  id: string
  number: string
  channel: "release" | "beta" | "alpha"
  published: string
  /** The versions of Minecraft it supports. */
  gameVersions?: string[]
}

/** A plugin file on a server; files from Modrinth and Hangar are recognised by their hash. */
export interface InstalledPlugin {
  fileName: string
  size: number
  project?: Project
  version?: string
  versionId?: string
  /** A newer release that suits the server. */
  update?: string
}

export interface PluginListing {
  folder: "plugins" | "mods"
  plugins: InstalledPlugin[]
  /** Why the plugins couldn't be looked up on Modrinth or Hangar. */
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

/** Searches Modrinth for plugins and mods, or Hangar for plugins. */
export const searchQuery = (search: Search) =>
  infiniteQueryOptions({
    queryKey: ["plugins", "search", search],
    queryFn: ({ pageParam }) => {
      const { source = "modrinth", query, kind = "", type = "", version = "", categories, sort, serverOnly } = search
      const params = new URLSearchParams({ source, query, kind, type, version, sort, offset: String(pageParam) })
      for (const category of categories) params.append("category", category)
      if (serverOnly) params.set("serverOnly", "true")
      return api<{ hits: SearchHit[]; total: number }>(`/plugins/search?${params}`)
    },
    initialPageParam: 0,
    getNextPageParam: (last, pages) => {
      const loaded = pages.reduce((n, p) => n + p.hits.length, 0)
      return loaded < last.total && last.hits.length > 0 ? loaded : undefined
    },
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  })

/** The releases of Minecraft, the newest first. */
export const gameVersionsQuery = queryOptions({
  queryKey: ["plugins", "game-versions"],
  queryFn: () => api<string[]>("/plugins/game-versions"),
  staleTime: 3_600_000,
})

/** The versions of a project that run on a server type and Minecraft version, the newest first. */
export const versionsQuery = (project: string, type: string, version: string) =>
  queryOptions({
    queryKey: ["plugins", "versions", project, type, version],
    queryFn: () => api<ProjectVersion[]>(`/plugins/projects/${project}/versions?${new URLSearchParams({ type, version })}`),
    staleTime: 60_000,
  })

const base = ({ nodeId, serverId }: ServerRef) => `/nodes/${nodeId}/servers/${serverId}/plugins`

export const pluginsQuery = (ref: ServerRef) =>
  queryOptions({
    queryKey: ["plugins", ref.nodeId, ref.serverId],
    queryFn: () => api<PluginListing>(base(ref)),
  })

/**
 * Installs or updates projects, with the projects they require, on servers: the newest release that suits each
 * server, or the version chosen for a project.
 */
/** Installs projects of Modrinth and Hangar on servers; on many, it takes a while. */
export function useInstallPlugins() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({
      projects,
      servers,
      versions,
      onStart,
    }: {
      projects: string[]
      servers: ServerRef[]
      versions?: Record<string, string>
      onStart?: (op: Operation) => void
    }) => operate<{ results: InstallResult[] }>("/plugins/install", { body: { projects, servers, versions } }, onStart).then((res) => res.results),
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
      if (!res.ok) throw await responseError(res, t("The upload failed with status {{status}}.", { status: res.status }))
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: pluginsQuery(ref).queryKey }),
  })
}
