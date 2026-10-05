import { queryOptions } from "@tanstack/react-query"
import { api } from "@/lib/api"

/** A version of a Modrinth modpack, which decides the Minecraft version and the mod loader of its servers. */
export interface ModpackVersion {
  id: string
  number: string
  channel: "release" | "beta" | "alpha"
  published: string
  gameVersions: string[]
  loaders: string[]
}

/** The version of a modpack a new server gets. */
export interface ModpackChoice {
  project: string
  version: string
}

/** The versions of a modpack for the mod loaders of servers, the newest first. */
export const modpackVersionsQuery = (project: string) =>
  queryOptions({
    queryKey: ["modpacks", project, "versions"],
    queryFn: () => api<ModpackVersion[]>(`/modpacks/${project}/versions`),
    staleTime: 60_000,
  })
