import { ClockIcon, CubeIcon, HardDrivesIcon, type Icon, PlayIcon, TrashIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
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
import type { Task, TaskApi, Target } from "./api"
import { describeSchedule } from "./describe"

function taskStatus(task: Task<unknown>): Status {
  if (task.running) return { tone: "warning", label: "Running", pulse: true }
  if (!task.enabled) return { tone: "neutral", label: "Paused" }
  if (task.lastRun?.error) return { tone: "destructive", label: "Failed" }
  return { tone: "success", label: "Active" }
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
}: {
  task: Task<S>
  taskApi: TaskApi<S>
  icon: Icon
  tone: Tone
  title: ReactNode
  summary: ReactNode
  edit: ReactElement
  confirmRun?: string
}) {
  const run = taskApi.useRunTask()
  const remove = taskApi.useDeleteTask()
  const start = () =>
    run.mutate(task.id, { onSuccess: () => toast.success(`Started ${task.name}`), onError: (e) => toast.error(e.message) })
  const runButton = (
    <Button size="sm" disabled={task.running || run.isPending} onClick={confirmRun ? undefined : start}>
      <PlayIcon />
      Run now
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
        <Callout tone="destructive" title="The last run failed" className="py-3">
          <span className="whitespace-pre-line">{task.lastRun.error}</span>
        </Callout>
      )}
      <p className="text-xs text-muted-foreground">
        {task.lastRun ? `Last run ${formatDateTime(task.lastRun.at)}` : "Never run"}
        {task.nextRun && ` · next ${formatDateTime(task.nextRun)}`}
      </p>
      <div className="mt-auto flex flex-wrap items-center gap-2 border-t pt-4">
        {confirmRun ? (
          <ConfirmDialog trigger={runButton} title={`Run ${task.name} now?`} description={confirmRun} action="Run now" onConfirm={start} />
        ) : (
          runButton
        )}
        {edit}
        <ConfirmDialog
          trigger={
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label={`Delete ${task.name}`}
              title="Delete"
              className="ml-auto text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
            >
              <TrashIcon />
            </Button>
          }
          title={`Delete ${task.name}?`}
          description="It no longer runs. A run in progress finishes."
          action="Delete"
          destructive
          onConfirm={() =>
            remove.mutate(task.id, { onSuccess: () => toast.success(`Deleted ${task.name}`), onError: (e) => toast.error(e.message) })
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
  if (targets.length === 0) return <p className="text-sm text-muted-foreground">No servers. Edit it to choose some.</p>
  return (
    <div className="flex flex-wrap gap-1.5">
      {targets.map((t) => {
        const node = nodes.find((n) => n.id === t.nodeId)?.name ?? "Node"
        const server = servers.find((s) => s.nodeId === t.nodeId && s.id === t.serverId)
        return (
          <Chip key={`${t.nodeId}/${t.serverId}`} icon={t.serverId ? CubeIcon : HardDrivesIcon} className="font-normal">
            {t.serverId ? (server?.name ?? "Unreachable server") : `${node} · all servers`}
          </Chip>
        )
      })}
    </div>
  )
}
