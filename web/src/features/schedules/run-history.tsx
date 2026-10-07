import { CheckCircleIcon, ClockCounterClockwiseIcon, UserIcon, WarningCircleIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { IconTile } from "@/components/icon-tile"
import { Section } from "@/components/section"
import { Skeleton } from "@/components/ui/skeleton"
import { formatDateTime, formatElapsed } from "@/lib/format"
import type { Run, TaskApi } from "./api"

/** The kept runs of a task, newest first, with what failed and what they left out. */
export function RunHistory<S>({ taskApi, id }: { taskApi: TaskApi<S>; id: string }) {
  const { data: runs, isPending, error } = useQuery(taskApi.runsQuery(id))
  return (
    <Section title={t("Runs")} description={t("The latest 50 runs are kept.")}>
      {isPending ? (
        <Skeleton className="h-32 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : runs.length === 0 ? (
        <p className="surface flex items-center gap-3 rounded-xl px-4 py-5 text-sm text-muted-foreground">
          <ClockCounterClockwiseIcon className="size-5" weight="duotone" />
          {t("It hasn't run yet.")}
        </p>
      ) : (
        <ol className="surface divide-y rounded-xl">
          {runs.map((run) => (
            <RunRow key={run.id} run={run} />
          ))}
        </ol>
      )}
    </Section>
  )
}

function RunRow({ run }: { run: Run }) {
  const failed = run.outcome === "failed"
  return (
    <li className="flex gap-3 px-4 py-3">
      <IconTile icon={failed ? WarningCircleIcon : CheckCircleIcon} tone={failed ? "destructive" : "success"} size="sm" />
      <div className="min-w-0 flex-1 space-y-1">
        <p className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-sm">
          <time dateTime={run.startedAt} className="font-semibold">
            {formatDateTime(run.startedAt)}
          </time>
          <span className="text-xs text-muted-foreground">
            {failed ? t("Failed") : t("Succeeded")} ·{" "}
            {t("took {{time}}", { time: formatElapsed(Date.parse(run.endedAt) - Date.parse(run.startedAt)) })}
          </span>
          {run.startedBy && (
            <span className="flex items-center gap-1 text-xs text-muted-foreground">
              <UserIcon className="size-3.5" weight="duotone" />
              {t("started by {{user}}", { user: run.startedBy })}
            </span>
          )}
        </p>
        {run.error && <p className="text-sm break-words whitespace-pre-line text-destructive">{run.error}</p>}
        {run.note && <p className="text-sm break-words whitespace-pre-line text-muted-foreground">{run.note}</p>}
      </div>
    </li>
  )
}
