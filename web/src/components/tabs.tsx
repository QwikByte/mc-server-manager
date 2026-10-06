import { LayoutGroup } from "motion/react"
import { type ReactNode, useId } from "react"

/** Navigation between the parts of a page, which are child routes; its links are TabLinks. */
export function Tabs({ label, children }: { label: string; children: ReactNode }) {
  return (
    <nav aria-label={label} className="mb-8 flex max-w-full gap-1 overflow-x-auto rounded-xl bg-muted/80 p-1 sm:w-fit">
      <LayoutGroup id={useId()}>{children}</LayoutGroup>
    </nav>
  )
}
