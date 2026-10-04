import { CheckCircleIcon, CircleIcon, CircleNotchIcon, XCircleIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { formatElapsed } from "@/lib/format"
import { msg } from "@/lib/i18n"
import { useNow } from "@/lib/use-now"
import { cn } from "@/lib/utils"
import { type Operation, useLiveOperation } from "./api"
import { amountOf, shareOf, stepOf } from "./labels"

type StepState = "done" | "current" | "failed" | "pending"

function stateOf(op: Operation, i: number): StepState {
  if (op.error && i === op.step) return "failed"
  if ((op.finishedAt && !op.error) || i < op.step) return "done"
  return i === op.step && !op.finishedAt ? "current" : "pending"
}

const icons: Record<StepState, { icon: typeof CircleIcon; className: string; label: string }> = {
  done: { icon: CheckCircleIcon, className: "text-success", label: msg("Done") },
  current: { icon: CircleNotchIcon, className: "animate-spin text-primary motion-reduce:animate-none", label: msg("In progress") },
  failed: { icon: XCircleIcon, className: "text-destructive", label: msg("Failed") },
  pending: { icon: CircleIcon, className: "text-muted-foreground/50", label: msg("To do") },
}

/** How far the current step is: a bar that fills, or runs while that is unknown. */
export function Bar({ op, className }: { op: Operation; className?: string }) {
  const share = shareOf(op)
  return (
    <div
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={share === undefined ? undefined : Math.round(share * 100)}
      className={cn("h-1.5 overflow-hidden rounded-full bg-muted", className)}
    >
      {share === undefined ? (
        <div className="h-full w-2/5 animate-indeterminate rounded-full bg-primary motion-reduce:w-full motion-reduce:animate-none motion-reduce:opacity-40" />
      ) : (
        <div className="h-full rounded-full bg-primary transition-[width] duration-700" style={{ width: `${share * 100}%` }} />
      )}
    </div>
  )
}

/** The steps of an operation, which are done, the one it is at with its progress, and how long it took. */
export function OperationProgress({ op }: { op: Operation }) {
  const now = useNow(!op.finishedAt)
  const elapsed = (op.finishedAt ? Date.parse(op.finishedAt) : now) - Date.parse(op.startedAt)
  return (
    <div className="grid gap-4">
      <ol className="grid gap-3">
        {op.steps.map((step, i) => {
          const state = stateOf(op, i)
          const { icon: Icon, className, label } = icons[state]
          const amount = state === "current" ? amountOf(op) : undefined
          return (
            <li key={step} className="flex gap-3">
              <Icon
                aria-label={t(label)}
                weight={state === "pending" ? "regular" : "fill"}
                className={cn("mt-0.5 size-5 shrink-0", className)}
              />
              <div className="min-w-0 flex-1 space-y-2">
                <p className={cn("text-sm", state === "pending" ? "text-muted-foreground" : "font-medium")}>{stepOf(op, step)}</p>
                {state === "current" && (
                  <div className="space-y-1.5">
                    <Bar op={op} />
                    {amount && <p className="text-xs text-muted-foreground tabular-nums">{amount}</p>}
                  </div>
                )}
              </div>
            </li>
          )
        })}
      </ol>
      <p className="text-xs text-muted-foreground tabular-nums">
        {op.finishedAt ? t("Took {{time}}", { time: formatElapsed(elapsed) }) : t("Running for {{time}}", { time: formatElapsed(elapsed) })}
      </p>
      {op.error && (
        <p role="alert" className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive ring-1 ring-destructive/20 ring-inset">
          {op.error}
        </p>
      )}
    </div>
  )
}

/** The step an operation is at with its progress, compactly, e.g. for notifications. */
export function OperationLine({ op }: { op: Operation }) {
  const amount = amountOf(op)
  const step = op.steps[op.step]
  return (
    <div className="grid w-full gap-1.5 text-xs text-muted-foreground">
      <p className="truncate">
        {step ? stepOf(op, step) : ""}
        {op.steps.length > 1 && ` · ${Math.min(op.step + 1, op.steps.length)}/${op.steps.length}`}
      </p>
      <Bar op={op} />
      {amount && <p className="text-right tabular-nums">{amount}</p>}
    </div>
  )
}

/** Shows the progress of an operation in a notification. */
export function LiveToast({ id, title }: { id: string; title: string }) {
  const op = useLiveOperation(id)
  return (
    <div className="grid w-full gap-2">
      <p className="font-medium">{title}</p>
      {op && <OperationLine op={op} />}
    </div>
  )
}
