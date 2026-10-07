import type { Icon } from "@phosphor-icons/react"
import type { ReactNode } from "react"
import { IconTile } from "@/components/icon-tile"
import { StatusBadge } from "@/components/status"
import type { Tone } from "@/components/tone"

/** A part of the account, e.g. a way of signing in, with what it is and the actions that change it. */
export function AccountRow({
  icon,
  tone,
  title,
  status,
  actions,
  children,
}: {
  icon: Icon
  tone: Tone
  title: string
  status?: { tone: Tone; label: string }
  actions: ReactNode
  children: ReactNode
}) {
  return (
    <div className="flex flex-wrap items-center gap-4 rounded-xl border p-4">
      <IconTile icon={icon} tone={tone} />
      <div className="min-w-48 flex-1 space-y-0.5">
        <p className="flex flex-wrap items-center gap-2 font-medium break-words">
          {title}
          {status && <StatusBadge status={status} />}
        </p>
        <div className="text-sm text-muted-foreground">{children}</div>
      </div>
      <div className="flex flex-wrap gap-2">{actions}</div>
    </div>
  )
}
