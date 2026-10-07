import { ArrowRightIcon, type Icon } from "@phosphor-icons/react"
import { Link } from "@tanstack/react-router"
import { motion } from "motion/react"
import type { ReactNode } from "react"
import { IconTile } from "@/components/icon-tile"
import type { Tone } from "@/components/tone"

/** A titled widget of the overview, with a link to all of it and room for actions. */
export function Panel({
  title,
  count,
  more,
  actions,
  children,
}: {
  title: string
  count?: number
  more?: { to: "/nodes" | "/networks" | "/servers" | "/logs" | "/backups" | "/policies"; label: string }
  actions?: ReactNode
  children: ReactNode
}) {
  return (
    <section aria-label={title} className="flex h-full flex-col">
      <div className="mb-3 flex min-h-8 flex-wrap items-center gap-2">
        <h2 className="heading text-lg">{title}</h2>
        {!!count && (
          // Pops whenever the count changes.
          <motion.span
            key={count}
            initial={{ scale: 0.5 }}
            animate={{ scale: 1 }}
            className="rounded-full bg-destructive/10 px-2 text-xs font-semibold text-destructive tabular-nums"
          >
            {count}
          </motion.span>
        )}
        <span data-slot="panel-actions" className="ml-auto flex items-center gap-3">
          {actions}
          {more && (
            <Link to={more.to} className="group inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
              {more.label}
              <ArrowRightIcon className="size-3.5 transition-transform group-hover:translate-x-0.5" />
            </Link>
          )}
        </span>
      </div>
      <div className="surface max-h-[28rem] flex-1 overflow-y-auto rounded-xl">{children}</div>
    </section>
  )
}

/** What a widget says when it has nothing to list. */
export function Calm({ icon, tone = "neutral", children }: { icon: Icon; tone?: Tone; children: ReactNode }) {
  return (
    <div className="flex items-center gap-3 p-5 text-sm text-muted-foreground">
      <IconTile icon={icon} tone={tone} size="sm" />
      <span>{children}</span>
    </div>
  )
}
