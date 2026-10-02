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
