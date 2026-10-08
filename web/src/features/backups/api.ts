import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { t } from "i18next"
import type { Listing, ServerFiles } from "@/features/files/api"
import { type Operation, operate } from "@/features/operations/api"
import { defaultSchedule, type TaskInput, taskApi } from "@/features/schedules/api"
import { api } from "@/lib/api"
import { locale } from "@/lib/i18n"

/** What of a server is backed up; the agent finds the matching files when it backs up. */
export interface Selection {
  /** The whole folder of the server; the other choices don't matter then. */
  everything: boolean
  /** Folders with a level.dat, e.g. world and world_nether. */
  worlds: boolean
  /** The plugins or mods folder; for mods also config/. */
  plugins: boolean
  /** Files in the server's folder except .jar files and logs, and the config folder. */
  config: boolean
  /** Further files or folders, relative to the server's folder. */
  paths: string[]
  /** Files and folders inside the others that are left out, e.g. the tiles of a map plugin; jobs from before have none. */
  exclude?: string[]
}

export interface Backup {
  id: string
  label: string
  createdAt: string
  size: number
  /** Storage location on the node. */
  location: string
  /** Files and folders in the backup; "." is everything. */
  paths: string[]
  /** Files and folders inside the paths that the backup left out. */
  exclude: string[]
  /** The job that made the backup; empty for backups made by hand. */
  jobId?: string
  /** Kept backups are never deleted by their job. */
  kept: boolean
}

export interface JobSettings {
  selection: Selection
  /** Datastores backed up with all their databases; jobs from before datastores have none. */
  datastores?: string[]
  /** Storage location on each node; empty means the default one. */
  location: string
  /** The newest backups of the job kept per server; 0, like the others, keeps all. */
  keep: number
  /** The newest backup of each of the last days, weeks and months with backups is kept too; jobs from before have none. */
  keepDays?: number
  keepWeeks?: number
  keepMonths?: number
}

export const jobs = taskApi<JobSettings>("/backup-jobs")

export const defaultSelection: Selection = { everything: false, worlds: true, plugins: true, config: true, paths: [], exclude: [] }

export const emptyJob: TaskInput<JobSettings> = {
  name: "",
  enabled: true,
  schedule: defaultSchedule,
  targets: [],
  settings: { selection: defaultSelection, datastores: [], location: "", keep: 0, keepDays: 7, keepWeeks: 4, keepMonths: 0 },
}

/** Why the master refuses paths of a selection or a restore, if it does. */
export function pathsError(paths: string[]): string | undefined {
  if (paths.length > 20) return t("Enter at most 20 files or folders.")
  const invalid = paths.find((p) => p.length > 1024 || /[\0\\]/.test(p) || p.split("/").includes(".."))
  if (invalid !== undefined) {
    return t("The path {{path}} is invalid. Enter paths inside the server's folder, like plugins/LuckPerms.", { path: invalid })
  }
}

/** Whether a selection backs up nothing. */
export const nothingSelected = (s: Selection) => !s.everything && !s.worlds && !s.plugins && !s.config && s.paths.length === 0

const and = (parts: string[]) => new Intl.ListFormat(locale, { type: "conjunction" }).format(parts)

/** Adds what is left out to a description of what is backed up. */
const without = (text: string, exclude: string[] = []) =>
  exclude.length > 0 ? t("{{content}} without {{paths}}", { content: text, paths: and(exclude) }) : text

/** Describes a selection, e.g. "Worlds, plugins and configuration". */
export function describeSelection(s: Selection): string {
  if (s.everything) return without(t("Everything"), s.exclude)
  const parts = [s.worlds && t("worlds"), s.plugins && t("plugins or mods"), s.config && t("configuration"), ...s.paths].filter(
    (p): p is string => !!p,
  )
  const text = and(parts)
  return without(text.charAt(0).toUpperCase() + text.slice(1), s.exclude)
}

