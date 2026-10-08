import { ArrowRightIcon, type Icon } from "@phosphor-icons/react"
import { Link } from "@tanstack/react-router"
import { motion } from "motion/react"
import { type ReactNode, useId } from "react"
import { IconTile } from "@/components/icon-tile"
import type { Tone } from "@/components/tone"

/** A widget of the overview: a card titled in its header, with a link to all of it and room for actions. */
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
  const id = useId()
  return (
    <section aria-labelledby={id} className="surface flex h-full flex-col overflow-hidden rounded-xl">
      <div className="flex min-h-12 shrink-0 flex-wrap items-center gap-2 border-b px-4 py-2">
        <h2 id={id} className="heading text-[0.9375rem]">
          {title}
        </h2>
        {!!count && (
          // Pops whenever the count changes.
          <motion.span
            key={count}
            initial={{ scale: 0.5 }}
            animate={{ scale: 1 }}
            className="rounded-md bg-destructive/10 px-1.5 text-xs font-semibold text-destructive tabular-nums"
          >
            {count}
          </motion.span>
        )}
        <span data-slot="panel-actions" className="ml-auto flex items-center gap-3">
          {actions}
          {more && (
            <Link
              to={more.to}
              className="group inline-flex items-center gap-1 rounded-sm text-[0.8125rem] text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
            >
              {more.label}
              <ArrowRightIcon className="size-3.5 transition-transform group-hover:translate-x-0.5" />
            </Link>
          )}
        </span>
      </div>
      <div className="max-h-[28rem] flex-1 overflow-y-auto">{children}</div>
    </section>
  )
}

/** What a widget says when it has nothing to list. */
export function Calm({ icon, tone = "neutral", children }: { icon: Icon; tone?: Tone; children: ReactNode }) {
  return (
    <div className="flex items-center gap-3 px-4 py-5 text-sm text-muted-foreground">
      <IconTile icon={icon} tone={tone} size="sm" />
      <span>{children}</span>
    </div>
  )
}
