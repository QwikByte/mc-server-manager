import { type Icon, InfoIcon, WarningCircleIcon } from "@phosphor-icons/react"
import type { ReactNode } from "react"
import { cn } from "@/lib/utils"
import { type Tone, toneClasses } from "./tone"

/** A tinted note, e.g. a hint about what a change does or why something isn't available. */
export function Callout({
  tone = "info",
  icon: Icon = InfoIcon,
  title,
  role,
  className,
  children,
}: {
  tone?: Tone
  icon?: Icon
  title?: ReactNode
  role?: "alert" | "note" | "status"
  className?: string
  children: ReactNode
}) {
  return (
    <div role={role} className={cn("flex gap-3 rounded-xl p-4 text-sm ring-1 ring-inset", toneClasses[tone], className)}>
      <Icon className="mt-px size-5 shrink-0" weight="duotone" />
      <div className="min-w-0 space-y-1">
        {title && <p className="font-semibold">{title}</p>}
        <div className="text-foreground/80">{children}</div>
      </div>
    </div>
  )
}

/** Shows why something couldn't be loaded. */
export function ErrorCallout({ error, className }: { error: Error; className?: string }) {
  return (
    <Callout tone="destructive" icon={WarningCircleIcon} role="alert" className={className}>
      {error.message}
    </Callout>
  )
}
