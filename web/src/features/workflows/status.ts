import { CheckCircleIcon, CircleNotchIcon, type Icon, MinusCircleIcon, ProhibitIcon, WarningCircleIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import type { Status } from "@/components/status"
import type { Tone } from "@/components/tone"
import { msg } from "@/lib/i18n"
import type { Outcome, Run, TriggerKind, Workflow } from "./api"
import { triggers } from "./catalog"

/** How the outcomes of runs and steps look. */
export const outcomes: Record<Outcome, { icon: Icon; tone: Tone; color: string; label: string }> = {
  running: { icon: CircleNotchIcon, tone: "warning", color: "text-warning", label: msg("Running") },
  succeeded: { icon: CheckCircleIcon, tone: "success", color: "text-success", label: msg("Succeeded") },
  failed: { icon: WarningCircleIcon, tone: "destructive", color: "text-destructive", label: msg("Failed") },
  cancelled: { icon: ProhibitIcon, tone: "neutral", color: "text-muted-foreground", label: msg("Cancelled") },
  skipped: { icon: MinusCircleIcon, tone: "neutral", color: "text-muted-foreground", label: msg("Skipped") },
}

export function workflowStatus(w: Workflow): Status {
  if (w.running > 0) return { tone: "warning", label: msg("Running"), pulse: true }
  if (!w.enabled) return { tone: "neutral", label: msg("Paused") }
  if (w.lastRun?.outcome === "failed") return { tone: "destructive", label: msg("Failed") }
  return { tone: "success", label: msg("Active") }
}

/** What started a run, e.g. "Schedule" or "By hand". */
export function startedBy(run: Run) {
  if (run.trigger === "manual") return t("By hand")
  if (run.trigger === "workflow") return t("By a workflow")
  const info = triggers[run.trigger as TriggerKind]
  return info ? t(info.label) : run.trigger
}

