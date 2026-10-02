import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import type { Target } from "@/features/servers/api"
import { api } from "@/lib/api"

/** When a task runs: at the times of day on the weekdays, in a time zone. */
export interface Schedule {
  /** Weekdays, 0 for Sunday; empty means every day. */
  days: number[]
  /** Times of day as HH:MM. */
  times: string[]
  timeZone: string
}

/** A task that runs on servers on a schedule: a backup job or a policy. */
export interface Task<S> {
  id: string
  name: string
  enabled: boolean
  schedule: Schedule
  targets: Target[]
  settings: S
  lastRun?: { at: string; error?: string }
  /** The scheduled time of the next run of an enabled task. */
  nextRun?: string
  running: boolean
  createdAt: string
}

export type TaskInput<S> = Pick<Task<S>, "name" | "enabled" | "schedule" | "targets" | "settings">

/** The time zone of the browser, which new schedules start with. */
export const localTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone

export const defaultSchedule: Schedule = { days: [], times: ["04:00"], timeZone: localTimeZone }

/** Queries and changes the tasks of one kind, e.g. the backup jobs under /backup-jobs. */
export function taskApi<S>(path: string) {
  const tasksQuery = queryOptions({
    queryKey: [path],
    queryFn: () => api<Task<S>[]>(path),
    // Runs end in the background, so a running task is watched more closely.
    refetchInterval: (query) => (query.state.data?.some((t) => t.running) ? 2_000 : 30_000),
  })
  const taskQuery = (id: string) =>
    queryOptions({
      queryKey: [path, id],
      queryFn: () => api<Task<S>>(`${path}/${id}`),
    })

  /** Creates a task, or changes it if an ID is given. */
  function useSaveTask(id?: string) {
    const queryClient = useQueryClient()
    return useMutation({
      mutationFn: (input: TaskInput<S>) =>
        id ? api<Task<S>>(`${path}/${id}`, { method: "PUT", body: input }) : api<Task<S>>(path, { body: input }),
      onSuccess: (task) => {
        queryClient.setQueryData(taskQuery(task.id).queryKey, task)
        return queryClient.invalidateQueries({ queryKey: tasksQuery.queryKey, exact: true })
      },
    })
  }

  function useDeleteTask() {
    const queryClient = useQueryClient()
    return useMutation({
      mutationFn: (id: string) => api(`${path}/${id}`, { method: "DELETE" }),
      onSuccess: () => queryClient.invalidateQueries({ queryKey: tasksQuery.queryKey }),
    })
  }

  /** Starts a run now; it continues in the background. */
  function useRunTask() {
    const queryClient = useQueryClient()
    return useMutation({
      mutationFn: (id: string) => api<Task<S>>(`${path}/${id}/run`, { method: "POST" }),
      onSuccess: () => queryClient.invalidateQueries({ queryKey: tasksQuery.queryKey }),
    })
  }

  return { tasksQuery, taskQuery, useSaveTask, useDeleteTask, useRunTask }
}

export type TaskApi<S> = ReturnType<typeof taskApi<S>>

/** Whether a task runs on a server. */
export const covers = (targets: Target[], nodeId: string, serverId: string) =>
  targets.some((t) => t.nodeId === nodeId && (t.serverId === "" || t.serverId === serverId))
