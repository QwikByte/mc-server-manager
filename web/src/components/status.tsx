import { t } from "i18next"
import type { ReactNode } from "react"
import { cn } from "@/lib/utils"
import { type Tone, toneClasses, toneDots } from "./tone"

export interface Status {
  tone: Tone
  /** In English, marked with msg; StatusBadge translates it. */
  label: string
  /** Pulses while something is in transition, e.g. a starting server. */
  pulse?: boolean
}

/**
 * A coloured lamp, square like a block, as are the servers it mostly stands for; with a label it is
 * announced, otherwise it is decorative.
 */
export function StatusDot({ status, label, className }: { status: Status; label?: string; className?: string }) {
  return (
    <span
      role={label ? "img" : undefined}
      aria-label={label}
      aria-hidden={!label}
      title={label}
      className={cn("relative flex size-2 shrink-0 rounded-[2px]", className)}
    >
      {status.pulse && (
        <span className={cn("absolute inset-0 animate-ping rounded-[2px] opacity-60 motion-reduce:hidden", toneDots[status.tone])} />
      )}
      <span className={cn("relative size-2 rounded-[2px]", toneDots[status.tone])} />
    </span>
  )
}

/** A small tinted label, e.g. "Unsaved". */
export function Pill({ tone, children, className }: { tone: Tone; children: ReactNode; className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex h-6 items-center gap-1.5 rounded-md px-2 text-xs font-medium whitespace-nowrap ring-1 ring-inset",
        toneClasses[tone],
        className,
      )}
    >
      {children}
    </span>
  )
}

/** A pill with a dot and the label of a status. */
export function StatusBadge({ status, className }: { status: Status; className?: string }) {
  return (
    <Pill tone={status.tone} className={className}>
      <StatusDot status={status} />
      {t(status.label)}
    </Pill>
  )
}
