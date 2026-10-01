import type { Icon } from "@phosphor-icons/react"
import { cn } from "@/lib/utils"
import { type Tone, toneClasses } from "./tone"

const sizes = {
  sm: "size-8 rounded-lg [&>svg]:size-4",
  md: "size-10 rounded-xl [&>svg]:size-5",
  lg: "size-12 rounded-2xl [&>svg]:size-6",
}

/** An icon on a tinted tile, which tells at a glance what kind of thing is shown. */
export function IconTile({
  icon: Icon,
  tone = "success",
  size = "md",
  className,
}: {
  icon: Icon
  tone?: Tone
  size?: keyof typeof sizes
  className?: string
}) {
  return (
    <span aria-hidden className={cn("grid shrink-0 place-items-center ring-1 ring-inset", toneClasses[tone], sizes[size], className)}>
      <Icon weight="duotone" />
    </span>
  )
}
