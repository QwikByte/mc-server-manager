import type { Icon } from "@phosphor-icons/react"
import type { ReactNode } from "react"
import { IconTile } from "./icon-tile"
import type { Tone } from "./tone"

export function PageHeader({
  title,
  description,
  actions,
  icon,
  tone,
  badge,
}: {
  title: ReactNode
  description?: ReactNode
  actions?: ReactNode
  icon?: Icon
  tone?: Tone
  badge?: ReactNode
}) {
  return (
    <header className="mb-8 flex flex-wrap items-start justify-between gap-x-6 gap-y-4">
      <div className="flex min-w-0 items-start gap-4">
        {icon && <IconTile icon={icon} tone={tone} size="lg" />}
        <div className="min-w-0 space-y-1.5">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <h1 className="heading text-2xl break-words sm:text-3xl">{title}</h1>
            {badge}
          </div>
          {description && <div className="text-sm text-muted-foreground">{description}</div>}
        </div>
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </header>
  )
}
