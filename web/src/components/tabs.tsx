import type { ReactNode } from "react"

/** Navigation between the parts of a page, which are child routes; its links are TabLinks. */
export function Tabs({ label, children }: { label: string; children: ReactNode }) {
  return (
    <nav aria-label={label} className="mb-8 flex max-w-full gap-1 overflow-x-auto rounded-xl bg-muted/80 p-1 sm:w-fit">
      {children}
    </nav>
  )
}
