import { PushPinIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { motion } from "motion/react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { usePinned } from "./api"

/** Pins a server to the sidebar and the overview, or unpins it. */
export function PinButton({
  nodeId,
  server,
  size = "icon-sm",
  className,
}: {
  nodeId: string
  server: { id: string; name: string }
  size?: "icon-xs" | "icon-sm"
  className?: string
}) {
  const { isPinned, toggle } = usePinned()
  const pinned = isPinned(server.id)
  const label = pinned ? t("Unpin {{name}}", { name: server.name }) : t("Pin {{name}}", { name: server.name })
  return (
    <Button
      variant="ghost"
      size={size}
      aria-label={label}
      aria-pressed={pinned}
      title={label}
      onClick={() => toggle({ nodeId, serverId: server.id })}
      className={cn("text-muted-foreground aria-pressed:text-primary", className)}
    >
      {/* Pops when it is pinned, not when it first shows. */}
      <motion.span initial={false} animate={{ scale: pinned ? [1, 1.4, 1] : 1 }} className="grid place-items-center">
        <PushPinIcon weight={pinned ? "fill" : "regular"} />
      </motion.span>
    </Button>
  )
}
