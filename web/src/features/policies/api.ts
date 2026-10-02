import { ArrowClockwiseIcon, type Icon, PlayIcon, StopIcon, TerminalWindowIcon } from "@phosphor-icons/react"
import { defaultSchedule, type TaskInput, taskApi } from "@/features/schedules/api"

export type PolicyAction = "restart" | "stop" | "start" | "command"

export interface PolicySettings {
  action: PolicyAction
  /** Minutes before a restart or stop at which players are warned. */
  warnings: number[]
  /** The warning; {minutes} is replaced by the minutes left. */
  message: string
  /** The console command of the command action. */
  command: string
}

export const policies = taskApi<PolicySettings>("/policies")

export const actions: Record<PolicyAction, { label: string; description: string; icon: Icon }> = {
  restart: { label: "Restart", description: "Running servers restart, after a countdown for the players.", icon: ArrowClockwiseIcon },
  stop: { label: "Stop", description: "Running servers stop, e.g. at the end of the opening hours.", icon: StopIcon },
  start: { label: "Start", description: "Stopped servers start, e.g. when the opening hours begin.", icon: PlayIcon },
  command: { label: "Console command", description: "Running game servers run a command, e.g. a broadcast.", icon: TerminalWindowIcon },
}

/** Whether the players are warned before the action. */
export const warns = (action: PolicyAction) => action === "restart" || action === "stop"

export const emptyPolicy: TaskInput<PolicySettings> = {
  name: "",
  enabled: true,
  schedule: defaultSchedule,
  targets: [],
  settings: { action: "restart", warnings: [10, 5, 1], message: "", command: "" },
}

/** Describes what a policy does, e.g. "Restart, warnings 10, 5 and 1 min before". */
export function describePolicy(s: PolicySettings): string {
  if (s.action === "command") return `Runs “${s.command}”`
  const label = actions[s.action].label
  return warns(s.action) && s.warnings.length > 0 ? `${label} with warnings ${s.warnings.join(", ")} min before` : label
}
