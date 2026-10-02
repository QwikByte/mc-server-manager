import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import type { NodeLimits } from "@/features/nodes/api"
import { api } from "@/lib/api"

/** Settings of the master; they apply right away. */
export interface MasterSettings {
  /** host:port join tokens tell agents to enroll at; empty means the address from the command line. */
  enrollAddr: string
  sessionHours: number
  joinTokenMinutes: number
  /** Limits new nodes get. */
  nodeDefaults: NodeLimits
  /** How long log entries are kept. */
  logDays: number
}

/** The running master; apart from the certificate, it only changes with a restart. */
export interface Master {
  version: string
  startedAt: string
  panelAddr: string
  /** Whether the panel serves HTTPS itself instead of behind a reverse proxy. */
  panelTls: boolean
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
