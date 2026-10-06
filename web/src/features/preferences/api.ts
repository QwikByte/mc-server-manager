import { queryOptions, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { api } from "@/lib/api"

/** A widget of the overview and how many of its three columns it spans on large screens. */
export interface Widget {
  id: string
  columns: 1 | 2 | 3
  hidden?: boolean
}

export interface ServerRef {
  nodeId: string
  serverId: string
}

/** What the signed-in user set up in the panel; the master keeps it, so it applies in all their browsers. */
export interface Preferences {
  /** The order of the widgets of the overview; empty shows the default layout. */
  dashboard: Widget[]
  pinned: ServerRef[]
}

export const preferencesQuery = queryOptions({
  queryKey: ["preferences"],
  queryFn: () => api<Preferences>("/preferences"),
  // Loaded again now and then, e.g. when the window gets the focus, for changes in other tabs.
  staleTime: 30_000,
})

/**
 * Changes a part of the preferences right away and stores it. Changes are sent one after the
 * other, so that the last one wins, and a failed one is undone.
 */
function useChange<K extends keyof Preferences>(part: K, field: string) {
  const queryClient = useQueryClient()
  const key = preferencesQuery.queryKey
  return useMutation({
    scope: { id: `preferences/${part}` },
    mutationFn: (value: Preferences[K]) => api<Preferences>(`/preferences/${part}`, { method: "PUT", body: { [field]: value } }),
    onMutate: async (value) => {
      await queryClient.cancelQueries({ queryKey: key })
      const before = queryClient.getQueryData(key)
      if (before) queryClient.setQueryData(key, { ...before, [part]: value })
      return { before }
    },
    onError: (error, _, context) => {
      if (context?.before) queryClient.setQueryData(key, context.before)
      toast.error(error.message)
    },
  })
}

export const useSetDashboard = () => useChange("dashboard", "widgets")

/**
 * The servers the user pinned, and a toggle that pins or unpins one. Servers are found by
 * their ID, which stays the same when they move to another node.
 */
export function usePinned() {
  const { data } = useQuery(preferencesQuery)
  const set = useChange("pinned", "servers")
  const pinned = data?.pinned ?? []
  const isPinned = (serverId: string) => pinned.some((p) => p.serverId === serverId)
  return {
    pinned,
    isPinned,
    toggle: (s: ServerRef) => set.mutate(isPinned(s.serverId) ? pinned.filter((p) => p.serverId !== s.serverId) : [...pinned, s]),
  }
}
