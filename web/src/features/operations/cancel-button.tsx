import { CircleNotchIcon, StopIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { type Operation, useCancelOperation } from "./api"

/** Cancels an operation that the user may cancel, and tells while it stops. */
export function CancelButton({ op, size, className }: { op: Operation; size?: "sm"; className?: string }) {
  const cancel = useCancelOperation()
  if (op.finishedAt || !(op.cancellable || op.cancelled)) return null
  const stopping = op.cancelled || cancel.isPending || cancel.isSuccess
  return (
    <Button
      variant="outline"
      size={size}
      className={cn("shrink-0", className)}
      disabled={stopping}
      onClick={() => cancel.mutate(op.id, { onError: (e) => toast.error(e.message) })}
    >
      {stopping ? <CircleNotchIcon className="animate-spin motion-reduce:animate-none" /> : <StopIcon />}
      {stopping ? t("Cancelling…") : t("Cancel")}
    </Button>
  )
}
