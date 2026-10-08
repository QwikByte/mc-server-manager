import { t } from "i18next"
import { useState } from "react"
import type { ServerRef } from "@/features/networks/api"
import { key } from "@/features/networks/servers"
import type { PlayerAction } from "./api"

/** What the selection bar of each tab of players offers, and what the servers of the selected players are. */
export const bulkActions: Record<
  "online" | "seen" | "banned" | "whitelisted" | "operators",
  { actions: (PlayerAction | "message")[]; where: () => string }
> = {
  online: { actions: ["message", "kick", "ban", "whitelist_add"], where: () => t("Where they are online") },
  seen: { actions: ["ban", "whitelist_add"], where: () => t("Where they played") },
  banned: { actions: ["pardon"], where: () => t("Where they are banned") },
  whitelisted: { actions: ["whitelist_remove"], where: () => t("Where they are on the whitelist") },
  operators: { actions: ["deop"], where: () => t("Where they are operators") },
}

/** A selected player, with the servers it was selected on: where they are online, played or are listed. */
export interface Picked {
  name: string
  servers: ServerRef[]
}

export type Selection = ReturnType<typeof useSelection>

export const unique = (refs: ServerRef[]) => [...new Map(refs.map((r) => [key(r), r])).values()]

/** The players selected in a list, by name regardless of case. */
export function useSelection() {
  const [picked, setPicked] = useState(() => new Map<string, Picked>())
  return {
    picked: [...picked.values()],
    has: (name: string) => picked.has(name.toLowerCase()),
    set: (players: Picked[], on: boolean) =>
      setPicked((prev) => {
        const next = new Map(prev)
        for (const p of players) {
          const k = p.name.toLowerCase()
          if (!on) next.delete(k)
          else next.set(k, { name: p.name, servers: unique([...(next.get(k)?.servers ?? []), ...p.servers]) })
        }
        return next
      }),
    clear: () => setPicked(new Map()),
  }
}
