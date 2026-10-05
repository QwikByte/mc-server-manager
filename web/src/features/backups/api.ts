import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { t } from "i18next"
import { defaultSchedule, type TaskInput, taskApi } from "@/features/schedules/api"
import { type Operation, operate } from "@/features/operations/api"
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
  /** The job that made the backup; empty for backups made by hand. */
  jobId?: string
}

export interface JobSettings {
  selection: Selection
  /** Storage location on each node; empty means the default one. */
  location: string
  /** Backups of the job kept per server; 0 keeps all. */
  keep: number
}

export const jobs = taskApi<JobSettings>("/backup-jobs")

export const defaultSelection: Selection = { everything: false, worlds: true, plugins: true, config: true, paths: [] }

export const emptyJob: TaskInput<JobSettings> = {
  name: "",
  enabled: true,
  schedule: defaultSchedule,
  targets: [],
  settings: { selection: defaultSelection, location: "", keep: 7 },
}

/** Why the master refuses the further paths of a selection, if it does. */
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

/** Describes a selection, e.g. "Worlds, plugins and configuration". */
export function describeSelection(s: Selection): string {
  if (s.everything) return t("Everything")
  const parts = [s.worlds && t("worlds"), s.plugins && t("plugins or mods"), s.config && t("configuration"), ...s.paths].filter(
    (p): p is string => !!p,
  )
  const text = and(parts)
  return text.charAt(0).toUpperCase() + text.slice(1)
}

/** Describes what a backup contains, e.g. "world, plugins and server.properties". */
export const describeContent = (paths: string[]) => (paths.includes(".") ? t("Everything") : and(paths))

const base = (nodeId: string, serverId: string) => `/nodes/${nodeId}/servers/${serverId}/backups`

export const backupsQuery = (nodeId: string, serverId: string) =>
  queryOptions({
    queryKey: ["nodes", nodeId, "servers", serverId, "backups"],
    queryFn: () => api<Backup[]>(base(nodeId, serverId)),
  })

export const downloadUrl = (nodeId: string, serverId: string, id: string) => `/api${base(nodeId, serverId)}/${id}/download`

/** Backs up a server, restores and deletes its backups. */
export function useBackups(nodeId: string, serverId: string) {
  const queryClient = useQueryClient()
  const onSettled = () => queryClient.invalidateQueries({ queryKey: backupsQuery(nodeId, serverId).queryKey })
  return {
    create: useMutation({
      mutationFn: ({ onStart, ...input }: { label: string; selection: Selection; location: string; onStart?: (op: Operation) => void }) =>
        operate<Backup>(base(nodeId, serverId), { body: input }, onStart),
      onSettled,
    }),
    restore: useMutation({
      mutationFn: ({ id, onStart }: { id: string; onStart?: (op: Operation) => void }) =>
        operate(`${base(nodeId, serverId)}/${id}/restore`, { method: "POST" }, onStart),
      onSettled: () => queryClient.invalidateQueries({ queryKey: ["nodes", nodeId, "servers"] }),
    }),
    remove: useMutation({ mutationFn: (id: string) => api(`${base(nodeId, serverId)}/${id}`, { method: "DELETE" }), onSettled }),
  }
}
