import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"

/** When a task runs: at the times of day on weekdays, on days of the month or on single dates, in a time zone. */
export interface Schedule {
  /** Weekdays, 0 for Sunday; empty means every day. */
  days: number[]
  /** Days of the month from 1 to 31, instead of weekdays; a month without one runs on its last day. */
  monthDays?: number[]
  /** Single days as YYYY-MM-DD, instead of weekdays; after the last one, the task turns itself off. */
  dates?: string[]
  /** Times of day as HH:MM. */
  times: string[]
  timeZone: string
}

/** Which servers of a network a target names; empty names all of them. */
export type NetworkRole = "" | "servers" | "proxy"

/**
 * Servers a task runs on: a server, all servers of a node (without serverId), those with a tag, or those of a
 * network. Nodes, tags and networks include the servers they get later.
 */
export type TaskTarget =
  | { kind: "server"; nodeId: string; serverId?: string }
  | { kind: "tag"; value: string }
  | { kind: "network"; value: string; role?: NetworkRole }

/** A run of a task. */
export interface Run {
  id: number
  startedAt: string
  endedAt: string
  /** The user who started it by hand; none for its schedule. */
  startedBy?: string
  outcome: "succeeded" | "failed"
  error?: string
  /** What the run left out, e.g. servers without data to back up. */
  note?: string
}

/** A scheduled run of a task. */
export interface Upcoming {
  id: string
  at: string
}

/** A task that runs on servers on a schedule: a backup job or a policy. */
export interface Task<S> {
  id: string
  name: string
  enabled: boolean
  schedule: Schedule
  targets: TaskTarget[]
  settings: S
  lastRun?: Run
  /** The scheduled time of the next run of an enabled task. */
  nextRun?: string
  running: boolean
  createdAt: string
}

export type TaskInput<S> = Pick<Task<S>, "name" | "enabled" | "schedule" | "targets" | "settings">

/** The time zone of the browser, which new schedules start with. */
const localTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone

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
  /** The tasks whose targets include a server now, also through its tags and network. */
  const coveringQuery = (nodeId: string, serverId: string) =>
    queryOptions({
      queryKey: [path, "covering", nodeId, serverId],
      queryFn: () => api<Task<S>[]>(`${path}?${new URLSearchParams({ node: nodeId, server: serverId })}`),
    })
  /** The kept runs of a task, newest first. */
  const runsQuery = (id: string) =>
    queryOptions({
      queryKey: [path, id, "runs"],
      queryFn: () => api<Run[]>(`${path}/${id}/runs`),
      refetchInterval: 30_000,
    })
  /** The runs of the enabled tasks in the next 7 days, in order. */
  const upcomingQuery = queryOptions({
    queryKey: [path, "upcoming"],
    queryFn: () => api<Upcoming[]>(`${path}/upcoming`),
    refetchInterval: 60_000,
  })

  /** Creates a task, or changes it if an ID is given. */
  function useSaveTask(id?: string) {
    const queryClient = useQueryClient()
    return useMutation({
      mutationFn: (input: TaskInput<S>) =>
        id ? api<Task<S>>(`${path}/${id}`, { method: "PUT", body: input }) : api<Task<S>>(path, { body: input }),
      onSuccess: (task) => {
        queryClient.setQueryData(taskQuery(task.id).queryKey, task)
        return queryClient.invalidateQueries({ queryKey: [path], predicate: ({ queryKey }) => queryKey[1] !== task.id })
      },
    })
  }

  function useDeleteTask() {
    const queryClient = useQueryClient()
    return useMutation({
      mutationFn: (id: string) => api(`${path}/${id}`, { method: "DELETE" }),
      onSuccess: () => queryClient.invalidateQueries({ queryKey: [path] }),
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

  return { tasksQuery, taskQuery, coveringQuery, runsQuery, upcomingQuery, useSaveTask, useDeleteTask, useRunTask }
}

export type TaskApi<S> = ReturnType<typeof taskApi<S>>
