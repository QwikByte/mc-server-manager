import { ClockIcon, CubeIcon, HardDrivesIcon, type Icon, PlayIcon, TrashIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import type { ReactElement, ReactNode } from "react"
import { toast } from "sonner"
import { Callout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { IconTile } from "@/components/icon-tile"
import { type Status, StatusBadge } from "@/components/status"
import type { Tone } from "@/components/tone"
import { Button } from "@/components/ui/button"
import { nodesQuery } from "@/features/nodes/api"
import { allServersQuery } from "@/features/servers/api"
import { formatDateTime } from "@/lib/format"
import type { Target } from "@/features/servers/api"
import { msg } from "@/lib/i18n"
import type { Task, TaskApi } from "./api"
import { describeSchedule } from "./describe"

function taskStatus(task: Task<unknown>): Status {
  if (task.running) return { tone: "warning", label: msg("Running"), pulse: true }
  if (!task.enabled) return { tone: "neutral", label: msg("Paused") }
  if (task.lastRun?.error) return { tone: "destructive", label: msg("Failed") }
  return { tone: "success", label: msg("Active") }
}

/**
 * A task with its schedule, servers and latest run, which can be run right away. A
 * confirmation is asked first if confirmRun describes what running does.
 */
export function TaskCard<S>({
  task,
  taskApi,
  icon,
  tone,
  title,
  summary,
  edit,
  confirmRun,
  manage,
}: {
  task: Task<S>
  taskApi: TaskApi<S>
  icon: Icon
  tone: Tone
  title: ReactNode
  summary: ReactNode
  edit: ReactElement
  confirmRun?: string
  /** Whether the user may run, change and delete the task. */
  manage: boolean
}) {
  const run = taskApi.useRunTask()
  const remove = taskApi.useDeleteTask()
  const start = () =>
    run.mutate(task.id, {
      onSuccess: () => toast.success(t("Started {{name}}", { name: task.name })),
      onError: (e) => toast.error(e.message),
    })
  const runButton = (
    <Button size="sm" disabled={task.running || run.isPending} onClick={confirmRun ? undefined : start}>
      <PlayIcon />
      {t("Run now")}
    </Button>
  )

  return (
    <li className="surface flex min-w-0 flex-col gap-4 rounded-xl p-5">
      <div className="flex items-start gap-3">
        <IconTile icon={icon} tone={tone} />
        <div className="min-w-0 flex-1">
          {title}
          <p className="truncate text-xs text-muted-foreground">{summary}</p>
        </div>
        <StatusBadge status={taskStatus(task)} />
      </div>
      <div className="grid gap-2.5">
        <p className="flex items-center gap-2 text-sm">
          <ClockIcon className="size-4 shrink-0 text-muted-foreground" weight="duotone" />
          {describeSchedule(task.schedule)}
          <span className="truncate text-xs text-muted-foreground">{task.schedule.timeZone.replaceAll("_", " ")}</span>
        </p>
        <Targets targets={task.targets} />
      </div>
      {task.lastRun?.error && (
        <Callout tone="destructive" title={t("The last run failed")} className="py-3">
          <span className="whitespace-pre-line">{task.lastRun.error}</span>
        </Callout>
      )}
      <p className="text-xs text-muted-foreground">
        {task.lastRun ? t("Last run {{time}}", { time: formatDateTime(task.lastRun.at) }) : t("Never run")}
        {task.nextRun && ` · ${t("next {{time}}", { time: formatDateTime(task.nextRun) })}`}
      </p>
      <div className="mt-auto flex flex-wrap items-center gap-2 border-t pt-4" hidden={!manage}>
        {confirmRun ? (
          <ConfirmDialog
            trigger={runButton}
            title={t("Run {{name}} now?", { name: task.name })}
            description={confirmRun}
            action={t("Run now")}
            onConfirm={start}
          />
        ) : (
          runButton
        )}
        {edit}
        <ConfirmDialog
          trigger={
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label={t("Delete {{name}}", { name: task.name })}
              title={t("Delete")}
              className="ml-auto text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
            >
              <TrashIcon />
            </Button>
          }
          title={t("Delete {{name}}?", { name: task.name })}
          description={t("It no longer runs. A run in progress finishes.")}
          action={t("Delete")}
          destructive
          onConfirm={() =>
            remove.mutate(task.id, {
              onSuccess: () => toast.success(t("Deleted {{name}}", { name: task.name })),
              onError: (e) => toast.error(e.message),
            })
          }
        />
      </div>
    </li>
  )
}

/** The nodes and servers a task runs on. */
function Targets({ targets }: { targets: Target[] }) {
  const { data: nodes = [] } = useQuery(nodesQuery)
  const { data: servers = [] } = useQuery(allServersQuery)
  if (targets.length === 0) return <p className="text-sm text-muted-foreground">{t("No servers. Edit it to choose some.")}</p>
  return (
    <div className="flex flex-wrap gap-1.5">
      {targets.map((target) => {
        const node = nodes.find((n) => n.id === target.nodeId)?.name ?? t("Node")
        const server = servers.find((s) => s.nodeId === target.nodeId && s.id === target.serverId)
        return (
          <Chip key={`${target.nodeId}/${target.serverId}`} icon={target.serverId ? CubeIcon : HardDrivesIcon} className="font-normal">
            {target.serverId ? (server?.name ?? t("Unreachable server")) : t("{{node}} · all servers", { node })}
          </Chip>
        )
      })}
    </div>
  )
}
