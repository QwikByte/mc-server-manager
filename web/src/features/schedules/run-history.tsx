import {
  CaretDownIcon,
  CaretRightIcon,
  CheckCircleIcon,
  ClockCounterClockwiseIcon,
  type Icon,
  MinusCircleIcon,
  UserIcon,
  WarningCircleIcon,
} from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { useState } from "react"
import { ErrorCallout } from "@/components/callout"
import { IconTile } from "@/components/icon-tile"
import { Section } from "@/components/section"
import type { Tone } from "@/components/tone"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { formatDateTime, formatElapsed } from "@/lib/format"
import { msg } from "@/lib/i18n"
import { cn } from "@/lib/utils"
import type { Outcome, Run, Step, TaskApi } from "./api"
import { describeStep } from "./describe"

/** How the outcomes of runs and steps look. */
const outcomes: Record<Outcome, { icon: Icon; tone: Tone; color: string; label: string }> = {
  succeeded: { icon: CheckCircleIcon, tone: "success", color: "text-success", label: msg("Succeeded") },
  failed: { icon: WarningCircleIcon, tone: "destructive", color: "text-destructive", label: msg("Failed") },
  skipped: { icon: MinusCircleIcon, tone: "neutral", color: "text-muted-foreground", label: msg("Skipped") },
}

/** The kept runs of a task, newest first, with what failed, what they left out and their steps. */
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
          <ClockCounterClockwiseIcon className="size-5" />
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
  const [open, setOpen] = useState(false)
  const outcome = outcomes[run.outcome] ?? outcomes.succeeded
  const changed = run.steps.filter((s) => s.change).length
  return (
    <li className="flex gap-3 px-4 py-3">
      <IconTile icon={outcome.icon} tone={outcome.tone} size="sm" />
      <div className="min-w-0 flex-1 space-y-1">
        <p className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-sm">
          <time dateTime={run.startedAt} className="font-semibold">
            {formatDateTime(run.startedAt)}
          </time>
          <span className="text-xs text-muted-foreground">
            {[t(outcome.label), t("took {{time}}", { time: formatElapsed(Date.parse(run.endedAt) - Date.parse(run.startedAt)) })].join(
              " · ",
            )}
          </span>
          {run.startedBy && (
            <span className="flex items-center gap-1 text-xs text-muted-foreground">
              <UserIcon className="size-3.5" />
              {t("started by {{user}}", { user: run.startedBy })}
            </span>
          )}
        </p>
        {run.error && <p className="text-sm break-words whitespace-pre-line text-destructive">{run.error}</p>}
        {run.note && <p className="text-sm break-words whitespace-pre-line text-muted-foreground">{run.note}</p>}
        {run.steps.length > 0 && (
          <>
            <Button
              type="button"
              size="xs"
              variant="ghost"
              className="-ml-2 text-muted-foreground"
              aria-expanded={open}
              onClick={() => setOpen(!open)}
            >
              {open ? <CaretDownIcon /> : <CaretRightIcon />}
              {t("{{count}} steps", { count: run.steps.length, defaultValue_one: "{{count}} step" })}
              {changed > 0 && ` · ${t("{{count}} with changes", { count: changed })}`}
            </Button>
            {open && (
              <ul className="grid gap-2 border-l pl-3">
                {run.steps.map((step, i) => (
                  // Steps have no identity of their own, and don't change.
                  <StepRow key={i} step={step} />
                ))}
              </ul>
            )}
          </>
        )}
      </div>
    </li>
  )
}

function StepRow({ step }: { step: Step }) {
  const outcome = outcomes[step.outcome] ?? outcomes.succeeded
  return (
    <li className="flex gap-2 text-sm">
      <outcome.icon className={cn("mt-0.5 size-4 shrink-0", outcome.color)} aria-label={t(outcome.label)} />
      <div className="min-w-0 space-y-0.5">
        <p>
          {describeStep(step)} <span className="text-xs text-muted-foreground">{step.node}</span>
        </p>
        {step.detail && <p className={cn("break-words", outcome.color)}>{step.detail}</p>}
        {step.change && <p className="break-words text-foreground/80">{step.change}</p>}
      </div>
    </li>
  )
}
