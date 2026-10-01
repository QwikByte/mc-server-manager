import type { Icon } from "@phosphor-icons/react"
import type { ReactNode } from "react"
import { IconTile } from "./icon-tile"
import type { Tone } from "./tone"

/** A key figure, e.g. how many servers run, with an optional detail below it. */
export function StatCard({
  icon,
  tone,
  label,
  value,
  children,
}: {
  icon: Icon
  tone?: Tone
  label: string
  value: ReactNode
  children?: ReactNode
}) {
  return (
    <div className="flex min-w-0 flex-col gap-3 rounded-xl bg-card p-4 shadow-xs ring-1 ring-foreground/8 dark:shadow-none">
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">{label}</p>
        <IconTile icon={icon} tone={tone} size="sm" />
      </div>
      <div className="min-w-0 text-2xl font-bold tracking-tight break-words">{value}</div>
      {children && <div className="text-xs text-muted-foreground">{children}</div>}
    </div>
  )
}
