import { GraphIcon, TagIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import type { Status } from "@/components/status"
import type { Network } from "@/features/networks/api"
import { msg } from "@/lib/i18n"
import type { Action, State, Target } from "./api"

/** How the states of a set on a server look, in the order the panel lists them. */
export const states: Record<State, Status & { description: string }> = {
  current: { tone: "success", label: msg("Up to date"), description: msg("It has the newest files of the set.") },
  outdated: { tone: "warning", label: msg("Outdated"), description: msg("It has an older version, or its variables or secrets changed.") },
  changed: { tone: "warning", label: msg("Changed on the server"), description: msg("Files changed on the server since they were written.") },
  missing: { tone: "info", label: msg("Missing"), description: msg("It doesn't have the set yet.") },
  left: { tone: "neutral", label: msg("Left"), description: msg("It is no longer a target but still has files of the set.") },
  unreachable: { tone: "destructive", label: msg("Unreachable"), description: msg("Its node can't be reached.") },
}

export const actions: Record<Action, string> = {
  unchanged: msg("Unchanged"),
  created: msg("Created"),
  changed: msg("Replaced"),
  removed: msg("Removed"),
  kept: msg("Kept, as it changed on the server"),
}

/** A target as the panel names it, e.g. "Game servers of Main". */
export function targetLabel(target: Target, networks: Network[] | undefined) {
  if (target.kind === "tag") return { icon: TagIcon, label: target.value }
  const network = networks?.find((n) => n.id === target.value)?.name
  if (!network) return { icon: GraphIcon, label: t("A deleted network") }
  return {
    icon: GraphIcon,
    label: target.role === "proxy" ? t("Proxy of {{network}}", { network }) : t("Game servers of {{network}}", { network }),
  }
}

export const targetKey = (target: Target) => `${target.kind}:${target.value}:${target.role ?? ""}`
