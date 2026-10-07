import { ArrowClockwiseIcon, type Icon, InfoIcon, WarningCircleIcon } from "@phosphor-icons/react"
import { useQueryClient } from "@tanstack/react-query"
import { t } from "i18next"
import { type ReactNode, useState } from "react"
import { Button } from "@/components/ui/button"
import { ApiError } from "@/lib/api"
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

/**
 * Shows why something couldn't be loaded. **Try again** loads what failed on the page again,
 * unless the master's answer is final, e.g. that something doesn't exist. retry={false} leaves
 * it out, e.g. for a change that failed.
 */
export function ErrorCallout({ error, className, retry = true }: { error: Error; className?: string; retry?: boolean }) {
  const queryClient = useQueryClient()
  const [retrying, setRetrying] = useState(false)
  const final = error instanceof ApiError && error.status < 500
  const again = () => {
    setRetrying(true)
    void queryClient
      .refetchQueries({ type: "active", predicate: (query) => query.state.status === "error" })
      .finally(() => setRetrying(false))
  }
  return (
    <Callout tone="destructive" icon={WarningCircleIcon} role="alert" className={className}>
      {error.message}
      {retry && !final && (
        <Button size="sm" variant="outline" className="mt-3 flex" disabled={retrying} onClick={again}>
          <ArrowClockwiseIcon className={cn(retrying && "animate-spin")} />
          {t("Try again")}
        </Button>
      )}
    </Callout>
  )
}
