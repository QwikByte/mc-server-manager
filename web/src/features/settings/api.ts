import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import type { NodeLimits } from "@/features/nodes/api"
import { api } from "@/lib/api"

/** Settings of the master; they apply right away, the panel's address when the master starts again. */
export interface MasterSettings {
  /** IP:port the panel listens at from the next start; empty means the address from the command line. */
  panelAddr: string
  /** host:port join tokens tell agents to enroll at; empty means the address from the command line. */
  enrollAddr: string
  sessionHours: number
  joinTokenMinutes: number
  /** Limits new nodes get. */
  nodeDefaults: NodeLimits
  /** How long log entries are kept. */
  logDays: number
  /** Whether the master looks for new releases, which administrators can install. */
  checkUpdates: boolean
}

/** The running master; apart from the certificate, it only changes with a restart. */
export interface Master {
  version: string
  startedAt: string
  /** Where the panel listens. */
  panelAddr: string
  /** Whether the panel serves HTTPS itself instead of behind a reverse proxy. */
  panelTls: boolean
  /** The panel's address from the command line, used while the settings name none. */
  panelDefaultAddr: string
  /** Why the panel didn't listen at the address from the settings when the master started. */
  panelAddrError?: string
  enrollListenAddr: string
  /** The enrollment address from the command line. */
  enrollAddr: string
  caFingerprint: string
  certificateExpiresAt: string
  /** Whether administrators can restart the master from the panel. */
  restartable: boolean
}

export interface SettingsView {
  settings: MasterSettings
  master: Master
}

export const settingsQuery = queryOptions({
  queryKey: ["settings"],
  queryFn: () => api<SettingsView>("/settings"),
})

export function useUpdateSettings() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (settings: MasterSettings) => api<SettingsView>("/settings", { method: "PUT", body: settings }),
    onSuccess: (view) => queryClient.setQueryData(settingsQuery.queryKey, view),
  })
}

/** Key of the restart, so that every restart button tells while one runs. */
export const restartKey = ["restart-master"]

/**
 * Restarts the master and waits until it answers again, which takes a few seconds. next is
 * where the panel listens then, for the error if it doesn't come back at this address.
 */
export function useRestartMaster() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationKey: restartKey,
    mutationFn: async ({ master, next }: { master: Master; next: string }) => {
      await api("/master/restart", { method: "POST" })
      for (let attempt = 0; attempt < 30; attempt++) {
        await new Promise((resolve) => setTimeout(resolve, 2_000))
        const view = await api<SettingsView>("/settings").catch(() => undefined)
        if (view && view.master.startedAt !== master.startedAt) return view
      }
      throw new Error(
        next === master.panelAddr
          ? "The master hasn't answered for a minute. See: journalctl -u mcsm-master"
          : `The master hasn't answered here for a minute. It listens at ${next} now: open the panel there, or point your reverse proxy to it.`,
      )
    },
    onSuccess: async (view) => {
      queryClient.setQueryData(settingsQuery.queryKey, view)
      await queryClient.invalidateQueries()
    },
  })
}
