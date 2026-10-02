import type { ReactNode } from "react"
import { cn } from "@/lib/utils"
import { tabClass } from "./tab-link"

/** Navigation between the parts of a page, which are child routes; its links are TabLinks. */
export function Tabs({ label, children }: { label: string; children: ReactNode }) {
  return (
    <nav aria-label={label} className="mb-8 flex max-w-full gap-1 overflow-x-auto rounded-xl bg-muted/80 p-1 sm:w-fit">
      {children}
    </nav>
  )
}

/** A tab for a part that doesn't exist yet. */
export function UpcomingTab({ children }: { children: ReactNode }) {
  return (
    <span aria-disabled className={cn(tabClass, "cursor-not-allowed opacity-60")}>
      {children}
      <span className="rounded bg-background/70 px-1.5 py-px text-[0.625rem] font-semibold tracking-wide uppercase">Soon</span>
    </span>
  )
}
