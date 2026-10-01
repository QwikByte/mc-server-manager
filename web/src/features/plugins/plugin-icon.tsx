import { PuzzlePieceIcon } from "@phosphor-icons/react"
import { useState } from "react"
import { IconTile } from "@/components/icon-tile"
import { cn } from "@/lib/utils"

/** The icon of a Modrinth project, or a puzzle piece for files without one. */
export function PluginIcon({ src, className }: { src?: string; className?: string }) {
  const [failed, setFailed] = useState(false)
  if (!src || failed) return <IconTile icon={PuzzlePieceIcon} tone="neutral" className={className} />
  return (
    <img
      src={src}
      alt=""
      loading="lazy"
      onError={() => setFailed(true)}
      className={cn("size-10 shrink-0 rounded-xl bg-muted object-cover ring-1 ring-foreground/10", className)}
    />
  )
}
