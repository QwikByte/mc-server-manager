import type { Icon } from "@phosphor-icons/react"
import { Link } from "@tanstack/react-router"
import type { ReactNode } from "react"
import { cn } from "@/lib/utils"
import { IconTile } from "./icon-tile"
import type { Tone } from "./tone"

/** A key figure, e.g. how many servers run, with an optional detail below it; with to, it opens their page. */
export function StatCard({
  icon,
  tone,
  label,
  value,
  children,
  to,
  search,
}: {
  icon: Icon
  tone?: Tone
  label: string
  value: ReactNode
  children?: ReactNode
  to?: "/servers" | "/nodes" | "/players"
  /** The search of the page it opens, e.g. the players of one network. */
  search?: { network: string }
}) {
  const className = "flex min-w-0 flex-col gap-3 rounded-xl bg-card p-4 shadow-xs ring-1 ring-foreground/8 dark:shadow-none"
  const content = (
    <>
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">{label}</p>
        <IconTile icon={icon} tone={tone} size="sm" />
      </div>
      <div className="min-w-0 text-2xl font-bold tracking-tight break-words">{value}</div>
      {children && <div className="text-xs text-muted-foreground">{children}</div>}
    </>
  )
  return to ? (
    <Link to={to} search={search} className={cn(className, "lift outline-none hover:ring-primary/30 focus-visible:ring-2 focus-visible:ring-ring")}>
      {content}
    </Link>
  ) : (
    <div className={className}>{content}</div>
  )
}
