import { queryOptions, useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"

/** What the panel shows before anyone signs in, which the settings say is public. */
export interface PublicSettings {
  /** The name of the panel: Noryx unless the settings give another one. */
  name: string
  /** Plain text for the sign-in page, e.g. whom to ask for access; empty for none. */
  notice: string
}

export const publicSettingsQuery = queryOptions({
  queryKey: ["settings", "public"],
  queryFn: () => api<PublicSettings>("/panel"),
})

/** The name of the panel, Noryx until it is loaded. */
export const usePanelName = () => useQuery(publicSettingsQuery).data?.name ?? "Noryx"
