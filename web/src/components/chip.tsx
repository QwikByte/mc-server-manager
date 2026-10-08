import type { Icon } from "@phosphor-icons/react"
import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

/** A small fact with an icon, e.g. the port or memory of a server. */
export function Chip({ icon: Icon, children, className, title }: { icon?: Icon; children: ReactNode; className?: string; title?: string }) {
  return (
    <span
      title={title}
      className={cn("inline-flex h-6 items-center gap-1.5 rounded-md bg-muted/60 px-2 text-xs font-medium whitespace-nowrap ring-1 ring-border ring-inset", className)}
    >
      {Icon && <Icon className="size-3.5 shrink-0 text-muted-foreground" />}
      {children}
    </span>
  )
}
