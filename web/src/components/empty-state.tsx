import type { Icon } from "@phosphor-icons/react"
import type { ReactNode } from "react"
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { IconTile } from "./icon-tile"
import type { Tone } from "./tone"

/** Explains an empty list and offers the action that fills it. */
export function EmptyState({
  icon,
  tone,
  title,
  description,
  children,
}: {
  icon: Icon
  tone?: Tone
  title: string
  description: ReactNode
  children?: ReactNode
}) {
  return (
    <Empty className="rounded-2xl border border-dashed bg-card/40 py-14">
      <EmptyHeader>
        <EmptyMedia>
          <IconTile icon={icon} tone={tone} size="lg" />
        </EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        <EmptyDescription>{description}</EmptyDescription>
      </EmptyHeader>
      {children && <EmptyContent>{children}</EmptyContent>}
    </Empty>
  )
}
