import type { Icon } from "@phosphor-icons/react"
import { type ReactNode, useId } from "react"

/** A part of the side panel of a set, with an action next to its title. */
export function PanelSection({ icon: SectionIcon, title, action, children }: { icon: Icon; title: string; action?: ReactNode; children: ReactNode }) {
  const id = useId()
  return (
    <section aria-labelledby={id} className="surface grid gap-3 rounded-xl p-4">
      <div className="flex min-h-7 items-center gap-2">
        <SectionIcon className="size-4 text-muted-foreground" />
        <h2 id={id} className="flex-1 text-sm font-semibold">
          {title}
        </h2>
        {action}
      </div>
      {children}
    </section>
  )
}