/** Describes what a backup contains, e.g. "world, plugins and server.properties". */
export const describeContent = (b: Pick<Backup, "paths" | "exclude">) =>
  without(b.paths.includes(".") ? t("Everything") : and(b.paths), b.exclude)

/** Describes which backups a job keeps, e.g. "keeps 7 daily and 4 weekly". */
export function describeRetention(s: JobSettings): string {
  const parts = [
    s.keep > 0 && t("{{count}} newest", { count: s.keep }),
    !!s.keepDays && t("{{count}} daily", { count: s.keepDays }),
    !!s.keepWeeks && t("{{count}} weekly", { count: s.keepWeeks }),
    !!s.keepMonths && t("{{count}} monthly", { count: s.keepMonths }),
  ].filter((p): p is string => !!p)
  return parts.length > 0 ? t("keeps {{what}}", { what: and(parts) }) : t("keeps all")
}

const base = (nodeId: string, serverId: string) => `/nodes/${nodeId}/servers/${serverId}/backups`

export const backupsQuery = (nodeId: string, serverId: string) =>
  queryOptions({
    queryKey: ["nodes", nodeId, "servers", serverId, "backups"],
    queryFn: () => api<Backup[]>(base(nodeId, serverId)),
    refetchInterval: 30_000, // e.g. backup jobs add some
  })

/** A folder of a backup, listed like one of the server. */
export const backupFilesQuery = (s: ServerFiles, id: string, path: string) =>
  queryOptions({
    queryKey: ["nodes", s.nodeId, "servers", s.serverId, "backups", id, "files", path],
    queryFn: () => api<Listing>(`${base(s.nodeId, s.serverId)}/${id}/files?path=${encodeURIComponent(path)}`),
    staleTime: Infinity, // a backup doesn't change
  })

export const downloadUrl = (nodeId: string, serverId: string, id: string) => `/api${base(nodeId, serverId)}/${id}/download`

/** What a restore restores, and where. */
export interface Restore {
  id: string
  /** Files and folders of the backup; none restores all of it. */
  paths: string[]
  /** Backs up what the restore replaces first. */
  snapshotFirst: boolean
  /** Another server to restore the backup into, also on another node. */
  into?: ServerFiles
  onStart?: (op: Operation) => void
}

/** How a restore went. */
export interface Restored {
  /** The backup of what the restore replaced, if one was made. */
  snapshot?: Backup
  /** Tells that the network of the server couldn't be configured again. */
  warning?: string
}

/** Backs up a server, changes, restores and deletes its backups. */
export function useBackups(nodeId: string, serverId: string) {
  const queryClient = useQueryClient()
  const onSettled = () => queryClient.invalidateQueries({ queryKey: backupsQuery(nodeId, serverId).queryKey })
  return {
    create: useMutation({
      mutationFn: ({ onStart, ...input }: { label: string; selection: Selection; location: string; onStart?: (op: Operation) => void }) =>
        operate<Backup>(base(nodeId, serverId), { body: input }, onStart),
      onSettled,
    }),
    update: useMutation({
      mutationFn: ({ id, ...change }: { id: string; label?: string; kept?: boolean }) =>
        api<Backup>(`${base(nodeId, serverId)}/${id}`, { method: "PATCH", body: change }),
      onSettled,
    }),
    restore: useMutation({
      mutationFn: ({ id, into, onStart, ...what }: Restore) =>
        into
          ? operate<Restored>(`${base(nodeId, serverId)}/${id}/restore-into`, { body: { node: into.nodeId, server: into.serverId, ...what } }, onStart)
          : operate<Restored>(`${base(nodeId, serverId)}/${id}/restore`, { body: what }, onStart),
      // Also the backups of the server restored into, which got the backup of what was replaced.
      onSettled: (_, __, { into }) =>
        Promise.all([nodeId, into?.nodeId].map((node) => node && queryClient.invalidateQueries({ queryKey: ["nodes", node, "servers"] }))),
    }),
    remove: useMutation({ mutationFn: (id: string) => api(`${base(nodeId, serverId)}/${id}`, { method: "DELETE" }), onSettled }),
  }
}
