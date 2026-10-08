import type { Icon } from "@phosphor-icons/react"
import type { ReactNode } from "react"
import { IconTile } from "./icon-tile"
import { takeFocus } from "./page-focus"
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
    <header className="mb-7 flex flex-wrap items-start justify-between gap-x-6 gap-y-4">
      <div className="flex min-w-0 flex-[1_1_18rem] items-start gap-3.5">
        {icon && <IconTile icon={icon} tone={tone} className="mt-0.5" />}
        <div className="min-w-0 flex-1 space-y-1.5">
          <div className="flex min-h-10 flex-wrap items-center gap-x-3 gap-y-1">
            {/* Takes the focus when the page opens; see PageFocus. */}
            <h1 ref={takeFocus} tabIndex={-1} className="heading min-w-0 text-[1.375rem] break-words outline-none sm:text-[1.625rem]">
              {title}
            </h1>
            {badge}
          </div>
          {description && <div className="text-sm text-muted-foreground">{description}</div>}
        </div>
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </header>
  )
}
