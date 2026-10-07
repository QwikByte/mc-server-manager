import { queryOptions, useIsMutating, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect } from "react"
import { toast } from "sonner"
import type { Grouping, Sort, View } from "@/features/servers/browse"
import { api } from "@/lib/api"
import { type Clock, chooseClock } from "@/lib/i18n"
import type { Order } from "@/lib/sort"
import { setTheme, type Theme } from "@/lib/theme"

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

/**
 * The user's other choices. The master takes the same keys and values (internal/master/preference);
 * those the user never made follow the browser.
 */
export interface Settings {
  theme?: Theme
  clock?: Clock
  /** How lists of servers are shown where their address doesn't say. */
  serverView?: View
  serverSort?: Sort
  serverOrder?: Order
  serverGroup?: Grouping
}

/** What the signed-in user set up in the panel; the master keeps it, so it applies in all their browsers. */
export interface Preferences {
  /** The order of the widgets of the overview; empty shows the default layout. */
  dashboard: Widget[]
  pinned: ServerRef[]
  settings: Settings
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

const settingsKey = ["preferences", "settings"]

/**
 * The user's settings, and a change of some of them. It shows right away, and changes are
 * stored one after the other with only their own keys, so that changes in other browsers stay.
 */
export function useSettings() {
  const queryClient = useQueryClient()
  const key = preferencesQuery.queryKey
  const { data } = useQuery(preferencesQuery)
  const { mutate } = useMutation({
    mutationKey: settingsKey,
    scope: { id: "preferences/settings" },
    mutationFn: (change: Settings) => api<Preferences>("/preferences/settings", { method: "PATCH", body: change }),
    onMutate: async (change) => {
      await queryClient.cancelQueries({ queryKey: key })
      queryClient.setQueryData(key, (p) => p && { ...p, settings: { ...p.settings, ...change } })
    },
    onError: (error) => {
      toast.error(error.message)
      void queryClient.invalidateQueries({ queryKey: key })
    },
  })
  return {
    settings: data?.settings ?? {},
    change: (change: Settings) => {
      if (change.theme) setTheme(change.theme)
      mutate(change)
    },
  }
}

/** Applies the signed-in user's settings in this browser, which keeps them for the next visit. */
export function useApplySettings() {
  const { data } = useQuery(preferencesQuery)
  // Another clock reloads the panel, so it waits until the changes are stored.
  const storing = useIsMutating({ mutationKey: settingsKey }) > 0
  const { theme, clock } = data?.settings ?? {}
  useEffect(() => {
    if (theme) setTheme(theme)
  }, [theme])
  useEffect(() => {
    if (clock && !storing) chooseClock(clock)
  }, [clock, storing])
}
