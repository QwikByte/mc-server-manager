import type { Icon } from "@phosphor-icons/react"
import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

/** A small fact with an icon, e.g. the port or memory of a server. */
export function Chip({ icon: Icon, children, className, title }: { icon?: Icon; children: ReactNode; className?: string; title?: string }) {
  return (
    <span title={title} className={cn("inline-flex items-center gap-1.5 rounded-md bg-muted px-2 py-1 text-xs font-medium whitespace-nowrap", className)}>
      {Icon && <Icon className="size-3.5 text-muted-foreground" weight="duotone" />}
      {children}
    </span>
  )
}
